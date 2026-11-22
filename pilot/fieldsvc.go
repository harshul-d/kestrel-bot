package pilot

import (
	"math"
	"sync/atomic"
	"time"
)

type FieldsvcJob = func()

type FieldsvcPend struct {
	key    [4]float64
	t      float64
	rx     chan Field
	cancel *atomic.Bool
}

// FieldsvcFieldSvc runs field builds on one long-lived background worker. The zero value is ready.
type FieldsvcFieldSvc struct {
	tx   chan<- FieldsvcJob
	dead bool
	zone *FieldsvcPend
	nav  *FieldsvcPend
	sq   *FieldsvcPend
	late uint64
}

// sender starts the worker on first use. Sends on the returned channel never block for long:
// a queue goroutine buffers jobs without bound and hands them to the worker in order.
func (s *FieldsvcFieldSvc) sender() chan<- FieldsvcJob {
	if s.tx == nil && !s.dead {
		in := make(chan FieldsvcJob)
		out := make(chan FieldsvcJob)
		go fieldsvcQueue(in, out)
		go fieldsvcWorker(out)
		s.tx = in
	}
	return s.tx
}

func fieldsvcQueue(in <-chan FieldsvcJob, out chan<- FieldsvcJob) {
	var q []FieldsvcJob
	for {
		if len(q) == 0 {
			j, ok := <-in
			if !ok {
				close(out)
				return
			}
			q = append(q, j)
			continue
		}
		select {
		case j, ok := <-in:
			if !ok {
				for _, r := range q {
					out <- r
				}
				close(out)
				return
			}
			q = append(q, j)
		case out <- q[0]:
			q[0] = nil
			q = q[1:]
		}
	}
}

func fieldsvcWorker(rx <-chan FieldsvcJob) {
	for j := range rx {
		fieldsvcRunJob(j)
	}
}

func fieldsvcRunJob(j FieldsvcJob) {
	defer func() { _ = recover() }()
	j()
}

func fieldsvcPoll(p **FieldsvcPend) *struct {
	f0 [4]float64
	f1 float64
	f2 Field
} {
	q := *p
	if q == nil {
		return nil
	}
	select {
	case f, ok := <-q.rx:
		if !ok {
			*p = nil
			return nil
		}
		*p = nil
		return &struct {
			f0 [4]float64
			f1 float64
			f2 Field
		}{q.key, q.t, f}
	default:
		return nil
	}
}

func fieldsvcNear(a *[4]float64, b *[4]float64, tol float64) bool {
	return math.Hypot(math.Hypot(a[0]-b[0], a[1]-b[1]), a[2]-b[2]) <= tol && math.Abs(a[3]-b[3]) < 1e-9
}

// fieldsvcRequest returns the field when it is ready while the budget keeps keepMs; otherwise it stays pending.
func fieldsvcRequest(svc *FieldsvcFieldSvc, which uint8, shared *Shared, key [4]float64, tol float64, t float64, budget *Budget, keepMs float64, build func(nav *Nav) Field) *Field {
	if budget.isUnlimited() {
		f := build(&shared.nav)
		return &f
	}
	tx := svc.sender()
	var p **FieldsvcPend
	switch which {
	case 0:
		p = &svc.zone
	case 1:
		p = &svc.nav
	default:
		p = &svc.sq
	}
	same := *p != nil && fieldsvcNear(&(*p).key, &key, tol)
	if !same {
		if q := *p; q != nil {
			*p = nil
			q.cancel.Store(true)
		}
		rrx := make(chan Field, 1)
		cancel := &atomic.Bool{}
		sh := shared
		job := func() {
			defer close(rrx)
			if cancel.Load() {
				return
			}
			f := build(&sh.nav)
			rrx <- f
		}
		sent := false
		if tx != nil {
			tx <- job
			sent = true
		}
		if !sent {
			job()
		}
		*p = &FieldsvcPend{key: key, t: t, rx: rrx, cancel: cancel}
	}
	wait := budget.remainingMs() - keepMs
	q := *p
	if q == nil {
		return nil
	}
	var f Field
	var ok, timeout bool
	if wait > 0.0 {
		tm := time.NewTimer(time.Duration(wait / 1000.0 * float64(time.Second)))
		select {
		case f, ok = <-q.rx:
		case <-tm.C:
			timeout = true
		}
		tm.Stop()
	} else {
		select {
		case f, ok = <-q.rx:
		default:
			timeout = true
		}
	}
	if timeout {
		svc.late += 1
		return nil
	}
	if ok {
		*p = nil
		return &f
	}
	*p = nil
	return nil
}
