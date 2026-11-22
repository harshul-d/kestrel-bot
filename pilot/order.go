package pilot

import "math"

type OrderRole int

const (
	OrderRoleThreat OrderRole = iota
	OrderRolePest
	OrderRolePassive
)

type OrderOrder struct {
	hits []struct {
		f0 int64
		f1 float64
		f2 float64
	}
	hostile []struct {
		f0 int64
		f1 float64
	}
	came []struct {
		f0 int64
		f1 float64
	}
	aimSince []struct {
		f0 int64
		f1 float64
		f2 float64
	}
	halted bool
}

func (o *OrderOrder) update(obs *Obs) {
	t := obs.T
	me := obs.Me.Pos
	keptHits := o.hits[:0]
	for _, h := range o.hits {
		if t-h.f1 < 10.0 {
			keptHits = append(keptHits, h)
		}
	}
	o.hits = keptHits
	for i := range obs.Events {
		ev := &obs.Events[i]
		if ev.kind == EventKindDamaged {
			d := &ev.damaged0
			if d.Weapon == "zone" {
				continue
			}
			if d.By != nil {
				by := *d.By
				o.hits = append(o.hits, struct {
					f0 int64
					f1 float64
					f2 float64
				}{by, t, fmax(d.Amount, 0.0)})
				o.mark(by, t)
				orderOrderPut(&o.came, by, t)
			}
		}
	}
	for i := range obs.Mechs {
		a := &obs.Mechs[i]
		if a.Kind != ActorKindMech || a.Id == obs.Me.Id || !(isFinite(a.Pos[0]) && isFinite(a.Pos[1]) && isFinite(a.Pos[2])) {
			continue
		}
		dx := me[0] - a.Pos[0]
		dy := me[1] - a.Pos[1]
		d := math.Hypot(dx, dy)
		if d < 1e-6 {
			continue
		}
		vr := (a.Vel[0]*dx + a.Vel[1]*dy) / d
		aimd := math.Abs(adiff(a.Aim[0], math.Atan2(dy, dx)))
		if (vr > 2.5 && d < 250.0) || (aimd < 0.2 && d < 230.0) {
			o.mark(a.Id, t)
		}
		if aimd < 0.25 && vr > 0.5 && d < 250.0 {
			orderOrderPut(&o.came, a.Id, t)
		}
		if aimd < 0.25 && d < 150.0 {
			holding := false
			for _, x := range o.aimSince {
				if x.f0 == a.Id && t-x.f2 < 0.25 {
					holding = true
					break
				}
			}
			if !holding {
				kept := o.aimSince[:0]
				for _, x := range o.aimSince {
					if x.f0 != a.Id {
						kept = append(kept, x)
					}
				}
				o.aimSince = kept
				o.aimSince = append(o.aimSince, struct {
					f0 int64
					f1 float64
					f2 float64
				}{a.Id, t, t})
			} else {
				for j := range o.aimSince {
					if o.aimSince[j].f0 == a.Id {
						o.aimSince[j].f2 = t
						break
					}
				}
			}
		}
	}
}

func orderOrderPut(v *[]struct {
	f0 int64
	f1 float64
}, id int64, t float64) {
	for i := range *v {
		if (*v)[i].f0 == id {
			(*v)[i].f1 = t
			return
		}
	}
	*v = append(*v, struct {
		f0 int64
		f1 float64
	}{id, t})
}

func (o *OrderOrder) cameAt(id int64, t float64, holdS float64) bool {
	for _, h := range o.came {
		if h.f0 == id && t-h.f1 < holdS {
			return true
		}
	}
	return false
}

// onUs: distinct mechs that damaged us in the last 10 s or have held aim on us from within 150 m for 1 s.
func (o *OrderOrder) onUs(t float64) int {
	ids := make([]int64, 0, len(o.hits))
	for _, h := range o.hits {
		ids = append(ids, h.f0)
	}
	for _, x := range o.aimSince {
		if t-x.f2 < 0.25 && x.f2-x.f1 >= 1.0 {
			ids = append(ids, x.f0)
		}
	}
	sortStable(ids, func(a, b *int64) int {
		switch {
		case *a < *b:
			return -1
		case *a > *b:
			return 1
		}
		return 0
	})
	n := 0
	for i := range ids {
		if i == 0 || ids[i] != ids[i-1] {
			n++
		}
	}
	return n
}

func (o *OrderOrder) mark(id int64, t float64) {
	for i := range o.hostile {
		if o.hostile[i].f0 == id {
			o.hostile[i].f1 = t
			return
		}
	}
	o.hostile = append(o.hostile, struct {
		f0 int64
		f1 float64
	}{id, t})
}

func (o *OrderOrder) dmg(id int64) float64 {
	s := math.Copysign(0, -1) // float sum starts at -0.0
	for _, h := range o.hits {
		if h.f0 == id {
			s += h.f2
		}
	}
	return s
}

func (o *OrderOrder) role(id int64, t float64, pestDmg float64, holdS float64) OrderRole {
	if o.dmg(id) > pestDmg {
		return OrderRoleThreat
	}
	for _, h := range o.hostile {
		if h.f0 == id && t-h.f1 < holdS {
			return OrderRolePest
		}
	}
	return OrderRolePassive
}

func (b *Brain) orderOn() bool {
	return b.p.orderMode >= 1.0
}

func (b *Brain) roleOf(id int64, t float64) OrderRole {
	return b.order.role(id, t, b.p.pestDmg, b.p.orderHostileS)
}

func (b *Brain) pestOk(tr *Track, obs *Obs) bool {
	t := obs.T
	if t >= b.p.orderLateS {
		return true
	}
	d := dist(tr.posAt(t), obs.Me.Pos)
	if d < b.p.orderPestClose {
		return true
	}
	if b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) {
		return true
	}
	return b.p.orderSpare < 0.5 && !b.orderHasPassive
}

func (b *Brain) passiveOk(tr *Track, obs *Obs) bool {
	me := &obs.Me
	if frac(me.Hp, me.MaxHp) < b.p.orderAbortHp {
		return false
	}
	if b.p.orderSkipLaser >= 0.5 && tr.has(WeaponKindLaser) && obs.T < b.p.orderLateS {
		return false
	}
	if b.p.orderSkipCubes >= 0.5 && tr.cubes > me.Cubes && obs.T < b.p.orderLateS {
		return false
	}
	return true
}

// orderFireOk: whether fire control may shoot this mech when it is not the chosen target.
func (b *Brain) orderFireOk(tr *Track, obs *Obs) bool {
	t := obs.T
	if !b.orderOn() || b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) {
		return true
	}
	role := b.roleOf(tr.id, t)
	if b.p.orderMode >= 2.0 {
		return role == OrderRoleThreat || b.order.cameAt(tr.id, t, b.p.orderHostileS)
	}
	switch role {
	case OrderRoleThreat:
		return true
	case OrderRolePest:
		return b.pestOk(tr, obs)
	default:
		return b.passiveOk(tr, obs)
	}
}

func (b *Brain) chooseTargetOrder(obs *Obs, zi *StrategyZoneInfo) *Track {
	type cand struct {
		f0 float64
		f1 *Track
	}
	cloneTr := func(tr *Track) *Track {
		c := *tr
		c.weapons = append([]WeaponKind(nil), tr.weapons...)
		return &c
	}
	p := b.p
	t := obs.T
	pos := obs.Me.Pos
	z := &obs.Zone
	type liveT struct {
		f0 int64
		f1 V3
	}
	var live []liveT
	for i := range b.mem.tracks {
		x := &b.mem.tracks[i]
		if x.kind == ActorKindMech && t-x.lastSeen < 3.0 {
			live = append(live, liveT{x.id, x.posAt(t)})
		}
	}
	var bestThreat, bestPassive, bestPest *cand
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech {
			continue
		}
		role := b.order.role(tr.id, t, p.pestDmg, p.orderHostileS)
		if p.orderMode >= 2.0 && role != OrderRoleThreat && !b.order.cameAt(tr.id, t, p.orderHostileS) {
			continue
		}
		if role == OrderRolePassive && !b.passiveOk(tr, obs) {
			continue
		}
		var memS float64
		if role == OrderRolePassive {
			memS = p.orderMemoryS
		} else {
			memS = p.targetMemoryS
		}
		if t-tr.lastSeen > memS {
			continue
		}
		tp := tr.posAt(t)
		d := dist(tp, pos)
		attacker := t-tr.lastDmgT < 5.0
		if !attacker && b.mem.blacklisted(tr.id, t) {
			continue
		}
		var reach float64
		if role == OrderRolePassive {
			reach = p.orderMaxDist
		} else {
			reach = p.engageRange
		}
		if d > reach && !attacker {
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
		hpl := 1.0 - frac(tr.hp, tr.maxHp)
		s := 60.0*(1.0-d/200.0) + 80.0*hpl
		if b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) {
			s += p.targetSticky
		}
		if t-tr.lastSeen < p.freshS {
			s += 30.0
		}
		switch role {
		case OrderRoleThreat:
			s += 100.0 + b.order.dmg(tr.id)*0.1
			if bestThreat == nil || s > bestThreat.f0 {
				bestThreat = &cand{s, tr}
			}
		case OrderRolePassive:
			toUs := math.Atan2(pos[1]-tp[1], pos[0]-tp[0])
			s += 25.0 * (1.0 - math.Cos(adiff(tr.aim[0], toUs)))
			crowd := 0
			for _, l := range live {
				if l.f0 != tr.id && dist(l.f1, tp) < 80.0 {
					crowd++
				}
			}
			if crowd == 0 {
				s += 40.0
			}
			if bestPassive == nil || s > bestPassive.f0 {
				bestPassive = &cand{s, tr}
			}
		case OrderRolePest:
			if bestPest == nil || s > bestPest.f0 {
				bestPest = &cand{s, tr}
			}
		}
	}
	hasPassive := bestPassive != nil
	var pick *Track
	if bestThreat != nil {
		pick = cloneTr(bestThreat.f1)
	} else if p.orderMode >= 2.0 {
		if bestPest != nil {
			pick = cloneTr(bestPest.f1)
		} else if bestPassive != nil {
			pick = cloneTr(bestPassive.f1)
		}
	} else {
		late := t >= p.orderLateS
		var closePest *cand
		if bestPest != nil && dist(bestPest.f1.posAt(t), pos) < p.orderPestClose {
			closePest = bestPest
		}
		if closePest != nil {
			pick = cloneTr(closePest.f1)
		} else if late {
			switch {
			case bestPassive != nil && bestPest != nil:
				if bestPassive.f0 >= bestPest.f0 {
					pick = cloneTr(bestPassive.f1)
				} else {
					pick = cloneTr(bestPest.f1)
				}
			case bestPassive != nil:
				pick = cloneTr(bestPassive.f1)
			case bestPest != nil:
				pick = cloneTr(bestPest.f1)
			}
		} else if bestPassive != nil {
			pick = cloneTr(bestPassive.f1)
		} else if p.orderSpare < 0.5 {
			if bestPest != nil {
				pick = cloneTr(bestPest.f1)
			}
		}
	}
	b.orderHasPassive = hasPassive
	return pick
}
