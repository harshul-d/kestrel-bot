package pilot

import (
	"bufio"
	"fmt"
	"os"
	"testing"
)

// TestReplay is an offline tool, not a unit test: with REPLAY_IN set it feeds a recorded
// hello + observation stream (JSONL) to the brain with an unlimited budget and writes one
// action per observation to REPLAY_OUT. REPLAY_PARAMS optionally names a params JSON file.
func TestReplay(t *testing.T) {
	in := os.Getenv("REPLAY_IN")
	if in == "" {
		t.Skip("REPLAY_IN not set")
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	o, err := os.Create(os.Getenv("REPLAY_OUT"))
	if err != nil {
		t.Fatal(err)
	}
	defer o.Close()
	w := bufio.NewWriter(o)
	defer w.Flush()
	var sw *bufio.Writer
	if sp := os.Getenv("REPLAY_STATE"); sp != "" {
		sf, err := os.Create(sp)
		if err != nil {
			t.Fatal(err)
		}
		defer sf.Close()
		sw = bufio.NewWriter(sf)
		defer sw.Flush()
	}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	var b *Brain
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if b == nil {
			h := runtimeParseHello(string(line))
			if h == nil {
				continue
			}
			shared := sharedLoad(h.Map.Dir, &h.Config)
			p := paramsDefault()
			if pp := os.Getenv("REPLAY_PARAMS"); pp != "" {
				data, err := os.ReadFile(pp)
				if err != nil {
					t.Fatal(err)
				}
				if p, err = paramsFromJSON(data); err != nil {
					t.Fatal(err)
				}
			}
			b = brainWithParams(h, shared, p)
			continue
		}
		kind, _, obs, _, err := protoParseServerMsg(line)
		if err != nil || kind != "obs" {
			continue
		}
		budget := budgetUnlimited()
		a := b.act(obs, &budget)
		a.Tick = obs.Tick
		a.Debug = nil
		s, err := a.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		w.Write(s)
		w.WriteByte('\n')
		if sw != nil {
			sw.WriteString(replayState(b))
			sw.WriteByte('\n')
		}
	}
}

// replayState mirrors the debug state line of the brain in a neutral form for diffing.
func replayState(b *Brain) string {
	modes := []string{"Idle", "Zone", "Fight", "Retreat", "Loot", "Drop", "Hunt", "Roof"}
	g := b.goal
	gs := "None"
	switch g.kind {
	case GoalKindNav:
		gs = fmt.Sprintf("Nav %.3f %.3f %.3f %.3f", g.navPos[0], g.navPos[1], g.navPos[2], g.navArrive)
	case GoalKindField:
		gs = fmt.Sprintf("Field %.3f %.3f %.3f", g.fieldCenter[0], g.fieldCenter[1], g.fieldArrive)
	case GoalKindRange:
		gs = fmt.Sprintf("Range %.3f %.3f %.3f %.3f", g.rangePos[0], g.rangePos[1], g.rangePos[2], g.rangePref)
	}
	ts := "None"
	switch b.target.kind {
	case TgtKindMech:
		ts = fmt.Sprintf("Mech(%d)", b.target.mech0)
	case TgtKindNpc:
		ts = fmt.Sprintf("Npc(%d)", b.target.npc0)
	case TgtKindCrate:
		ts = fmt.Sprintf("Crate(%d)", b.target.crate0)
	}
	return fmt.Sprintf("%s | %s | %s | tracks=%d items=%d/%d/%d", modes[b.mode], gs, ts, len(b.mem.tracks), len(b.mem.crates), len(b.mem.cubes), len(b.mem.pickups))
}
