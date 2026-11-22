package pilot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// BotName is the name sent in the ready message.
const BotName = "aurora-pilot"

var logOn = os.Getenv("MECH_LOG") == "1"

// logf writes one line to stderr when MECH_LOG=1.
func logf(format string, a ...any) {
	if logOn {
		msg := strings.TrimRight(fmt.Sprintf(format, a...), "\n")
		fmt.Fprintf(os.Stderr, "[pilot] %s\n", msg)
	}
}

// Answer deadline used by the IO loop, measured from reading the observation.
const runtimeIOFraction = 0.70

// Deadline handed to the brain.
const runtimeBrainFraction = 0.60

type runtimeLine struct {
	t    time.Time
	line string
}

type runtimeJob struct {
	obs    *Obs
	budget Budget
}

type runtimeDone struct {
	tick     int64
	act      Action
	panicked bool
}

// runtimeSpawnReader reads stdin lines; lines that arrive together (already buffered) form one batch.
func runtimeSpawnReader(r io.Reader) <-chan []runtimeLine {
	ch := make(chan []runtimeLine, 1024)
	go func() {
		defer close(ch)
		rd := bufio.NewReaderSize(r, 1<<20)
		var batch []runtimeLine
		eof := false
		for !eof {
			for {
				b, err := rd.ReadBytes('\n')
				if len(b) > 0 {
					s := strings.TrimSpace(strings.ToValidUTF8(string(b), "�"))
					if s != "" {
						batch = append(batch, runtimeLine{time.Now(), s})
					}
				}
				if err != nil {
					eof = true
					break
				}
				if rd.Buffered() == 0 {
					break
				}
			}
			if len(batch) > 0 {
				ch <- batch
				batch = nil
			}
		}
	}()
	return ch
}

func runtimeNewBrain(hello *Hello, shared *Shared) (b *Brain) {
	defer func() {
		if recover() != nil {
			logf("brain init panicked")
			b = nil
		}
	}()
	return brainNew(hello, shared)
}

func runtimeSafeAct(b *Brain, obs *Obs, budget *Budget) (a Action, ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	return b.act(obs, budget), true
}

func runtimeSpawnWorker(hello *Hello, shared *Shared, brain *Brain) (chan<- runtimeJob, <-chan runtimeDone) {
	jobs := make(chan runtimeJob, 1024)
	done := make(chan runtimeDone, 1024)
	go func() {
		b := brain
		for job := range jobs {
		drain:
			for {
				select {
				case j, ok := <-jobs:
					if !ok {
						break drain
					}
					job = j
				default:
					break drain
				}
			}
			tick := job.obs.Tick
			if b == nil {
				b = runtimeNewBrain(hello, shared)
			}
			if b == nil {
				done <- runtimeDone{tick: tick, panicked: true}
				continue
			}
			a, ok := runtimeSafeAct(b, job.obs, &job.budget)
			if ok {
				a.Tick = tick
				done <- runtimeDone{tick: tick, act: a}
			} else {
				logf("act panicked at tick %d", tick)
				done <- runtimeDone{tick: tick, panicked: true}
				b = runtimeNewBrain(hello, shared)
			}
		}
	}()
	return jobs, done
}

type runtimeOut struct {
	w         *bufio.Writer
	maxLine   int
	lastMove  *V2
	lastBoost *bool
	lastAim   *V2
	lastFire  [2]bool
}

func (o *runtimeOut) write(b []byte) {
	o.w.Write(b)
	o.w.WriteByte('\n')
	o.w.Flush()
}

func (o *runtimeOut) action(a Action) {
	o.lastMove = a.Move
	o.lastBoost = a.Boost
	if a.Aim != nil {
		o.lastAim = a.Aim
	}
	if a.Fire != nil {
		o.lastFire = *a.Fire
	} else {
		o.lastFire = [2]bool{}
	}
	s, err := a.MarshalJSON()
	if err != nil {
		s = nil
	}
	if len(s) > o.maxLine && a.Debug != nil {
		a.Debug = nil
		s, err = a.MarshalJSON()
		if err != nil {
			s = nil
		}
	}
	if len(s) == 0 {
		return
	}
	o.write(s)
}

func (o *runtimeOut) fallback(tick int64) Action {
	return Action{Tick: tick, Move: o.lastMove, Boost: o.lastBoost, Aim: o.lastAim}
}

// fallbackHold also keeps holding a laser trigger that was charging, so the charge is not lost.
func (o *runtimeOut) fallbackHold(tick int64, hold [2]bool) Action {
	a := o.fallback(tick)
	f := [2]bool{o.lastFire[0] && hold[0], o.lastFire[1] && hold[1]}
	if f[0] || f[1] {
		a.Fire = &f
	}
	return a
}

// runtimeRawObsTick peeks the tick of a line that did not parse as an observation.
func runtimeRawObsTick(line string) (int64, bool) {
	var v map[string]any
	if json.Unmarshal([]byte(line), &v) != nil {
		return 0, false
	}
	if ty, _ := v["type"].(string); ty != "obs" {
		return 0, false
	}
	t, ok := v["tick"].(float64)
	if !ok {
		return 0, false
	}
	return f2i64(t), true
}

func runtimeParseHello(line string) *Hello {
	var v map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &v) != nil {
		return nil
	}
	var ty string
	if json.Unmarshal(v["type"], &ty) != nil || ty != "hello" {
		return nil
	}
	h := helloDefault()
	if err := protoDecodeStrict([]byte(line), &h); err == nil {
		return &h
	} else {
		logf("hello parse error: %v", err)
	}
	h = helloDefault()
	if m, ok := v["map"]; ok {
		var mm map[string]any
		if json.Unmarshal(m, &mm) == nil {
			if d, ok := mm["dir"].(string); ok {
				h.Map.Dir = d
			}
		}
	}
	if s, ok := v["slot"]; ok {
		var f float64
		if json.Unmarshal(s, &f) == nil {
			h.Slot = f2i64(f)
		}
	}
	if c, ok := v["config"]; ok {
		cfg := configDefault()
		if protoDecodeStrict(c, &cfg) == nil {
			h.Config = cfg
		}
	}
	return &h
}

// Run is the process main loop. It returns the exit code.
func Run(stdin io.Reader, stdout io.Writer) int {
	start := time.Now()
	rx := runtimeSpawnReader(stdin)

	var pending []runtimeLine
	var hello *Hello
	for hello == nil {
		batch, ok := <-rx
		if !ok {
			return 0
		}
		for i, l := range batch {
			if h := runtimeParseHello(l.line); h != nil {
				hello = h
				pending = append(pending, batch[i+1:]...)
				break
			}
			logf("ignoring line before hello")
		}
	}
	shared := sharedLoad(hello.Map.Dir, &hello.Config)
	loaded := time.Since(start)
	brain := runtimeNewBrain(hello, shared)
	tickMs := 50.0
	if hello.Config.Bot.TickBudgetMs > 0 {
		tickMs = hello.Config.Bot.TickBudgetMs
	}
	ioWait := time.Duration(tickMs * runtimeIOFraction * float64(time.Millisecond))
	brainWait := time.Duration(tickMs * runtimeBrainFraction * float64(time.Millisecond))
	maxLine := 65536
	if hello.Config.Bot.MaxLineBytes > 0 {
		maxLine = f2u(hello.Config.Bot.MaxLineBytes)
	}
	jobs, done := runtimeSpawnWorker(hello, shared, brain)

	out := &runtimeOut{w: bufio.NewWriterSize(stdout, 1<<16), maxLine: maxLine}
	if r, err := readyNew(BotName).MarshalJSON(); err == nil {
		out.write(r)
	}
	logf("ready after %v (shared loaded after %v)", time.Since(start), loaded)

	var answered, fallbacks, panics uint64
	timer := time.NewTimer(time.Hour)
	timer.Stop()
	for {
		lines := pending
		pending = nil
		if len(lines) == 0 {
			first, ok := <-rx
			if !ok {
				break
			}
			lines = first
		}
	more:
		for {
			select {
			case x, ok := <-rx:
				if !ok {
					break more
				}
				lines = append(lines, x...)
			default:
				break more
			}
		}
		// Newest observation wins; "end" anywhere ends the process.
		var newestT time.Time
		var newestObs *Obs
		newestTick, haveNewest, newestBad := int64(0), false, false
		ended := false
		for _, l := range lines {
			kind, _, obs, end, err := protoParseServerMsg([]byte(l.line))
			if err == nil {
				switch kind {
				case "obs":
					newestT, newestObs, haveNewest, newestBad = l.t, obs, true, false
				case "end":
					logf("end: place %d kills %d", end.Place, end.Kills)
					ended = true
				}
				continue
			}
			if tick, ok := runtimeRawObsTick(l.line); ok {
				logf("obs parse error at tick %d: %v", tick, err)
				newestT, newestObs, newestTick, haveNewest, newestBad = l.t, nil, tick, true, true
			} else if strings.Contains(l.line, "\"end\"") {
				ended = true
			}
		}
		if ended {
			break
		}
		if !haveNewest {
			continue
		}
		if newestBad {
			out.action(out.fallback(newestTick))
			fallbacks++
			continue
		}
		obs := newestObs
		tick := obs.Tick
		var hold [2]bool
		for i, w := range obs.Me.Weapons {
			if i >= 2 {
				break
			}
			hold[i] = w != nil && w.Kind == WeaponKindLaser && w.Charge > 0
		}
		deadline := newestT.Add(ioWait)
		jobs <- runtimeJob{obs: obs, budget: budgetUntil(newestT.Add(brainWait))}
		var action *Action
	wait:
		for {
			now := time.Now()
			if !now.Before(deadline) {
				break
			}
			timer.Reset(deadline.Sub(now))
			select {
			case d := <-done:
				if !timer.Stop() {
					<-timer.C
				}
				if d.tick != tick {
					continue
				}
				if d.panicked {
					panics++
				} else {
					a := d.act
					action = &a
				}
				break wait
			case <-timer.C:
				break wait
			}
		}
		if action != nil {
			answered++
			out.action(*action)
		} else {
			fallbacks++
			logf("fallback at tick %d", tick)
			out.action(out.fallbackHold(tick, hold))
		}
	}
	logf("exit: answered %d, fallbacks %d, panics %d", answered, fallbacks, panics)
	out.w.Flush()
	return 0
}
