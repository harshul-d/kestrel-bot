package pilot

import (
	"math"
	"slices"
)

// Grid navigation graph over standable surfaces.

// navCELL: cell size of the nav grid in metres.
const navCELL float64 = 2.0

// navMapRect: map rectangle [minX, minY, maxX, maxY] used when no other rect is given.
var navMapRect = [4]float64{-512.38, -333.09, 555.08, 388.64}

const navEdgeWalk uint8 = 0
const navEdgeDrop uint8 = 1
const navEdgeJet uint8 = 2

const navINF uint32 = math.MaxUint32
const navSAMPLE float64 = 1.0 // gap sample spacing; CELL must be an integer multiple
const navK int = 2            // CELL / SAMPLE
const navLONG int32 = 3       // reach of drop and jet edges, in cells
const navLOOKAHEAD int = 12
const navPackCostBits uint32 = 12

var navRadsPerDeg = func() float64 { p := float64(math.Pi); return p / 180.0 }()

type Gap struct {
	s   float32
	c   float32
	ter bool
}

// Edge is a directed edge. In the reverse list, to is the source node.
type Edge struct {
	to uint32
	// decimetres of equivalent walking distance
	cost uint16
	// energy needed (jet edges)
	energy uint8
	kind   uint8
}

type Nav struct {
	ox  float64
	oy  float64
	cnx int
	cny int
	// gap samples
	snx       int
	sny       int
	pad       int
	gapFirst  []uint32
	gaps      []Gap
	cellFirst []uint32
	nodeCell  []uint32
	nodeZ     []float32
	nodeRoof  []bool
	fwdFirst  []uint32
	fwd       []Edge
	revFirst  []uint32
	rev       []Edge
	// packed reverse walk/drop edges (from << navPackCostBits | cost) and reverse jet edges
	rpFirst   []uint32
	rp        []uint32
	rjFirst   []uint32
	rj        []Edge
	maxCost   uint32
	walkSpeed float64
	stepUp    float64
	height    float64
}

// Field is a distance-to-goal field over the nav graph.
type Field struct {
	goal     V3
	goalNode *uint32
	d        []uint32
	jet      float64
}

func (f *Field) goalFn() V3 {
	return f.goal
}

func (f *Field) goalNodeFn() *uint32 {
	return f.goalNode
}

// jetLimit: max jet energy this field allows per jet edge (negative: no jet edges).
func (f *Field) jetLimit() float64 {
	return f.jet
}

// nodeDist: distance in metres from node i to the goal.
func (f *Field) nodeDist(i uint32) *float64 {
	if int(i) < len(f.d) {
		if v := f.d[i]; v != navINF {
			r := float64(v) * 0.1
			return &r
		}
	}
	return nil
}

// Step is one step of guidance from Nav.step.
type Step struct {
	// unit horizontal direction (zero at the goal)
	dir V2
	// point the direction aims at (feet position)
	target V3
	// navEdgeWalk, navEdgeDrop or navEdgeJet: what the next move is
	kind uint8
	// remaining distance in metres
	dist float64
	// energy the next edge needs (jet), else 0
	energy float64
}

func navHyp(a, b V3) float64 {
	return math.Hypot(a[0]-b[0], a[1]-b[1])
}

// navJetCost: (time s, energy) for jump+jet to rise dz then cross dh horizontally.
func navJetCost(m *MechCfg, dz, dh float64) (float64, float64, bool) {
	dt := 0.05
	z, vz, t, e := 0.0, fmax(m.JumpSpeed, 0.0), 0.0, 0.0
	need := dz + 0.5
	for z < need {
		vz = fmin(vz+m.JetAccel*dt, m.MaxRiseSpeed)
		vz -= m.Gravity * dt
		z += vz * dt
		t += dt
		e += m.JetDrain * dt
		if t > 30.0 || (vz <= 0.0 && t > 1.0) {
			return 0, 0, false
		}
	}
	th := 2.0 * math.Sqrt(fmax(dh, 0.5)/fmax(m.AirAccel, 0.5))
	time := t + 0.6*th + 0.2
	energy := e + m.JetDrain*0.4*th
	return time, energy, true
}

func navFmax32(a, b float32) float32 {
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

func navFmin32(a, b float32) float32 {
	if a != a {
		return b
	}
	if b != b {
		return a
	}
	if b < a {
		return b
	}
	return a
}

func navAbs32(a float32) float32 {
	return math.Float32frombits(math.Float32bits(a) &^ (1 << 31))
}

// navF2u16 is `x as u16` (saturating, NaN -> 0).
func navF2u16(x float64) uint16 {
	switch {
	case math.IsNaN(x) || x <= 0:
		return 0
	case x >= 65535:
		return 65535
	}
	return uint16(x)
}

func navBuild(world *World, mech *MechCfg) Nav {
	b := &world.bounds
	var rect [4]float64
	if b.maxX-b.minX < 1.0e5 && b.maxY-b.minY < 1.0e5 && b.maxX > b.minX && b.maxY > b.minY {
		rect = [4]float64{b.minX, b.minY, b.maxX, b.maxY}
	} else {
		rect = navMapRect
	}
	return navBuildRect(world, mech, rect)
}

func navBuildRect(world *World, mech *MechCfg, rect [4]float64) Nav {
	rad := fmax(mech.Radius, 0.5) + 0.5
	height := fmax(mech.Height, 1.0)
	stepUp := fmax(mech.StepUp, 0.0)
	tan := math.Tan(fclamp(mech.MaxSlopeDeg, 0.0, 89.0) * navRadsPerDeg)
	pad := f2u(math.Ceil(rad)) + 1
	ox := math.Ceil((rect[0]+rad)/navCELL) * navCELL
	oy := math.Ceil((rect[1]+rad)/navCELL) * navCELL
	cnx := f2u(fmax(math.Floor((rect[2]-rad-ox)/navCELL), 0.0)) + 1
	cny := f2u(fmax(math.Floor((rect[3]-rad-oy)/navCELL), 0.0)) + 1
	sx0 := ox - float64(pad)*navSAMPLE
	sy0 := oy - float64(pad)*navSAMPLE
	snx := (cnx-1)*navK + 2*pad + 1
	sny := (cny-1)*navK + 2*pad + 1

	// 1. free gaps per sample column
	gapFirst := make([]uint32, 0, snx*sny+1)
	gaps := make([]Gap, 0, snx*sny+snx*sny/8)
	for j := 0; j < sny; j++ {
		for i := 0; i < snx; i++ {
			gapFirst = append(gapFirst, uint32(len(gaps)))
			x := sx0 + float64(i)*navSAMPLE
			y := sy0 + float64(j)*navSAMPLE
			if x < rect[0] || x > rect[2] || y < rect[1] || y > rect[3] {
				continue
			}
			t := world.terrainZ(x, y)
			start := len(gaps)
			z := 1.0e4
			for it := 0; it < 16; it++ {
				s := world.floorBelow(x, y, z)
				if !isFinite(s) {
					break
				}
				ter := s <= t+1e-3
				if !world.solid(x, y, s+0.05) {
					c := world.ceilingAbove(x, y, s)
					gaps = append(gaps, Gap{s: float32(s), c: float32(fmin(c, 1.0e6)), ter: ter})
				}
				if ter {
					break
				}
				z = s - 0.02
			}
			slices.Reverse(gaps[start:]) // ascending s
		}
	}
	gapFirst = append(gapFirst, uint32(len(gaps)))

	nav := Nav{
		ox: ox, oy: oy, cnx: cnx, cny: cny, snx: snx, sny: sny, pad: pad,
		gapFirst: gapFirst, gaps: gaps,
		maxCost:   1,
		walkSpeed: fmax(mech.WalkSpeed, 0.1),
		stepUp:    stepUp, height: height,
	}

	// disk of sample offsets
	type diskT struct {
		di, dj int32
		d      float32
	}
	rI := f2i32(math.Ceil(rad))
	var disk []diskT
	for dj := -rI; dj <= rI; dj++ {
		for di := -rI; di <= rI; di++ {
			d := math.Sqrt(float64(di*di+dj*dj)) * navSAMPLE
			if d <= rad+1e-9 {
				disk = append(disk, diskT{di, dj, float32(d)})
			}
		}
	}
	stepF, hF, tanF := float32(stepUp), float32(height), float32(tan)

	// 2. nodes
	cf := make([]uint32, 0, cnx*cny+1)
	var ncell []uint32
	var nz []float32
	var nroof []bool
	for cj := 0; cj < cny; cj++ {
		for ci := 0; ci < cnx; ci++ {
			cf = append(cf, uint32(len(nz)))
			si, sj := ci*navK+pad, cj*navK+pad
			cg := nav.col(si, sj)
			for _, g := range cg {
				z := g.s
				if g.c-z < hF {
					continue
				}
				ok := true
				for _, dk := range disk {
					q := nav.col(int(int32(si)+dk.di), int(int32(sj)+dk.dj))
					found := false
					for _, h := range q {
						if h.c >= z+hF-0.01 && (h.s <= z+stepF || (h.ter && h.s <= z+stepF+dk.d*tanF)) {
							found = true
							break
						}
					}
					if !found {
						ok = false
						break
					}
				}
				if ok {
					ncell = append(ncell, uint32(cj*cnx+ci))
					nz = append(nz, z)
					nroof = append(nroof, !g.ter)
				}
			}
		}
	}
	cf = append(cf, uint32(len(nz)))
	nav.cellFirst = cf
	nav.nodeCell = ncell
	nav.nodeZ = nz
	nav.nodeRoof = nroof
	n := len(nav.nodeZ)

	// 3. edges
	type srcEdge struct {
		a uint32
		e Edge
	}
	edges := make([]srcEdge, 0, n*9)
	var tmp []Edge
	for a := 0; a < n; a++ {
		tmp = tmp[:0]
		ci, cj := nav.cellIj(uint32(a))
		za := nav.nodeZ[a]
		saI, saJ := ci*navK+nav.pad, cj*navK+nav.pad
		// walk edges
		for dj := int32(-1); dj <= 1; dj++ {
			for di := int32(-1); di <= 1; di++ {
				if di == 0 && dj == 0 {
					continue
				}
				bi, bj := int32(ci)+di, int32(cj)+dj
				if bi < 0 || bj < 0 || bi >= int32(cnx) || bj >= int32(cny) {
					continue
				}
				z := za
				ok := true
				for k := int32(1); k <= int32(navK); k++ {
					q := nav.col(int(int32(saI)+di*k), int(int32(saJ)+dj*k))
					have := false
					var m float32
					for _, h := range q {
						if h.s <= z+stepF+1e-3 && h.c >= h.s+hF {
							if have {
								m = navFmax32(m, h.s)
							} else {
								m = h.s
								have = true
							}
						}
					}
					if have {
						z = m
					} else {
						ok = false
						break
					}
				}
				if !ok {
					continue
				}
				bcell := int(bj)*cnx + int(bi)
				l := navCELL * math.Sqrt(float64(di*di+dj*dj))
				lo, hi := nav.cellNodes(bcell)
				for b := lo; b < hi; b++ {
					zb := nav.nodeZ[b]
					if navAbs32(zb-z) < 0.05 {
						rise := float64(zb - za)
						if rise > fmax(stepUp, l*tan)+1e-6 {
							continue
						}
						tmp = append(tmp, Edge{to: b, cost: navF2u16(math.Round(l * 10.0)), energy: 0, kind: navEdgeWalk})
					}
				}
			}
		}
		// long edges: drops and jets
		// lowest ceiling over the body column at a (jet room)
		jetColTop := float32(math.Inf(1))
		for _, dk := range disk {
			if dk.d > float32(mech.Radius)+0.01 {
				continue
			}
			q := nav.col(int(int32(saI)+dk.di), int(int32(saJ)+dk.dj))
			c := float32(0.0)
			for _, h := range q {
				if h.c >= za+hF-0.01 && (h.s <= za+stepF || (h.ter && h.s <= za+stepF+dk.d*tanF)) {
					c = navFmax32(c, h.c)
				}
			}
			jetColTop = navFmin32(jetColTop, c)
		}
		for dj := -navLONG; dj <= navLONG; dj++ {
			for di := -navLONG; di <= navLONG; di++ {
				if di == 0 && dj == 0 {
					continue
				}
				bi, bj := int32(ci)+di, int32(cj)+dj
				if bi < 0 || bj < 0 || bi >= int32(cnx) || bj >= int32(cny) {
					continue
				}
				dh := navCELL * math.Sqrt(float64(di*di+dj*dj))
				if dh > float64(navLONG)*navCELL+0.1 {
					continue
				}
				bcell := int(bj)*cnx + int(bi)
				nlo, nhi := nav.cellNodes(bcell)
			nodes:
				for b := nlo; b < nhi; b++ {
					for k := range tmp {
						if tmp[k].to == b {
							continue nodes
						}
					}
					zb := nav.nodeZ[b]
					ns := max(f2i32(math.Ceil(dh/(0.5*navSAMPLE))), 2)
					if zb < za-stepF-0.3 {
						// drop: cross at roof height, then fall at b
						lo := za + 0.3
						ok := true
						for k := int32(1); k < ns; k++ {
							f := float64(k) / float64(ns)
							si := f2u(math.Round(float64(saI) + float64(di)*float64(navK)*f))
							sj := f2u(math.Round(float64(saJ) + float64(dj)*float64(navK)*f))
							q := nav.col(si, sj)
							found := false
							for _, h := range q {
								if h.s <= lo && h.c >= za+hF {
									found = true
									break
								}
							}
							if !found {
								ok = false
								break
							}
						}
						if ok {
							bg := nav.col(int(bi)*navK+nav.pad, int(bj)*navK+nav.pad)
							found := false
							for _, h := range bg {
								if navAbs32(h.s-zb) < 0.01 && h.c >= za+hF {
									found = true
									break
								}
							}
							if found {
								fall := math.Sqrt(2.0 * float64(za-zb) / fmax(mech.Gravity, 1.0))
								cost := dh + 0.3*fall*nav.walkSpeed
								tmp = append(tmp, Edge{to: b, cost: navF2u16(fmin(math.Round(cost*10.0), 60000.0)), energy: 0, kind: navEdgeDrop})
							}
						}
					} else if zb > za+stepF {
						dz := float64(zb - za)
						if jetColTop < zb+hF+0.3 {
							continue
						}
						time, energy, ok := navJetCost(mech, dz, dh)
						if !ok {
							continue
						}
						if energy > mech.Energy || energy > 255.0 {
							continue
						}
						lo := zb + 0.5
						ok = true
						for k := int32(1); k < ns; k++ {
							f := float64(k) / float64(ns)
							si := f2u(math.Round(float64(saI) + float64(di)*float64(navK)*f))
							sj := f2u(math.Round(float64(saJ) + float64(dj)*float64(navK)*f))
							q := nav.col(si, sj)
							found := false
							for _, h := range q {
								if h.s <= lo && h.c >= zb+hF {
									found = true
									break
								}
							}
							if !found {
								ok = false
								break
							}
						}
						if ok {
							cost := time * nav.walkSpeed
							tmp = append(tmp, Edge{to: b, cost: navF2u16(fmin(math.Round(cost*10.0), 60000.0)), energy: f2u8(math.Ceil(energy)), kind: navEdgeJet})
						}
					}
				}
			}
		}
		for _, e := range tmp {
			edges = append(edges, srcEdge{uint32(a), e})
		}
	}
	// CSR forward and reverse
	fwdFirst := make([]uint32, n+1)
	revFirst := make([]uint32, n+1)
	for _, se := range edges {
		fwdFirst[se.a+1]++
		revFirst[se.e.to+1]++
	}
	for i := 0; i < n; i++ {
		fwdFirst[i+1] += fwdFirst[i]
		revFirst[i+1] += revFirst[i]
	}
	fwd := make([]Edge, len(edges))
	rev := make([]Edge, len(edges))
	fp := slices.Clone(fwdFirst)
	rpos := slices.Clone(revFirst)
	maxCost := uint32(1)
	for _, se := range edges {
		a, e := se.a, se.e
		fwd[fp[a]] = e
		fp[a]++
		re := e
		re.to = a
		rev[rpos[e.to]] = re
		rpos[e.to]++
		maxCost = max(maxCost, uint32(e.cost))
	}
	nav.fwdFirst = fwdFirst
	nav.fwd = fwd
	nav.revFirst = revFirst
	nav.rev = rev
	nav.maxCost = maxCost
	// packed reverse lists for the field search
	rpFirst := make([]uint32, 0, n+1)
	rjFirst := make([]uint32, 0, n+1)
	rp := make([]uint32, 0, len(nav.rev))
	var rj []Edge
	packable := uint64(n) < (uint64(1) << (32 - navPackCostBits))
	for v := 0; v < n; v++ {
		rpFirst = append(rpFirst, uint32(len(rp)))
		rjFirst = append(rjFirst, uint32(len(rj)))
		for _, e := range nav.rev[nav.revFirst[v]:nav.revFirst[v+1]] {
			if e.kind != navEdgeJet && packable && uint32(e.cost) < (1<<navPackCostBits) {
				rp = append(rp, e.to<<navPackCostBits|uint32(e.cost))
			} else {
				rj = append(rj, e)
			}
		}
	}
	rpFirst = append(rpFirst, uint32(len(rp)))
	rjFirst = append(rjFirst, uint32(len(rj)))
	nav.rpFirst = rpFirst
	nav.rp = rp
	nav.rjFirst = rjFirst
	nav.rj = rj
	return nav
}

func (nav *Nav) col(i, j int) []Gap {
	if uint(i) >= uint(nav.snx) || uint(j) >= uint(nav.sny) {
		return nil
	}
	k := j*nav.snx + i
	return nav.gaps[nav.gapFirst[k]:nav.gapFirst[k+1]]
}

func (nav *Nav) cellIj(node uint32) (int, int) {
	c := int(nav.nodeCell[node])
	return c % nav.cnx, c / nav.cnx
}

// cellNodes: node id range [lo, hi) of a cell.
func (nav *Nav) cellNodes(cell int) (uint32, uint32) {
	return nav.cellFirst[cell], nav.cellFirst[cell+1]
}

func (nav *Nav) nodeCount() int {
	return len(nav.nodeZ)
}

func (nav *Nav) edgeCount() int {
	return len(nav.fwd)
}

// nodePos: feet position of node i.
func (nav *Nav) nodePos(i uint32) V3 {
	ci, cj := nav.cellIj(i)
	return V3{nav.ox + float64(ci)*navCELL, nav.oy + float64(cj)*navCELL, float64(nav.nodeZ[i])}
}

// nodeRoofFn: true when node i stands on a prism top (roof, bridge deck, stadium).
func (nav *Nav) nodeRoofFn(i uint32) bool {
	return nav.nodeRoof[i]
}

func (nav *Nav) edges(i uint32) []Edge {
	return nav.fwd[nav.fwdFirst[i]:nav.fwdFirst[i+1]]
}

func (nav *Nav) edgesIn(i uint32) []Edge {
	return nav.rev[nav.revFirst[i]:nav.revFirst[i+1]]
}

// grid: grid size (cells x, cells y) and the centre of cell (0, 0).
func (nav *Nav) grid() struct {
	f0 int
	f1 int
	f2 V2
} {
	return struct {
		f0 int
		f1 int
		f2 V2
	}{nav.cnx, nav.cny, V2{nav.ox, nav.oy}}
}

// cellNodeIds: node id range [lo, hi) of cell (ci, cj).
func (nav *Nav) cellNodeIds(ci, cj int) (uint32, uint32) {
	if uint(ci) >= uint(nav.cnx) || uint(cj) >= uint(nav.cny) {
		return 0, 0
	}
	return nav.cellNodes(cj*nav.cnx + ci)
}

// snapNode: nearest standable node to p (feet), preferring the surface under p.
func (nav *Nav) snapNode(p V3) *uint32 {
	if nd, ok := nav.snapNodeT(p); ok {
		return &nd
	}
	return nil
}

// snapNodeT is snapNode without the pointer result.
func (nav *Nav) snapNodeT(p V3) (uint32, bool) {
	if !(isFinite(p[0]) && isFinite(p[1])) {
		return 0, false
	}
	pz := p[2]
	if !isFinite(pz) {
		pz = 1.0e4
	}
	fi := f2i64(fclamp(math.Round((p[0]-nav.ox)/navCELL), -1.0e6, 1.0e6))
	fj := f2i64(fclamp(math.Round((p[1]-nav.oy)/navCELL), -1.0e6, 1.0e6))
	have := false
	bestS := 0.0
	bestN := uint32(0)
	for r := int64(0); r <= 8; r++ {
		if have {
			rmin := (float64(r) - 1.0) * navCELL
			if rmin > 0.0 && rmin*rmin > bestS {
				break
			}
		}
		for dj := -r; dj <= r; dj++ {
			for di := -r; di <= r; di++ {
				adi, adj := di, dj
				if adi < 0 {
					adi = -adi
				}
				if adj < 0 {
					adj = -adj
				}
				if max(adi, adj) != r {
					continue
				}
				ci, cj := fi+di, fj+dj
				if ci < 0 || cj < 0 || ci >= int64(nav.cnx) || cj >= int64(nav.cny) {
					continue
				}
				lo, hi := nav.cellNodes(int(cj)*nav.cnx + int(ci))
				for nd := lo; nd < hi; nd++ {
					q := nav.nodePos(nd)
					h2 := powi(q[0]-p[0], 2) + powi(q[1]-p[1], 2)
					dz := pz - q[2]
					var pen float64
					if dz < -(nav.stepUp + 0.6) {
						pen = powi(dz*3.0, 2) + 100.0
					} else if dz > 0.0 {
						pen = powi(dz*0.3, 2)
					} else {
						pen = dz * dz
					}
					s := h2 + pen
					if !have || s < bestS {
						have = true
						bestS = s
						bestN = nd
					}
				}
			}
		}
	}
	return bestN, have
}

func (nav *Nav) snap(p V3) *V3 {
	if nd, ok := nav.snapNodeT(p); ok {
		q := nav.nodePos(nd)
		return &q
	}
	return nil
}

func (nav *Nav) field(goal V3) Field {
	return nav.fieldWith(goal, -1.0)
}

// fieldWith: field that may use jet edges needing at most maxJetEnergy each (negative: none).
func (nav *Nav) fieldWith(goal V3, maxJetEnergy float64) Field {
	return nav.fieldMulti([]struct {
		f0 V3
		f1 float64
	}{{goal, 0.0}}, maxJetEnergy)
}

// fieldMulti: field from several goals, each with a start cost in metres (for instance nearest of
// many items). goalFn() of the result is the first goal.
func (nav *Nav) fieldMulti(goals []struct {
	f0 V3
	f1 float64
}, maxJetEnergy float64) Field {
	return nav.fieldLimited(goals, maxJetEnergy, math.Inf(1))
}

// fieldLimited: fieldMulti that stops expanding past maxM metres (nodes beyond stay unreachable).
func (nav *Nav) fieldLimited(goals []struct {
	f0 V3
	f1 float64
}, maxJetEnergy, maxM float64) Field {
	var limit uint32
	if isFinite(maxM) {
		limit = f2u32(fmin(fmax(maxM, 0.0)*10.0, 4.0e9))
	} else {
		limit = math.MaxUint32 - 70000
	}
	n := nav.nodeCount()
	d := make([]uint32, n)
	for i := range d {
		d[i] = navINF
	}
	type seedT struct{ nd, c uint32 }
	seeds := make([]seedT, 0, len(goals))
	for _, g := range goals {
		if nd, ok := nav.snapNodeT(g.f0); ok {
			seeds = append(seeds, seedT{nd, f2u32(fmin(math.Round(fmax(g.f1, 0.0)*10.0), 1.0e9))})
		}
	}
	sortStable(seeds, func(a, b *seedT) int {
		switch {
		case a.c < b.c:
			return -1
		case a.c > b.c:
			return 1
		}
		return 0
	})
	goal := V3{}
	var goalNode *uint32
	if len(goals) > 0 {
		goal = goals[0].f0
		goalNode = nav.snapNode(goals[0].f0)
	}
	jl := maxJetEnergy
	bsz := int(nav.maxCost) + 1
	buckets := make([][]uint32, bsz)
	pending := 0
	sp := 0
	cur := uint32(0)
	if len(seeds) > 0 {
		cur = seeds[0].c
	}
	const mask = uint32(1)<<navPackCostBits - 1
	for {
		for sp < len(seeds) && uint64(seeds[sp].c) < uint64(cur)+uint64(bsz) {
			nd, c := seeds[sp].nd, seeds[sp].c
			if c < d[nd] {
				d[nd] = c
				buckets[int(c)%bsz] = append(buckets[int(c)%bsz], nd)
				pending++
			}
			sp++
		}
		if pending == 0 {
			if sp >= len(seeds) {
				break
			}
			cur = seeds[sp].c
			continue
		}
		if cur > limit {
			break
		}
		bi := int(cur) % bsz
		bk := buckets[bi]
		if len(bk) == 0 {
			cur++
			continue
		}
		v := bk[len(bk)-1]
		buckets[bi] = bk[:len(bk)-1]
		pending--
		if d[v] != cur {
			continue
		}
		for _, pe := range nav.rp[nav.rpFirst[v]:nav.rpFirst[v+1]] {
			nd := cur + (pe & mask)
			u := pe >> navPackCostBits
			if nd < d[u] {
				d[u] = nd
				buckets[int(nd)%bsz] = append(buckets[int(nd)%bsz], u)
				pending++
			}
		}
		rj := nav.rj[nav.rjFirst[v]:nav.rjFirst[v+1]]
		for k := range rj {
			e := &rj[k]
			if e.kind == navEdgeJet && float64(e.energy) > jl {
				continue
			}
			nd := cur + uint32(e.cost)
			u := e.to
			if nd < d[u] {
				d[u] = nd
				buckets[int(nd)%bsz] = append(buckets[int(nd)%bsz], e.to)
				pending++
			}
		}
	}
	return Field{goal: goal, goalNode: goalNode, d: d, jet: maxJetEnergy}
}

func (nav *Nav) dist(f *Field, p V3) *float64 {
	n, ok := nav.snapNodeT(p)
	if !ok {
		return nil
	}
	if int(n) >= len(f.d) {
		return nil
	}
	v := f.d[n]
	if v == navINF {
		return nil
	}
	r := float64(v)*0.1 + navHyp(p, nav.nodePos(n))
	return &r
}

func (nav *Nav) edgeOk(f *Field, e *Edge) bool {
	return e.kind != navEdgeJet || float64(e.energy) <= f.jet
}

func (nav *Nav) bestNext(f *Field, cur uint32) (Edge, bool) {
	dc := f.d[cur]
	have := false
	var bestS uint32
	var bestE Edge
	es := nav.edges(cur)
	for k := range es {
		e := &es[k]
		if !nav.edgeOk(f, e) {
			continue
		}
		dt := f.d[e.to]
		if dt == navINF || dt >= dc {
			continue
		}
		s := dt + uint32(e.cost)
		if !have || s < bestS {
			have = true
			bestS = s
			bestE = *e
		}
	}
	return bestE, have
}

// path: node path from p along the field descent (at most maxNodes nodes, starting at the snapped node).
func (nav *Nav) path(f *Field, p V3, maxNodes int) []V3 {
	var out []V3
	cur, ok := nav.snapNodeT(p)
	if !ok {
		return out
	}
	if int(cur) >= len(f.d) || f.d[cur] == navINF {
		return out
	}
	out = append(out, nav.nodePos(cur))
	for len(out) < maxNodes {
		e, ok := nav.bestNext(f, cur)
		if !ok {
			break
		}
		cur = e.to
		out = append(out, nav.nodePos(cur))
	}
	return out
}

// step: guidance from p: smoothed direction plus what kind of move comes next.
func (nav *Nav) step(f *Field, p V3) *Step {
	n, ok := nav.snapNodeT(p)
	if !ok {
		return nil
	}
	if int(n) >= len(f.d) {
		return nil
	}
	dn := f.d[n]
	if dn == navINF {
		return nil
	}
	dist := float64(dn)*0.1 + navHyp(p, nav.nodePos(n))
	mk := func(t V3, kind uint8, energy float64) *Step {
		dx, dy := t[0]-p[0], t[1]-p[1]
		l := math.Hypot(dx, dy)
		var dir V2
		if l < 0.3 {
			dir = V2{0.0, 0.0}
		} else {
			dir = V2{dx / l, dy / l}
		}
		return &Step{dir: dir, target: t, kind: kind, dist: dist, energy: energy}
	}
	if dn == 0 {
		g := f.goal
		var t V3
		if gn, ok := nav.snapNodeT(g); ok && gn == n {
			t = V3{g[0], g[1], float64(nav.nodeZ[n])}
		} else {
			t = nav.nodePos(n)
		}
		return mk(t, navEdgeWalk, 0.0)
	}
	var pathBuf [navLOOKAHEAD]uint32
	path := pathBuf[:0]
	cur := n
	firstKind := navEdgeWalk
	firstEnergy := 0.0
	for len(path) < navLOOKAHEAD {
		e, ok := nav.bestNext(f, cur)
		if !ok {
			break
		}
		if e.kind != navEdgeWalk {
			if len(path) == 0 {
				firstKind = e.kind
				firstEnergy = float64(e.energy)
				path = append(path, e.to)
			}
			break
		}
		path = append(path, e.to)
		cur = e.to
	}
	if len(path) == 0 {
		return mk(nav.nodePos(n), navEdgeWalk, 0.0)
	}
	if firstKind != navEdgeWalk {
		return mk(nav.nodePos(path[0]), firstKind, firstEnergy)
	}
	for k := len(path) - 1; k >= 1; k-- {
		q := nav.nodePos(path[k])
		if nav.clearFrom(n, p, q) {
			return mk(q, navEdgeWalk, 0.0)
		}
	}
	return mk(nav.nodePos(path[0]), navEdgeWalk, 0.0)
}

func (nav *Nav) dir(f *Field, p V3) *V2 {
	if s := nav.step(f, p); s != nil {
		d := s.dir
		return &d
	}
	return nil
}

func (nav *Nav) clear(a, b V3) bool {
	if n, ok := nav.snapNodeT(a); ok {
		return nav.clearFrom(n, a, b)
	}
	return false
}

func (nav *Nav) clearFrom(start uint32, a, b V3) bool {
	l := navHyp(a, b)
	ns := f2u(fmax(math.Ceil(l/0.5), 1.0))
	cur := start
	ci, cj := nav.cellIj(cur)
	for k := 1; k <= ns; k++ {
		f := float64(k) / float64(ns)
		x := a[0] + (b[0]-a[0])*f
		y := a[1] + (b[1]-a[1])*f
		nif := math.Round((x - nav.ox) / navCELL)
		njf := math.Round((y - nav.oy) / navCELL)
		if nif < 0.0 || njf < 0.0 || nif >= float64(nav.cnx) || njf >= float64(nav.cny) {
			return false
		}
		ni, nj := f2u(nif), f2u(njf)
		if ni == ci && nj == cj {
			continue
		}
		zc := nav.nodeZ[cur]
		have := false
		var bestDz float32
		var bestTo uint32
		es := nav.edges(cur)
		for q := range es {
			e := &es[q]
			if e.kind != navEdgeWalk {
				continue
			}
			ei, ej := nav.cellIj(e.to)
			if ei == ni && ej == nj {
				dz := navAbs32(nav.nodeZ[e.to] - zc)
				if !have || dz < bestDz {
					have = true
					bestDz = dz
					bestTo = e.to
				}
			}
		}
		if !have {
			return false
		}
		cur = bestTo
		ci = ni
		cj = nj
	}
	if isFinite(b[2]) {
		zc := float64(nav.nodeZ[cur])
		if math.Abs(zc-b[2]) > nav.stepUp+1.5 {
			return false
		}
	}
	return true
}
