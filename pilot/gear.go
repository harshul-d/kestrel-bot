package pilot

import "math"

type GearGear struct {
	sweepOn   bool
	sweepAcc  float64
	sweepYaw  float64
	lastSweep float64
	laserGoal bool
	jetScan   bool
	pursue    struct {
		f0 int64
		f1 float64
	}
}

func (b *Brain) gearOn() bool {
	return b.p.gearOn >= 0.5
}

func (b *Brain) gearHarmless(tr *Track, t float64, d float64) bool {
	p := b.p
	if tr.kind != ActorKindMech {
		return true
	}
	if b.sfNotHarmless(tr) {
		return false
	}
	if b.order.dmg(tr.id) >= p.gearHarmlessDmg {
		return false
	}
	if tr.has(WeaponKindLaser) && d < p.gearLaserThreatR {
		return false
	}
	if t >= p.gearLateS {
		return false
	}
	return true
}

func (b *Brain) gearNearReal(obs *Obs) float64 {
	t := obs.T
	near := math.Inf(1)
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > 1.5 {
			continue
		}
		d := dist(tr.posAt(t), obs.Me.Pos)
		if !b.gearHarmless(tr, t, d) {
			near = fmin(near, d)
		}
	}
	return near
}

func (b *Brain) gearTgtHarmless(obs *Obs, tgt *Track) bool {
	if tgt == nil {
		return true
	}
	tr := tgt
	d := dist(tr.posAt(obs.T), obs.Me.Pos)
	return b.gearHarmless(tr, obs.T, d)
}

func brainHasKind(obs *Obs, k WeaponKind) bool {
	for _, w := range obs.Me.Weapons {
		if w != nil && w.Kind == k {
			return true
		}
	}
	return false
}

func (b *Brain) gearGoal(obs *Obs, zi *StrategyZoneInfo, tgt *Track) *struct {
	f0 int64
	f1 V3
} {
	type cand struct {
		f0 float64
		f1 int64
		f2 V3
		f3 bool
	}
	b.gear.laserGoal = false
	if !b.gearOn() || zi.urgent {
		return nil
	}
	p := b.p
	t := obs.T
	me := &obs.Me
	notWanted := func(k WeaponKind) bool {
		w, _ := b.want(me, k)
		return !w
	}
	if !b.gearTgtHarmless(obs, tgt) {
		return nil
	}
	real_ := b.gearNearReal(obs)
	tgtD := math.Inf(1)
	if tgt != nil {
		tgtD = dist(tgt.posAt(t), me.Pos)
	}
	var best *cand
	if !brainHasKind(obs, WeaponKindLaser) && real_ > p.gearSafeR {
		for i := range b.mem.pickups {
			it := &b.mem.pickups[i]
			if it.kind != WeaponKindLaser || b.mem.blacklisted(it.id, t) || notWanted(it.kind) {
				continue
			}
			d := dist(it.pos, me.Pos)
			if b.sfFarBlock(d) {
				continue
			}
			if d > p.gearLaserMax || !b.inPlayPub(obs, zi, it.pos, d) {
				continue
			}
			bonus := 0.0
			if it.id == b.goalId {
				bonus = 15.0
			}
			s := d - 400.0 - bonus
			if best == nil || s < best.f0 {
				best = &cand{s, it.id, it.pos, true}
			}
		}
	}
	emptySlot := false
	for _, w := range me.Weapons {
		if w == nil {
			emptySlot = true
			break
		}
	}
	if best == nil && emptySlot && real_ > p.gearSafeR && tgtD > p.gearFightKeep {
		for i := range b.mem.pickups {
			it := &b.mem.pickups[i]
			if b.mem.blacklisted(it.id, t) || notWanted(it.kind) {
				continue
			}
			d := dist(it.pos, me.Pos)
			if d > p.gearWeaponMax || !b.inPlayPub(obs, zi, it.pos, d) {
				continue
			}
			bonus := 0.0
			if it.id == b.goalId {
				bonus = 15.0
			}
			s := d - bonus
			if best == nil || s < best.f0 {
				best = &cand{s, it.id, it.pos, false}
			}
		}
	}
	cap_ := p.gearCubeCap
	wantsCube := me.Cubes < cap_ || (me.Cubes >= 13.0 && me.Cubes < 15.0)
	if best == nil && wantsCube && real_ > p.gearCubeSafe {
		for i := range b.mem.cubes {
			it := &b.mem.cubes[i]
			if b.mem.blacklisted(it.id, t) {
				continue
			}
			d := dist(it.pos, me.Pos)
			var lim float64
			if tgtD < p.gearFightKeep {
				lim = p.gearCubeFightR
			} else {
				lim = p.gearCubeR
			}
			if d > lim || !b.inPlayPub(obs, zi, it.pos, d) {
				continue
			}
			bonus := 0.0
			if it.id == b.goalId {
				bonus = 10.0
			}
			s := d - bonus
			if best == nil || s < best.f0 {
				best = &cand{s, it.id, it.pos, false}
			}
		}
	}
	if best == nil && wantsCube && real_ > p.gearCubeSafe && p.gearSiteR > 0.0 && tgtD > p.gearFightKeep {
		for _, s := range b.econ.sites {
			d := dist(s.f0, me.Pos)
			if t-s.f1 < 1e8 && t-s.f1 < p.gearSiteS && d < p.gearSiteR && d > 4.0 && b.inPlayPub(obs, zi, s.f0, d) && (best == nil || d < best.f0) {
				best = &cand{d, -1, s.f0, false}
			}
		}
	}
	if best == nil && wantsCube && p.gearNpcGoalR > 0.0 && real_ > p.gearSafeR && tgtD > p.gearFightKeep && frac(me.Hp, me.MaxHp) > 0.6 {
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind != ActorKindNpc || t-tr.lastSeen > 20.0 || b.mem.blacklisted(tr.id, t) {
				continue
			}
			d := dist(tr.pos, me.Pos)
			if d > p.gearNpcGoalR || !b.inPlayPub(obs, zi, tr.pos, d) || (best != nil && d >= best.f0) {
				continue
			}
			k := 0.0
			if d > 1e-6 {
				k = fmin(p.gearNpcStand/d, 1.0)
			}
			q := V3{tr.pos[0] + (me.Pos[0]-tr.pos[0])*k, tr.pos[1] + (me.Pos[1]-tr.pos[1])*k, me.Pos[2]}
			if d <= p.gearNpcStand+8.0 {
				q = me.Pos
			}
			best = &cand{d, tr.id, q, false}
		}
	}
	if best != nil && best.f1 >= 0 && !best.f3 && p.gearPursuitS > 0.0 {
		if b.gear.pursue.f0 != best.f1 {
			b.gear.pursue = struct {
				f0 int64
				f1 float64
			}{best.f1, t}
		} else if t-b.gear.pursue.f1 > p.gearPursuitS {
			b.mem.ban(best.f1, t+p.blacklistS)
			b.gear.pursue = struct {
				f0 int64
				f1 float64
			}{-1, t}
			return nil
		}
	}
	if best != nil {
		b.gear.laserGoal = best.f3
		return &struct {
			f0 int64
			f1 V3
		}{best.f1, best.f2}
	}
	return nil
}

func (b *Brain) gearGrab(obs *Obs, zi *StrategyZoneInfo) *struct {
	f0 int64
	f1 V3
} {
	type cand struct {
		f0 float64
		f1 int64
		f2 V3
	}
	p := b.p
	if !b.gearOn() || p.gearGrabR <= 0.0 || zi.urgent {
		return nil
	}
	t := obs.T
	me := &obs.Me
	notWanted := func(k WeaponKind) bool {
		w, _ := b.want(me, k)
		return !w
	}
	wantsCube := me.Cubes < p.gearCubeCap || (me.Cubes >= 13.0 && me.Cubes < 15.0)
	var best *cand
	for i := range b.mem.pickups {
		it := &b.mem.pickups[i]
		if it.kind != WeaponKindLaser || b.mem.blacklisted(it.id, t) || notWanted(it.kind) || math.Abs(it.pos[2]-me.Pos[2]) > 3.5 {
			continue
		}
		d := dist(it.pos, me.Pos)
		if d < p.gearGrabR && (best == nil || d < best.f0) {
			best = &cand{d, it.id, it.pos}
		}
	}
	if best == nil && wantsCube {
		for i := range b.mem.cubes {
			it := &b.mem.cubes[i]
			if b.mem.blacklisted(it.id, t) || math.Abs(it.pos[2]-me.Pos[2]) > 3.5 {
				continue
			}
			d := dist(it.pos, me.Pos)
			if d < p.gearGrabR && (best == nil || d < best.f0) {
				best = &cand{d, it.id, it.pos}
			}
		}
	}
	if best == nil {
		return nil
	}
	return &struct {
		f0 int64
		f1 V3
	}{best.f1, best.f2}
}

func (b *Brain) gearNpcOk(obs *Obs, tr *Track) bool {
	p := b.p
	if !b.gearOn() || p.gearNpcFireR <= 0.0 || b.mode == ModeRetreat {
		return false
	}
	me := &obs.Me
	d := dist(tr.pos, me.Pos)
	wantsCube := me.Cubes < p.gearCubeCap || (me.Cubes >= 13.0 && me.Cubes < 15.0)
	return wantsCube && d <= p.gearNpcFireR && frac(me.Hp, me.MaxHp) > 0.5
}

func (b *Brain) gearDropOk(obs *Obs, tgt *Track) bool {
	return b.gearOn() && b.p.gearDrop >= 0.5 && b.gearTgtHarmless(obs, tgt) && b.gearNearReal(obs) > b.p.gearSafeR
}

func (b *Brain) gearFarmMode(obs *Obs, tgt *Track) bool {
	if !b.gearOn() || b.p.gearFarm < 0.5 {
		return false
	}
	if tgt == nil {
		return false
	}
	tr := tgt
	d := dist(tr.posAt(obs.T), obs.Me.Pos)
	return b.gearHarmless(tr, obs.T, d) && d > b.p.gearFarmFar && b.gearNearReal(obs) > b.p.gearSafeR
}

func (b *Brain) gearSkipFire(tr *Track, t float64, d float64, clear_ bool) bool {
	if !b.gearOn() || b.p.gearFarm < 0.5 || !(b.mode == ModeHunt || b.mode == ModeLoot) {
		return false
	}
	if t-tr.lastDmgT < 1.0 && d < b.p.gearFarmFar {
		return false
	}
	return b.gearHarmless(tr, t, d) && (!clear_ || d > b.p.gearFarmFar)
}

func (b *Brain) gearRange(obs *Obs, tr *Track, pref float64) float64 {
	if !b.gearOn() || b.p.gearEasyRange <= 0.0 {
		return pref
	}
	d := dist(tr.posAt(obs.T), obs.Me.Pos)
	if b.gearHarmless(tr, obs.T, d) && !tr.has(WeaponKindLaser) {
		return fmin(pref, b.p.gearEasyRange)
	}
	return pref
}

func (b *Brain) gearJet(obs *Obs) *struct {
	f0 bool
	f1 bool
} {
	if !b.gear.sweepOn || !b.gear.jetScan {
		return nil
	}
	me := &obs.Me
	if me.Energy < b.p.gearJetStop || b.sfQuietBlock() {
		b.gear.jetScan = false
		return nil
	}
	return &struct {
		f0 bool
		f1 bool
	}{me.Grounded, !me.Grounded}
}

func (b *Brain) gearBoost(obs *Obs) bool {
	me := &obs.Me
	return b.gearOn() && b.p.gearBoostE > 0.0 && b.gear.laserGoal && b.mode == ModeLoot && me.Grounded && me.Energy > b.p.gearBoostE
}

func (b *Brain) gearSweep(obs *Obs, busy bool) *float64 {
	p := b.p
	if !b.gearOn() || p.gearSweepEvery <= 0.0 {
		return nil
	}
	t := obs.T
	yaw := obs.Me.Aim[0]
	calm := !busy && !b.sfQuietBlock() && t-b.mem.lastDamageT > 3.0
	if calm {
		anyNear := false
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind == ActorKindMech && t-tr.lastSeen < 2.0 {
				d := dist(tr.pos, obs.Me.Pos)
				if d < p.gearSweepMechR && (d < p.gearSweepAnyR || !b.gearHarmless(tr, t, d)) {
					anyNear = true
					break
				}
			}
		}
		calm = !anyNear
	}
	calm = calm && (b.mode == ModeIdle || b.mode == ModeLoot || b.mode == ModeHunt || b.mode == ModeZone || b.mode == ModeDrop)
	if b.gear.sweepOn {
		if !calm {
			b.gear.sweepOn = false
			return nil
		}
		b.gear.sweepAcc += math.Abs(adiff(yaw, b.gear.sweepYaw))
		b.gear.sweepYaw = yaw
		pi := math.Pi // runtime float64 arithmetic below (not exact constant folding)
		if b.gear.sweepAcc >= 2.0*pi-0.4 {
			b.gear.sweepOn = false
			b.gear.lastSweep = t
			return nil
		}
		return ptr(wrap(yaw + 1.5))
	}
	first := b.gear.lastSweep <= 0.0 && t < 3.0
	var every float64
	if p.gearSweepNolaser > 0.0 && !brainHasKind(obs, WeaponKindLaser) {
		every = p.gearSweepNolaser
	} else {
		every = p.gearSweepEvery
	}
	if calm && (first || t-b.gear.lastSweep > every) {
		b.gear.sweepOn = true
		b.gear.sweepAcc = 0.0
		b.gear.sweepYaw = yaw
		b.gear.lastSweep = t
		b.gear.jetScan = p.gearJetScan > 0.0 && obs.Me.Energy >= p.gearJetScan && (first || !brainHasKind(obs, WeaponKindLaser))
		return ptr(wrap(yaw + 1.5))
	}
	return nil
}
