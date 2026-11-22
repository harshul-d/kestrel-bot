package pilot

import (
	"math"
	"time"
)

// Budget is the time the Brain may still spend on the current observation.
type Budget struct {
	dl *time.Time
}

func budgetUnlimited() Budget {
	return Budget{dl: nil}
}

func budgetUntil(deadline time.Time) Budget {
	return Budget{dl: &deadline}
}

func budgetFromNowMs(ms float64) Budget {
	s := fmax(ms, 0.0) / 1000.0
	if !isFinite(s) || s*1e9 >= 9.223372036854775807e18 {
		panic("budgetFromNowMs: duration out of range")
	}
	return budgetUntil(time.Now().Add(time.Duration(s * 1e9)))
}

func (x Budget) isUnlimited() bool {
	return x.dl == nil
}

func (x Budget) deadline() *time.Time {
	return x.dl
}

func (x Budget) remaining() time.Duration {
	if x.dl == nil {
		return time.Duration(math.MaxInt64)
	}
	r := time.Until(*x.dl)
	if r < 0 {
		return 0
	}
	return r
}

// remainingMs is +Inf when unlimited.
func (x Budget) remainingMs() float64 {
	if x.dl == nil {
		return math.Inf(1)
	}
	return x.remaining().Seconds() * 1000.0
}

func (x Budget) expired() bool {
	return x.dl != nil && !time.Now().Before(*x.dl)
}
