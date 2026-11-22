package pilot

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

type PrismKind int

const (
	PrismKindBuilding PrismKind = iota
	PrismKindStadium
	PrismKindBridge
	PrismKindOther
)

type Prism struct {
	kind  PrismKind
	base  float64
	top   float64
	ring  []V2
	holes [][]V2
	minX  float64
	maxX  float64
	minY  float64
	maxY  float64
}

func worldPnpoly(poly []V2, x, y float64) bool {
	n := len(poly)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

// containsXy: inside the ring and outside every hole.
func (p *Prism) containsXy(x, y float64) bool {
	if x < p.minX || x > p.maxX || y < p.minY || y > p.maxY {
		return false
	}
	if !worldPnpoly(p.ring, x, y) {
		return false
	}
	for _, h := range p.holes {
		if worldPnpoly(h, x, y) {
			return false
		}
	}
	return true
}

// distXy: (signed distance to the footprint boundary, negative inside; closest boundary point;
// outward unit normal pointing from solid to free space).
func (p *Prism) distXy(x, y float64) struct {
	f0 float64
	f1 V2
	f2 V2
} {
	best := math.Inf(1)
	bp := V2{x, y}
	be := V2{1.0, 0.0}
	for pk := -1; pk < len(p.holes); pk++ {
		poly := p.ring
		if pk >= 0 {
			poly = p.holes[pk]
		}
		n := len(poly)
		if n < 2 {
			continue
		}
		j := n - 1
		for i := 0; i < n; i++ {
			u, v := poly[j], poly[i]
			ex := v[0] - u[0]
			ey := v[1] - u[1]
			l2 := ex*ex + ey*ey
			s := 0.0
			if l2 > 0.0 {
				s = fclamp(((x-u[0])*ex+(y-u[1])*ey)/l2, 0.0, 1.0)
			}
			cx := u[0] + s*ex
			cy := u[1] + s*ey
			d2 := (x-cx)*(x-cx) + (y-cy)*(y-cy)
			if d2 < best {
				best = d2
				bp = V2{cx, cy}
				be = V2{ex, ey}
			}
			j = i
		}
	}
	type res = struct {
		f0 float64
		f1 V2
		f2 V2
	}
	if !isFinite(best) {
		return res{math.Inf(1), V2{x, y}, V2{1.0, 0.0}}
	}
	d := math.Sqrt(best)
	inside := p.containsXy(x, y)
	var n V2
	if d > 1e-9 {
		s := 1.0
		if inside {
			s = -1.0
		}
		n = V2{s * (x - bp[0]) / d, s * (y - bp[1]) / d}
	} else {
		l := fmax(math.Sqrt(be[0]*be[0]+be[1]*be[1]), 1e-12)
		n = V2{be[1] / l, -be[0] / l}
		if p.containsXy(bp[0]+n[0]*1e-4, bp[1]+n[1]*1e-4) {
			n = V2{-n[0], -n[1]}
		}
	}
	sd := d
	if inside {
		sd = -d
	}
	return res{sd, bp, n}
}

type Tree struct {
	x float64
	y float64
	// ground height at the trunk
	z float64
	// height above z
	height float64
	crownR float64
	// crown base above z
	crownBase float64
	trunkR    float64
	crownX    float64
	crownY    float64
}

type Bounds struct {
	minX float64
	maxX float64
	minY float64
	maxY float64
}

type Entry struct {
	base  float64
	top   float64
	e0    uint32
	en    uint16
	prism uint16
	refIn bool
	full  bool
}

const worldRefDx float64 = 0.512_345_67
const worldRefDy float64 = 0.497_654_32
const worldTreeCell float64 = 8.0
const worldPrismCell float64 = 16.0

type World struct {
	bounds        Bounds
	prisms        []Prism
	trees         []Tree
	canopyDensity float64
	canopyBlockT  float64
	maxStep       float64
	maxSlopeDeg   float64
	// terrain vertex grid, step 1 m; cell (i, j) spans [gx0+i, gx0+i+1] x [gy0+j, gy0+j+1]
	ogx0 float64
	ogy0 float64
	onx  int
	ony  int
	opl  int
	opb  int
	gx0  float64
	gy0  float64
	nx   int
	ny   int
	h    []float32
	// per cell (cnx = nx-1, cny = ny-1)
	cnx       int
	cny       int
	cellHmax  []float32
	cellZtop  []float32
	cellStart []uint32
	entries   []Entry
	edges     [][4]float64
	// trees
	tnx       int
	tny       int
	treeStart []uint32
	treeIdx   []uint16
	// prisms by bbox
	pnx     int
	pny     int
	prStart []uint32
	prIdx   []uint16
}

// ---- JSON access helpers (decoded values: map[string]any, []any, float64, string, bool, nil) ----

func worldGet(v any, k string) any {
	if m, ok := v.(map[string]any); ok {
		return m[k]
	}
	return nil
}

func worldAsF64(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func worldF(v any, k string, d float64) float64 {
	if f, ok := worldAsF64(worldGet(v, k)); ok {
		return f
	}
	return d
}

var worldPow10 = func() (t [309]float64) {
	for i := range t {
		t[i], _ = strconv.ParseFloat("1e"+strconv.Itoa(i), 64)
	}
	return
}()

var worldErrRange = errors.New("number out of range")

func worldDigit(s string, i int) bool { return i < len(s) && s[i] >= '0' && s[i] <= '9' }

func worldOverflowU64(a, b uint64) bool {
	return a >= math.MaxUint64/10 && (a > math.MaxUint64/10 || b > math.MaxUint64%10)
}

// worldParseNum decodes a JSON number literal: integers exactly when they fit in 64 bits,
// otherwise significand (at most 64 bits) scaled by one power of ten.
func worldParseNum(s string) (float64, error) {
	i := 0
	positive := true
	if i < len(s) && s[i] == '-' {
		positive = false
		i++
	}
	var sig uint64
	if i < len(s) && s[i] == '0' {
		i++
	} else {
		for worldDigit(s, i) {
			d := uint64(s[i] - '0')
			if worldOverflowU64(sig, d) {
				exp := int32(0)
				for worldDigit(s, i) {
					i++
					exp++
				}
				return worldParseTail(s, i, positive, sig, exp)
			}
			sig = sig*10 + d
			i++
		}
	}
	if i < len(s) && (s[i] == '.' || s[i] == 'e' || s[i] == 'E') {
		return worldParseTail(s, i, positive, sig, 0)
	}
	if positive {
		return float64(sig), nil
	}
	return -float64(sig), nil
}

func worldParseTail(s string, i int, positive bool, sig uint64, exp int32) (float64, error) {
	if i < len(s) && s[i] == '.' {
		i++
		after := int32(0)
		for worldDigit(s, i) {
			d := uint64(s[i] - '0')
			if worldOverflowU64(sig, d) {
				for worldDigit(s, i) {
					i++
				}
				break
			}
			sig = sig*10 + d
			after--
			i++
		}
		exp = exp + after
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		posExp := true
		if i < len(s) && s[i] == '+' {
			i++
		} else if i < len(s) && s[i] == '-' {
			posExp = false
			i++
		}
		if !worldDigit(s, i) {
			return 0, errors.New("invalid number")
		}
		e := int32(s[i] - '0')
		i++
		for worldDigit(s, i) {
			d := int32(s[i] - '0')
			i++
			if e >= math.MaxInt32/10 && (e > math.MaxInt32/10 || d > math.MaxInt32%10) {
				if sig != 0 && posExp {
					return 0, worldErrRange
				}
				if positive {
					return 0.0, nil
				}
				return math.Copysign(0, -1), nil
			}
			e = e*10 + d
		}
		if posExp {
			r := int64(exp) + int64(e)
			if r > math.MaxInt32 {
				r = math.MaxInt32
			}
			exp = int32(r)
		} else {
			r := int64(exp) - int64(e)
			if r < math.MinInt32 {
				r = math.MinInt32
			}
			exp = int32(r)
		}
	}
	return worldF64FromParts(positive, sig, exp)
}

func worldF64FromParts(positive bool, sig uint64, exp int32) (float64, error) {
	f := float64(sig)
	for {
		a := exp
		if a < 0 {
			a = -a
		}
		if a >= 0 && int(a) < len(worldPow10) {
			pow := worldPow10[a]
			if exp >= 0 {
				f *= pow
				if math.IsInf(f, 0) {
					return 0, worldErrRange
				}
			} else {
				f /= pow
			}
			break
		}
		if f == 0.0 {
			break
		}
		if exp >= 0 {
			return 0, worldErrRange
		}
		f /= 1e308
		exp += 308
	}
	if positive {
		return f, nil
	}
	return -f, nil
}

func worldConvNums(v any) (any, error) {
	switch t := v.(type) {
	case json.Number:
		return worldParseNum(string(t))
	case map[string]any:
		for k, x := range t {
			c, err := worldConvNums(x)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
		return t, nil
	case []any:
		for k, x := range t {
			c, err := worldConvNums(x)
			if err != nil {
				return nil, err
			}
			t[k] = c
		}
		return t, nil
	}
	return v, nil
}

func worldParseJSON(txt []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(txt))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing characters")
	}
	return worldConvNums(v)
}

func worldFlatPairs(v any) []V2 {
	a, _ := v.([]any)
	out := make([]V2, 0, len(a)/2)
	for k := 0; k+1 < len(a); k += 2 {
		x, ok := worldAsF64(a[k])
		if !ok {
			x = 0.0
		}
		y, ok := worldAsF64(a[k+1])
		if !ok {
			y = 0.0
		}
		out = append(out, V2{x, y})
	}
	if len(out) > 1 && out[0] == out[len(out)-1] {
		out = out[:len(out)-1]
	}
	return out
}

func worldOrient(a, b, c V2) float64 {
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}

// worldSegCross: segment c->p crosses edge u->v (half-open convention, orientation 0 counts as positive)
func worldSegCross(c, p, u, v V2) bool {
	o1 := worldOrient(u, v, c) >= 0.0
	o2 := worldOrient(u, v, p) >= 0.0
	if o1 == o2 {
		return false
	}
	o3 := worldOrient(c, p, u) >= 0.0
	o4 := worldOrient(c, p, v) >= 0.0
	return o3 != o4
}

// worldSegBox: segment intersects closed axis-aligned box
func worldSegBox(ax, ay, bx, by, x0, y0, x1, y1 float64) bool {
	t0 := 0.0
	t1 := 1.0
	d := [2]float64{bx - ax, by - ay}
	o := [2]float64{ax, ay}
	lo := [2]float64{x0, y0}
	hi := [2]float64{x1, y1}
	for k := 0; k < 2; k++ {
		if math.Abs(d[k]) < 1e-15 {
			if o[k] < lo[k] || o[k] > hi[k] {
				return false
			}
		} else {
			ta := (lo[k] - o[k]) / d[k]
			tb := (hi[k] - o[k]) / d[k]
			if ta > tb {
				ta, tb = tb, ta
			}
			t0 = fmax(t0, ta)
			t1 = fmin(t1, tb)
			if t0 > t1 {
				return false
			}
		}
	}
	return true
}

func worldLoad(dir string) (World, error) {
	base := dir
	if _, err := os.Stat(filepath.Join(dir, "v2", "world.json")); err == nil {
		base = filepath.Join(dir, "v2")
	}
	txt, err := os.ReadFile(filepath.Join(base, "world.json"))
	if err != nil {
		return World{}, fmt.Errorf("world.json: %v", err)
	}
	j, err := worldParseJSON(txt)
	if err != nil {
		return World{}, fmt.Errorf("world.json parse: %v", err)
	}
	f := worldF
	b := worldGet(j, "bounds")
	bounds := Bounds{minX: f(b, "minX", -1e6), maxX: f(b, "maxX", 1e6), minY: f(b, "minY", -1e6), maxY: f(b, "maxY", 1e6)}
	r := worldGet(j, "rules")
	tr := worldGet(j, "terrain")
	nx := f2u(f(tr, "nx", 0.0))
	ny := f2u(f(tr, "ny", 0.0))
	gx0 := f(tr, "minX", 0.0)
	gy0 := f(tr, "minY", 0.0)
	scale := f(tr, "scale", 0.01)
	step := f(tr, "step", 1.0)
	if math.Abs(step-1.0) > 1e-9 {
		return World{}, errors.New("terrain step != 1")
	}
	if nx < 2 || ny < 2 {
		return World{}, errors.New("terrain too small")
	}
	tfile := "terrain.i16"
	if s, ok := worldGet(tr, "file").(string); ok {
		tfile = s
	}
	tpath := filepath.Join(base, tfile)
	if filepath.IsAbs(tfile) {
		tpath = tfile
	}
	raw, err := os.ReadFile(tpath)
	if err != nil {
		return World{}, fmt.Errorf("terrain: %v", err)
	}
	if len(raw) < nx*ny*2 {
		return World{}, errors.New("terrain file short")
	}
	h := make([]float32, nx*ny)
	for k := range h {
		h[k] = float32(float64(int16(binary.LittleEndian.Uint16(raw[2*k:]))) * scale)
	}

	var prisms []Prism
	if ps, ok := worldGet(j, "prisms").([]any); ok {
		for _, p := range ps {
			kstr, _ := worldGet(p, "kind").(string)
			var kind PrismKind
			switch kstr {
			case "building":
				kind = PrismKindBuilding
			case "stadium":
				kind = PrismKindStadium
			case "bridge":
				kind = PrismKindBridge
			default:
				kind = PrismKindOther
			}
			ring := worldFlatPairs(worldGet(p, "ring"))
			if len(ring) < 3 {
				continue
			}
			var holes [][]V2
			if hs, ok := worldGet(p, "holes").([]any); ok {
				for _, hv := range hs {
					hp := worldFlatPairs(hv)
					if len(hp) >= 3 {
						holes = append(holes, hp)
					}
				}
			}
			a, bx, c, d := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
			for _, q := range ring {
				a = fmin(a, q[0])
				bx = fmax(bx, q[0])
				c = fmin(c, q[1])
				d = fmax(d, q[1])
			}
			prisms = append(prisms, Prism{kind: kind, base: f(p, "base", 0.0), top: f(p, "top", 0.0), ring: ring, holes: holes, minX: a, maxX: bx, minY: c, maxY: d})
		}
	}
	var trees []Tree
	tj := worldGet(j, "trees")
	fields, okF := worldGet(tj, "fields").([]any)
	data, okD := worldGet(tj, "data").([]any)
	if okF && okD {
		names := make([]string, len(fields))
		for k, x := range fields {
			s, _ := x.(string)
			names[k] = s
		}
		nf := len(names)
		col := func(n string) int {
			for k, x := range names {
				if x == n {
					return k
				}
			}
			return -1
		}
		cx, cy, cz, ch, ccr, ccb, ctr, ccx, ccy := col("x"), col("y"), col("z"), col("height"), col("crownR"), col("crownBase"), col("trunkR"), col("crownX"), col("crownY")
		if nf > 0 {
			for start := 0; start < len(data); start += nf {
				row := data[start:min(start+nf, len(data))]
				if len(row) < nf {
					break
				}
				g := func(c int, d float64) float64 {
					if c >= 0 {
						if v, ok := worldAsF64(row[c]); ok {
							return v
						}
					}
					return d
				}
				x := g(cx, 0.0)
				y := g(cy, 0.0)
				trees = append(trees, Tree{x: x, y: y, z: g(cz, 0.0), height: g(ch, 0.0), crownR: g(ccr, 0.0), crownBase: g(ccb, 0.0), trunkR: g(ctr, 0.0), crownX: g(ccx, x), crownY: g(ccy, y)})
			}
		}
	}
	// pad the grid (clamped heights) so every prism and tree footprint lies inside it
	ex0, ex1, ey0, ey1 := gx0, gx0+float64(nx-1), gy0, gy0+float64(ny-1)
	for k := range prisms {
		p := &prisms[k]
		ex0 = fmin(ex0, p.minX)
		ex1 = fmax(ex1, p.maxX)
		ey0 = fmin(ey0, p.minY)
		ey1 = fmax(ey1, p.maxY)
	}
	for k := range trees {
		t := &trees[k]
		rr := fmax(t.crownR, t.trunkR)
		ex0 = fmin(ex0, fmin(t.x, t.crownX)-rr)
		ex1 = fmax(ex1, fmax(t.x, t.crownX)+rr)
		ey0 = fmin(ey0, fmin(t.y, t.crownY)-rr)
		ey1 = fmax(ey1, fmax(t.y, t.crownY)+rr)
	}
	pl := f2u(math.Ceil(fmax(gx0-ex0, 0.0))) + 2
	pb := f2u(math.Ceil(fmax(gy0-ey0, 0.0))) + 2
	pr := f2u(math.Ceil(fmax(ex1-(gx0+float64(nx-1)), 0.0))) + 2
	pt := f2u(math.Ceil(fmax(ey1-(gy0+float64(ny-1)), 0.0))) + 2
	onx, ony := nx, ny
	nx2 := nx + pl + pr
	ny2 := ny + pb + pt
	h2 := make([]float32, nx2*ny2)
	for jj := 0; jj < ny2; jj++ {
		sj := int(iclamp(int64(jj)-int64(pb), 0, int64(ony)-1))
		for ii := 0; ii < nx2; ii++ {
			si := int(iclamp(int64(ii)-int64(pl), 0, int64(onx)-1))
			h2[jj*nx2+ii] = h[sj*onx+si]
		}
	}
	w := World{
		bounds:        bounds,
		ogx0:          gx0,
		ogy0:          gy0,
		onx:           onx,
		ony:           ony,
		opl:           pl,
		opb:           pb,
		prisms:        prisms,
		trees:         trees,
		canopyDensity: f(r, "canopyDensity", 0.35),
		canopyBlockT:  f(r, "canopyBlockT", 0.25),
		maxStep:       f(r, "maxStep", 1.5),
		maxSlopeDeg:   f(r, "maxSlopeDeg", 50.0),
		gx0:           gx0 - float64(pl),
		gy0:           gy0 - float64(pb),
		nx:            nx2,
		ny:            ny2,
		h:             h2,
	}
	w.buildIndex()
	return w, nil
}

func worldEmpty() World {
	return World{
		bounds:        Bounds{minX: -1e6, maxX: 1e6, minY: -1e6, maxY: 1e6},
		canopyDensity: 0.35,
		canopyBlockT:  0.25,
		maxStep:       1.5,
		maxSlopeDeg:   50.0,
		cellStart:     []uint32{0},
		treeStart:     []uint32{0},
		prStart:       []uint32{0},
	}
}

func (w *World) buildIndex() {
	cnx := w.nx - 1
	cny := w.ny - 1
	w.cnx = cnx
	w.cny = cny
	nc := cnx * cny
	hmax := make([]float32, nc)
	for j := 0; j < cny; j++ {
		for i := 0; i < cnx; i++ {
			k := j*w.nx + i
			hmax[j*cnx+i] = worldFmax32(worldFmax32(worldFmax32(w.h[k], w.h[k+1]), w.h[k+w.nx]), w.h[k+w.nx+1])
		}
	}
	w.cellHmax = hmax
	// (cell, prism, edge) triples for boundary cells
	type bndT struct {
		c  uint32
		pi uint16
		e  [4]float64
	}
	type fullT struct {
		c  uint32
		pi uint16
	}
	var bnd []bndT
	var full []fullT
	gx0, gy0 := w.gx0, w.gy0
	ci := func(x float64) int { return int(iclamp(f2i64(math.Floor(x-gx0)), 0, int64(cnx)-1)) }
	cj := func(y float64) int { return int(iclamp(f2i64(math.Floor(y-gy0)), 0, int64(cny)-1)) }
	mark := make([]uint32, nc)
	for k := range mark {
		mark[k] = math.MaxUint32
	}
	var xs []float64
	for pi := range w.prisms {
		p := &w.prisms[pi]
		pi16 := uint16(pi)
		for pk := -1; pk < len(p.holes); pk++ {
			poly := p.ring
			if pk >= 0 {
				poly = p.holes[pk]
			}
			n := len(poly)
			jv := n - 1
			for iv := 0; iv < n; iv++ {
				u, v := poly[jv], poly[iv]
				jv = iv
				pad := 1e-7
				i0, i1 := ci(fmin(u[0], v[0])-pad), ci(fmax(u[0], v[0])+pad)
				j0, j1 := cj(fmin(u[1], v[1])-pad), cj(fmax(u[1], v[1])+pad)
				for jj := j0; jj <= j1; jj++ {
					for ii := i0; ii <= i1; ii++ {
						x0 := gx0 + float64(ii)
						y0 := gy0 + float64(jj)
						if worldSegBox(u[0], u[1], v[0], v[1], x0-pad, y0-pad, x0+1.0+pad, y0+1.0+pad) {
							c := uint32(jj*cnx + ii)
							bnd = append(bnd, bndT{c, pi16, [4]float64{u[0], u[1], v[0], v[1]}})
							mark[c] = uint32(pi)
						}
					}
				}
			}
		}
		// interior fill by scanline at cell-centre rows: cells whose centre is inside and not boundary
		j0, j1 := cj(p.minY), cj(p.maxY)
		for jj := j0; jj <= j1; jj++ {
			yc := gy0 + float64(jj) + 0.5
			xs = xs[:0]
			for pk := -1; pk < len(p.holes); pk++ {
				poly := p.ring
				if pk >= 0 {
					poly = p.holes[pk]
				}
				n := len(poly)
				jv := n - 1
				for iv := 0; iv < n; iv++ {
					a, bb := poly[iv], poly[jv]
					if (a[1] > yc) != (bb[1] > yc) {
						xs = append(xs, (bb[0]-a[0])*(yc-a[1])/(bb[1]-a[1])+a[0])
					}
					jv = iv
				}
			}
			sortStable(xs, func(a, b *float64) int { return partialCmp(*a, *b) })
			for q := 0; q < len(xs); q += 2 {
				if q+1 >= len(xs) {
					break
				}
				xa, xb := xs[q], xs[q+1]
				// cells whose centre lies in [xa, xb]
				ia := max(f2i64(math.Ceil(xa-gx0-0.5)), 0)
				ib := min(f2i64(math.Floor(xb-gx0-0.5)), int64(cnx)-1)
				for ii := ia; ii <= ib; ii++ {
					c := uint32(jj*cnx + int(ii))
					if mark[c] != uint32(pi) {
						full = append(full, fullT{c, pi16})
					}
				}
			}
		}
	}
	sortStable(bnd, func(a, b *bndT) int {
		if a.c != b.c {
			if a.c < b.c {
				return -1
			}
			return 1
		}
		if a.pi != b.pi {
			if a.pi < b.pi {
				return -1
			}
			return 1
		}
		return 0
	})
	fullCmp := func(a, b *fullT) int {
		if a.c != b.c {
			if a.c < b.c {
				return -1
			}
			return 1
		}
		if a.pi != b.pi {
			if a.pi < b.pi {
				return -1
			}
			return 1
		}
		return 0
	}
	sortStable(full, fullCmp)
	full = slices.Compact(full)
	// merge into per-cell entries
	counts := make([]uint32, nc+1)
	type entT struct {
		c uint32
		e Entry
	}
	var entries []entT
	edges := make([][4]float64, 0, len(bnd))
	k := 0
	for k < len(bnd) {
		c, pi := bnd[k].c, bnd[k].pi
		e0 := uint32(len(edges))
		for k < len(bnd) && bnd[k].c == c && bnd[k].pi == pi {
			edges = append(edges, bnd[k].e)
			k++
		}
		p := &w.prisms[pi]
		ii := int(c) % cnx
		jj := int(c) / cnx
		rx := gx0 + float64(ii) + worldRefDx
		ry := gy0 + float64(jj) + worldRefDy
		entries = append(entries, entT{c, Entry{base: p.base, top: p.top, e0: e0, en: uint16(uint32(len(edges)) - e0), prism: pi, refIn: p.containsXy(rx, ry), full: false}})
	}
	for _, fe := range full {
		p := &w.prisms[fe.pi]
		entries = append(entries, entT{fe.c, Entry{base: p.base, top: p.top, e0: 0, en: 0, prism: fe.pi, refIn: true, full: true}})
	}
	sortStable(entries, func(a, b *entT) int {
		if a.c != b.c {
			if a.c < b.c {
				return -1
			}
			return 1
		}
		if a.e.prism != b.e.prism {
			if a.e.prism < b.e.prism {
				return -1
			}
			return 1
		}
		return 0
	})
	for _, e := range entries {
		counts[e.c+1]++
	}
	for c := 0; c < nc; c++ {
		counts[c+1] += counts[c]
	}
	ztop := make([]float32, nc)
	for c := range ztop {
		ztop[c] = float32(math.Inf(-1))
	}
	for _, e := range entries {
		t := float32(e.e.top)
		if t > ztop[e.c] {
			ztop[e.c] = t
		}
	}
	w.cellZtop = ztop
	w.cellStart = counts
	w.entries = make([]Entry, len(entries))
	for i, e := range entries {
		w.entries[i] = e.e
	}
	w.edges = edges

	pairCmp := func(a, b *fullT) int { return fullCmp(a, b) }

	// trees
	tnx := f2u(math.Ceil(float64(cnx) / worldTreeCell))
	tny := f2u(math.Ceil(float64(cny) / worldTreeCell))
	var tl []fullT
	tc := func(v, o float64, n int) int {
		return int(iclamp(f2i64(math.Floor((v-o)/worldTreeCell)), 0, int64(n)-1))
	}
	for ti := range w.trees {
		t := &w.trees[ti]
		x0 := fmin(t.x-t.trunkR, t.crownX-t.crownR)
		x1 := fmax(t.x+t.trunkR, t.crownX+t.crownR)
		y0 := fmin(t.y-t.trunkR, t.crownY-t.crownR)
		y1 := fmax(t.y+t.trunkR, t.crownY+t.crownR)
		for jj, je := tc(y0, gy0, tny), tc(y1, gy0, tny); jj <= je; jj++ {
			for ii, ie := tc(x0, gx0, tnx), tc(x1, gx0, tnx); ii <= ie; ii++ {
				tl = append(tl, fullT{uint32(jj*tnx + ii), uint16(ti)})
			}
		}
	}
	sortStable(tl, pairCmp)
	ts := make([]uint32, tnx*tny+1)
	for _, e := range tl {
		ts[e.c+1]++
	}
	for c := 0; c < tnx*tny; c++ {
		ts[c+1] += ts[c]
	}
	w.tnx = tnx
	w.tny = tny
	w.treeStart = ts
	w.treeIdx = make([]uint16, len(tl))
	for i, e := range tl {
		w.treeIdx[i] = e.pi
	}

	// prisms by bbox
	pnx := f2u(math.Ceil(float64(cnx) / worldPrismCell))
	pny := f2u(math.Ceil(float64(cny) / worldPrismCell))
	var pl []fullT
	pc := func(v, o float64, n int) int {
		return int(iclamp(f2i64(math.Floor((v-o)/worldPrismCell)), 0, int64(n)-1))
	}
	for pi := range w.prisms {
		p := &w.prisms[pi]
		for jj, je := pc(p.minY, gy0, pny), pc(p.maxY, gy0, pny); jj <= je; jj++ {
			for ii, ie := pc(p.minX, gx0, pnx), pc(p.maxX, gx0, pnx); ii <= ie; ii++ {
				pl = append(pl, fullT{uint32(jj*pnx + ii), uint16(pi)})
			}
		}
	}
	sortStable(pl, pairCmp)
	ps := make([]uint32, pnx*pny+1)
	for _, e := range pl {
		ps[e.c+1]++
	}
	for c := 0; c < pnx*pny; c++ {
		ps[c+1] += ps[c]
	}
	w.pnx = pnx
	w.pny = pny
	w.prStart = ps
	w.prIdx = make([]uint16, len(pl))
	for i, e := range pl {
		w.prIdx[i] = e.pi
	}
}

// worldFmax32 is f32::max (NaN-ignoring).
func worldFmax32(a, b float32) float32 {
	if a != a {
		return b
	}
	if b != b {
		return a
	}
	if b > a {
		return b
	}
	return a
}

func (w *World) cellOf(x, y float64) (int, bool) {
	if w.cnx == 0 {
		return 0, false
	}
	fx := x - w.gx0
	fy := y - w.gy0
	if !(fx >= 0.0 && fy >= 0.0) {
		return 0, false
	}
	i := f2u(fx)
	j := f2u(fy)
	if i >= w.cnx || j >= w.cny {
		return 0, false
	}
	return j*w.cnx + i, true
}

func (w *World) cellEntries(c int) []Entry {
	return w.entries[w.cellStart[c]:w.cellStart[c+1]]
}

// insideLocal: point-in-footprint using only the edges stored for this cell; (x, y) must lie in cell c
func (w *World) insideLocal(c int, e *Entry, x, y float64) bool {
	if e.full {
		return true
	}
	ii := c % w.cnx
	jj := c / w.cnx
	r := V2{w.gx0 + float64(ii) + worldRefDx, w.gy0 + float64(jj) + worldRefDy}
	p := V2{x, y}
	inside := e.refIn
	for _, ed := range w.edges[e.e0 : e.e0+uint32(e.en)] {
		if worldSegCross(r, p, V2{ed[0], ed[1]}, V2{ed[2], ed[3]}) {
			inside = !inside
		}
	}
	return inside
}

// terrainZ: bilinear terrain height (clamped to the grid edge outside the grid).
func (w *World) terrainZ(x, y float64) float64 {
	if w.nx < 2 {
		return 0.0
	}
	fx := fclamp(x-w.gx0, 0.0, float64(w.nx-1))
	fy := fclamp(y-w.gy0, 0.0, float64(w.ny-1))
	i := min(f2u(fx), w.nx-2)
	j := min(f2u(fy), w.ny-2)
	u := fx - float64(i)
	v := fy - float64(j)
	k := j*w.nx + i
	h00 := float64(w.h[k])
	h10 := float64(w.h[k+1])
	h01 := float64(w.h[k+w.nx])
	h11 := float64(w.h[k+w.nx+1])
	return h00 + (h10-h00)*u + (h01-h00)*v + (h00-h10-h01+h11)*u*v
}

// terrainVertex: terrain height at vertex (i, j) of the file grid.
func (w *World) terrainVertex(i, j int) float64 {
	return float64(w.h[(j+w.opb)*w.nx+i+w.opl])
}

// terrainGrid: (minX, minY, nx, ny) of the terrain file grid (step 1 m).
func (w *World) terrainGrid() struct {
	f0 float64
	f1 float64
	f2 int
	f3 int
} {
	return struct {
		f0 float64
		f1 float64
		f2 int
		f3 int
	}{w.ogx0, w.ogy0, w.onx, w.ony}
}

// floorBelow: highest walkable surface (terrain or prism top) at or below z (1e-6 tolerance).
func (w *World) floorBelow(x, y, z float64) float64 {
	best := w.terrainZ(x, y)
	if c, ok := w.cellOf(x, y); ok {
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			if e.top <= z+1e-6 && e.top > best && w.insideLocal(c, e, x, y) {
				best = e.top
			}
		}
	}
	return best
}

// ceilingAbove: lowest prism base strictly above z, or +Inf.
func (w *World) ceilingAbove(x, y, z float64) float64 {
	best := math.Inf(1)
	if c, ok := w.cellOf(x, y); ok {
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			if e.base > z && e.base < best && w.insideLocal(c, e, x, y) {
				best = e.base
			}
		}
	}
	return best
}

// solid: inside terrain (z < ground) or inside a prism (base <= z < top).
func (w *World) solid(x, y, z float64) bool {
	if z < w.terrainZ(x, y) {
		return true
	}
	if c, ok := w.cellOf(x, y); ok {
		if float32(z) > w.cellZtop[c]+0.01 {
			return false
		}
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			if z >= e.base && z < e.top && w.insideLocal(c, e, x, y) {
				return true
			}
		}
	}
	return false
}

// columns: (base, top) of every prism whose footprint contains (x, y).
func (w *World) columns(x, y float64, out *[]struct {
	f0 float64
	f1 float64
}) {
	*out = (*out)[:0]
	if c, ok := w.cellOf(x, y); ok {
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			if w.insideLocal(c, e, x, y) {
				*out = append(*out, struct {
					f0 float64
					f1 float64
				}{e.base, e.top})
			}
		}
	}
}

// prismsAt: indices of prisms containing (x, y) via callback.
func (w *World) prismsAt(x, y float64, f func(int, *Prism)) {
	if c, ok := w.cellOf(x, y); ok {
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			if w.insideLocal(c, e, x, y) {
				f(int(e.prism), &w.prisms[e.prism])
			}
		}
	}
}

// prismsNear visits every prism whose footprint comes within r of (x, y) (inside counts), each once.
func (w *World) prismsNear(x, y, r float64, f func(int, *Prism)) {
	if w.pnx == 0 {
		return
	}
	pc := func(v, o float64, n int) int {
		return int(iclamp(f2i64(math.Floor((v-o)/worldPrismCell)), 0, int64(n)-1))
	}
	qx0, qx1, qy0, qy1 := x-r, x+r, y-r, y+r
	for jj, je := pc(qy0, w.gy0, w.pny), pc(qy1, w.gy0, w.pny); jj <= je; jj++ {
		for ii, ie := pc(qx0, w.gx0, w.pnx), pc(qx1, w.gx0, w.pnx); ii <= ie; ii++ {
			c := jj*w.pnx + ii
			for _, pi := range w.prIdx[w.prStart[c]:w.prStart[c+1]] {
				p := &w.prisms[pi]
				if p.minX > qx1 || p.maxX < qx0 || p.minY > qy1 || p.maxY < qy0 {
					continue
				}
				// dedupe: handle only in the cell holding the low corner of the overlap
				if pc(fmax(qx0, p.minX), w.gx0, w.pnx) != ii || pc(fmax(qy0, p.minY), w.gy0, w.pny) != jj {
					continue
				}
				dx := fmax(fmax(p.minX-x, 0.0), x-p.maxX)
				dy := fmax(fmax(p.minY-y, 0.0), y-p.maxY)
				if dx*dx+dy*dy > r*r {
					continue
				}
				if p.distXy(x, y).f0 <= r {
					f(int(pi), p)
				}
			}
		}
	}
}

// ray: t in [0,1] of the first hit of segment a->b with terrain or a prism.
func (w *World) ray(a, b V3) *float64 {
	if t, ok := w.rayT(a, b); ok {
		return &t
	}
	return nil
}

// rayT is ray without the pointer result.
func (w *World) rayT(a, b V3) (float64, bool) {
	if w.solid(a[0], a[1], a[2]) {
		return 0.0, true
	}
	if w.cnx == 0 {
		// flat ground at z = 0
		if b[2] < 0.0 && a[2] >= 0.0 {
			return fclamp(a[2]/(a[2]-b[2]), 0.0, 1.0), true
		}
		return 0, false
	}
	d := V3{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	// clip xy to grid rect
	x0, x1 := w.gx0, w.gx0+float64(w.cnx)
	y0, y1 := w.gy0, w.gy0+float64(w.cny)
	ts := 0.0
	te := 1.0
	for k := 0; k < 2; k++ {
		var o, dk, lo, hi float64
		if k == 0 {
			o, dk, lo, hi = a[0], d[0], x0, x1
		} else {
			o, dk, lo, hi = a[1], d[1], y0, y1
		}
		if math.Abs(dk) < 1e-12 {
			if o < lo || o >= hi {
				return 0, false
			}
		} else {
			t0 := (lo - o) / dk
			t1 := (hi - o) / dk
			if t0 > t1 {
				t0, t1 = t1, t0
			}
			ts = fmax(ts, t0)
			te = fmin(te, t1)
		}
	}
	if ts > te {
		return 0, false
	}
	px := a[0] + d[0]*ts
	py := a[1] + d[1]*ts
	i := iclamp(f2i64(math.Floor(px-w.gx0)), 0, int64(w.cnx)-1)
	j := iclamp(f2i64(math.Floor(py-w.gy0)), 0, int64(w.cny)-1)
	var sx, sy int64
	var tdx, tmx, tdy, tmy float64
	if d[0] > 1e-12 {
		sx, tdx, tmx = 1, 1.0/d[0], (w.gx0+float64(i+1)-a[0])/d[0]
	} else if d[0] < -1e-12 {
		sx, tdx, tmx = -1, -1.0/d[0], (w.gx0+float64(i)-a[0])/d[0]
	} else {
		sx, tdx, tmx = 0, math.Inf(1), math.Inf(1)
	}
	if d[1] > 1e-12 {
		sy, tdy, tmy = 1, 1.0/d[1], (w.gy0+float64(j+1)-a[1])/d[1]
	} else if d[1] < -1e-12 {
		sy, tdy, tmy = -1, -1.0/d[1], (w.gy0+float64(j)-a[1])/d[1]
	} else {
		sy, tdy, tmy = 0, math.Inf(1), math.Inf(1)
	}
	t0 := ts
	for {
		t1 := fmin(fmin(tmx, tmy), te)
		c := int(j)*w.cnx + int(i)
		za := a[2] + d[2]*t0
		zb := a[2] + d[2]*t1
		zmin := fmin(za, zb)
		if t, ok := w.cellHit(c, int(i), int(j), a, d, t0, t1, zmin); ok {
			return fclamp(t, 0.0, 1.0), true
		}
		if t1 >= te {
			return 0, false
		}
		if tmx < tmy {
			i += sx
			tmx += tdx
			if i < 0 || i >= int64(w.cnx) {
				return 0, false
			}
		} else {
			j += sy
			tmy += tdy
			if j < 0 || j >= int64(w.cny) {
				return 0, false
			}
		}
		t0 = t1
	}
}

func (w *World) cellHit(c, i, j int, a, d V3, t0, t1, zmin float64) (float64, bool) {
	best := math.Inf(1)
	// terrain
	if zmin <= float64(w.cellHmax[c])+1e-6 {
		k := j*w.nx + i
		h00 := float64(w.h[k])
		h10 := float64(w.h[k+1])
		h01 := float64(w.h[k+w.nx])
		h11 := float64(w.h[k+w.nx+1])
		ua := a[0] - (w.gx0 + float64(i))
		va := a[1] - (w.gy0 + float64(j))
		bu, bv, buv := h10-h00, h01-h00, h00-h10-h01+h11
		// f(t) = h(u(t), v(t)) - z(t) = A + B t + C t^2
		ca := h00 + bu*ua + bv*va + buv*ua*va - a[2]
		cb := bu*d[0] + bv*d[1] + buv*(ua*d[1]+va*d[0]) - d[2]
		cc := buv * d[0] * d[1]
		fv := func(t float64) float64 { return ca + cb*t + cc*t*t }
		if fv(t0) >= 0.0 {
			best = t0
		} else {
			tr := math.Inf(1)
			if math.Abs(cc) < 1e-12 {
				if math.Abs(cb) > 1e-15 {
					t := -ca / cb
					if t >= t0 && t <= t1 {
						tr = t
					}
				}
			} else {
				disc := cb*cb - 4.0*cc*ca
				if disc >= 0.0 {
					sq := math.Sqrt(disc)
					q := -0.5 * (cb + signum(cb)*sq)
					r1 := q / cc
					r2 := r1
					if q != 0.0 {
						r2 = ca / q
					}
					for _, t := range [2]float64{r1, r2} {
						if t >= t0 && t <= t1 && t < tr {
							tr = t
						}
					}
				}
			}
			if math.IsInf(tr, 0) && fv(t1) >= 0.0 {
				tr = t1
			}
			best = tr
		}
	}
	// prisms
	if zmin < float64(w.cellZtop[c])+1e-6 {
		es := w.cellEntries(c)
		for k := range es {
			e := &es[k]
			// walls
			for _, ed := range w.edges[e.e0 : e.e0+uint32(e.en)] {
				ex := ed[2] - ed[0]
				ey := ed[3] - ed[1]
				den := d[0]*ey - d[1]*ex
				if math.Abs(den) < 1e-14 {
					continue
				}
				wx := ed[0] - a[0]
				wy := ed[1] - a[1]
				t := (wx*ey - wy*ex) / den
				if t < t0-1e-9 || t > t1+1e-9 || t >= best {
					continue
				}
				s := (wx*d[1] - wy*d[0]) / den
				if !(s >= -1e-9 && s <= 1.0+1e-9) {
					continue
				}
				z := a[2] + d[2]*t
				if z >= e.base && z < e.top {
					best = t
				}
			}
			// caps
			if d[2] != 0.0 {
				hh := e.base
				if d[2] < 0.0 {
					hh = e.top
				}
				t := (hh - a[2]) / d[2]
				if t >= t0-1e-9 && t <= t1+1e-9 && t < best {
					x := a[0] + d[0]*t
					y := a[1] + d[1]*t
					if w.insideLocal(c, e, x, y) {
						best = t
					}
				}
			}
		}
	}
	if isFinite(best) {
		return best, true
	}
	return 0, false
}

// sightNoTrees: no terrain or prism on the segment.
func (w *World) sightNoTrees(a, b V3) bool {
	_, hit := w.rayT(a, b)
	return !hit
}

// sight: buildings, terrain and trunks block; canopy blocks when transmittance < canopyBlockT.
func (w *World) sight(a, b V3) bool {
	if _, hit := w.rayT(a, b); hit {
		return false
	}
	trunk, l := w.treesOn(a, b, true, w.canopyBlockLen())
	return !trunk && !w.canopyBlocks(l)
}

// sightNear is sight with canopy ignored: buildings, terrain and trunks block.
func (w *World) sightNear(a, b V3) bool {
	if _, hit := w.rayT(a, b); hit {
		return false
	}
	trunk, _ := w.treesOn(a, b, true, math.Inf(1))
	return !trunk
}

// trunkBlocks: any tree trunk on the segment.
func (w *World) trunkBlocks(a, b V3) bool {
	trunk, _ := w.treesOn(a, b, true, math.Inf(1))
	return trunk
}

// canopyLen: metres of the segment inside tree crowns (ellipsoids, summed over overlapping crowns).
func (w *World) canopyLen(a, b V3) float64 {
	_, l := w.treesOn(a, b, false, math.Inf(1))
	return l
}

// canopyTransmittance: transmittance through len metres of canopy.
func (w *World) canopyTransmittance(len_ float64) float64 {
	return math.Exp(-w.canopyDensity * len_)
}

func (w *World) canopyBlocks(len_ float64) bool {
	return w.canopyTransmittance(len_) < w.canopyBlockT
}

// canopyBlockLen: canopy length at which sight is lost.
func (w *World) canopyBlockLen() float64 {
	if w.canopyDensity <= 0.0 || w.canopyBlockT <= 0.0 {
		return math.Inf(1)
	}
	return -math.Log(w.canopyBlockT) / w.canopyDensity
}

// treesOn: crown of a tree is an ellipsoid centred at (crownX, crownY, z + (crownBase + height)/2),
// horizontal radius crownR, vertical half-axis (height - crownBase)/2. Trunk: vertical cylinder at
// (x, y), radius trunkR, from z to z + crownBase. Returns (trunk hit, canopy length); stops early
// once canopy length exceeds stopLen.
func (w *World) treesOn(a, b V3, trunks bool, stopLen float64) (bool, float64) {
	if len(w.trees) == 0 || w.tnx == 0 {
		return false, 0.0
	}
	d := V3{b[0] - a[0], b[1] - a[1], b[2] - a[2]}
	segLen := math.Sqrt(d[0]*d[0] + d[1]*d[1] + d[2]*d[2])
	inv := 1.0 / worldTreeCell
	fx := (a[0] - w.gx0) * inv
	fy := (a[1] - w.gy0) * inv
	dx := d[0] * inv
	dy := d[1] * inv
	nX := int64(w.tnx)
	nY := int64(w.tny)
	i := f2i64(math.Floor(fx))
	j := f2i64(math.Floor(fy))
	var sx, sy int64
	var tdx, tmx, tdy, tmy float64
	if dx > 1e-12 {
		sx, tdx, tmx = 1, 1.0/dx, (float64(i+1)-fx)/dx
	} else if dx < -1e-12 {
		sx, tdx, tmx = -1, -1.0/dx, (float64(i)-fx)/dx
	} else {
		sx, tdx, tmx = 0, math.Inf(1), math.Inf(1)
	}
	if dy > 1e-12 {
		sy, tdy, tmy = 1, 1.0/dy, (float64(j+1)-fy)/dy
	} else if dy < -1e-12 {
		sy, tdy, tmy = -1, -1.0/dy, (float64(j)-fy)/dy
	} else {
		sy, tdy, tmy = 0, math.Inf(1), math.Inf(1)
	}
	t0 := 0.0
	l := 0.0
	trunkHit := false
	for {
		t1 := fmin(fmin(tmx, tmy), 1.0)
		if i >= 0 && j >= 0 && i < nX && j < nY {
			c := int(j*nX + i)
			for _, ti := range w.treeIdx[w.treeStart[c]:w.treeStart[c+1]] {
				t := &w.trees[ti]
				// crown, clipped to this cell's t-range so each tree is counted once
				rz := 0.5 * (t.height - t.crownBase)
				if t.crownR > 0.0 && rz > 0.0 {
					cz := t.z + 0.5*(t.crownBase+t.height)
					ox := (a[0] - t.crownX) / t.crownR
					oy := (a[1] - t.crownY) / t.crownR
					oz := (a[2] - cz) / rz
					ux := d[0] / t.crownR
					uy := d[1] / t.crownR
					uz := d[2] / rz
					qa := ux*ux + uy*uy + uz*uz
					qb := 2.0 * (ox*ux + oy*uy + oz*uz)
					qc := ox*ox + oy*oy + oz*oz - 1.0
					if qa > 0.0 {
						disc := qb*qb - 4.0*qa*qc
						if disc > 0.0 {
							sq := math.Sqrt(disc)
							ta := fmax((-qb-sq)/(2.0*qa), t0)
							tb := fmin((-qb+sq)/(2.0*qa), t1)
							if tb > ta {
								l += (tb - ta) * segLen
							}
						}
					}
				}
				if trunks && !trunkHit && t.trunkR > 0.0 && t.crownBase > 0.0 {
					ox := a[0] - t.x
					oy := a[1] - t.y
					qa := d[0]*d[0] + d[1]*d[1]
					qb := 2.0 * (ox*d[0] + oy*d[1])
					qc := ox*ox + oy*oy - t.trunkR*t.trunkR
					var ta, tb float64
					if qa < 1e-12 {
						if qc <= 0.0 {
							ta, tb = 0.0, 1.0
						} else {
							ta, tb = 1.0, 0.0
						}
					} else {
						disc := qb*qb - 4.0*qa*qc
						if disc < 0.0 {
							ta, tb = 1.0, 0.0
						} else {
							sq := math.Sqrt(disc)
							ta, tb = (-qb-sq)/(2.0*qa), (-qb+sq)/(2.0*qa)
						}
					}
					ta = fmax(ta, 0.0)
					tb = fmin(tb, 1.0)
					if tb >= ta {
						za := a[2] + d[2]*ta
						zb := a[2] + d[2]*tb
						lo, hi := fmin(za, zb), fmax(za, zb)
						if hi >= t.z && lo <= t.z+t.crownBase {
							trunkHit = true
						}
					}
				}
			}
			if trunkHit || l > stopLen {
				return trunkHit, l
			}
		}
		if t1 >= 1.0 {
			break
		}
		if tmx < tmy {
			i += sx
			tmx += tdx
			if (sx > 0 && i >= nX) || (sx < 0 && i < 0) {
				break
			}
		} else {
			j += sy
			tmy += tdy
			if (sy > 0 && j >= nY) || (sy < 0 && j < 0) {
				break
			}
		}
		t0 = t1
	}
	return trunkHit, l
}

// Shared holds the map and its navigation graph.
type Shared struct {
	world World
	nav   Nav
}

// sharedLoad never fails: it falls back to worldEmpty().
func sharedLoad(mapDir string, cfg *Config) *Shared {
	world := func() (w World) {
		defer func() {
			if recover() != nil {
				logf("world load panicked")
				w = worldEmpty()
			}
		}()
		wl, err := worldLoad(mapDir)
		if err != nil {
			logf("world load failed: %v", err)
			return worldEmpty()
		}
		return wl
	}()
	nav := func() (n Nav) {
		defer func() {
			if recover() != nil {
				logf("nav build panicked, using empty world")
				e := worldEmpty()
				n = navBuild(&e, &cfg.Mech)
			}
		}()
		return navBuild(&world, &cfg.Mech)
	}()
	return &Shared{world: world, nav: nav}
}
