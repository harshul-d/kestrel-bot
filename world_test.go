package main

import (
	"os"
	"testing"
	"time"
)

func TestPath(t *testing.T) {
	dir := os.Getenv("ARENA")
	if dir == "" {
		dir = "../kit/mechfight-kit/arena"
	}
	w := LoadWorld(dir)
	if !w.ok {
		t.Skip("no map")
	}
	n := 0
	for _, s := range w.stand {
		if s {
			n++
		}
	}
	t.Logf("standable %d / %d", n, len(w.stand))
	var buf astarBuf
	cases := [][4]float64{{320, -280, -42, 7}, {-400, 300, 400, -250}, {0, 0, 200, 200}, {-217, -22, -23, 2}}
	for _, c := range cases {
		st := time.Now()
		s := w.NearestStand(c[0], c[1], 6, false)
		g := w.NearestStand(c[2], c[3], 15, true)
		p := w.Path(&buf, s, g, 10000, nil)
		end := [2]float64{}
		if len(p) > 0 {
			end = p[len(p)-1]
		}
		t.Logf("%v -> %d waypoints, end %v, %v", c, len(p), end, time.Since(st))
	}
}

func TestRoofs(t *testing.T) {
	w := LoadWorld("../kit/mechfight-kit/arena")
	if !w.ok {
		t.Skip("no map")
	}
	if w.Roofs == nil {
		w.ComputeRoofs()
	}
	n, routes := 0, 0
	for _, r := range w.Roofs {
		if len(r.Edges) > 0 {
			n++
			routes += len(r.Edges)
			if n <= 8 {
				t.Logf("roof %d cells %d H %.1f rise %.1f centre %.0f,%.0f edges %d", r.ID, len(r.Cells), r.H, r.Rise, r.CX, r.CY, len(r.Edges))
			}
		}
	}
	t.Logf("roofs total %d, climbable %d, routes %d", len(w.Roofs), n, routes)
}
