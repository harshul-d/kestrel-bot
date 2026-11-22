package pilot

type SafetySafety struct {
	dealt []struct {
		f0 int64
		f1 float64
	}
	exposed bool
	threat  bool
}

func (b *Brain) sfUpdate(obs *Obs, world *World) {
	p := b.p
	t := obs.T
	for i := range obs.Events {
		ev := &obs.Events[i]
		if ev.kind == EventKindDamaged {
			d := &ev.damaged0
			if d.By != nil {
				by := *d.By
				found := false
				for j := range b.sf.dealt {
					x := &b.sf.dealt[j]
					if x.f0 == by {
						x.f1 += fmax(d.Amount, 0.0)
						found = true
						break
					}
				}
				if !found {
					b.sf.dealt = append(b.sf.dealt, struct {
						f0 int64
						f1 float64
					}{by, fmax(d.Amount, 0.0)})
				}
			}
		}
	}
	b.sf.exposed = false
	b.sf.threat = false
	if p.sfOpenR <= 0.0 && p.sfSweep < 0.5 {
		return
	}
	me := obs.Me.Pos
	eye := up(me, obs.Me.Stats.Height*0.6)
	exposed := false
	threat := false
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.sfMemS {
			continue
		}
		q := tr.posAt(t)
		d := dist(q, me)
		if d > fmax(p.sfLosR, p.sfSweepR) || b.gearHarmless(tr, t, d) {
			continue
		}
		if !world.sight(up(q, b.cfg.Mech.EyeHeight), eye) {
			continue
		}
		if d <= p.sfLosR {
			exposed = true
		}
		if d <= p.sfSweepR {
			threat = true
		}
	}
	b.sf.exposed = exposed
	b.sf.threat = threat
}

func (b *Brain) sfDealt(id int64) float64 {
	for _, x := range b.sf.dealt {
		if x.f0 == id {
			return x.f1
		}
	}
	return 0.0
}

func (b *Brain) sfNotHarmless(tr *Track) bool {
	p := b.p
	if p.sfHarm < 0.5 {
		return false
	}
	return tr.has(WeaponKindLaser) || tr.cubes > p.sfHarmCubes || b.sfDealt(tr.id) >= p.sfHarmDmg
}

func brainSfHasLaser(obs *Obs) bool {
	for _, w := range obs.Me.Weapons {
		if w != nil && w.Kind == WeaponKindLaser {
			return true
		}
	}
	return false
}

func (b *Brain) sfHunter(tr *Track, t float64) bool {
	return t-tr.lastDmgT < b.p.sfHuntS || b.order.role(tr.id, t, 0.0, b.p.sfHuntS) != OrderRolePassive
}

func (b *Brain) sfHold(obs *Obs, tr *Track) bool {
	if b.esHold(obs, tr) {
		return true
	}
	p := b.p
	if p.sfNofirst < 0.5 || tr.kind != ActorKindMech || brainSfHasLaser(obs) {
		return false
	}
	t := obs.T
	if b.sfHunter(tr, t) {
		return false
	}
	d := dist(tr.posAt(t), obs.Me.Pos)
	return tr.has(WeaponKindLaser) || d > p.sfNfClose
}

func (b *Brain) sfNoMortar() bool {
	return b.p.sfMortar >= 0.5
}

func (b *Brain) sfDrop(obs *Obs, act *Action) {
	if b.p.sfMortar < 1.5 || act.Take != nil || act.Drop != nil {
		return
	}
	for i, w := range obs.Me.Weapons {
		if w != nil && w.Kind == WeaponKindMortar {
			act.Drop = ptr(int64(i))
			return
		}
	}
}

func (b *Brain) sfQuietBlock() bool {
	return b.p.sfSweep >= 0.5 && b.sf.threat
}

func (b *Brain) sfFarBlock(d float64) bool {
	return b.p.sfOpenR > 0.0 && d > b.p.sfOpenR && b.sf.exposed
}
