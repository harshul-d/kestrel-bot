package pilot

import "math"

func brainEsLaser(obs *Obs) bool {
	for _, w := range obs.Me.Weapons {
		if w != nil && w.Kind == WeaponKindLaser {
			return true
		}
	}
	return false
}

func (b *Brain) esHold(obs *Obs, tr *Track) bool {
	p := b.p
	if p.esOn < 0.5 || tr.kind != ActorKindMech {
		return false
	}
	t := obs.T
	if t-tr.lastDmgT < p.esAttS {
		return false
	}
	me := &obs.Me
	hpf := frac(me.Hp, me.MaxHp)
	early := t < p.esUntil && !brainEsLaser(obs)
	weak := hpf < p.esHp
	if !(early || weak) {
		return false
	}
	d := dist(tr.posAt(t), me.Pos)
	if d < p.esFreeD && !tr.has(WeaponKindLaser) && tr.cubes <= p.esFreeCubes && !weak {
		return false
	}
	return true
}

func (b *Brain) esFilter(obs *Obs, fc *AimFireOut) {
	if b.p.esOn < 0.5 || b.p.esCone <= 0.0 {
		return
	}
	me := &obs.Me
	yaw := fc.aim[0]
	if !isFinite(yaw) {
		return
	}
	var best *struct {
		f0 float64
		f1 int64
	}
	for i := range obs.Mechs {
		a := &obs.Mechs[i]
		if a.Kind != ActorKindMech || a.Id == me.Id || !(isFinite(a.Pos[0]) && isFinite(a.Pos[1]) && isFinite(a.Pos[2])) {
			continue
		}
		dx := a.Pos[0] - me.Pos[0]
		dy := a.Pos[1] - me.Pos[1]
		d := math.Hypot(dx, dy)
		if d < 1.0 || d > 400.0 {
			continue
		}
		e := math.Abs(adiff(yaw, math.Atan2(dy, dx)))
		cone := fmax(b.p.esCone, math.Atan(6.0/d))
		if e < cone && (best == nil || d < best.f0) {
			best = &struct {
				f0 float64
				f1 int64
			}{d, a.Id}
		}
	}
	if best == nil {
		return
	}
	id := best.f1
	var tr *Track
	for i := range b.mem.tracks {
		if b.mem.tracks[i].id == id {
			tr = &b.mem.tracks[i]
			break
		}
	}
	if tr == nil {
		return
	}
	if !b.esHold(obs, tr) {
		return
	}
	for i := 0; i < len(me.Weapons) && i < 2; i++ {
		w := me.Weapons[i]
		if w != nil {
			if w.Kind == WeaponKindLaser && w.Charge > 0.0 {
				continue
			}
			fc.fire[i] = false
		}
	}
}
