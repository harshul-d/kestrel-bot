package pilot

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// TestFfaDetect runs the shipped brain over FFA_STREAM (JSONL or .gz, hello first) and reports
// whether and when free-for-all mode switched on. FFA_EXPECT=on|off makes it a check.
func TestFfaDetect(t *testing.T) {
	in := os.Getenv("FFA_STREAM")
	if in == "" {
		t.Skip("FFA_STREAM not set")
	}
	f, err := os.Open(in)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var r io.Reader = f
	if strings.HasSuffix(in, ".gz") {
		if r, err = gzip.NewReader(f); err != nil {
			t.Fatal(err)
		}
	}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	sc.Scan()
	h := runtimeParseHello(sc.Text())
	if d := os.Getenv("FFA_MAP"); d != "" {
		h.Map.Dir = d
	}
	b := brainNew(h, sharedLoad(h.Map.Dir, &h.Config))
	last := 0.0
	var worst time.Duration
	worstT := 0.0
	for sc.Scan() {
		_, _, obs, _, err := protoParseServerMsg(sc.Bytes())
		if err != nil || obs == nil {
			continue
		}
		bud := budgetFromNowMs(18)
		t0 := time.Now()
		b.act(obs, &bud)
		if d := time.Since(t0); d > worst && obs.Tick > 0 {
			worst, worstT = d, obs.T
		}
		last = obs.T
	}
	t.Logf("slowest decision after the first: %v at t=%.1f", worst, worstT)
	t.Logf("ffa on=%v at t=%.1f why=%d sig=%d kills=%d alive0=%d (stream to t=%.1f)", b.ffa.on, b.ffa.onT, b.ffa.why, b.ffa.sig, b.ffa.kills, b.ffa.alive0, last)
	switch os.Getenv("FFA_EXPECT") {
	case "on":
		if !b.ffa.on {
			t.Fatal("expected free-for-all mode on")
		}
	case "off":
		if b.ffa.on {
			t.Fatal("expected free-for-all mode off")
		}
	}
}
