package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"time"
)

const botName = "osu-mech"

func logf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
}

type typeOnly struct {
	Type string `json:"type"`
}

func main() {
	debug.SetGCPercent(400)
	debug.SetMemoryLimit(300 << 20)

	lines := make(chan []byte, 4096)
	go func() {
		r := bufio.NewReaderSize(os.Stdin, 1<<20)
		for {
			b, err := r.ReadBytes('\n')
			if len(b) > 0 {
				lines <- b
			}
			if err != nil {
				close(lines)
				return
			}
		}
	}()
	out := bufio.NewWriterSize(os.Stdout, 1<<16)
	send := func(v any) {
		b, err := json.Marshal(v)
		if err != nil {
			logf("marshal: %v", err)
			return
		}
		out.Write(b)
		out.WriteByte('\n')
		out.Flush()
	}

	var brain *Brain
	var last *Action
	for {
		line, ok := <-lines
		if !ok {
			return
		}
		// newest-observation rule: drain queue, keep the latest obs, but handle hello/end immediately
		var t typeOnly
		if json.Unmarshal(line, &t) != nil {
			continue
		}
		if t.Type == "obs" {
		drain:
			for {
				select {
				case nl, ok2 := <-lines:
					if !ok2 {
						break drain
					}
					var t2 typeOnly
					if json.Unmarshal(nl, &t2) != nil {
						continue
					}
					if t2.Type == "obs" {
						line = nl
						continue
					}
					if t2.Type == "end" {
						return
					}
				default:
					break drain
				}
			}
		}
		switch t.Type {
		case "hello":
			var h Hello
			if err := json.Unmarshal(line, &h); err != nil {
				logf("hello parse: %v", err)
			}
			start := time.Now()
			brain = NewBrain(&h)
			logf("init %v", time.Since(start))
			send(map[string]string{"type": "ready", "name": botName})
		case "obs":
			var o Obs
			if err := json.Unmarshal(line, &o); err != nil {
				logf("obs parse: %v", err)
				continue
			}
			act := safeAct(brain, &o, last)
			last = act
			send(act)
		case "end":
			return
		}
	}
}

func safeAct(b *Brain, o *Obs, last *Action) (a *Action) {
	defer func() {
		if r := recover(); r != nil {
			logf("panic: %v\n%s", r, debug.Stack())
			if last != nil {
				c := *last
				c.Tick = o.Tick
				c.Fire = [2]bool{}
				c.Pick, c.Take, c.Drop, c.Debug = nil, nil, nil, nil
				c.Jump = false
				a = &c
			} else {
				a = &Action{Tick: o.Tick}
			}
		}
	}()
	if b == nil {
		return &Action{Tick: o.Tick}
	}
	a = b.Act(o)
	a.Tick = o.Tick
	return a
}
