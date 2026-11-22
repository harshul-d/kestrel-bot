package pilot

import "math"

type VxVx struct {
	lastLineT  float64
	nLine      int
	nStrong    int
	strongKill bool
	regroup    bool
	hold       bool
	kite       bool
	kiteLowT   float64
	kiteGoal   *struct {
		f0 V3
		f1 float64
	}
	eyes    []V3
	seeking bool
	skBan   []struct {
		f0 int64
		f1 float64
	}
}

func (b *Brain) vxStrong(tr *Track) bool {
	return tr.has(WeaponKindLaser) || b.sfNotHarmless(tr) || b.order.dmg(tr.id) >= b.p.gearHarmlessDmg
}

func (b *Brain) tkBonus(tr *Track) float64 {
	if b.p.tkOn < 0.5 || tr.kind != ActorKindMech || !b.vxStrong(tr) {
		return 0.0
	}
	return b.p.tkFocus * (1.0 - frac(tr.hp, tr.maxHp))
}

func (b *Brain) tkNoDetour() bool {
	return b.p.tkOn >= 0.5 && float64(b.wave.kills) >= b.p.tkKills
}

func (b *Brain) spareTr(tr *Track, t float64, pos V3) bool {
	if !b.spare(tr.hp, tr.maxHp) {
		return false
	}
	if b.p.readySpareHarmless < 0.5 {
		return true
	}
	return t-tr.lastDmgT > 5.0 || b.gearHarmless(tr, t, dist(tr.posAt(t), pos))
}

func (b *Brain) vxUpdate(obs *Obs, world *World) {
	p := b.p
	if p.rgOn < 0.5 && p.ktOn < 0.5 {
		return
	}
	t := obs.T
	me := &obs.Me
	if b.vx.lastLineT == 0.0 {
		b.vx.lastLineT = -1e9
	}
	for i := range obs.Events {
		ev := &obs.Events[i]
		if ev.kind == EventKindKill {
			k := &ev.kill0
			for j := range b.mem.tracks {
				tr := &b.mem.tracks[j]
				if tr.kind == ActorKindMech && tr.id == k.Target {
					if b.vxStrong(tr) {
						b.vx.strongKill = true
					}
					break
				}
			}
		}
	}
	body := up(me.Pos, fmax(me.Stats.Height, 1.0)*0.6)
	eyeH := b.cfg.Mech.EyeHeight
	n := 0
	ns := 0
	b.vx.eyes = b.vx.eyes[:0]
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech {
			continue
		}
		hit := t-tr.lastDmgT < 0.5
		fresh := t-tr.lastSeen < 0.3
		if !hit && !fresh {
			continue
		}
		e := up(tr.posAt(t), eyeH)
		if hit || (dist(e, body) < 320.0 && world.sight(e, body)) {
			n += 1
			if b.vxStrong(tr) {
				ns += 1
				if len(b.vx.eyes) < 6 {
					b.vx.eyes = append(b.vx.eyes, e)
				}
			}
		}
	}
	b.vx.nLine = n
	b.vx.nStrong = ns
	if n > 0 || t-b.mem.lastDamageT < 0.3 {
		b.vx.lastLineT = t
	}
	hpf := frac(me.Hp, me.MaxHp)
	if p.rgOn >= 0.5 {
		quiet := t-b.vx.lastLineT >= p.rgQuietS && t-b.mem.lastDamageT >= p.rgQuietS
		low := hpf < p.rgHp || (b.vx.strongKill && p.rgStrong >= 0.5 && hpf < p.rgExit)
		if hpf >= p.rgExit {
			b.vx.strongKill = false
		}
		if b.vx.regroup {
			b.vx.regroup = quiet && hpf < p.rgExit
		} else {
			b.vx.regroup = quiet && low && me.InZone
		}
		b.vx.hold = b.vx.regroup
	} else {
		b.vx.strongKill = false
	}
	if p.ktOn >= 0.5 {
		kills := float64(b.wave.kills)
		if kills < p.ktKills {
			b.vx.kite = false
		} else if ns >= f2u(fmax(p.ktN, 1.0)) {
			b.vx.kite = true
			b.vx.kiteLowT = t
		} else if b.vx.kite && ns >= 2 {
			b.vx.kiteLowT = t
		} else if b.vx.kite && t-b.vx.kiteLowT > p.ktExitS {
			b.vx.kite = false
		}
	}
}

func (b *Brain) kiteGoal(obs *Obs, world *World, nav *Nav, zi *StrategyZoneInfo) Goal {
	t := obs.T
	pos := obs.Me.Pos
	if kg := b.vx.kiteGoal; kg != nil {
		q, tq := kg.f0, kg.f1
		if t-tq < 0.5 && dist(q, pos) > 6.0 {
			return Goal{kind: GoalKindNav, navPos: q, navArrive: 2.0}
		}
	}
	if len(b.vx.eyes) == 0 {
		b.vx.kiteGoal = nil
		return Goal{}
	}
	k := float64(len(b.vx.eyes))
	sx := math.Copysign(0, -1)
	for _, e := range b.vx.eyes {
		sx += e[0]
	}
	cx := sx / k
	sy := math.Copysign(0, -1)
	for _, e := range b.vx.eyes {
		sy += e[1]
	}
	cy := sy / k
	away := unit2(pos[0]-cx, pos[1]-cy)
	tc := unit2(zi.safe[0]-pos[0], zi.safe[1]-pos[1])
	base := math.Atan2(away[1], away[0])
	h := fmax(obs.Me.Stats.Height, 1.0)
	r := fmax(b.p.ktStep, 10.0)
	var best *struct {
		f0 float64
		f1 V3
	}
	for _, da := range [5]float64{0.0, 0.5, -0.5, 1.0, -1.0} {
		if b.spentMs() > 9.0 {
			break
		}
		a := base + da
		x := pos[0] + r*math.Cos(a)
		y := pos[1] + r*math.Sin(a)
		q0 := V3{x, y, fin(world.floorBelow(x, y, pos[2]+3.0), pos[2])}
		qp := nav.snap(q0)
		if qp == nil {
			continue
		}
		q := *qp
		if dist(q, q0) > 8.0 || !(isFinite(q[0]) && isFinite(q[1]) && isFinite(q[2])) || dist2v(q, zi.curC) > zi.curR-12.0 {
			continue
		}
		seenN := 0
		for _, e := range b.vx.eyes {
			if world.sight(e, up(q, h*0.6)) {
				seenN++
			}
		}
		seen := float64(seenN)
		dir := unit2(q[0]-pos[0], q[1]-pos[1])
		s := seen*2.0 + math.Abs(da)*0.5 - 0.6*(dir[0]*tc[0]+dir[1]*tc[1])
		if best == nil || s < best.f0 {
			best = &struct {
				f0 float64
				f1 V3
			}{s, q}
		}
	}
	if best != nil {
		q := best.f1
		b.vx.kiteGoal = &struct {
			f0 V3
			f1 float64
		}{q, t}
		return Goal{kind: GoalKindNav, navPos: q, navArrive: 2.0}
	}
	b.vx.kiteGoal = nil
	return Goal{}
}

func (b *Brain) dpPick(obs *Obs, drop *struct {
	f0 V3
	f1 float64
	f2 float64
}, tgt *Track, wantRetreat bool, hpf float64) *struct {
	f0 V3
	f1 float64
	f2 float64
} {
	p := b.p
	if p.dpOn < 0.5 {
		return nil
	}
	if drop == nil {
		return nil
	}
	d := drop
	if d.f2 > p.dpDist {
		return nil
	}
	// Do not abandon a live fight for a drop unless the ring is right here and the foe is weak.
	strongFight := tgt != nil && obs.T-tgt.lastSeen < 1.0 && (b.vxStrong(tgt) || d.f2 > 40.0)
	leave := hpf < p.dpLeaveHp && d.f2 < p.dpLeaveR
	if leave || (!wantRetreat && !strongFight) {
		return d
	}
	return nil
}

func (b *Brain) seekGoal(obs *Obs, zi *StrategyZoneInfo) *struct {
	f0 int64
	f1 V3
} {
	type cand struct {
		f0 float64
		f1 int64
		f2 V3
	}
	p := b.p
	b.vx.seeking = false
	if p.skOn < 0.5 {
		return nil
	}
	me := &obs.Me
	t := obs.T
	if !brainHasKind(obs, WeaponKindLaser) || frac(me.Hp, me.MaxHp) < p.skMinHp {
		return nil
	}
	keptBan := b.vx.skBan[:0]
	for _, x := range b.vx.skBan {
		if x.f1 > t {
			keptBan = append(keptBan, x)
		}
	}
	b.vx.skBan = keptBan
	pos := me.Pos
	var best *cand
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > p.skMemS {
			continue
		}
		banned := false
		for _, x := range b.vx.skBan {
			if x.f0 == tr.id {
				banned = true
				break
			}
		}
		if banned {
			continue
		}
		q := tr.posAt(t)
		d := dist(q, pos)
		if d > p.skDist || !b.inPlayPub(obs, zi, q, fmin(d, 200.0)) {
			continue
		}
		if d < 20.0 && t-tr.lastSeen > 2.0 {
			continue
		}
		iso := true
		for j := range b.mem.tracks {
			o := &b.mem.tracks[j]
			if o.kind == ActorKindMech && o.id != tr.id && t-o.lastSeen < 5.0 && dist(o.posAt(t), q) < 60.0 {
				iso = false
				break
			}
		}
		isoBonus := 0.0
		if iso {
			isoBonus = 30.0
		}
		s := d + 80.0*frac(tr.hp, tr.maxHp) + (t-tr.lastSeen)*3.0 - isoBonus
		if b.goalId == tr.id {
			s -= 25.0
		}
		if best == nil || s < best.f0 {
			best = &cand{s, tr.id, q}
		}
	}
	if best == nil {
		sc := &b.cfg.Sound
		for i := range b.mem.sounds {
			h := &b.mem.sounds[i]
			if h.kind == "npc" || t-h.t > 4.0 {
				continue
			}
			var r float64
			switch h.dist {
			case SoundDistNear:
				r = sc.Near * 0.7
			case SoundDistMid:
				r = (sc.Near + sc.Mid) * 0.5
			default:
				r = sc.Mid * 1.3
			}
			r = fmin(fmax(r, 20.0), p.skDist)
			q := V3{pos[0] + r*math.Cos(h.bearing), pos[1] + r*math.Sin(h.bearing), pos[2]}
			if !(isFinite(q[0]) && isFinite(q[1]) && isFinite(q[2])) || !b.inPlayPub(obs, zi, q, r) {
				continue
			}
			s := r + (t-h.t)*10.0 + 100.0
			if best == nil || s < best.f0 {
				best = &cand{s, -2, q}
			}
		}
	}
	if best != nil {
		id, q := best.f1, best.f2
		if id >= 0 && dist(q, pos) < 20.0 {
			b.vx.skBan = append(b.vx.skBan, struct {
				f0 int64
				f1 float64
			}{id, t + 8.0})
		}
		b.vx.seeking = true
		return &struct {
			f0 int64
			f1 V3
		}{id, q}
	}
	return nil
}
