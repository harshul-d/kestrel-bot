package pilot

func (b *Brain) holdLostCharge(obs *Obs, out *AimFireOut) {
	memS := b.p.lzHoldLost
	if memS <= 0.0 || b.mode == ModeRetreat || b.mode == ModeZone {
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
	if !(w.Charge > 0.0) || i > 1 {
		return
	}
	full := fmax(w.Stats.Charge, 0.05)
	if w.Charge >= full+fmax(w.Stats.HoldS-0.3, 0.0) {
		return
	}
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
		if tr.kind != ActorKindMech || t-tr.lastSeen > memS {
			continue
		}
		tp := tr.posAt(fmin(t, tr.lastSeen+1.0))
		d := dist(tp, me.Pos)
		if !(d <= w.Stats.Range) {
			continue
		}
		tb := 0.0
		if tr.id == tid {
			tb = 50.0
		}
		s := (t-tr.lastSeen)*100.0 + d*0.1 - tb
		if best == nil || s < best.f0 {
			best = &struct {
				f0 float64
				f1 V3
			}{s, tp}
		}
	}
	if best == nil {
		return
	}
	tp := best.f1
	out.fire[i] = true
	out.charging = true
	out.armed = true
	muzzle := up(me.Pos, me.Stats.MuzzleHeight)
	a := dirToAim(sub(up(tp, b.cfg.Mech.Height*0.5), muzzle))
	if isFinite(a[0]) && isFinite(a[1]) {
		out.aim = a
	}
}
