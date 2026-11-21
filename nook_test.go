package main

import (
	"testing"
	"time"
)

func TestNooks(t *testing.T) {
	w := LoadWorld("../kit/mechfight-kit/arena")
	t0 := time.Now()
	n := buildNooks(w)
	if n == nil {
		t.Skip("no map")
	}
	cnt := 0
	for _, q := range n.q {
		if q > 0 {
			cnt++
		}
	}
	t.Logf("nooks: %d of %d cells, built in %v", cnt, len(n.q), time.Since(t0))
}
