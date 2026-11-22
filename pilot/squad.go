package pilot

import "math"

type SquadRec struct {
	id          int64
	vr          float64
	hunterUntil float64
	eta         float64
}

type SquadBook struct {
	recs  []SquadRec
	field *struct {
		f0 V3
		f1 float64
		f2 *Field
	}
	shooters int
	hunters  []struct {
		f0 int64
		f1 float64
	}
}

func squadBookNew() SquadBook {
	return SquadBook{recs: []SquadRec{}, field: nil, shooters: 0, hunters: []struct {
		f0 int64
		f1 float64
	}{}}
}

func (k *SquadBook) rec(id int64) *SquadRec {
	for i := range k.recs {
		if k.recs[i].id == id {
			return &k.recs[i]
		}
	}
	return nil
}

func (k *SquadBook) isHunter(id int64, t float64) bool {
	r := k.rec(id)
	return r != nil && r.hunterUntil > t
}

func (b *Brain) sqUpdate(obs *Obs, nav *Nav) {
	p := b.p
	if p.sqOn < 0.5 {
		return
	}
	t := obs.T
	pos := obs.Me.Pos
	stale := func() bool {
		f := b.sq.field
		if f == nil {
			return true
		}
		q, tf := f.f0, f.f1
		return t-tf > 0.5 && dist(q, pos) > 4.0 || t-tf > 2.0
	}
	rebuild := stale()
	if r := fieldsvcPoll(&b.fsvc.sq); r != nil {
		k, tk := r.f0, r.f1
		b.sq.field = &struct {
			f0 V3
			f1 float64
			f2 *Field
		}{V3{k[0], k[1], k[2]}, tk, &r.f2}
	}
	rebuild = rebuild && stale()
	if rebuild && b.spentMs() < 4.0 {
		budget := b.budget
		shared := b.shared
		rng := p.sqEtaRange
		bpos := pos
		if f := fieldsvcRequest(&b.fsvc, 2, shared, [4]float64{pos[0], pos[1], pos[2], 0.0}, 4.0, t, &budget, 14.0, func(nav *Nav) Field {
			return nav.fieldLimited([]struct {
				f0 V3
				f1 float64
			}{{bpos, 0.0}}, -1.0, rng)
		}); f != nil {
			b.sq.field = &struct {
				f0 V3
				f1 float64
				f2 *Field
			}{pos, t, f}
		}
	}
	walk := fmax(obs.Me.Stats.WalkSpeed, 1.0)
	shooters := 0
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech {
			continue
		}
		if t-tr.lastDmgT < p.sqShooterS {
			shooters += 1
		}
		if b.sq.rec(tr.id) == nil {
			b.sq.recs = append(b.sq.recs, SquadRec{id: tr.id, vr: 0.0, hunterUntil: -1.0, eta: math.Inf(1)})
		}
		tp := tr.posAt(t)
		d := dist(tp, pos)
		fresh := t-tr.lastSeen < 0.15
		u := unit2(pos[0]-tp[0], pos[1]-tp[1])
		vrNow := tr.vel[0]*u[0] + tr.vel[1]*u[1]
		path := d * 1.3
		if b.sq.field != nil {
			if pd := nav.dist(b.sq.field.f2, tp); pd != nil && isFinite(*pd) {
				path = *pd
			}
		}
		var eta float64
		if fresh && t-tr.lastSeen < 0.15 && d < p.sqLosRange {
			eta = 0.0
		} else {
			eta = fmax(path-p.sqLosRange*0.5, 0.0) / walk
		}
		aimAtUs := fresh && math.Abs(adiff(tr.aim[0], math.Atan2(pos[1]-tp[1], pos[0]-tp[0]))) < p.sqAimTol && d < 220.0
		r := b.sq.rec(tr.id)
		if fresh {
			r.vr = r.vr*(1.0-p.sqVrAlpha) + vrNow*p.sqVrAlpha
		}
		r.eta = eta
		damaged := t-tr.lastDmgT < p.sqDmgMemoryS
		if damaged || (fresh && d < p.sqHunterRange && r.vr > p.sqHunterVr && d > 35.0) || (aimAtUs && tr.charging) {
			r.hunterUntil = t + p.sqHunterHoldS
		}
	}
	kept := b.sq.recs[:0]
	for _, r := range b.sq.recs {
		if r.hunterUntil > t-60.0 || isFinite(r.eta) {
			kept = append(kept, r)
		}
	}
	b.sq.recs = kept
	if len(b.sq.recs) > 32 {
		cut := len(b.sq.recs) - 32
		b.sq.recs = append(b.sq.recs[:0], b.sq.recs[cut:]...)
	}
	b.sq.shooters = shooters
	hs := []struct {
		f0 int64
		f1 float64
	}{}
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.sqTrackS {
			continue
		}
		if r := b.sq.rec(tr.id); r != nil {
			if r.hunterUntil > t {
				hs = append(hs, struct {
					f0 int64
					f1 float64
				}{tr.id, r.eta})
			}
		}
	}
	sortStable(hs, func(a, c *struct {
		f0 int64
		f1 float64
	}) int {
		return totalCmp(a.f1, c.f1)
	})
	if len(hs) > 4 {
		hs = hs[:4]
	}
	b.sq.hunters = hs
}

func (b *Brain) sqTtk(obs *Obs, tr *Track) float64 {
	_ = obs
	return fmax(tr.hp, 1.0) / fmax(b.p.sqOurDps, 1.0)
}

func (b *Brain) sqIsolated(obs *Obs, tr *Track) bool {
	t := obs.T
	tp := tr.posAt(t)
	for i := range b.mem.tracks {
		o := &b.mem.tracks[i]
		if o.kind == ActorKindMech && o.id != tr.id && t-o.lastSeen < b.p.sqTrackS && dist(o.posAt(t), tp) < b.p.sqIsoRange {
			return false
		}
	}
	return true
}

func (b *Brain) sqAllow(obs *Obs, tr *Track) bool {
	p := b.p
	if p.sqOn < 0.5 {
		return true
	}
	t := obs.T
	if t-tr.lastDmgT < 5.0 || b.sq.isHunter(tr.id, t) {
		return true
	}
	if t > p.sqLateT {
		return true
	}
	hpf := frac(obs.Me.Hp, obs.Me.MaxHp)
	ttk := b.sqTtk(obs, tr)
	firstH := math.Inf(1)
	if len(b.sq.hunters) > 0 {
		firstH = b.sq.hunters[0].f1
	}
	weak := frac(tr.hp, tr.maxHp) < p.sqAmbushTgtHp
	return hpf >= p.sqAmbushHp && (weak || ttk < p.sqAmbushTtk) && b.sqIsolated(obs, tr) && firstH > ttk+p.sqGapMargin
}

func (b *Brain) sqGate(obs *Obs, tgt *Track) bool {
	p := b.p
	if p.sqOn < 0.5 || p.sqGateOn < 0.5 {
		return false
	}
	t := obs.T
	hpf := frac(obs.Me.Hp, obs.Me.MaxHp)
	if hpf > p.sqGateMaxHp {
		return false
	}
	ttk := 0.0
	if tgt != nil {
		ttk = b.sqTtk(obs, tgt)
	}
	etas := []float64{}
	for _, h := range b.sq.hunters {
		if tgt == nil || tgt.id != h.f0 {
			etas = append(etas, h.f1)
		}
	}
	if tgt != nil {
		tr := tgt
		if t-tr.lastDmgT < 3.0 && tr.hp < obs.Me.Hp*p.sqFinishRatio {
			return false
		}
	}
	sortStable(etas, func(a, c *float64) int { return totalCmp(*a, *c) })
	nSoon := 0
	for _, e := range etas {
		if e < ttk+p.sqGapMargin {
			nSoon++
		}
	}
	return b.sq.shooters >= 2 || nSoon >= 1 && tgt != nil
}

func (b *Brain) sqRegen(obs *Obs) bool {
	p := b.p
	if p.sqOn < 0.5 || p.sqRegenOn < 0.5 || !obs.Me.InZone {
		return false
	}
	t := obs.T
	hpf := frac(obs.Me.Hp, obs.Me.MaxHp)
	fighting := false
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastDmgT < p.sqRegenFightS {
			fighting = true
			break
		}
	}
	return hpf < p.sqRegenHp && !fighting && t < p.sqRegenUntilT
}

func (b *Brain) sqAvoid(obs *Obs) bool {
	p := b.p
	if p.sqOn < 0.5 || p.sqAvoidOn < 0.5 || obs.T > p.sqLateT {
		return false
	}
	t := obs.T
	pos := obs.Me.Pos
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 1.0 && !b.sq.isHunter(tr.id, t) && dist(tr.posAt(t), pos) < p.sqAvoidRange {
			return true
		}
	}
	return false
}
