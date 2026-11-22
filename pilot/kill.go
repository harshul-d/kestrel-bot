package pilot

func (b *Brain) finishBonus(tr *Track) float64 {
	f := frac(tr.hp, tr.maxHp)
	if f < b.p.finishHp {
		return b.p.finishBonus * fclamp(1.0-f/fmax(b.p.finishHp, 0.01), 0.3, 1.0)
	}
	return 0.0
}

func (b *Brain) rushOk(obs *Obs, tr *Track) bool {
	t := obs.T
	tp := tr.posAt(t)
	others := 0
	for i := range b.mem.tracks {
		o := &b.mem.tracks[i]
		if o.kind != ActorKindMech || o.id == tr.id || t-o.lastSeen > 3.0 {
			continue
		}
		if dist(o.posAt(t), tp) < b.p.rushAloneR || t-o.lastDmgT < 2.5 {
			others += 1
		}
	}
	if others > 0 {
		return false
	}
	need := fmin(fmax(tr.hp, 0.0)/20.0, b.p.rushEnergyCap) * b.p.rushEnergyFrac
	return obs.Me.Energy >= need
}

func (b *Brain) precharge(obs *Obs, out *AimFireOut) {
	if b.p.precharge <= 0.0 || b.mode == ModeRetreat || b.mode == ModeZone {
		return
	}
	me := &obs.Me
	t := obs.T
	i := -1
	var w *WeaponObs
	for k, wk := range me.Weapons {
		if wk != nil && wk.Kind == WeaponKindLaser {
			i, w = k, wk
			break
		}
	}
	if w == nil {
		return
	}
	full := fmax(w.Stats.Charge, 0.05)
	var tid int64
	if b.target.kind == TgtKindMech {
		tid = b.target.mech0
	} else {
		tid = -1
	}
	var best *struct {
		f0 float64
		f1 V3
	}
	for k := range b.mem.tracks {
		tr := &b.mem.tracks[k]
		if tr.kind != ActorKindMech || t-tr.lastSeen > b.p.prechargeMemoryS {
			continue
		}
		tp := tr.posAt(fmin(t, tr.lastSeen+1.0))
		d := dist(tp, me.Pos)
		if d > w.Stats.Range*0.9 {
			continue
		}
		tb := 0.0
		if tr.id == tid {
			tb = 100.0
		}
		s := d - tb
		if best == nil || s < best.f0 {
			best = &struct {
				f0 float64
				f1 V3
			}{s, tp}
		}
	}
	if w.Charge > 0.0 {
		if w.Charge < full+fmax(w.Stats.HoldS-0.3, 0.0) {
			out.fire[i] = true
			out.charging = true
		}
	} else if w.Cooldown <= 0.1 && best != nil {
		out.fire[i] = true
		out.charging = true
	}
	if best != nil {
		tp := best.f1
		c := up(tp, b.cfg.Mech.Height*0.5)
		muzzle := up(me.Pos, me.Stats.MuzzleHeight)
		out.aim = dirToAim(sub(c, muzzle))
		out.armed = true
	}
}
