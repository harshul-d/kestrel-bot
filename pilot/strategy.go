package pilot

import (
	"math"
	"time"
)

type StrategyZoneInfo struct {
	urgent  bool
	safe    V2
	curC    V2
	curR    float64
	closing bool
}

// strategyZoneAt is the circle at time tau, given the zone reported at time now.
func strategyZoneAt(z *ZoneObs, now, tau float64, phaseStart bool) struct {
	f0 V2
	f1 float64
} {
	var t0 float64
	if phaseStart {
		t0 = z.ShrinkStart
	} else {
		t0 = fmax(z.ShrinkStart, now)
	}
	span := z.ShrinkEnd - t0
	if tau <= t0 || !(span > 1e-6) {
		return struct {
			f0 V2
			f1 float64
		}{z.Center, z.Radius}
	}
	k := fclamp((tau-t0)/span, 0.0, 1.0)
	return struct {
		f0 V2
		f1 float64
	}{
		V2{z.Center[0] + (z.NextCenter[0]-z.Center[0])*k, z.Center[1] + (z.NextCenter[1]-z.Center[1])*k},
		z.Radius + (z.NextRadius-z.Radius)*k,
	}
}

type StrategyLootPick struct {
	id    int64
	pos   V3
	quick bool
}

func strategyGround(world *World, p V2) V3 {
	return V3{p[0], p[1], fin(world.terrainZ(p[0], p[1]), 0.0)}
}

func (b *Brain) safeRadius(z *ZoneObs) float64 {
	p := b.p
	if z.NextRadius <= p.finalRadius {
		return 0.0
	}
	return fmax(z.NextRadius-fmin(p.zoneMargin, z.NextRadius*0.5)-p.zoneExtra, 0.0)
}

func (b *Brain) seedRadius(z *ZoneObs) float64 {
	return fmin(b.safeRadius(z), z.NextRadius*b.p.idleFrac)
}

func (b *Brain) ensureZoneField(obs *Obs, world *World, nav *Nav) {
	z := &obs.Zone
	if r := fieldsvcPoll(&b.fsvc.zone); r != nil {
		b.zoneField = &struct {
			f0 V2
			f1 float64
			f2 *Field
		}{V2{r.f0[0], r.f0[1]}, r.f0[2], &r.f2}
	}
	if b.zoneField != nil {
		c, r := b.zoneField.f0, b.zoneField.f1
		if math.Hypot(c[0]-z.NextCenter[0], c[1]-z.NextCenter[1]) < 0.5 && math.Abs(r-z.NextRadius) < 0.5 {
			return
		}
	}
	if !(isFinite(z.NextCenter[0]) && isFinite(z.NextCenter[1]) && isFinite(z.NextRadius)) {
		return
	}
	safeR := b.seedRadius(z)
	var seeds []struct {
		f0 V3
		f1 float64
	}
	c := z.NextCenter
	seeds = append(seeds, struct {
		f0 V3
		f1 float64
	}{strategyGround(world, c), 0.0})
	if safeR >= 8.0 {
		for _, rn := range [3]struct {
			fr float64
			n  int
		}{{0.35, 8}, {0.7, 12}, {1.0, 20}} {
			fr, n := rn.fr, rn.n
			for k := 0; k < n; k++ {
				a := float64(k) * 2.0 * math.Pi / float64(n)
				seeds = append(seeds, struct {
					f0 V3
					f1 float64
				}{strategyGround(world, V2{c[0] + safeR*fr*math.Cos(a), c[1] + safeR*fr*math.Sin(a)}), 0.0})
			}
		}
	} else {
		for _, rn := range [5]struct {
			r float64
			n int
		}{{4.0, 6}, {9.0, 8}, {15.0, 10}, {23.0, 12}, {33.0, 14}} {
			r, n := rn.r, rn.n
			for k := 0; k < n; k++ {
				a := float64(k) * 2.0 * math.Pi / float64(n)
				seeds = append(seeds, struct {
					f0 V3
					f1 float64
				}{strategyGround(world, V2{c[0] + r*math.Cos(a), c[1] + r*math.Sin(a)}), 4.0 * r})
			}
		}
	}
	jet := -1.0
	if safeR < 8.0 {
		jet = obs.Me.MaxEnergy * 0.8
	}
	_ = nav
	key := [4]float64{c[0], c[1], z.NextRadius, jet}
	budget := b.budget
	shared := b.shared
	got := fieldsvcRequest(&b.fsvc, 0, shared, key, 0.0, obs.T, &budget, 14.0, func(nv *Nav) Field { return nv.fieldMulti(seeds, jet) })
	if got != nil {
		b.zoneField = &struct {
			f0 V2
			f1 float64
			f2 *Field
		}{c, z.NextRadius, got}
	} else {
		b.zoneField = nil
	}
}

func (b *Brain) zoneInfo(obs *Obs, world *World, nav *Nav) StrategyZoneInfo {
	b.ensureZoneField(obs, world, nav)
	p := b.p
	z := &obs.Zone
	t := obs.T
	pos := obs.Me.Pos
	za := strategyZoneAt(z, t, t, false)
	cA, rA := za.f0, za.f1
	if !obs.Me.InZone && dist2v(pos, cA) < rA-1.0 && z.Radius > 0.0 {
		b.zoneB = true
	}
	zc := strategyZoneAt(z, t, t, b.zoneB)
	curC, curR := zc.f0, zc.f1
	dn := dist2v(pos, z.NextCenter)
	isFinal := z.NextRadius <= p.finalRadius
	margin := fmin(p.zoneMargin, z.NextRadius*0.5)
	insideNext := dn <= z.NextRadius-margin
	safeR := b.safeRadius(z)
	_ = isFinal
	var safe V2
	if dn > safeR && dn > 1e-6 {
		safe = V2{z.NextCenter[0] + (pos[0]-z.NextCenter[0])/dn*safeR, z.NextCenter[1] + (pos[1]-z.NextCenter[1])/dn*safeR}
	} else {
		safe = V2{pos[0], pos[1]}
	}
	dNeed := fmax(dn-(z.NextRadius-margin), 0.0)
	outNow := dist2v(pos, curC) - curR
	tHit := math.Inf(1)
	if z.ShrinkEnd > t {
		tau := t
		for tau <= z.ShrinkEnd+0.5 {
			zz := strategyZoneAt(z, t, tau, b.zoneB)
			if dist2v(pos, zz.f0) > zz.f1-p.zoneEdge {
				tHit = tau
				break
			}
			tau += 0.5
		}
	}
	walk := fmax(obs.Me.Stats.WalkSpeed, 1.0)
	var dPath *float64
	if b.zoneField != nil {
		if d := nav.dist(b.zoneField.f2, pos); d != nil && isFinite(*d) {
			dPath = d
		}
	}
	seedR := b.seedRadius(z)
	straight := fmax(dn-seedR, 0.0)
	var needS float64
	if dPath != nil {
		needS = fmax(*dPath-fmax(z.NextRadius-margin-seedR, 0.0), dNeed) * 1.15 / walk
	} else {
		needS = dNeed * p.pathFactor / walk
	}
	detourS := 0.0
	if dPath != nil {
		detourS = fmax(*dPath-straight, 0.0) / walk
	}
	slack := p.zoneSlackS
	if b.ffaBanking(t) {
		slack += p.ffaBankSlack
	}
	urgent := !obs.Me.InZone ||
		outNow > -p.zoneEdge ||
		(tHit-t) < p.zoneLeadS+detourS ||
		(!insideNext && (z.ShrinkEnd-t) < needS+slack)
	closing := !insideNext && (z.ShrinkStart-t) < 25.0
	return StrategyZoneInfo{urgent: urgent, safe: safe, curC: curC, curR: curR, closing: closing}
}

func (b *Brain) decide(obs *Obs, world *World, nav *Nav) {
	p := b.p
	me := &obs.Me
	t := obs.T
	pos := me.Pos
	hpf := frac(me.Hp, me.MaxHp)
	zi := b.zoneInfo(obs, world, nav)
	b.sqUpdate(obs, nav)
	muzzle := up(pos, me.Stats.MuzzleHeight)

	if b.goalUnreachable {
		if b.goalId >= 0 {
			b.mem.ban(b.goalId, t+p.blacklistS)
		}
		b.goalUnreachable = false
	}
	if b.goalId >= 0 {
		if b.pursuit.f0 != b.goalId {
			b.pursuit = struct {
				f0 int64
				f1 float64
			}{b.goalId, t}
		} else if t-b.pursuit.f1 > p.pursuitMaxS {
			b.mem.ban(b.goalId, t+p.blacklistS)
			b.pursuit = struct {
				f0 int64
				f1 float64
			}{-1, t}
		}
	}

	nAtt := 0
	nearMech := math.Inf(1)
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech {
			continue
		}
		if t-tr.lastDmgT < 2.0 {
			nAtt += 1
		}
		if t-tr.lastSeen < 1.0 {
			nearMech = fmin(nearMech, dist(tr.pos, pos))
		}
	}
	threatNear := nearMech < 200.0 || t-b.mem.lastDamageT < 3.0
	laserNear := false
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 1.0 && tr.has(WeaponKindLaser) && dist(tr.posAt(t), pos) < 130.0 && world.sight(up(tr.posAt(t), b.cfg.Mech.EyeHeight), up(pos, me.Stats.Height*0.6)) {
			laserNear = true
			break
		}
	}
	pressured := nAtt >= 2 || (laserNear && t-b.mem.lastDamageT < 4.0)
	held := b.held(t)
	var tgt *Track
	if held {
		tgt = nil
	} else if b.orderOn() {
		tgt = b.chooseTargetOrder(obs, &zi)
	} else {
		tgt = b.chooseTarget(obs, &zi)
	}
	regen := b.sqRegen(obs)
	if regen {
		if tgt != nil && !(t-tgt.lastDmgT < p.sqRegenFightS) {
			tgt = nil
		}
	}
	nearMem := math.Inf(1)
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 4.0 {
			nearMem = fmin(nearMem, dist(tr.posAt(t), pos))
		}
	}

	was := b.mode == ModeRetreat
	var wantRetreat bool
	if was {
		wantRetreat = hpf < p.retreatExitHp
	} else {
		wantRetreat = (hpf < p.retreatHp && threatNear) || (nAtt >= 2 && hpf < p.retreatMultiHp) || (pressured && hpf < p.retreatPressHp)
	}
	if wantRetreat && !was {
		if tr := tgt; tr != nil {
			if nAtt <= 1 && tr.hp < me.Hp*p.finishRatio && t-tr.lastSeen < 0.3 {
				wantRetreat = false
			}
		}
	}

	if held && (nAtt >= 1 || nearMech < 120.0 || nearMem < 100.0 || t-b.mem.lastDamageT < 3.0 || (was && nearMem < 170.0)) {
		wantRetreat = true
	}
	if !wantRetreat && (b.sqGate(obs, tgt) || regen && tgt == nil) {
		wantRetreat = true
	}
	avoid := !wantRetreat && tgt == nil && b.sqAvoid(obs)
	loot := b.chooseLoot(obs, &zi)
	var harvest *struct {
		f0 int64
		f1 V3
	}
	if !held {
		harvest = b.harvest(obs, &zi, nAtt, nearMech)
	}
	var gearg *struct {
		f0 int64
		f1 V3
	}
	if held {
		gearg = nil
	} else if wantRetreat || avoid {
		gearg = b.gearGrab(obs, &zi)
	} else {
		gearg = b.gearGoal(obs, &zi, tgt)
		if gearg == nil {
			if p.gearGrabAlways > 0.5 {
				gearg = b.gearGrab(obs, &zi)
			} else {
				gearg = nil
			}
		}
	}
	if b.tkNoDetour() {
		if gearg != nil && !(dist(gearg.f1, pos) < p.skDetour) {
			gearg = nil
		}
		if harvest != nil && !(dist(harvest.f1, pos) < p.skDetour) {
			harvest = nil
		}
	}
	farm := !held && b.gearFarmMode(obs, tgt)
	gearDrop := b.gearDropOk(obs, tgt)
	var huntNear float64
	if b.gearOn() {
		huntNear = b.gearNearReal(obs)
	} else {
		huntNear = nearMech
	}
	rest := !wantRetreat && !held && b.wantRest(obs, &zi, nAtt, nearMech, hpf)
	if wantRetreat || held {
		b.econ.resting = false
	}
	drop := b.chooseDrop(obs, &zi)
	if drop != nil && !(!held || nearMem > 100.0) {
		drop = nil
	}
	vdrop := b.dpPick(obs, drop, tgt, wantRetreat, hpf)
	nearItem := func(g *struct {
		f0 int64
		f1 V3
	}) bool {
		return g != nil && dist(g.f1, pos) < p.skDetour
	}
	var vseek *struct {
		f0 int64
		f1 V3
	}
	if tgt == nil && !wantRetreat && !avoid && !held && !nearItem(gearg) && !nearItem(harvest) {
		vseek = b.seekGoal(obs, &zi)
	} else {
		b.vx.seeking = false
		vseek = nil
	}
	z := &obs.Zone
	endgame := held && z.NextRadius <= 0.5 && z.ShrinkEnd-t < 40.0
	var mode Mode
	var goal Goal
	target := Tgt{}
	goalId := int64(-1)
	if tgt != nil {
		target = Tgt{kind: TgtKindMech, mech0: tgt.id}
	}

	if endgame {
		mode = ModeZone
		goal = Goal{kind: GoalKindField, fieldCenter: z.NextCenter, fieldArrive: 0.8}
	} else if zi.urgent {
		mode = ModeZone
		goal = Goal{kind: GoalKindField, fieldCenter: obs.Zone.NextCenter, fieldArrive: 1.5}
	} else if b.vx.regroup || b.vx.kite {
		mode = ModeRetreat
		if b.vx.kite {
			goal = b.kiteGoal(obs, world, nav, &zi)
		} else {
			goal = Goal{}
		}
		if goal.kind == GoalKindNone {
			goal = b.coverGoal(obs, world, nav, &zi)
		}
		if b.vx.regroup {
			target = Tgt{}
		}
	} else if vdrop != nil {
		d := vdrop
		mode = ModeDrop
		goal = Goal{kind: GoalKindNav, navPos: d.f0, navArrive: fmax(d.f1*0.3, 1.0)}
	} else if vseek != nil {
		id, q := vseek.f0, vseek.f1
		mode = ModeHunt
		goal = Goal{kind: GoalKindNav, navPos: q, navArrive: 15.0}
		if id >= 0 {
			target = Tgt{kind: TgtKindMech, mech0: id}
		}
	} else if gearg != nil && (wantRetreat || avoid) {
		id, gp := gearg.f0, gearg.f1
		mode = ModeLoot
		goal = Goal{kind: GoalKindNav, navPos: gp, navArrive: 1.0}
		goalId = id
	} else if wantRetreat || avoid {
		mode = ModeRetreat
		goal = b.coverGoal(obs, world, nav, &zi)
	} else if gearg != nil {
		id, gp := gearg.f0, gearg.f1
		mode = ModeLoot
		goal = Goal{kind: GoalKindNav, navPos: gp, navArrive: 1.0}
		goalId = id
	} else if harvest != nil {
		id, hp := harvest.f0, harvest.f1
		mode = ModeLoot
		goal = Goal{kind: GoalKindNav, navPos: hp, navArrive: 1.0}
		goalId = id
	} else if rest {
		mode = ModeRetreat
		goal = b.coverGoal(obs, world, nav, &zi)
	} else if loot != nil && loot.quick {
		l := loot
		mode = ModeLoot
		goal = Goal{kind: GoalKindNav, navPos: l.pos, navArrive: 1.2}
		goalId = l.id
	} else if drop != nil && (tgt == nil || drop.f2 < 50.0 || hpf < 0.6 || gearDrop) {
		d := drop
		mode = ModeDrop
		goal = Goal{kind: GoalKindNav, navPos: d.f0, navArrive: fmax(d.f1*0.3, 1.0)}
	} else if tgt != nil && (!farm || b.chooseHunt(obs, world, &zi, hpf, huntNear, muzzle) == nil) {
		tr := tgt
		mode = ModeFight
		tp := tr.posAt(t)
		c := up(tp, b.cfg.Mech.Height*0.5)
		los := t-tr.lastSeen < p.freshS && world.ray(muzzle, c) == nil
		if los || t-tr.lastDmgT < 3.0 {
			b.noLos = struct {
				f0 int64
				f1 float64
			}{tr.id, t}
		} else if b.noLos.f0 != tr.id {
			b.noLos = struct {
				f0 int64
				f1 float64
			}{tr.id, t}
		} else {
			mult := 1.0
			if b.finishBonus(tr) > 0.0 {
				mult = p.finishChaseMult
			}
			if t-b.noLos.f1 > p.noLosDropS*mult {
				b.mem.ban(tr.id, t+8.0)
			}
		}
		pref := b.gearRange(obs, tr, b.prefRange(obs, tr, hpf))
		if p.tkOn >= 0.5 {
			pref = fmin(pref, p.tkRange)
		}
		var g0 Goal
		if los {
			g0 = Goal{kind: GoalKindRange, rangePos: tp, rangePref: pref}
		} else {
			g0 = Goal{kind: GoalKindNav, navPos: tp, navArrive: fmax(pref*0.5, 15.0)}
		}
		if b.p.svSpot > 0.0 {
			b.mode = ModeFight
		}
		goal = b.spotGoal(obs, world, nav, tr, pref, g0)
	} else if loot != nil {
		l := loot
		mode = ModeLoot
		goal = Goal{kind: GoalKindNav, navPos: l.pos, navArrive: 1.2}
		goalId = l.id
	} else {
		mode = ModeIdle
		if h := b.chooseHunt(obs, world, &zi, hpf, huntNear, muzzle); h != nil {
			mode = ModeHunt
			target = h.f0
			goal = h.f1
			goalId = h.f2
		} else if q := b.prepGoal(); q != nil {
			goal = Goal{kind: GoalKindNav, navPos: *q, navArrive: 3.0}
		} else {
			goal = Goal{kind: GoalKindField, fieldCenter: obs.Zone.NextCenter, fieldArrive: 2.0}
		}
	}
	if (mode == ModeIdle || mode == ModeHunt || mode == ModeDrop) || (mode == ModeLoot && goalId >= 0 && (loot == nil || !loot.quick)) {
		if b.ffaAvoid(obs) {
			mode = ModeRetreat
			goal = b.coverGoal(obs, world, nav, &zi)
			target = Tgt{}
			goalId = -1
		}
	}
	allow := false
	if (mode == ModeHunt || mode == ModeLoot || mode == ModeIdle) && hpf >= p.crateMinHp {
		var thr float64
		if b.gateClosed(obs) {
			thr = p.readySafeRange
		} else {
			thr = p.huntSafeRange
		}
		allow = huntNear > thr
	}
	b.allowCrates = allow
	if mode != ModeRetreat {
		b.cover = nil
	}
	if goalId != b.goalId {
		b.pursuit = struct {
			f0 int64
			f1 float64
		}{goalId, t}
	}
	if mode != ModeHunt {
		b.vx.seeking = false
	}
	b.mode = mode
	b.goal = goal
	b.goalId = goalId
	b.target = target
	b.roofDecide(obs, world, nav, &zi)
}

func (b *Brain) chooseTarget(obs *Obs, zi *StrategyZoneInfo) *Track {
	p := b.p
	t := obs.T
	pos := obs.Me.Pos
	z := &obs.Zone
	gate := b.gateClosed(obs)
	var best *Track
	bestS := 0.0
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.targetMemoryS {
			continue
		}
		if gate && b.spareTr(tr, t, pos) {
			continue
		}
		tp := tr.posAt(t)
		d := dist(tp, pos)
		attacker := t-tr.lastDmgT < 5.0
		if !attacker && b.mem.blacklisted(tr.id, t) {
			continue
		}
		if !b.sqAllow(obs, tr) || b.sfHold(obs, tr) {
			continue
		}
		if d > p.engageRange && !attacker {
			continue
		}
		if !b.ffaAllow(obs, tr, attacker, d) {
			continue
		}
		if d > 60.0 {
			if dist2v(tp, zi.curC) > zi.curR-2.0 {
				continue
			}
			if zi.closing && dist2v(tp, z.NextCenter) > z.NextRadius+p.chaseOutNext {
				continue
			}
		}
		s := 60.0*(1.0-d/200.0) + 80.0*(1.0-frac(tr.hp, tr.maxHp))
		if attacker {
			s += 100.0 + tr.dmgToUs*0.1
		}
		if b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) {
			s += p.targetSticky
			s += b.finishBonus(tr)
		}
		if b.ffaEndRival(obs, tr) {
			s += p.ffaEndBonus
		}
		s += b.tkBonus(tr)
		s += b.ffaOppBonus(tr, t)
		if t-tr.lastSeen < p.freshS {
			s += 30.0
		}
		if best == nil || s > bestS {
			bestS = s
			best = tr
		}
	}
	if best == nil {
		return nil
	}
	return ptr(*best)
}

func (b *Brain) prefRange(obs *Obs, tr *Track, hpf float64) float64 {
	p := b.p
	me := &obs.Me
	has := func(k WeaponKind) bool {
		for _, w := range me.Weapons {
			if w != nil && w.Kind == k {
				return true
			}
		}
		return false
	}
	pref := p.rangeGun
	if has(WeaponKindLaser) {
		pref = p.rangeLaser
	} else if has(WeaponKindMortar) && !has(WeaponKindGun) {
		pref = p.rangeMortar
	}
	if has(WeaponKindLightning) && me.Energy > 35.0 && hpf > p.lightningRushHp && (p.rushSolo <= 0.0 || b.rushOk(obs, tr)) {
		pref = p.rangeLightning
	} else if tr.has(WeaponKindLightning) {
		pref = fmax(pref, p.rangeVsLightning)
	}
	if b.ffaGunVsLaser(obs, tr) {
		pref = fmin(pref, p.ffaCloseRange)
	}
	return pref
}

func (b *Brain) inPlayPub(obs *Obs, zi *StrategyZoneInfo, q V3, d float64) bool {
	return b.inPlay(obs, zi, q, d)
}

func (b *Brain) inPlay(obs *Obs, zi *StrategyZoneInfo, q V3, d float64) bool {
	z := &obs.Zone
	if dist2v(q, zi.curC) > zi.curR-6.0 {
		return false
	}
	walk := fmax(obs.Me.Stats.WalkSpeed, 1.0)
	outNext := dist2v(q, z.NextCenter) - z.NextRadius
	if outNext > -5.0 && (z.ShrinkEnd-obs.T) < 25.0+d/walk+fmax(outNext, 0.0)/walk {
		return false
	}
	return true
}

func (b *Brain) chooseLoot(obs *Obs, zi *StrategyZoneInfo) *StrategyLootPick {
	p := b.p
	me := &obs.Me
	t := obs.T
	var best *StrategyLootPick
	bestS := 0.0
	for i := range b.mem.pickups {
		it := &b.mem.pickups[i]
		if b.mem.blacklisted(it.id, t) {
			continue
		}
		wanted, slot := b.want(me, it.kind)
		if !wanted {
			continue
		}
		d := dist(it.pos, me.Pos)
		empty := slot == nil
		lim := p.lootSwapMax
		if empty {
			lim = p.lootWeaponMax
		}
		if d > lim || !b.inPlay(obs, zi, it.pos, d) {
			continue
		}
		e := 0.0
		if empty {
			e = 80.0
		}
		s := d - e - b.prefOf(it.kind)*5.0
		if it.id == b.goalId {
			s -= 15.0
		}
		if best == nil || s < bestS {
			bestS = s
			best = &StrategyLootPick{id: it.id, pos: it.pos, quick: d < p.lootQuick && empty}
		}
	}
	if me.Cubes < b.cfg.Cubes.StatCap {
		for i := range b.mem.cubes {
			it := &b.mem.cubes[i]
			if b.mem.blacklisted(it.id, t) {
				continue
			}
			d := dist(it.pos, me.Pos)
			if d > p.lootCubeMax || !b.inPlay(obs, zi, it.pos, d) {
				continue
			}
			s := d - 20.0
			if it.id == b.goalId {
				s -= 15.0
			}
			if best == nil || s < bestS {
				bestS = s
				best = &StrategyLootPick{id: it.id, pos: it.pos, quick: d < p.lootQuick*0.5}
			}
		}
	}
	return best
}

func (b *Brain) chooseDrop(obs *Obs, zi *StrategyZoneInfo) *struct {
	f0 V3
	f1 float64
	f2 float64
} {
	p := b.p
	me := &obs.Me
	walk := fmax(me.Stats.WalkSpeed, 1.0)
	var best *struct {
		f0 V3
		f1 float64
		f2 float64
	}
	for i := range obs.Drops {
		d := &obs.Drops[i]
		if d.Holder != nil {
			h := *d.Holder
			if h != me.Id && d.Progress > d.CaptureS*0.5 {
				continue
			}
		}
		dd := dist(d.Pos, me.Pos)
		if dd > p.dropMaxDist || !memoryAllFinite3(d.Pos) {
			continue
		}
		if d.LandsIn > dd*p.pathFactor/walk+p.dropEarlyS {
			continue
		}
		if dist2v(d.Pos, zi.curC) > zi.curR-4.0 {
			continue
		}
		if best == nil || dd < best.f2 {
			best = &struct {
				f0 V3
				f1 float64
				f2 float64
			}{d.Pos, fmax(d.Radius, 2.0), dd}
		}
	}
	return best
}

func (b *Brain) chooseHunt(obs *Obs, world *World, zi *StrategyZoneInfo, hpf float64, nearMech float64, muzzle V3) *struct {
	f0 Tgt
	f1 Goal
	f2 int64
} {
	p := b.p
	me := &obs.Me
	t := obs.T
	var safe float64
	if b.gateClosed(obs) {
		safe = p.readySafeRange
	} else {
		safe = p.huntSafeRange
	}
	if hpf < p.huntMinHp || nearMech < safe {
		return nil
	}
	moreCubes := me.Cubes < b.cfg.Cubes.StatCap
	emptySlot := false
	for _, w := range me.Weapons {
		if w == nil {
			emptySlot = true
			break
		}
	}
	var best *struct {
		f0 Tgt
		f1 Goal
		f2 int64
	}
	bestS := 0.0
	if moreCubes {
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind != ActorKindNpc || b.mem.blacklisted(tr.id, t) {
				continue
			}
			d := dist(tr.pos, me.Pos)
			if d > p.huntMaxDist || !b.inPlay(obs, zi, tr.pos, d) {
				continue
			}
			s := d
			if b.goalId == tr.id {
				s -= 20.0
			}
			if best == nil || s < bestS {
				c := up(tr.pos, b.cfg.Npc.Height*0.5)
				los := t-tr.lastSeen < p.freshS && world.ray(muzzle, c) == nil
				var g Goal
				if los {
					g = Goal{kind: GoalKindRange, rangePos: tr.pos, rangePref: p.rangeNpc}
				} else {
					g = Goal{kind: GoalKindNav, navPos: tr.pos, navArrive: p.rangeNpc * 0.8}
				}
				bestS = s
				best = &struct {
					f0 Tgt
					f1 Goal
					f2 int64
				}{Tgt{kind: TgtKindNpc, npc0: tr.id}, g, tr.id}
			}
		}
	}
	if (moreCubes || emptySlot) && hpf >= p.crateMinHp {
		for i := range b.mem.crates {
			it := &b.mem.crates[i]
			if b.mem.blacklisted(it.id, t) || it.hp <= 0.0 {
				continue
			}
			d := dist(it.pos, me.Pos)
			if d > p.crateMaxDist || !b.inPlay(obs, zi, it.pos, d) {
				continue
			}
			s := d - 20.0
			if b.goalId == it.id {
				s -= 20.0
			}
			if best == nil || s < bestS {
				c := up(it.pos, b.cfg.Crates.Height*0.5)
				los := world.ray(muzzle, c) == nil
				var g Goal
				if los && d < 120.0 {
					g = Goal{kind: GoalKindRange, rangePos: it.pos, rangePref: p.rangeCrate}
				} else {
					g = Goal{kind: GoalKindNav, navPos: it.pos, navArrive: p.rangeCrate}
				}
				bestS = s
				best = &struct {
					f0 Tgt
					f1 Goal
					f2 int64
				}{Tgt{kind: TgtKindCrate, crate0: it.id}, g, it.id}
			}
		}
	}
	return best
}

func (b *Brain) threatEyes(obs *Obs) []V3 {
	t := obs.T
	pos := obs.Me.Pos
	var v []struct {
		f0 float64
		f1 V3
	}
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > 3.0 {
			continue
		}
		tp := tr.posAt(t)
		s := 1.0 + b.cfg.Cubes.SizePerCube*fmax(tr.cubes, 0.0)
		v = append(v, struct {
			f0 float64
			f1 V3
		}{dist(tp, pos), up(tp, b.cfg.Mech.EyeHeight*s)})
	}
	if u := b.mem.unseen; u != nil {
		if t-u.t < 4.0 {
			q := V3{u.from[0] + math.Cos(u.bearing)*b.p.ghostRange, u.from[1] + math.Sin(u.bearing)*b.p.ghostRange, u.from[2] + b.cfg.Mech.EyeHeight}
			v = append(v, struct {
				f0 float64
				f1 V3
			}{b.p.ghostRange, q})
		}
	}
	sortStable(v, func(x, y *struct {
		f0 float64
		f1 V3
	}) int {
		return totalCmp(x.f0, y.f0)
	})
	if len(v) > 4 {
		v = v[:4]
	}
	out := make([]V3, 0, len(v))
	for _, x := range v {
		out = append(out, x.f1)
	}
	return out
}

func (b *Brain) coverGoal(obs *Obs, world *World, nav *Nav, zi *StrategyZoneInfo) Goal {
	p := b.p
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	if b.cover != nil {
		c, tc := b.cover.f0, b.cover.f1
		if t-tc < p.coverPeriodS {
			return Goal{kind: GoalKindNav, navPos: c, navArrive: 1.5}
		}
	}
	eyes := b.threatEyes(obs)
	if len(eyes) == 0 {
		b.cover = nil
		return Goal{}
	}
	h := fmax(me.Stats.Height, 1.0)
	hidden := func(q V3) bool {
		for _, e := range eyes {
			if !(!world.sight(e, up(q, h*0.5)) && !world.sight(e, up(q, h*0.92))) {
				return false
			}
		}
		return true
	}
	z := &obs.Zone
	var best *struct {
		f0 float64
		f1 V3
	}
	consider := func(q V3, bonus float64) {
		if dist2v(q, zi.curC) > zi.curR-12.0 {
			return
		}
		if !hidden(q) {
			return
		}
		outNext := fmax(dist2v(q, z.NextCenter)-z.NextRadius+10.0, 0.0)
		nearThreat := math.Inf(1)
		for _, e := range eyes {
			nearThreat = fmin(nearThreat, dist2(e, q))
		}
		s := dist(q, pos) + 0.6*outNext - 0.15*fmin(nearThreat, 150.0) - bonus
		if best == nil || s < best.f0 {
			best = &struct {
				f0 float64
				f1 V3
			}{s, q}
		}
	}
	if b.cover != nil {
		consider(b.cover.f0, 10.0)
	}
	consider(pos, 6.0)
	tAct := b.tAct
	for _, r := range [4]float64{12.0, 24.0, 40.0, 60.0} {
		if time.Since(tAct).Seconds() > 0.010 {
			break
		}
		for k := 0; k < 12; k++ {
			a := float64(k)*math.Pi/6.0 + r*0.1
			x := pos[0] + r*math.Cos(a)
			y := pos[1] + r*math.Sin(a)
			q0 := V3{x, y, fin(world.floorBelow(x, y, pos[2]+3.0), pos[2])}
			sq := nav.snap(q0)
			if sq == nil {
				continue
			}
			q := *sq
			if dist(q, q0) > 8.0 || !memoryAllFinite3(q) {
				continue
			}
			consider(q, 0.0)
		}
	}
	var q V3
	if best != nil {
		q = best.f1
	} else {
		e := eyes[0]
		away := unit2(pos[0]-e[0], pos[1]-e[1])
		tc := unit2(zi.safe[0]-pos[0], zi.safe[1]-pos[1])
		d := unit2(away[0]+0.4*tc[0], away[1]+0.4*tc[1])
		x := pos[0] + d[0]*p.fleeDist
		y := pos[1] + d[1]*p.fleeDist
		q0 := V3{x, y, fin(world.floorBelow(x, y, pos[2]+3.0), pos[2])}
		q = q0
		if sq := nav.snap(q0); sq != nil && memoryAllFinite3(*sq) {
			q = *sq
		}
	}
	b.cover = &struct {
		f0 V3
		f1 float64
	}{q, t}
	return Goal{kind: GoalKindNav, navPos: q, navArrive: 1.5}
}
