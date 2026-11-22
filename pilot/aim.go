package pilot

import "math"

type AimAimTarget struct {
	feet   V3
	vel    V3
	acc    V3
	radius float64
	height float64
	hp     float64
	fixed  bool
}

type AimFireOut struct {
	aim      V2
	fire     [2]bool
	reload   bool
	target   *AimAimTarget
	armed    bool
	charging bool
}

func (b *Brain) aimTarget(tr *Track) AimAimTarget {
	var radius, height float64
	if tr.kind == ActorKindNpc {
		radius, height = b.cfg.Npc.Radius, b.cfg.Npc.Height
	} else {
		s := 1.0 + b.cfg.Cubes.SizePerCube*fmax(tr.cubes, 0.0)
		radius, height = b.cfg.Mech.Radius*s, b.cfg.Mech.Height*s
	}
	return AimAimTarget{feet: tr.pos, vel: tr.vel, acc: tr.acc, radius: radius, height: height, hp: tr.hp, fixed: false}
}

func (b *Brain) idleAim(obs *Obs) V2 {
	t := obs.T
	if u := b.mem.unseen; u != nil {
		if t-u.t < 3.0 {
			return V2{u.bearing, 0.0}
		}
	}
	var best *struct {
		f0 float64
		f1 float64
	}
	for i := range b.mem.sounds {
		s := &b.mem.sounds[i]
		if t-s.t > 2.0 {
			continue
		}
		k := 0.0
		switch s.kind {
		case "gun", "laser", "laser_charge", "lightning", "mortar", "step", "boost", "jet", "reload":
			k = 1.0
		}
		d := 0.0
		switch s.dist {
		case SoundDistNear:
			d = 3.0
		case SoundDistMid:
			d = 1.5
		}
		pr := k*d - (t-s.t)*0.3
		if k*d > 0.0 && (best == nil || pr > best.f0) {
			best = &struct {
				f0 float64
				f1 float64
			}{pr, s.bearing}
		}
	}
	if best != nil {
		brg := best.f1
		if a := b.threatAim(obs); a != nil {
			return *a
		}
		return V2{brg, 0.0}
	}
	if a := b.threatAim(obs); a != nil {
		return *a
	}
	var base float64
	if math.Hypot(b.lastDir[0], b.lastDir[1]) > 0.1 {
		base = math.Atan2(b.lastDir[1], b.lastDir[0])
	} else {
		base = obs.Me.Aim[0]
	}
	ph := 2.0 * math.Pi * t / fmax(b.p.scanPeriod, 0.5)
	return V2{wrap(base + b.p.scanAmp*math.Sin(ph)), 0.0}
}

func (b *Brain) pickFireTarget(obs *Obs, world *World, muzzle V3) *struct {
	f0 AimAimTarget
	f1 bool
} {
	t := obs.T
	me := &obs.Me
	maxRange := 0.0
	for _, w := range me.Weapons {
		if w == nil {
			continue
		}
		var r float64
		switch w.Kind {
		case WeaponKindGun:
			r = w.Stats.Range * b.p.gunRangeFrac
		case WeaponKindMortar:
			if w.Stats.Gravity > 0.0 {
				r = w.Stats.Speed * w.Stats.Speed / w.Stats.Gravity * 0.95
			} else {
				r = 0.0
			}
		default:
			r = w.Stats.Range
		}
		maxRange = fmax(maxRange, r)
	}
	type cand struct {
		f0 float64
		f1 AimAimTarget
		f2 bool
	}
	var best *cand
	held := b.held(t)
	gate := b.gateClosed(obs)
	laserSlot := b.laserSlot(obs)
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if held || tr.kind != ActorKindMech || t-tr.lastSeen > 0.15 {
			continue
		}
		if gate && b.spareTr(tr, t, obs.Me.Pos) {
			continue
		}
		if !b.orderFireOk(tr, obs) {
			continue
		}
		if b.roofHold && t-tr.lastDmgT > 1.5 {
			continue
		}
		at := b.aimTarget(tr)
		c := up(at.feet, at.height*0.5)
		d := dist(c, muzzle)
		clear_ := world.ray(muzzle, c) == nil
		attacker := t-tr.lastDmgT < 3.0
		if b.sfHold(obs, tr) || b.gearSkipFire(tr, t, d, clear_) {
			continue
		}
		if !b.ffaFireOk(obs, tr, attacker, d, clear_) {
			continue
		}
		s := -0.3*d + 40.0*(1.0-frac(tr.hp, tr.maxHp))
		if clear_ {
			s += 100.0
		}
		if attacker {
			s += 40.0
		}
		if b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) {
			s += 50.0
		}
		s += b.tkBonus(tr)
		if d < maxRange {
			s += 50.0
		}
		s += b.finishBonus(tr)
		toUs := math.Atan2(me.Pos[1]-tr.pos[1], me.Pos[0]-tr.pos[0])
		aligned := math.Abs(adiff(tr.aim[0], toUs)) < 0.5
		var allow0 bool
		if b.econ.resting {
			allow0 = attacker || d < 45.0
		} else {
			allow0 = b.mode != ModeRetreat || attacker || (aligned && clear_) || d < 60.0
		}
		allow := allow0 && !b.holdFire(obs, d, tr.hp)
		if laserSlot != nil && allow && clear_ {
			w := me.Weapons[*laserSlot]
			if w.Cooldown <= 0.1 && d <= w.Stats.Range*0.98 {
				// Favor an easy beam while it is available. Moving targets qualify
				// when their projected path stays exposed through the charge.
				if len_(tr.vel) < 2 {
					s += 38
				} else {
					left := fclamp(w.Stats.Charge-w.Charge, 0.1, 0.5)
					if world.ray(muzzle, add(c, scale(tr.vel, left))) == nil {
						s += 22
					}
				}
			}
		}
		if best == nil || s > best.f0 {
			best = &cand{s, at, allow}
		}
	}
	if best != nil {
		return &struct {
			f0 AimAimTarget
			f1 bool
		}{best.f1, best.f2}
	}
	var bn *cand
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindNpc || t-tr.lastSeen > 0.15 {
			continue
		}
		attacker := t-tr.lastDmgT < 3.0
		hunted := b.target == (Tgt{kind: TgtKindNpc, npc0: tr.id}) || b.gearNpcOk(obs, tr)
		if (!attacker && !hunted) || (b.roofHold && t-tr.lastDmgT > 1.5) {
			continue
		}
		d := dist(tr.pos, me.Pos)
		hb := 0.0
		if hunted {
			hb = 50.0
		}
		s := -d + hb
		if bn == nil || s > bn.f0 {
			bn = &cand{s, b.aimTarget(tr), b.mode != ModeRetreat || attacker}
		}
	}
	if bn != nil {
		return &struct {
			f0 AimAimTarget
			f1 bool
		}{bn.f1, bn.f2}
	}
	if !b.allowCrates {
		return nil
	}
	var bc *struct {
		f0 float64
		f1 AimAimTarget
	}
	rays := 0
	for i := range b.mem.crates {
		it := &b.mem.crates[i]
		if t-it.lastSeen > 0.3 || it.hp <= 0.0 || b.mem.blacklisted(it.id, t) {
			continue
		}
		c := up(it.pos, b.cfg.Crates.Height*0.5)
		d := dist(c, muzzle)
		if d > fmin(maxRange, 170.0) {
			continue
		}
		cb := 0.0
		if b.target == (Tgt{kind: TgtKindCrate, crate0: it.id}) {
			cb = 60.0
		}
		s := -d + cb
		if (bc == nil || s > bc.f0) && rays < 8 {
			rays += 1
			if world.ray(muzzle, c) == nil {
				at := AimAimTarget{feet: it.pos, vel: V3{}, acc: V3{}, radius: b.cfg.Crates.Radius, height: b.cfg.Crates.Height, hp: it.hp, fixed: true}
				bc = &struct {
					f0 float64
					f1 AimAimTarget
				}{s, at}
			}
		}
	}
	if bc == nil {
		return nil
	}
	return &struct {
		f0 AimAimTarget
		f1 bool
	}{bc.f1, true}
}

func (b *Brain) mortarSolve(obs *Obs, world *World, tg *AimAimTarget, w *WeaponObs) *V2 {
	p := b.p
	st := &obs.Me.Stats
	tube := up(obs.Me.Pos, st.Height+0.5)
	v := w.Stats.Speed
	g := w.Stats.Gravity
	if !(v > 0.0) || !(g > 0.0) {
		return nil
	}
	vxy := V3{tg.vel[0], tg.vel[1], 0.0}
	speed := math.Hypot(vxy[0], vxy[1])
	if speed > p.mortarMaxTargetSpeed {
		return nil
	}
	for _, high := range [2]bool{false, true} {
		tf := dist2(tg.feet, tube) / (v * 0.9)
		var sol *struct {
			f0 float64
			f1 V3
			f2 float64
		}
		for it := 0; it < 3; it++ {
			tp := add(up(tg.feet, 0.5), scale(vxy, tf+p.latency))
			d := dist2(tp, tube)
			ms := mortarSolution(d, tp[2]-tube[2], v, g, high)
			if ms != nil {
				tf = ms.f1
				sol = &struct {
					f0 float64
					f1 V3
					f2 float64
				}{ms.f0, tp, d}
			} else {
				sol = nil
				break
			}
		}
		if sol == nil {
			continue
		}
		pitch, tp, d := sol.f0, sol.f1, sol.f2
		if d < p.mortarMinRange || pitch < st.MinPitch+0.01 || pitch > st.MaxPitch-0.01 {
			continue
		}
		if tf > p.mortarMaxFlight && speed > 2.0 && !tg.fixed {
			continue
		}
		if w.Stats.Ttl > 0.0 && tf > w.Stats.Ttl*0.95 {
			continue
		}
		yaw := math.Atan2(tp[1]-tube[1], tp[0]-tube[0])
		vel := scale(aimDir(yaw, pitch), v)
		prev := tube
		clear_ := true
		n := 10
		for k := 1; k <= n; k++ {
			tt := tf * 0.93 * float64(k) / float64(n)
			q := V3{tube[0] + vel[0]*tt, tube[1] + vel[1]*tt, tube[2] + vel[2]*tt - 0.5*g*tt*tt}
			if world.ray(prev, q) != nil {
				clear_ = false
				break
			}
			prev = q
		}
		if clear_ {
			return &V2{yaw, pitch}
		}
	}
	return nil
}

func (b *Brain) fireControl(obs *Obs, world *World) AimFireOut {
	p := b.p
	me := &obs.Me
	t := obs.T
	st := &me.Stats
	muzzle := up(me.Pos, st.MuzzleHeight)
	out := AimFireOut{aim: b.idleAim(obs), fire: [2]bool{false, false}, reload: false, target: nil, armed: false, charging: false}
	tgt := b.pickFireTarget(obs, world, muzzle)
	if u := b.mem.unseen; u != nil {
		attackedByTgt := false
		if tgt != nil {
			for i := range b.mem.tracks {
				tr := &b.mem.tracks[i]
				if t-tr.lastDmgT < 1.5 && t-tr.lastSeen < 0.3 {
					attackedByTgt = true
					break
				}
			}
		}
		if t-u.t < 1.2 && !attackedByTgt {
			if tgt != nil {
				tg := &tgt.f0
				c := up(tg.feet, tg.height*0.5)
				if world.ray(muzzle, c) != nil || dist(c, muzzle) > 60.0 {
					tgt = nil
				}
			}
			if tgt == nil {
				out.aim = V2{u.bearing, 0.0}
			}
		}
	}

	if b.held(t) {
		var face *struct {
			f0 float64
			f1 V3
		}
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind != ActorKindMech || t-tr.lastSeen > 2.5 {
				continue
			}
			tp := tr.posAt(t)
			d := dist(tp, me.Pos)
			db := 0.0
			if t-tr.lastDmgT < 3.0 {
				db = 200.0
			}
			s := d - db
			if t-tr.lastDmgT < 3.0 && (face == nil || s < face.f0) {
				face = &struct {
					f0 float64
					f1 V3
				}{s, tp}
			}
		}
		if face != nil {
			tp := face.f1
			tgt = nil
			out.aim = dirToAim(sub(up(tp, b.cfg.Mech.Height*0.5), muzzle))
		}
	}
	if tgt == nil {
		b.holdLostCharge(obs, &out)
		b.precharge(obs, &out)
		for _, w := range me.Weapons {
			if w == nil {
				continue
			}
			if w.Kind == WeaponKindGun && w.Reload <= 0.0 && w.Ammo < w.Stats.Mag {
				if w.Ammo <= w.Stats.Mag*p.reloadFrac || t-b.lastTargetT > 1.5 {
					out.reload = true
				}
			}
		}
		return out
	}
	tg := tgt.f0
	allow := tgt.f1
	b.lastTargetT = t
	c := up(tg.feet, tg.height*0.5)
	rel := sub(c, muzzle)
	d := fmax(len_(rel), 0.1)
	direct := dirToAim(add(rel, scale(tg.vel, p.laserLatency)))
	los := world.ray(muzzle, c) == nil
	tolY := math.Atan2(tg.radius*p.aimTol, d)
	tolP := math.Atan2(tg.height*0.5*p.aimTol, d)

	var gun *struct {
		f0 int
		f1 V2
		f2 bool
	}
	var laser *int
	var lightning *int
	var mortar *struct {
		f0 int
		f1 V2
	}
	for i, w := range me.Weapons {
		if w == nil {
			continue
		}
		switch w.Kind {
		case WeaponKindGun:
			var acc V3
			if tg.fixed {
				acc = V3{}
			} else {
				acc = tg.acc
			}
			zon := p.aimzOn > 0.0 && !tg.fixed
			var lg float64
			if zon {
				lg = 0.0
			} else {
				lg = p.accGain
			}
			if ld := lead(rel, tg.vel, acc, fmax(w.Stats.Speed, 1.0), p.latency, lg, p.accTime); ld != nil {
				pt, tf := ld.f0, ld.f1
				pzOk := true
				if zon {
					hl := fmax(math.Hypot(pt[0], pt[1]), 0.1)
					nn := V2{-pt[1] / hl, pt[0] / hl}
					hw := tg.radius + b.cfg.Projectile.GunRadius
					if q := b.aimz.query(t, tg.feet, tf+p.latency, nn, hw, p.aimzShrink, f2u(fmax(p.aimzMinN, 3.0)), f2u(fmax(p.aimzShotMin, 0.0)), p.aimzGshot > 0.0); q != nil {
						lat, pr := q.f0, q.f1
						cur := (pt[0]-rel[0])*nn[0] + (pt[1]-rel[1])*nn[1]
						pt[0] += nn[0] * (lat - cur)
						pt[1] += nn[1] * (lat - cur)
						pzOk = pr >= p.aimzPmin
					}
				}
				ok := pzOk && len_(pt) <= w.Stats.Range*p.gunRangeFrac && world.ray(muzzle, add(muzzle, pt)) == nil
				gun = &struct {
					f0 int
					f1 V2
					f2 bool
				}{i, dirToAim(pt), ok}
			}
		case WeaponKindLaser:
			laser = ptr(i)
		case WeaponKindLightning:
			lightning = ptr(i)
		case WeaponKindMortar:
			if w.Cooldown <= 0.25 && allow {
				if a := b.mortarSolve(obs, world, &tg, w); a != nil {
					mortar = &struct {
						f0 int
						f1 V2
					}{i, *a}
				} else {
					mortar = nil
				}
			}
			if w.Cooldown <= 1.0 {
				out.armed = true
			}
		case WeaponKindUnknown:
		}
	}

	laserI := -1
	var laserW *WeaponObs
	if laser != nil {
		if lw := me.Weapons[*laser]; lw != nil {
			laserI = *laser
			laserW = lw
		}
	}
	laserIn := laserW != nil && d <= laserW.Stats.Range*0.98 && los
	laserCharging := laserW != nil && laserW.Charge > 0.0
	releaseWindow := false
	if laserCharging && laserIn {
		w := laserW
		full := fmax(w.Stats.Charge, 0.05)
		frac := fclamp(w.Charge/full, 0, 1)
		damage := w.Stats.Damage * (w.Stats.MinFrac + (1-w.Stats.MinFrac)*frac)
		lethal := tg.hp > 0 && damage >= tg.hp*1.05
		closing := frac >= 0.60 && !tg.fixed && world.ray(muzzle, add(c, scale(tg.vel, 0.2))) != nil
		releaseWindow = w.Charge >= w.Stats.MinCharge && (lethal || closing)
	}
	gunOk := gun != nil && gun.f2
	var cmd V2
	if gun != nil {
		cmd = gun.f1
	} else {
		cmd = direct
	}
	laserFinal := false
	if laserW != nil {
		w := laserW
		full := fmax(w.Stats.Charge, 0.05)
		if laserCharging && (w.Charge >= full-0.2 || releaseWindow) {
			cmd = direct
			laserFinal = true
		} else if !gunOk && (laserCharging || laserIn) {
			cmd = direct
		}
	}
	mortarPrimary := false
	if !laserFinal {
		if mortar != nil {
			i, a := mortar.f0, mortar.f1
			if (me.Weapons[i] != nil && me.Weapons[i].Cooldown <= 0.11) || !gunOk {
				cmd = a
				mortarPrimary = true
			}
		}
	}
	if !gunOk && !mortarPrimary && !laserFinal {
		if lightning != nil {
			i := *lightning
			if me.Weapons[i] != nil && d <= me.Weapons[i].Stats.Range*1.3 {
				cmd = direct
			}
		}
	}
	pred := slew(me.Aim, cmd, st.TurnRate*0.05, st.PitchRate*0.05)
	on := func(a V2, ty float64, tp float64) bool {
		return math.Abs(adiff(pred[0], a[0]))*fmax(math.Cos(a[1]), 0.2) <= ty && math.Abs(pred[1]-a[1]) <= tp
	}

	if gun != nil {
		i, a, ok := gun.f0, gun.f1, gun.f2
		if w := me.Weapons[i]; w != nil {
			if w.Ammo <= 0.0 && w.Reload <= 0.0 {
				out.reload = true
			}
			if !ok && w.Reload <= 0.0 && w.Ammo <= w.Stats.Mag*p.reloadFrac && w.Ammo < w.Stats.Mag {
				out.reload = true
			}
			if w.Ammo > 0.0 && w.Reload <= 0.7 {
				out.armed = true
			}
			if allow && ok && w.Ammo > 0.0 && w.Reload <= 0.0 && on(a, tolY, tolP) {
				out.fire[i] = true
				if p.aimzOn > 0.0 && !tg.fixed {
					b.aimz.noteFire(t, tg.feet)
				}
			}
		}
	}
	if laserW != nil {
		i, w := laserI, laserW
		full := fmax(w.Stats.Charge, 0.05)
		if w.Cooldown <= 1.0 {
			out.armed = true
		}
		if !laserCharging {
			if allow && laserIn && w.Cooldown <= 0.1 {
				out.fire[i] = true
			}
		} else {
			ready := w.Charge >= full-0.051
			aligned := on(direct, tolY, tolP)
			release := (ready || releaseWindow) && aligned && laserIn
			giveUp := !laserIn && w.Charge >= full+fmax(w.Stats.HoldS-0.4, 0.0)
			out.fire[i] = !(release || giveUp)
		}
		out.charging = out.fire[i]
	}
	if lightning != nil {
		i := *lightning
		if w := me.Weapons[i]; w != nil {
			inRange := d <= w.Stats.Range*0.95 && dist(c, up(me.Pos, st.Height*0.5)) <= w.Stats.Range*0.95
			if inRange {
				out.armed = true
			}
			yawTo := math.Atan2(rel[1], rel[0])
			cone := w.Stats.ConeDeg * (math.Pi / 180.0) * p.lightningConeFrac
			if allow && inRange && los && math.Abs(adiff(pred[0], yawTo)) <= cone && me.Energy >= fmax(p.lightningMinEnergy, w.Stats.Energy) {
				out.fire[i] = true
			}
		}
	}
	if mortarPrimary {
		if mortar != nil {
			i, a := mortar.f0, mortar.f1
			if on(a, math.Atan2(tg.radius+1.5, d), 0.02) {
				out.fire[i] = true
			}
		}
	}
	out.aim = cmd
	out.target = ptr(tg)
	return out
}
