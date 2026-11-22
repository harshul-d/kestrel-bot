package pilot

import "math"

type PlanMoveOut struct {
	dir   V2
	boost bool
	jump  bool
}

type PlanThreat struct {
	eye        V3
	aimYaw     float64
	gun        bool
	laser      bool
	lightning  bool
	mortar     bool
	ghost      bool
	isTarget   bool
	chargeLeft *float64
	losNow     bool
	dist       float64
}

type PlanShot struct {
	p   V3
	v   V3
	dmg float64
	r   float64
}

type PlanBlast struct {
	at     V3
	t      float64
	splash float64
	radius float64
	edge   float64
	direct float64
}

type PlanCand struct {
	dir     V2
	speed   float64
	boost   bool
	pts     []V3
	blocked float64
	score   float64
}

const planDT = 0.1

func planSimulate(world *World, me *SelfObs, dir V2, speed float64, mult float64, boost bool, n int) struct {
	f0 []V3
	f1 float64
} {
	st := &me.Stats
	ok := func(v V3) V3 { return V3{fin(v[0], 0.0), fin(v[1], 0.0), fin(v[2], 0.0)} }
	body := Body{pos: ok(me.Pos), vel: ok(me.Vel), grounded: me.Grounded, energy: fin(me.Energy, 0.0), energyWait: 0.0}
	mv := V2{0.0, 0.0}
	if speed > 0.0 {
		mv = dir
	}
	d := Drive{mv: mv, boost: boost, jump: false, speedMult: mult}
	pts := make([]V3, 0, n+1)
	pts = append(pts, body.pos)
	blocked := 0.0
	for i := 0; i < n; i++ {
		before := body.pos
		info := physicsStep(world, st, &body, &d, planDT)
		if info.blocked {
			if dist2(body.pos, before) < 0.3*speed*planDT {
				blocked += 1.0
			} else {
				blocked += 0.3
			}
		}
		pts = append(pts, body.pos)
	}
	return struct {
		f0 []V3
		f1 float64
	}{pts, blocked}
}

func (b *Brain) threatDps(th *PlanThreat, d float64) float64 {
	p := b.p
	if th.ghost {
		if d < 280.0 {
			return p.ghostDps
		}
		return 0.0
	}
	w := &b.cfg.Weapons
	s := 0.0
	if th.gun && d < w.Gun.Range {
		s += p.gunDps * fclamp(1.15-d/220.0, 0.2, 1.0)
	}
	if th.laser && d < w.Laser.Range {
		s += p.laserDps
	}
	if th.lightning && d < w.Lightning.Range+6.0 {
		s += p.lightningDps
	}
	if th.mortar && d > 20.0 && d < 235.0 {
		s += p.mortarDps
	}
	return s
}

func (b *Brain) threats(obs *Obs, world *World, center V3) []PlanThreat {
	t := obs.T
	var v []PlanThreat
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > 3.0 {
			continue
		}
		tp := tr.posAt(t)
		s := 1.0 + b.cfg.Cubes.SizePerCube*fmax(tr.cubes, 0.0)
		fresh := t-tr.lastSeen < b.p.freshS
		var chargeLeft *float64
		if tr.charging && fresh {
			chargeLeft = ptr(fclamp(b.cfg.Weapons.Laser.Charge-(t-tr.chargeSince), 0.1, 0.8))
		}
		v = append(v, PlanThreat{
			eye:        up(tp, b.cfg.Mech.EyeHeight*s),
			aimYaw:     tr.aim[0],
			gun:        tr.has(WeaponKindGun) || len(tr.weapons) == 0,
			laser:      tr.has(WeaponKindLaser),
			lightning:  tr.has(WeaponKindLightning),
			mortar:     tr.has(WeaponKindMortar),
			ghost:      false,
			isTarget:   b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}),
			chargeLeft: chargeLeft,
			losNow:     false,
			dist:       dist(tp, center),
		})
	}
	if u := b.mem.unseen; u != nil {
		if t-u.t < 4.0 {
			r := b.p.ghostRange
			q := V3{u.from[0] + math.Cos(u.bearing)*r, u.from[1] + math.Sin(u.bearing)*r, u.from[2] + b.cfg.Mech.EyeHeight}
			explained := false
			for j := range v {
				th := &v[j]
				if th.dist < 250.0 && math.Abs(adiff(math.Atan2(th.eye[1]-center[1], th.eye[0]-center[0]), u.bearing)) < 0.35 {
					explained = true
					break
				}
			}
			if !explained {
				v = append(v, PlanThreat{eye: q, aimYaw: wrap(u.bearing + math.Pi), gun: true, laser: u.laser, lightning: false, mortar: false, ghost: true, isTarget: false, chargeLeft: nil, losNow: false, dist: r})
			}
		}
	}
	sortStable(v, func(x, y *PlanThreat) int { return totalCmp(x.dist, y.dist) })
	if n := f2u(fmax(b.p.maxThreats, 0.0)); len(v) > n {
		v = v[:n]
	}
	for j := range v {
		v[j].losNow = world.sight(v[j].eye, center)
	}
	return v
}

func (b *Brain) incoming(obs *Obs, world *World, center V3) struct {
	f0 []PlanShot
	f1 []PlanBlast
} {
	var shots []PlanShot
	var blasts []PlanBlast
	m := &b.cfg.Weapons.Mortar
	for pi := 0; pi < len(obs.Projectiles) && pi < 64; pi++ {
		pr := &obs.Projectiles[pi]
		if !(memoryAllFinite3(pr.Pos) && memoryAllFinite3(pr.Vel)) {
			continue
		}
		if pr.Kind == ProjKindMortar {
			g := fmax(m.Gravity, 1.0)
			q := pr.Pos
			v := pr.Vel
			tt := 0.0
			h := 0.05
			hit := false
			for tt < fclamp(m.Ttl, 1.0, 8.0) {
				v[2] -= g * h
				q = add(q, scale(v, h))
				tt += h
				if world.solid(q[0], q[1], q[2]) || q[2] < world.terrainZ(q[0], q[1]) {
					hit = true
					break
				}
			}
			if hit && dist(q, center) < 60.0 {
				blasts = append(blasts, PlanBlast{at: q, t: tt, splash: m.Splash, radius: fmax(m.SplashRadius, 1.0), edge: m.SplashEdge, direct: m.Damage})
			}
		} else {
			rel := sub(pr.Pos, center)
			cx := closestXy(rel, pr.Vel, 0.0, 1.2)
			dxy := cx.f1
			if dxy > 30.0 {
				continue
			}
			var dmg, r float64
			if pr.Kind == ProjKindNpc {
				dmg, r = b.cfg.Npc.Damage, b.cfg.Projectile.NpcRadius
			} else {
				dmg, r = b.cfg.Weapons.Gun.Damage, b.cfg.Projectile.GunRadius
			}
			shots = append(shots, PlanShot{p: pr.Pos, v: pr.Vel, dmg: dmg, r: r})
		}
	}
	return struct {
		f0 []PlanShot
		f1 []PlanBlast
	}{shots, blasts}
}

func (b *Brain) incomingCost(c *PlanCand, shots []PlanShot, blasts []PlanBlast, radius float64, height float64) float64 {
	margin := fmax(b.p.projMargin, 0.1)
	n := len(c.pts) - 1
	cost := 0.0
	for si := range shots {
		s := &shots[si]
		worst := 0.0
		for k := 0; k < n; k++ {
			a := c.pts[k]
			bb := c.pts[k+1]
			ov := scale(sub(bb, a), 1.0/planDT)
			t0 := float64(k) * planDT
			sp := add(s.p, scale(s.v, t0))
			rel := sub(sp, up(a, height*0.5))
			rv := sub(s.v, ov)
			cx := closestXy(rel, rv, 0.0, planDT)
			dxy, dz := cx.f1, cx.f2
			if math.Abs(dz) > height*0.5+margin {
				continue
			}
			reach := radius + s.r + margin
			if dxy < reach {
				worst = fmax(worst, fclamp((reach-dxy)/margin, 0.0, 1.0))
			}
		}
		cost += worst * s.dmg
	}
	for bi := range blasts {
		bl := &blasts[bi]
		kf := bl.t / planDT
		var q V3
		if kf >= float64(n) {
			last := c.pts[n]
			prev := c.pts[n-1]
			ext := fmin(bl.t-float64(n)*planDT, 0.8)
			q = add(last, scale(sub(last, prev), ext/planDT))
		} else {
			k := f2u(math.Floor(kf))
			f := kf - float64(k)
			q = add(scale(c.pts[k], 1.0-f), scale(c.pts[min(k+1, n)], f))
		}
		d := fmax(dist2(q, bl.at)-radius, 0.0)
		if math.Abs(q[2]-bl.at[2]) > height+bl.radius {
			continue
		}
		if d < bl.radius+3.0 {
			fall := fmin(d/bl.radius, 1.0)
			dmg := bl.splash * (1.0 - (1.0-bl.edge)*fall)
			if d <= 0.3 {
				dmg += bl.direct
			}
			if d > bl.radius {
				dmg *= 0.3 * (1.0 - (d-bl.radius)/3.0)
			}
			cost += dmg
		}
	}
	return cost
}

func (b *Brain) planMove(obs *Obs, world *World, nav *Nav, budget *Budget, fc *AimFireOut) PlanMoveOut {
	p := b.p
	me := &obs.Me
	st := &me.Stats
	t := obs.T
	pos := me.Pos
	n := uclamp(f2u(p.horizonSteps), 2, 16)
	horizon := float64(n) * planDT
	radius := fmax(st.Radius, 0.5)
	height := fmax(st.Height, 1.0)
	center := up(pos, height*0.5)
	walk := fmax(st.WalkSpeed, 0.1)

	if b.jet != nil {
		tg, t0 := b.jet.f0, b.jet.f1
		dh := dist2(pos, tg)
		landed := me.Grounded && t-t0 > 0.35
		if landed || t-t0 > 4.0 || (me.Energy < 2.0 && me.Grounded) {
			b.jet = nil
		} else {
			d := V2{0.0, 0.0}
			if dh > 0.6 {
				d = unit2(tg[0]-pos[0], tg[1]-pos[1])
			}
			rise := pos[2] < tg[2]+1.5
			if math.Hypot(d[0], d[1]) > 0.1 {
				b.lastDir = d
			}
			return PlanMoveOut{dir: d, boost: !me.Grounded && rise, jump: me.Grounded}
		}
	}

	if t < b.unstickUntil {
		d := b.unstickDir
		b.lastDir = d
		jet := b.unstickLevel >= 2 && !me.Grounded && me.Energy > 15.0
		return PlanMoveOut{dir: d, boost: jet, jump: me.Grounded}
	}

	// nav field
	var d0 *float64
	var navdir *V2
	var goalPos *V3
	arrive := 0.0
	var fld *Field
	navWants := false
	stepDir := func(stp *Step) *V2 {
		if stp == nil {
			return nil
		}
		v := stp.dir
		if isFinite(v[0]) && isFinite(v[1]) && math.Hypot(v[0], v[1]) > 0.2 {
			return ptr(v)
		}
		return nil
	}
	if b.goal.kind == GoalKindField {
		gc := b.goal.fieldCenter
		arrive = b.goal.fieldArrive
		g := V3{gc[0], gc[1], pos[2]}
		goalPos = ptr(g)
		if b.zoneField != nil {
			f := b.zoneField.f2
			if d := nav.dist(f, pos); d != nil && isFinite(*d) {
				d0 = ptr(*d)
				stp := nav.step(f, pos)
				if stp != nil {
					s := *stp
					if s.kind == navEdgeJet && me.Grounded && me.Energy >= s.energy+8.0 && dist2(pos, s.target) < 14.0 {
						b.jet = &struct {
							f0 V3
							f1 float64
						}{s.target, t}
						dd := unit2(s.target[0]-pos[0], s.target[1]-pos[1])
						b.lastDir = dd
						return PlanMoveOut{dir: dd, boost: false, jump: true}
					}
				}
				navdir = stepDir(stp)
				navWants = navdir != nil
				fld = f
			}
		}
		if d0 == nil {
			d0 = ptr(dist2(pos, g))
		}
		if navdir == nil {
			navdir = ptr(unit2(g[0]-pos[0], g[1]-pos[1]))
		}
	}
	if b.goal.kind == GoalKindNav {
		g := b.goal.navPos
		goalPos = ptr(g)
		arrive = b.goal.navArrive
		calm := b.roofJet || (t-b.mem.lastDamageT > 3.0 && !(b.mode == ModeFight || b.mode == ModeRetreat))
		jetE := -1.0
		if calm {
			jetE = me.MaxEnergy * 0.8
		}
		if r := fieldsvcPoll(&b.fsvc.nav); r != nil {
			b.field = &struct {
				f0 V3
				f1 *Field
				f2 float64
			}{V3{r.f0[0], r.f0[1], r.f0[2]}, &r.f2, r.f1}
		}
		need := true
		if b.field != nil {
			fg, f, tb := b.field.f0, b.field.f1, b.field.f2
			moved := dist(fg, g)
			need = (moved > p.fieldRegoal && (t-tb >= 0.3 || moved > 25.0)) || ((f.jetLimit() >= 0.0) != (jetE >= 0.0) && t-tb >= 0.5)
		}
		if need && budget.remainingMs() > p.fieldMinBudgetMs && (b.field == nil || b.spentMs() < 8.0) {
			sg := g
			if q := nav.snap(g); q != nil && memoryAllFinite3(*q) {
				sg = *q
			}
			lim := dist2(pos, sg)*2.0 + 120.0
			shared := b.shared
			tol := fmax(p.fieldRegoal, 0.0)
			goals := []struct {
				f0 V3
				f1 float64
			}{{sg, 0.0}}
			if f := fieldsvcRequest(&b.fsvc, 1, shared, [4]float64{g[0], g[1], g[2], jetE}, tol, t, budget, 10.0, func(nv *Nav) Field { return nv.fieldLimited(goals, jetE, lim) }); f != nil {
				b.field = &struct {
					f0 V3
					f1 *Field
					f2 float64
				}{g, f, t}
			}
		}
		if b.field != nil {
			fg, f := b.field.f0, b.field.f1
			stale := dist(fg, g)
			if stale <= 25.0 {
				if d := nav.dist(f, pos); d != nil && isFinite(*d) {
					d0 = ptr(*d)
					stp := nav.step(f, pos)
					if stp != nil {
						s := *stp
						if s.kind == navEdgeJet && me.Grounded && me.Energy >= s.energy+8.0 && dist2(pos, s.target) < 14.0 {
							b.jet = &struct {
								f0 V3
								f1 float64
							}{s.target, t}
							dd := unit2(s.target[0]-pos[0], s.target[1]-pos[1])
							b.lastDir = dd
							return PlanMoveOut{dir: dd, boost: false, jump: true}
						}
					}
					navdir = stepDir(stp)
					navWants = navdir != nil
					fld = f
				} else {
					if stale <= p.fieldRegoal {
						b.goalUnreachable = true
					}
				}
			}
		}
		if d0 == nil {
			d0 = ptr(dist2(pos, g))
		}
		if navdir == nil {
			navdir = ptr(unit2(g[0]-pos[0], g[1]-pos[1]))
		}
	}
	var reached bool
	switch b.goal.kind {
	case GoalKindNav:
		if d0 != nil {
			reached = *d0 <= arrive || dist2(pos, b.goal.navPos) <= fmin(arrive, 1.0)
		} else {
			reached = false
		}
	case GoalKindField:
		reached = d0 != nil && *d0 <= arrive
	default:
		reached = true
	}

	// stuck
	pursuing := (b.goal.kind == GoalKindNav || b.goal.kind == GoalKindField) && !reached && !(b.mode == ModeFight || b.mode == ModeRetreat)
	if !(pursuing && (navWants || fld == nil) && (d0 != nil && *d0 > arrive+6.0)) {
		b.wantsSince = t
	}
	if !pursuing {
		b.movingSince = t
	} else if t-b.movingSince >= p.stuckWindowS && (t-b.cmdSince >= p.stuckWindowS || t-b.wantsSince >= p.stuckWindowS) {
		if mv := b.mem.moved(t, p.stuckWindowS); mv != nil {
			if *mv < p.stuckDist {
				if t-b.unstickUntil < 6.0 {
					b.unstickLevel = b.unstickLevel + 1
				} else {
					b.unstickLevel = 1
				}
				base := V2{1.0, 0.0}
				if navdir != nil && math.Hypot(navdir[0], navdir[1]) > 0.5 {
					base = *navdir
				}
				var turn float64
				if b.unstickLevel%2 == 0 {
					turn = b.rng.range_(-0.3, 0.3)
				} else {
					sgn := 1.0
					if b.rng.f64() < 0.5 {
						sgn = -1.0
					}
					turn = sgn * b.rng.range_(0.9, 2.2)
				}
				a := math.Atan2(base[1], base[0]) + turn
				b.unstickDir = V2{math.Cos(a), math.Sin(a)}
				b.unstickUntil = t + p.unstickS
				b.unstickCount += 1
				b.movingSince = t + p.unstickS
				b.cmdSince = t + p.unstickS
				b.wantsSince = t + p.unstickS
				b.lastDir = b.unstickDir
				return PlanMoveOut{dir: b.unstickDir, boost: false, jump: me.Grounded}
			}
		}
	}

	threats := b.threats(obs, world, center)
	inc := b.incoming(obs, world, center)
	shots, blasts := inc.f0, inc.f1
	var npcs []V3
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindNpc && t-tr.lastSeen < 2.0 && b.target != (Tgt{kind: TgtKindNpc, npc0: tr.id}) {
			npcs = append(npcs, tr.pos)
		}
	}

	// juke axis
	var juke *V2
	for ti := range threats {
		th := &threats[ti]
		if th.ghost || !th.losNow || !(th.gun || th.mortar) || th.dist < p.jukeMinDist {
			continue
		}
		toUs := math.Atan2(center[1]-th.eye[1], center[0]-th.eye[0])
		if math.Abs(adiff(th.aimYaw, toUs)) < 0.35 {
			u := unit2(center[0]-th.eye[0], center[1]-th.eye[1])
			juke = ptr(V2{-u[1], u[0]})
			break
		}
	}
	if juke != nil && t >= b.jukeNext {
		b.jukeSide = -b.jukeSide
		b.jukeNext = t + b.rng.range_(p.jukeMinS, fmax(p.jukeMaxS, p.jukeMinS))
	}
	wJuke := p.wJuke
	if p.wCad > 0.0 {
		var pick *struct {
			f0 float64
			f1 V2
		}
		for mi := range obs.Mechs {
			m := &obs.Mechs[mi]
			if m.Kind != ActorKindMech {
				continue
			}
			d := dist2(m.Pos, pos)
			if d < p.cadMin || d > p.cadMax {
				continue
			}
			if b.cad.due(m.Id, t, p.cadPeriod, p.cadWin, p.cadKeep, p.cadOff, f2u32(fmax(p.cadReg, 0.0))) && (pick == nil || d < pick.f0) {
				u := unit2(pos[0]-m.Pos[0], pos[1]-m.Pos[1])
				pick = &struct {
					f0 float64
					f1 V2
				}{d, V2{-u[1], u[0]}}
			}
		}
		if pick != nil {
			ax := pick.f1
			if t >= b.cadUntil-p.cadPeriod {
				b.jukeSide = -b.jukeSide
			}
			b.jukeNext = t + p.cadPeriod + 0.05
			b.cadUntil = t + p.cadPeriod + 0.05
			b.cadAxis = ax
		}
		if t < b.cadUntil {
			juke = ptr(b.cadAxis)
			wJuke = p.wCad
		}
	}

	underFire := t-b.mem.lastDamageT < 1.0 || len(shots) > 0 || len(blasts) > 0
	if !underFire {
		for ti := range threats {
			if threats[ti].chargeLeft != nil && threats[ti].losNow {
				underFire = true
				break
			}
		}
	}
	hasLightning := false
	for _, w := range me.Weapons {
		if w != nil && w.Kind == WeaponKindLightning {
			hasLightning = true
			break
		}
	}
	var reserve float64
	if b.mode == ModeZone {
		reserve = 3.0
	} else if hasLightning {
		reserve = p.energyReserveLightning
	} else {
		reserve = p.energyReserve
	}
	allowBoost := me.Grounded && me.Energy > reserve && ((b.mode == ModeZone || b.mode == ModeRetreat || b.mode == ModeDrop || b.mode == ModeRoof) || underFire || !me.InZone)
	slow := 1.0
	if fc.charging {
		slow = fclamp(st.ChargeMoveMult, 0.1, 1.0)
	}

	// candidates
	var base V2
	if navdir != nil && math.Hypot(navdir[0], navdir[1]) > 0.5 {
		base = *navdir
	} else if math.Hypot(b.lastDir[0], b.lastDir[1]) > 0.5 {
		base = b.lastDir
	} else {
		base = V2{1.0, 0.0}
	}
	a0 := math.Atan2(base[1], base[0])
	nd := uclamp(f2u(p.nDirs), 4, 32)
	nb := uclamp(f2u(p.nBoostDirs), 0, 32)
	cands := make([]PlanCand, 0, nd+nb+1)
	cands = append(cands, PlanCand{dir: V2{0.0, 0.0}, speed: 0.0, boost: false, pts: nil, blocked: 0.0, score: 0.0})
	for k := 0; k < nd; k++ {
		a := a0 + float64(k)*2.0*math.Pi/float64(nd)
		cands = append(cands, PlanCand{dir: V2{math.Cos(a), math.Sin(a)}, speed: walk * slow, boost: false, pts: nil, blocked: 0.0, score: 0.0})
	}
	if allowBoost {
		for k := 0; k < nb; k++ {
			a := a0 + float64(k)*2.0*math.Pi/float64(nb)
			cands = append(cands, PlanCand{dir: V2{math.Cos(a), math.Sin(a)}, speed: fmax(st.BoostSpeed, walk) * slow, boost: true, pts: nil, blocked: 0.0, score: 0.0})
		}
	}

	var wProg float64
	switch b.mode {
	case ModeIdle:
		wProg = p.wProgressIdle
	case ModeZone, ModeRoof:
		wProg = p.wProgressZone
	case ModeRetreat:
		wProg = p.wProgressRetreat
	case ModeLoot, ModeHunt:
		wProg = p.wProgressLoot
	case ModeDrop:
		wProg = p.wProgressDrop
	case ModeFight:
		wProg = p.wProgressFight
	}
	zz := strategyZoneAt(&obs.Zone, t, t+horizon, b.zoneB)
	zc, zr := zz.f0, zz.f1
	z := &obs.Zone
	nextSoon := z.ShrinkStart-t < 15.0

	for ci := range cands {
		c := &cands[ci]
		sim := planSimulate(world, me, c.dir, c.speed, slow, c.boost, n)
		c.pts = sim.f0
		c.blocked = sim.f1
		end := c.pts[n]
		s := 0.0
		switch b.goal.kind {
		case GoalKindNav, GoalKindField:
			if !reached {
				g := pos
				if goalPos != nil {
					g = *goalPos
				}
				startD := 0.0
				if d0 != nil {
					startD = *d0
				}
				var de float64
				if fld != nil {
					if v := nav.dist(fld, end); v != nil && isFinite(*v) {
						de = *v
					} else {
						de = startD + 6.0
					}
				} else {
					de = dist2(end, g)
				}
				s += wProg * fclamp(startD-fmax(de, arrive*0.5), -25.0, 25.0)
				if navdir != nil {
					ndv := *navdir
					s += p.wNavdir * (c.dir[0]*ndv[0] + c.dir[1]*ndv[1]) * fmin(c.speed/walk, 1.2)
				}
			} else {
				s -= 0.6 * dist2(end, pos)
			}
		case GoalKindRange:
			g, pref := b.goal.rangePos, b.goal.rangePref
			e0 := fmax(math.Abs(dist2(pos, g)-pref)-p.rangeBand, 0.0)
			e1 := fmax(math.Abs(dist2(end, g)-pref)-p.rangeBand, 0.0)
			s += p.wRange * (e0 - e1)
		case GoalKindNone:
			s -= 0.6 * dist2(end, pos)
		}
		if c.speed == 0.0 {
			s += p.wStand
			if t-b.mem.lastDamageT < 1.5 {
				s -= p.wJuke
			}
		}
		s -= p.wBlocked * c.blocked
		s += p.wSmooth * (c.dir[0]*b.lastDir[0] + c.dir[1]*b.lastDir[1])
		if juke != nil {
			j := *juke
			s += wJuke * b.jukeSide * (c.dir[0]*j[0] + c.dir[1]*j[1]) * fmin(c.speed/walk, 1.3)
			if c.speed == 0.0 {
				s -= wJuke * 0.6
			}
		}
		if c.boost {
			s -= p.wBoost * st.BoostDrain * horizon
		}
		// zone
		out := dist2v(end, zc) - zr + p.zoneInner
		if out > 0.0 {
			var k float64
			if b.mode == ModeZone || b.goal.kind == GoalKindField {
				k = p.zoneCostZoneMode
			} else {
				k = p.zoneCostMult
			}
			s -= k * z.Dps * (horizon*fmin(out/fmax(p.zoneInner, 0.1), 1.0) + out/walk)
		}
		if nextSoon && b.mode != ModeZone {
			on := dist2v(end, z.NextCenter) - z.NextRadius
			o0 := dist2v(pos, z.NextCenter) - z.NextRadius
			if on > 0.0 {
				s -= p.wNextZone * (on - fmax(o0, 0.0))
			}
		}
		for _, q := range npcs {
			d := dist(q, end)
			if d < p.npcAvoid {
				s -= p.wNpc*(p.npcAvoid-d)*0.2 + p.wNpc*fmax(dist(q, pos)-d, 0.0)
			}
		}
		s -= p.wProj * b.incomingCost(c, shots, blasts, radius, height)
		if p.wAntilead > 0.0 {
			var shooters []struct {
				f0 V3
				f1 float64
			}
			for ti := range threats {
				th := &threats[ti]
				if th.gun && !th.ghost && th.losNow {
					shooters = append(shooters, struct {
						f0 V3
						f1 float64
					}{th.eye, th.aimYaw})
				}
			}
			s -= p.wAntilead * aimzAntilead(c.pts, planDT, pos, me.Vel, shooters, radius+b.cfg.Projectile.GunRadius, fmax(b.cfg.Weapons.Gun.Speed, 1.0), p.antileadLat, p.antileadSigma, b.cfg.Weapons.Gun.Damage)
		}
		c.score = s
	}

	// exposure pass, best partial scores first
	if len(threats) > 0 {
		order := make([]int, len(cands))
		for i := range order {
			order[i] = i
		}
		sortStable(order, func(x, y *int) int { return totalCmp(cands[*y].score, cands[*x].score) })
		armed := fc.armed && fc.target != nil
		laserDmg := b.cfg.Weapons.Laser.Damage
		for ti := range threats {
			threats[ti].dist = fmax(threats[ti].dist, 1.0)
		}
		for _, i := range order {
			cheap := budget.remainingMs() < p.expoMinBudgetMs || b.spentMs() > 12.0
			c := &cands[i]
			cost := 0.0
			bonus := 0.0
			for ti := range threats {
				th := &threats[ti]
				var wMode float64
				switch {
				case b.mode == ModeRetreat:
					wMode = p.expoRetreat
				case b.mode == ModeZone:
					wMode = p.expoZone
				case b.mode == ModeFight && th.isTarget:
					if armed {
						wMode = p.expoFightTarget
					} else {
						wMode = 1.0
					}
				default:
					wMode = 1.0
				}
				e := 0.0
				losEnd := th.losNow
				losMid := th.losNow
				for j, k := range [2]int{n / 2, n} {
					cpt := up(c.pts[k], height*0.5)
					var los bool
					if cheap {
						los = th.losNow
					} else {
						los = world.sight(th.eye, cpt)
					}
					if j == 0 {
						losMid = los
					} else {
						losEnd = los
					}
					if los {
						d := dist(th.eye, cpt)
						toUs := math.Atan2(cpt[1]-th.eye[1], cpt[0]-th.eye[0])
						al := fclamp(1.0-math.Abs(adiff(th.aimYaw, toUs)), 0.0, 1.0)
						e += 0.5 * b.threatDps(th, d) * (0.4 + 0.6*al)
					}
				}
				cost += p.wExpo * wMode * e * horizon
				if th.isTarget && b.mode == ModeFight && armed && losEnd {
					bonus += p.wFightLos
				}
				if th.chargeLeft != nil {
					tl := *th.chargeLeft
					var hit bool
					if tl <= horizon*0.5+0.05 {
						hit = losMid
					} else {
						hit = losEnd
					}
					if hit {
						cost += p.wCharge * laserDmg
					}
				}
			}
			c.score += bonus - cost
		}
	}

	best := 0
	for i := 1; i < len(cands); i++ {
		if cands[i].score > cands[best].score {
			best = i
		}
	}
	c := &cands[best]
	fracSpeed := 0.0
	if c.speed > 0.0 {
		fracSpeed = 1.0
	}
	dir := V2{c.dir[0] * fracSpeed, c.dir[1] * fracSpeed}
	if fracSpeed > 0.0 {
		b.lastDir = c.dir
	} else {
		b.cmdSince = t + planDT
		b.lastDir = V2{b.lastDir[0] * 0.5, b.lastDir[1] * 0.5}
	}
	return PlanMoveOut{dir: dir, boost: c.boost, jump: false}
}
