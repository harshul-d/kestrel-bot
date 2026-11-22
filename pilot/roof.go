package pilot

import "math"

type RoofRegion struct {
	expo    float64
	keep    V3
	setback float64
	height  float64
	top     float64
}

type RoofRoofMap struct {
	regions    []RoofRegion
	nodeRegion map[uint32]uint32
}

func roofRoofMapBuild(world *World, nav *Nav, minNodes int, minSetback float64, height float64) RoofRoofMap {
	n := nav.nodeCount()
	comp := make([]uint32, n)
	for i := range comp {
		comp[i] = math.MaxUint32
	}
	var regions []RoofRegion
	nodeRegion := make(map[uint32]uint32)
	nextId := uint32(0)
	for s := uint32(0); s < uint32(n); s++ {
		if !nav.nodeRoofFn(s) || comp[s] != math.MaxUint32 {
			continue
		}
		id := nextId
		nextId += 1
		comp[s] = id
		stack := []uint32{s}
		var list []uint32
		for len(stack) > 0 {
			a := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			list = append(list, a)
			za := nav.nodePos(a)[2]
			for _, e := range nav.edges(a) {
				bn := e.to
				if e.kind != navEdgeWalk || int(bn) >= n || comp[bn] != math.MaxUint32 || !nav.nodeRoofFn(bn) {
					continue
				}
				if math.Abs(nav.nodePos(bn)[2]-za) > 1.3 {
					continue
				}
				comp[bn] = id
				stack = append(stack, bn)
			}
		}
		if len(list) < minNodes {
			continue
		}
		cnt := make([]int, 0, len(list))
		for _, a := range list {
			c := 0
			for _, e := range nav.edges(a) {
				if e.kind == navEdgeWalk && int(e.to) < n && comp[e.to] == id {
					c++
				}
			}
			cnt = append(cnt, c)
		}
		maxc := 0
		for k, c := range cnt {
			if k == 0 || c >= maxc {
				maxc = c
			}
		}
		var bnd []V3
		for k, a := range list {
			if cnt[k] < maxc {
				bnd = append(bnd, nav.nodePos(a))
			}
		}
		if len(bnd) == 0 {
			continue
		}
		type sbe struct {
			f0 float64
			f1 uint32
		}
		sb := make([]sbe, 0, len(list))
		for _, a := range list {
			q := nav.nodePos(a)
			m := math.Inf(1)
			for _, bp := range bnd {
				m = fmin(m, math.Hypot(q[0]-bp[0], q[1]-bp[1]))
			}
			sb = append(sb, sbe{m, a})
		}
		sortStable(sb, func(x, y *sbe) int { return totalCmp(y.f0, x.f0) })
		topSb := sb[0].f0
		if topSb < minSetback {
			continue
		}
		cut := fmax(topSb*0.5, minSetback)
		var cands []sbe
		for _, x := range sb {
			if x.f0 >= cut {
				cands = append(cands, x)
			}
		}
		stride := max(len(cands)/24, 1)
		var best *struct {
			f0 float64
			f1 float64
			f2 uint32
		}
		for k, c := range cands {
			m, a := c.f0, c.f1
			if k%stride != 0 {
				continue
			}
			e := roofExposure(world, nav.nodePos(a), height)
			s := e - 0.02*m
			if best == nil || s < best.f0 {
				best = &struct {
					f0 float64
					f1 float64
					f2 uint32
				}{s, m, a}
			}
		}
		if best == nil {
			continue
		}
		bm, ba := best.f1, best.f2
		keep := nav.nodePos(ba)
		ground := fin(world.terrainZ(keep[0], keep[1]), keep[2])
		ri := uint32(len(regions))
		for _, a := range list {
			nodeRegion[a] = ri
		}
		expo := roofExposure(world, keep, height)
		regions = append(regions, RoofRegion{expo: expo, keep: keep, setback: bm, height: keep[2] - ground, top: keep[2]})
	}
	return RoofRoofMap{regions: regions, nodeRegion: nodeRegion}
}

func (m *RoofRoofMap) regionOf(nav *Nav, p V3) *int {
	n := nav.snapNode(p)
	if n == nil {
		return nil
	}
	rv, ok := m.nodeRegion[*n]
	if !ok {
		return nil
	}
	r := int(rv)
	if p[2] > m.regions[r].top-2.0 {
		return ptr(r)
	}
	return nil
}

// roofExposure is the weighted count of ground and roof observers within 170 m that see the top of a mech standing at p.
func roofExposure(world *World, p V3, height float64) float64 {
	top := up(p, height*0.95)
	mid := up(p, height*0.5)
	e := 0.0
	for _, rw := range [4][2]float64{{50.0, 1.0}, {90.0, 0.8}, {130.0, 0.6}, {170.0, 0.4}} {
		r, w := rw[0], rw[1]
		for k := 0; k < 32; k++ {
			a := float64(k)*math.Pi/16.0 + r*0.01
			x := p[0] + r*math.Cos(a)
			y := p[1] + r*math.Sin(a)
			fz := world.floorBelow(x, y, 1.0e3)
			if !isFinite(fz) {
				continue
			}
			eye := V3{x, y, fz + 8.0}
			if world.ray(eye, mid) == nil {
				e += 2.0 * w
			} else if world.ray(eye, top) == nil {
				e += w
			}
		}
	}
	return e
}

type RoofFoe struct {
	id      int64
	pos     V3
	hpf     float64
	climber bool
	line    bool
}

// roofOptEq compares two optional indices by value.
func roofOptEq(x, y *int) bool {
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	return *x == *y
}

func (b *Brain) roofChoose(obs *Obs, zi *StrategyZoneInfo, rm *RoofRoofMap) *int {
	p := b.p
	z := &obs.Zone
	pos := obs.Me.Pos
	var best *struct {
		f0 float64
		f1 int
	}
	for i := range rm.regions {
		r := &rm.regions[i]
		if r.setback < p.roofMinSetback || r.height < p.roofMinH || r.height > p.roofMaxH {
			continue
		}
		banned := false
		for _, bn := range b.roofBan {
			if bn.f0 == i && obs.T < bn.f1 {
				banned = true
				break
			}
		}
		if banned {
			continue
		}
		dn := dist2v(r.keep, z.NextCenter)
		inner := z.NextRadius - dn - p.roofZoneMargin
		if inner < 0.0 || dist2v(r.keep, zi.curC) > zi.curR-p.roofZoneMargin {
			continue
		}
		d := dist2(r.keep, pos)
		s := 2.0*fmin(r.setback, 45.0) - p.roofWExpo*r.expo - p.roofWDist*d + 0.2*fmin(inner, 120.0) - 0.4*math.Abs(r.height-20.0)
		if b.roofSel != nil && *b.roofSel == i {
			s += 25.0
		}
		if best == nil || s > best.f0 {
			best = &struct {
				f0 float64
				f1 int
			}{s, i}
		}
	}
	if best == nil {
		return nil
	}
	return ptr(best.f1)
}

func (b *Brain) roofStepPoint(world *World, nav *Nav, rm *RoofRoofMap, ri int, tid int64, tp V3, t float64, muzzleH float64) *V3 {
	if rs := b.roofStep; rs != nil {
		if rs.f0 == tid && t-rs.f2 < 0.6 {
			return rs.f1
		}
	}
	r := &rm.regions[ri]
	keep := r.keep
	tc := up(tp, b.cfg.Mech.Height*0.5)
	b0 := math.Atan2(tp[1]-keep[1], tp[0]-keep[0])
	var best *struct {
		f0 float64
		f1 V3
	}
	maxd := r.setback + 12.0
	rays := 0
	for _, db := range [5]float64{0.0, 0.45, -0.45, 0.9, -0.9} {
		bb := b0 + db
		k := 0.0
		for k <= maxd && rays < 90 {
			q0 := V3{keep[0] + math.Cos(bb)*k, keep[1] + math.Sin(bb)*k, keep[2] + 1.0}
			k += 2.5
			n := nav.snapNode(q0)
			if n == nil {
				continue
			}
			if x, ok := rm.nodeRegion[*n]; !ok || int(x) != ri {
				continue
			}
			q := nav.nodePos(*n)
			if best != nil && dist2(q, keep) >= best.f0 {
				break
			}
			rays += 1
			if world.ray(up(q, muzzleH), tc) == nil {
				best = &struct {
					f0 float64
					f1 V3
				}{dist2(q, keep), q}
				break
			}
		}
	}
	var q *V3
	if best != nil {
		q = ptr(best.f1)
	}
	b.roofStep = &struct {
		f0 int64
		f1 *V3
		f2 float64
	}{tid, q, t}
	return q
}

func (b *Brain) roofDecide(obs *Obs, world *World, nav *Nav, zi *StrategyZoneInfo) {
	b.roofHold = false
	b.roofJet = false
	p := b.p
	if p.roofMode <= 0.0 {
		return
	}
	rm := b.roof
	if rm == nil {
		return
	}
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	hpf := frac(me.Hp, me.MaxHp)
	for i := range obs.Events {
		if obs.Events[i].kind == EventKindKill {
			b.kills += 1
		}
	}
	if float64(b.kills) < p.roofAfterKills || t < p.roofStartS || zi.urgent || b.held(t) {
		return
	}
	if b.mode == ModeLoot && b.roofOn == nil {
		if b.goal.kind == GoalKindNav {
			g := b.goal.navPos
			if dist(g, pos) < p.roofLootFirst {
				return
			}
		}
	}
	if b.roofSel == nil || t-b.roofSelT > 1.0 {
		prev := b.roofSel
		b.roofSel = b.roofChoose(obs, zi, rm)
		b.roofSelT = t
		if !roofOptEq(prev, b.roofSel) {
			b.roofStep = nil
			b.roofRest = false
		}
	}
	if b.roofSel == nil {
		return
	}
	ri := *b.roofSel
	r := &rm.regions[ri]
	ro := rm.regionOf(nav, pos)
	on := ro != nil && *ro == ri
	if on {
		b.roofOn = ptr(ri)
	} else {
		b.roofOn = nil
	}
	if on && b.roofFirstT < 0.0 {
		b.roofFirstT = t
	}

	center := up(pos, me.Stats.Height*0.5)
	var foes []RoofFoe
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.roofMemoryS {
			continue
		}
		tp := tr.posAt(t)
		nearRoof := dist2(tp, r.keep) < p.roofClimbR
		climber := nearRoof && t-tr.lastSeen < 1.5 && (tp[2] > r.top-4.0 || (tr.vel[2] > 2.0 && tp[2] > r.top-r.height*0.7))
		line := t-tr.lastSeen < 1.0 && len(foes) < 6 && world.sight(up(tp, b.cfg.Mech.EyeHeight), center)
		foes = append(foes, RoofFoe{id: tr.id, pos: tp, hpf: frac(tr.hp, tr.maxHp), climber: climber, line: line})
	}
	lines := 0
	nClimb := 0
	for i := range foes {
		if foes[i].line {
			lines++
		}
	}
	for i := range foes {
		if foes[i].climber {
			nClimb++
		}
	}
	nearestClimber := func() *RoofFoe {
		var c *RoofFoe
		for i := range foes {
			f := &foes[i]
			if !f.climber {
				continue
			}
			if c == nil || totalCmp(dist(f.pos, pos), dist(c.pos, pos)) < 0 {
				c = f
			}
		}
		return c
	}
	allFar := func() bool {
		for i := range foes {
			if !(dist(foes[i].pos, pos) > p.roofLootSafe) {
				return false
			}
		}
		return true
	}

	if !on {
		if c := nearestClimber(); c != nil {
			if b.roofFirstT >= 0.0 && dist(c.pos, pos) < 60.0 {
				return
			}
		}
		if b.roofFirstT >= 0.0 && b.mode == ModeLoot {
			safe := allFar()
			if safe && hpf > 0.5 {
				return
			}
		}
		if p.roofTravelFight > 0.0 && t-b.mem.lastDamageT < 3.0 && (b.mode == ModeFight || b.mode == ModeRetreat) {
			att := false
			for i := range b.mem.tracks {
				tr := &b.mem.tracks[i]
				if tr.kind == ActorKindMech && t-tr.lastDmgT < 3.0 && t-tr.lastSeen < 1.0 && dist(tr.posAt(t), pos) < 130.0 {
					att = true
					break
				}
			}
			if att && dist2(r.keep, pos) > p.roofTravelCommit {
				return
			}
		}
		b.mode = ModeRoof
		b.goal = Goal{kind: GoalKindNav, navPos: r.keep, navArrive: 2.5}
		b.goalId = -1
		b.roofJet = true
		b.allowCrates = false
		return
	}

	if nClimb >= 2 && hpf < p.roofLeaveHp {
		b.roofBan = append(b.roofBan, struct {
			f0 int
			f1 float64
		}{ri, t + p.roofBanS})
		b.roofSel = nil
		b.roofOn = nil
		return
	}
	if hpf >= p.roofFireHp && lines < 2 {
		b.roofRest = false
	}
	if hpf < p.roofRestHp || lines >= 2 {
		b.roofRest = true
	}
	b.allowCrates = false
	if c := nearestClimber(); c != nil {
		b.mode = ModeFight
		b.target = Tgt{kind: TgtKindMech, mech0: c.id}
		b.goal = Goal{kind: GoalKindNav, navPos: r.keep, navArrive: 6.0}
		b.goalId = -1
		return
	}
	if b.mode == ModeLoot && !b.roofRest {
		if b.goal.kind == GoalKindNav {
			g := b.goal.navPos
			gr := rm.regionOf(nav, g)
			if (gr != nil && *gr == ri) || allFar() && hpf > 0.7 {
				return
			}
		}
	}
	if !b.roofRest {
		var best *struct {
			f0 float64
			f1 int64
			f2 V3
		}
		for i := range foes {
			f := &foes[i]
			d := dist2(f.pos, r.keep)
			if d > p.roofEngage {
				continue
			}
			s := -d + 80.0*(1.0-f.hpf)
			if b.target == (Tgt{kind: TgtKindMech, mech0: f.id}) {
				s += 20.0
			}
			if best == nil || s > best.f0 {
				best = &struct {
					f0 float64
					f1 int64
					f2 V3
				}{s, f.id, f.pos}
			}
		}
		if best != nil {
			id, tp := best.f1, best.f2
			if q := b.roofStepPoint(world, nav, rm, ri, id, tp, t, me.Stats.MuzzleHeight); q != nil {
				b.mode = ModeFight
				b.target = Tgt{kind: TgtKindMech, mech0: id}
				b.goal = Goal{kind: GoalKindNav, navPos: *q, navArrive: 1.2}
				b.goalId = -1
				return
			}
		}
	}
	b.mode = ModeRoof
	b.target = Tgt{}
	b.goal = Goal{kind: GoalKindNav, navPos: r.keep, navArrive: 2.0}
	b.goalId = -1
	b.roofHold = true
}
