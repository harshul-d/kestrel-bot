package pilot

import "math"

type SurviveFoe struct {
	eye         V3
	w           float64
	laser       bool
	aimedCharge *float64
	target      bool
	d           float64
}

func (b *Brain) foes(obs *Obs, target *int64) []SurviveFoe {
	t := obs.T
	pos := obs.Me.Pos
	p := b.p
	v := []SurviveFoe{}
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.svMemoryS {
			continue
		}
		age := fmin(t-tr.lastSeen, 1.5)
		tp := tr.posAt(tr.lastSeen + age)
		if !(isFinite(tp[0]) && isFinite(tp[1]) && isFinite(tp[2])) {
			continue
		}
		s := 1.0 + b.cfg.Cubes.SizePerCube*fmax(tr.cubes, 0.0)
		d := dist(tp, pos)
		w := 0.0
		gun := tr.has(WeaponKindGun) || len(tr.weapons) == 0
		if gun && d < b.cfg.Weapons.Gun.Range {
			w += p.gunDps * fclamp(1.15-d/220.0, 0.2, 1.0)
		}
		laser := tr.has(WeaponKindLaser) && d < b.cfg.Weapons.Laser.Range
		if laser {
			w += p.laserDps * p.svLaserMult
		}
		if tr.has(WeaponKindLightning) && d < b.cfg.Weapons.Lightning.Range+10.0 {
			w += p.lightningDps
		}
		if tr.has(WeaponKindMortar) && d > 20.0 && d < 235.0 {
			w += p.mortarDps * 0.5
		}
		if t-tr.lastDmgT < 3.0 {
			w *= 1.3
		}
		eye := up(tp, b.cfg.Mech.EyeHeight*s)
		aimedCharge := surviveAimedLaser(tr, t, eye, pos, b.cfg.Weapons.Laser.Charge, p.freshS)
		v = append(v, SurviveFoe{eye: eye, w: w, laser: laser, aimedCharge: aimedCharge, target: target != nil && *target == tr.id, d: d})
	}
	sortStable(v, func(a, b *SurviveFoe) int { return totalCmp(a.d, b.d) })
	if len(v) > 6 {
		v = v[:6]
	}
	return v
}

func (b *Brain) spotGoal(obs *Obs, world *World, nav *Nav, tgt *Track, pref float64, cur Goal) Goal {
	type candT struct {
		f0 V3
		f1 float64
	}
	type scoredT struct {
		f0 float64
		f1 V3
		f2 bool
		f3 float64
	}
	p := b.p
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	if p.svSpot <= 0.0 {
		return cur
	}
	var tgtId *int64
	if tgt != nil {
		tgtId = ptr(tgt.id)
	}
	foes := b.foes(obs, tgtId)
	others := 0
	for i := range foes {
		if !foes[i].target && foes[i].w > 0.0 {
			others++
		}
	}
	laserEvade := false
	for i := range foes {
		if foes[i].aimedCharge != nil {
			laserEvade = true
			break
		}
	}
	var need bool
	switch b.mode {
	case ModeRetreat:
		need = false
	default:
		need = others >= 1 || p.svSpot >= 2.0 || laserEvade
	}
	if !need {
		b.spot = nil
		return cur
	}
	if sp := b.spot; sp != nil {
		q, tc, stay := sp.f0, sp.f1, sp.f2
		if t-tc < p.svPeriodS && !laserEvade {
			if stay {
				return cur
			}
			return Goal{kind: GoalKindNav, navPos: q, navArrive: 1.2}
		}
	}
	h := fmax(me.Stats.Height, 1.0)
	walk := fmax(me.Stats.WalkSpeed, 1.0)
	boost := fmax(me.Stats.BoostSpeed, walk)
	muzzleH := me.Stats.MuzzleHeight
	ziC := obs.Zone.Center
	ziR := obs.Zone.Radius
	cands := []candT{{pos, 0.0}}
	rings := [4]struct {
		f0 float64
		f1 int
	}{{6.0, 8}, {12.0, 12}, {19.0, 12}, {27.0, 12}}
	for k0, ring := range rings {
		r, n := ring.f0, ring.f1
		for k := 0; k < n; k++ {
			a := float64(k)*2.0*math.Pi/float64(n) + float64(k0)*0.37
			x := pos[0] + r*math.Cos(a)
			y := pos[1] + r*math.Sin(a)
			q0 := V3{x, y, fin(world.floorBelow(x, y, pos[2]+3.0), pos[2])}
			qp := nav.snap(q0)
			if qp == nil {
				continue
			}
			q := *qp
			if dist(q, q0) > 4.0 || !(isFinite(q[0]) && isFinite(q[1]) && isFinite(q[2])) || math.Abs(q[2]-pos[2]) > 6.0 {
				continue
			}
			if ziR > 0.0 && dist2v(q, ziC) > ziR-10.0 {
				continue
			}
			cands = append(cands, candT{q, dist2(q, pos)})
		}
	}
	if sp := b.spot; sp != nil && !sp.f2 {
		q := sp.f0
		if dist2(q, pos) < 30.0 {
			cands = append(cands, candT{q, dist2(q, pos)})
		}
	}
	chargeLeft := math.Inf(1)
	for i := range foes {
		if foes[i].aimedCharge != nil {
			chargeLeft = fmin(chargeLeft, *foes[i].aimedCharge)
		}
	}
	scored := make([]scoredT, 0, len(cands))
	for i, c0 := range cands {
		q, dq := c0.f0, c0.f1
		if b.spentMs() > p.svBudgetMs {
			break
		}
		c := up(q, h*0.5)
		mid := up(V3{(q[0] + pos[0]) * 0.5, (q[1] + pos[1]) * 0.5, (q[2] + pos[2]) * 0.5}, h*0.5)
		expo := 0.0
		path := 0.0
		hiddenAll := true
		laserHit := false
		for fi := range foes {
			f := &foes[fi]
			shot := world.ray(f.eye, c) == nil
			if shot {
				hiddenAll = false
				m := 1.0
				if f.target {
					m = p.svTargetExpo
				}
				expo += f.w * m
				if f.laser {
					laserHit = true
				}
				if f.aimedCharge != nil {
					laserHit = true
				}
			}
			if i > 0 && dq > 3.0 && world.ray(f.eye, mid) == nil {
				m := 1.0
				if f.target {
					m = p.svTargetExpo
				}
				path += f.w * m
			}
		}
		s := -p.svWCov*expo/100.0 - p.svWPath*path/100.0*fmin(dq/walk, 3.0)/3.0
		if hiddenAll {
			s += p.svWHidden
		}
		if laserHit {
			s -= p.svWLaser
		}
		if tgt != nil {
			tr := tgt
			tc := up(tr.posAt(t), b.cfg.Mech.Height*0.5)
			d := dist(q, tc)
			if world.ray(up(q, muzzleH), tc) == nil {
				s += p.svWShot - p.svWRange*fmax(math.Abs(d-pref)-p.rangeBand, 0.0)
			}
		}
		if isFinite(chargeLeft) && !laserHit {
			if dq/boost <= chargeLeft+0.1 {
				s += p.svWEvade
			}
		}
		s -= p.svWTravel * dq
		if sp := b.spot; sp != nil {
			sq := sp.f0
			if dist2(sq, q) < 3.0 {
				s += p.svHyst
			}
		}
		scored = append(scored, scoredT{s, q, hiddenAll, dq})
	}
	if len(scored) == 0 {
		return cur
	}
	n := len(scored)
	for i := 0; i < n; i++ {
		q := scored[i].f1
		near := false
		for j := 0; j < n; j++ {
			if j != i && scored[j].f2 && dist2(scored[j].f1, q) < 6.5 {
				near = true
				break
			}
		}
		if near || scored[i].f2 {
			scored[i].f0 += p.svWCover
		}
	}
	best := 0
	for i := 1; i < n; i++ {
		if scored[i].f0 > scored[best].f0 {
			best = i
		}
	}
	bs, q, dq := scored[best].f0, scored[best].f1, scored[best].f3
	stay := best == 0 || dq < 2.5 || bs < scored[0].f0+p.svMinGain
	b.spot = &struct {
		f0 V3
		f1 float64
		f2 bool
	}{q, t, stay}
	if stay {
		return cur
	}
	return Goal{kind: GoalKindNav, navPos: q, navArrive: 1.2}
}

func (b *Brain) threatAim(obs *Obs) *V2 {
	if b.p.svTorso <= 0.0 {
		return nil
	}
	pos := obs.Me.Pos
	foes := b.foes(obs, nil)
	var best *struct {
		f0 float64
		f1 V3
	}
	for i := range foes {
		f := &foes[i]
		if f.d > 220.0 || f.w <= 0.0 {
			continue
		}
		s := f.w / (1.0 + f.d/80.0)
		if best == nil || s > best.f0 {
			best = &struct {
				f0 float64
				f1 V3
			}{s, f.eye}
		}
	}
	if best == nil {
		return nil
	}
	e := best.f1
	return &V2{math.Atan2(e[1]-pos[1], e[0]-pos[0]), 0.0}
}

func (b *Brain) holdFire(obs *Obs, d float64, hp float64) bool {
	p := b.p
	if p.svHold <= 0.0 || b.mode != ModeRetreat {
		return false
	}
	lethal := hp > 0.0 && hp < p.svHoldLethal
	hitRecent := obs.T-b.mem.lastDamageT < p.svHoldHitS
	return d > p.svHoldRange && !lethal && !hitRecent
}

func surviveAimedLaser(tr *Track, t float64, eye V3, pos V3, charge float64, fresh float64) *float64 {
	if !tr.charging || t-tr.lastSeen > fmax(fresh, 0.3) || !tr.has(WeaponKindLaser) {
		return nil
	}
	d := fmax(dist2(eye, pos), 1.0)
	toUs := math.Atan2(pos[1]-eye[1], pos[0]-eye[0])
	if math.Abs(adiff(tr.aim[0], toUs)) > math.Atan2(4.5, d)+0.08 {
		return nil
	}
	return ptr(fclamp(charge-(t-tr.chargeSince), 0.1, 0.8))
}
