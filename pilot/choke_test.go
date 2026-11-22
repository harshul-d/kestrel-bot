package pilot

import (
	"bufio"
	"os"
	"testing"
	"time"
)

// TestChokeBuild needs CHOKE_STREAM (a recorded hello line first) and the map it names.
func TestChokeBuild(t *testing.T) {
	in := os.Getenv("CHOKE_STREAM")
	if in == "" {
		t.Skip("CHOKE_STREAM not set")
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	sc.Scan()
	h := runtimeParseHello(sc.Text())
	if h == nil {
		t.Fatal("no hello")
	}
	if d := os.Getenv("CHOKE_MAP"); d != "" {
		h.Map.Dir = d
	}
	t0 := time.Now()
	sh := sharedLoad(h.Map.Dir, &h.Config)
	t1 := time.Now()
	spots := chokeBuild(&sh.world, &sh.nav, time.Now().Add(10*time.Second))
	t.Logf("map %v, nodes %d, choke build %v, spots %d", t1.Sub(t0), sh.nav.nodeCount(), time.Since(t1), len(spots))
	for i := 0; i < len(spots); i += len(spots)/8 + 1 {
		s := spots[i]
		t.Logf("spot %.0f %.0f %.1f yaw %.2f half %.2f", s.pos[0], s.pos[1], s.pos[2], s.yaw, s.half)
	}
}

// TestChokeTwoFoes replays CHOKE_STREAM with a second attacker cloned next to every mech seen,
// and reports whether the choke play engages and the slowest decision (unlimited budget).
// CHOKE_FAKE=1 recentres the next zone circle on us so spots near the recording are allowed.
func TestChokeTwoFoes(t *testing.T) {
	in := os.Getenv("CHOKE_STREAM")
	if in == "" {
		t.Skip("CHOKE_STREAM not set")
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	sc.Scan()
	h := runtimeParseHello(sc.Text())
	if d := os.Getenv("CHOKE_MAP"); d != "" {
		h.Map.Dir = d
	}
	sh := sharedLoad(h.Map.Dir, &h.Config)
	p, err := paramsFromJSON(paramsShipped)
	if err != nil {
		t.Fatal(err)
	}
	b := brainWithParams(h, sh, p)
	t.Logf("spots %d", len(b.choke.spots))
	var worst time.Duration
	active, arrived := 0, 0
	for sc.Scan() {
		_, _, obs, _, err := protoParseServerMsg(sc.Bytes())
		if err != nil || obs == nil {
			continue
		}
		n := len(obs.Mechs)
		for i := 0; i < n; i++ {
			m := obs.Mechs[i]
			m.Id += 1
			m.Pos[0] += 15
			obs.Mechs = append(obs.Mechs, m)
		}
		if os.Getenv("CHOKE_FAKE") != "" {
			obs.Zone.NextCenter = V2{obs.Me.Pos[0], obs.Me.Pos[1]}
			obs.Zone.NextRadius = 300
		}
		if os.Getenv("CHOKE_FAKE") != "" && obs.Tick == 200 {
			q := obs.Me.Pos
			b.choke.spots = append(b.choke.spots, ChokeSpot{pos: V3{q[0] - 20, q[1] - 10, q[2]}, yaw: 0.5, half: 1.0})
			t.Logf("fake spot ok=%v", b.chokeSpotOk(obs, &b.choke.spots[len(b.choke.spots)-1]))
		}
		bud := budgetUnlimited()
		t0 := time.Now()
		b.act(obs, &bud)
		if d := time.Since(t0); d > worst {
			worst = d
		}
		if obs.Tick%100 == 0 {
			nh, _ := b.chokeHunters(obs, &sh.world)
			best, ok := 1e9, 0
			for i := range b.choke.spots {
				d := dist(b.choke.spots[i].pos, obs.Me.Pos)
				if d < best {
					best = d
				}
				if b.chokeSpotOk(obs, &b.choke.spots[i]) {
					ok++
				}
			}
			t.Logf("t=%.1f hunters=%d nearest=%.0f zoneOk=%d inzone=%v mode=%v", obs.T, nh, best, ok, obs.Me.InZone, b.mode)
		}
		if b.choke.active {
			active++
			if b.choke.arrived >= 0 {
				arrived++
			}
			if active%20 == 1 {
				sp := b.choke.spots[b.choke.spot]
				t.Logf("t=%.1f mode=%v spot=(%.0f,%.0f) me=(%.0f,%.0f) arrived=%v", obs.T, b.mode, sp.pos[0], sp.pos[1], obs.Me.Pos[0], obs.Me.Pos[1], b.choke.arrived >= 0)
			}
		}
	}
	t.Logf("active ticks %d, arrived ticks %d, worst decision %v", active, arrived, worst)
}
