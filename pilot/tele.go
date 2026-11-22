package pilot

type TeleTele struct {
	kills  uint32
	nlDone bool
	wlDone bool
}

func (b *Brain) teleUpdate(obs *Obs) {
	for i := range obs.Events {
		ev := &obs.Events[i]
		if ev.kind == EventKindKill {
			b.tele.kills += 1
		}
	}
	has := false
	for _, w := range obs.Me.Weapons {
		if w != nil && w.Kind == WeaponKindLaser {
			has = true
			break
		}
	}
	if b.p.haltNl >= 0.0 && !b.tele.nlDone && obs.T >= b.p.haltNl {
		b.tele.nlDone = true
		if !has {
			b.order.halted = true
		}
	}
	if b.p.haltWl >= 0.0 && !b.tele.wlDone && obs.T >= b.p.haltWl {
		b.tele.wlDone = true
		if has {
			b.order.halted = true
		}
	}
	if b.p.haltK >= 1.0 && float64(b.tele.kills) >= b.p.haltK {
		b.order.halted = true
	}
}
