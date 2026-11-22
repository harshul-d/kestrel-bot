package pilot

import (
	"math"
	"os"
)

type LaserPkSt int

const (
	LaserPkStOff LaserPkSt = iota
	LaserPkStHide
	LaserPkStOut
	LaserPkStBack
)

type LaserPeek struct {
	st     LaserPkSt
	tid    int64
	hide   V3
	peek   V3
	planT  float64
	stT    float64
	failT  float64
	shots  uint32
	dbg    bool
	lastSt LaserPkSt
	ebeamT float64
	dodge  *struct {
		f0 V3
		f1 float64
	}
}

func laserPeekNew() LaserPeek {
	_, dbg := os.LookupEnv("LZ_DEBUG")
	return LaserPeek{st: LaserPkStOff, tid: -1, hide: V3{}, peek: V3{}, planT: -1e9, stT: 0.0, failT: -1e9, shots: 0, dbg: dbg, lastSt: LaserPkStOff, ebeamT: -1e9, dodge: nil}
}

type LaserNode struct {
	q      V3
	cov    bool
	clr    bool
	seenBy uint32
}

func (b *Brain) laserSlot(obs *Obs) *int {
	for i, w := range obs.Me.Weapons {
		if w != nil && w.Kind == WeaponKindLaser {
			return ptr(i)
		}
	}
	return nil
}

func (b *Brain) laserPre(obs *Obs, world *World, nav *Nav) {
	p := b.p
	t := obs.T
	pos := obs.Me.Pos
	if p.lzSeek > 0.0 && b.laserSlot(obs) == nil && b.mode != ModeZone && (b.mode != ModeRetreat || p.lzSeekRetreat > 0.5) {
		if wanted, _ := b.want(&obs.Me, WeaponKindLaser); wanted {
			z := &obs.Zone
			hot := t-b.mem.lastDamageT < p.lzHotS
			if !hot {
				for i := range b.mem.tracks {
					tr := &b.mem.tracks[i]
					if tr.kind == ActorKindMech && t-tr.lastSeen < p.lzHotS && dist(tr.posAt(t), pos) < p.lzHotD {
						hot = true
						break
					}
				}
			}
			var best *struct {
				f0 float64
				f1 int64
				f2 V3
			}
			for i := range b.mem.pickups {
				it := &b.mem.pickups[i]
				if it.kind != WeaponKindLaser || b.mem.blacklisted(it.id, t) {
					continue
				}
				d := dist(it.pos, pos)
				var lim float64
				if hot {
					lim = p.lzSeekFight
				} else {
					lim = p.lzSeek
				}
				if b.sfFarBlock(d) {
					continue
				}
				if d > lim || dist2v(it.pos, z.Center) > z.Radius-8.0 {
					continue
				}
				if best == nil || d < best.f0 {
					best = &struct {
						f0 float64
						f1 int64
						f2 V3
					}{d, it.id, it.pos}
				}
			}
			if best != nil {
				id, q := best.f1, best.f2
				b.mode = ModeLoot
				b.goal = Goal{kind: GoalKindNav, navPos: q, navArrive: 1.2}
				if b.goalId != id {
					b.pursuit = struct {
						f0 int64
						f1 float64
					}{id, t}
				}
				b.goalId = id
			}
		}
	}
	if p.lzPeek <= 0.0 {
		b.pk.st = LaserPkStOff
		return
	}
	if b.laserSlot(obs) == nil {
		b.pk.st = LaserPkStOff
		return
	}
	var tid int64
	if b.target.kind == TgtKindMech {
		tid = b.target.mech0
	} else {
		tid = -1
	}
	if tid < 0 && p.lzAnyTgt > 0.5 {
		var best *struct {
			f0 float64
			f1 int64
		}
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind != ActorKindMech || t-tr.lastSeen > p.lzMemS {
				continue
			}
			a1 := 0.0
			if t-tr.lastDmgT < 3.0 {
				a1 = 100.0
			}
			a2 := 0.0
			if tr.has(WeaponKindLaser) {
				a2 = 50.0
			}
			s := dist(tr.posAt(t), pos) - a1 - a2
			if best == nil || s < best.f0 {
				best = &struct {
					f0 float64
					f1 int64
				}{s, tr.id}
			}
		}
		if best != nil {
			tid = best.f1
		}
	}
	var tr *Track
	for i := range b.mem.tracks {
		if b.mem.tracks[i].id == tid && b.mem.tracks[i].kind == ActorKindMech {
			c := b.mem.tracks[i]
			tr = &c
			break
		}
	}
	ok := b.mode == ModeFight || (b.mode == ModeRetreat && p.lzRetreat > 0.5) || (p.lzAnyTgt > 0.5 && (b.mode == ModeLoot || b.mode == ModeHunt || b.mode == ModeIdle || b.mode == ModeDrop)) || (b.pk.st != LaserPkStOff && b.mode != ModeZone)
	if tr != nil && !(ok && t-tr.lastSeen < p.lzMemS) {
		tr = nil
	}
	if tr == nil {
		if b.pk.dbg && f2i64(math.Round(t*10.0))%20 == 0 {
			logf("LZ t=%.2f no tgt mode=%v target=%v ok=%v\n", t, b.mode, b.target, ok)
		}
		b.pk.st = LaserPkStOff
		return
	}
	tp := tr.posAt(t)
	dt := dist(tp, pos)
	if dt < p.lzMinD || dt > p.lzMaxD || obs.Me.Hp < obs.Me.MaxHp*p.lzMinHp {
		if b.pk.dbg {
			logf("LZR range %.0f\n", dt)
		}
		b.pk.st = LaserPkStOff
		return
	}
	if tr.id != b.pk.tid {
		b.pk.tid = tr.id
		b.pk.planT = -1e9
	}
	replan := t-b.pk.planT >= p.lzReplanS && b.pk.st != LaserPkStOut
	if replan && t-b.pk.failT > 0.5 {
		b.pk.planT = t
		if pl := b.peekPlan(obs, world, nav, tp, tr.id); pl != nil {
			b.pk.hide = pl.f0
			b.pk.peek = pl.f1
			if b.pk.st == LaserPkStOff {
				b.pk.st = LaserPkStHide
				b.pk.stT = t
			}
		} else {
			if b.pk.dbg {
				logf("LZ t=%.2f plan fail d=%.0f\n", t, dt)
			}
			b.pk.failT = t
			b.pk.st = LaserPkStOff
		}
	}
	if b.pk.st != LaserPkStOff {
		b.goal = Goal{kind: GoalKindNav, navPos: b.pk.hide, navArrive: 0.8}
	}
}

func (b *Brain) peekPlan(obs *Obs, world *World, nav *Nav, tp V3, tid int64) *struct {
	f0 V3
	f1 V3
} {
	p := b.p
	pos := obs.Me.Pos
	t := obs.T
	h := b.cfg.Mech.Height
	te := up(tp, b.cfg.Mech.EyeHeight)
	tm := up(tp, b.cfg.Mech.MuzzleHeight)
	tc := up(tp, h*0.5)
	var others []V3
	for i := range b.mem.tracks {
		o := &b.mem.tracks[i]
		if !(o.kind == ActorKindMech && o.id != tid && t-o.lastSeen < 4.0) {
			continue
		}
		e := up(o.posAt(t), b.cfg.Mech.EyeHeight)
		if dist(e, pos) < 260.0 {
			others = append(others, e)
		}
	}
	g := nav.grid()
	cnx, cny, o := g.f0, g.f1, g.f2
	r := fmax(p.lzR, 4.0)
	cnxm := 0
	if cnx > 0 {
		cnxm = cnx - 1
	}
	cnym := 0
	if cny > 0 {
		cnym = cny - 1
	}
	i0 := f2u(fmax(math.Floor((pos[0]-r-o[0])/navCELL), 0.0))
	j0 := f2u(fmax(math.Floor((pos[1]-r-o[1])/navCELL), 0.0))
	i1 := min(f2u(fmax(math.Ceil((pos[0]+r-o[0])/navCELL), 0.0)), cnxm)
	j1 := min(f2u(fmax(math.Ceil((pos[1]+r-o[1])/navCELL), 0.0)), cnym)
	var nodes []LaserNode
	step := f2u(fmax(p.lzCell, 1.0))
	cj := j0
	for cj <= j1 {
		ci := i0
		for ci <= i1 {
			lo, hi := nav.cellNodeIds(ci, cj)
			for nd := lo; nd < hi; nd++ {
				q := nav.nodePos(nd)
				if math.Abs(q[2]-pos[2]) > 3.0 || dist(q, pos) > r {
					continue
				}
				cov := world.ray(te, up(q, h*0.5)) != nil && world.ray(tm, up(q, h*0.95)) != nil && world.ray(te, up(q, h*0.95)) != nil
				if cov && p.lzMargin > 0.0 {
					dx := q[0] - tp[0]
					dy := q[1] - tp[1]
					l := fmax(math.Hypot(dx, dy), 1e-6)
					nx, ny := -dy/l*p.lzMargin, dx/l*p.lzMargin
					for _, sgn := range [2]float64{-1.0, 1.0} {
						e2 := V3{te[0] + nx*sgn, te[1] + ny*sgn, te[2]}
						if world.ray(e2, up(q, h*0.6)) == nil {
							cov = false
						}
					}
				}
				clr := !cov && world.ray(up(q, b.cfg.Mech.MuzzleHeight), tc) == nil
				var seenBy uint32
				if cov || clr {
					for _, e := range others {
						if world.ray(e, up(q, h*0.6)) == nil {
							seenBy += 1
						}
					}
				}
				nodes = append(nodes, LaserNode{q: q, cov: cov, clr: clr, seenBy: seenBy})
			}
			if len(nodes) > 900 || b.spentMs() > p.lzMs {
				break
			}
			ci += step
		}
		cj += step
	}
	type cand struct {
		f0 float64
		f1 int
		f2 int
	}
	var cands []cand
	smax := p.lzStep
	for hi := range nodes {
		hn := &nodes[hi]
		if !hn.cov {
			continue
		}
		dh := dist(hn.q, pos)
		keep := 0.0
		if b.pk.st != LaserPkStOff && dist(hn.q, b.pk.hide) < 1.5 {
			keep = p.lzKeep
		}
		far := 0.0
		if dist(hn.q, tp) < p.lzMinD {
			far = 40.0
		}
		for pi := range nodes {
			pn := &nodes[pi]
			if !pn.clr {
				continue
			}
			s := dist(hn.q, pn.q)
			if s > smax || s < 0.8 {
				continue
			}
			sc := dh*p.lzWMove + s*p.lzWStep + float64(pn.seenBy)*p.lzWSeen + float64(hn.seenBy)*p.lzWSeen*1.5 + far - keep
			cands = append(cands, cand{sc, hi, pi})
		}
	}
	if b.pk.dbg {
		nc, nl := 0, 0
		for i := range nodes {
			if nodes[i].cov {
				nc++
			}
			if nodes[i].clr {
				nl++
			}
		}
		logf("LZP nodes=%d cov=%d clr=%d cands=%d ms=%.1f\n", len(nodes), nc, nl, len(cands), b.spentMs())
	}
	sortStable(cands, func(x, y *cand) int { return partialCmpOr(x.f0, y.f0) })
	for ki := 0; ki < len(cands) && ki < 12; ki++ {
		hq := nodes[cands[ki].f1].q
		pq := nodes[cands[ki].f2].q
		if (dist(hq, pos) < 1.0 || nav.clear(pos, hq)) && nav.clear(hq, pq) {
			return &struct {
				f0 V3
				f1 V3
			}{hq, pq}
		}
		if b.spentMs() > p.lzMs+3.0 {
			break
		}
	}
	if p.lzHideOnly > 0.5 {
		var best *struct {
			f0 float64
			f1 V3
		}
		for i := range nodes {
			n := &nodes[i]
			if !(n.cov && n.seenBy == 0) {
				continue
			}
			keep := 0.0
			if b.pk.st != LaserPkStOff && dist(n.q, b.pk.hide) < 1.5 {
				keep = p.lzKeep
			}
			s := dist(n.q, pos) - keep
			if best == nil || s < best.f0 {
				best = &struct {
					f0 float64
					f1 V3
				}{s, n.q}
			}
		}
		if best != nil {
			q := best.f1
			if dist(q, pos) < 1.0 || nav.clear(pos, q) {
				return &struct {
					f0 V3
					f1 V3
				}{q, q}
			}
		}
	}
	return nil
}

func (b *Brain) laserPost(obs *Obs, world *World, fc *AimFireOut, mv *PlanMoveOut) {
	p := b.p
	if p.lzScanS > 0.0 && fc.target == nil && !fc.charging && b.laserSlot(obs) == nil {
		t := obs.T
		quiet := t-b.mem.lastDamageT > 2.0
		per := p.lzScanEvery
		var phase float64
		if per > 0.0 {
			phase = math.Mod(t, per)
		} else {
			phase = 1e9
		}
		if quiet && !b.sfQuietBlock() && (t < p.lzScanS || phase < p.lzScanLen) {
			fc.aim = V2{wrap(obs.Me.Aim[0] + 1.5), 0.0}
		}
	}
	for i := range obs.Beams {
		bm := &obs.Beams[i]
		if bm.Kind == BeamKindLaser && bm.By == b.pk.tid {
			b.pk.ebeamT = obs.T - fmax(bm.Age, 0.0)
		}
	}
	for i := range obs.Events {
		e := &obs.Events[i]
		if e.kind == EventKindDamaged {
			d := &e.damaged0
			if d.Weapon == "laser" && d.By != nil && *d.By == b.pk.tid {
				b.pk.ebeamT = obs.T
			}
			if b.pk.dbg {
				logf("LZD %s %v %.0f mode=%v\n", d.Weapon, b.pk.st, d.Amount, b.mode)
			}
		}
	}
	if p.lzCl > 0.5 && b.pk.st == LaserPkStOff {
		b.counterLaser(obs, world, mv)
	}
	if b.pk.dbg && b.pk.st != b.pk.lastSt {
		logf("LZ t=%.2f %v->%v pos=%.0f hide=%.0f peek=%.0f hp=%.0f\n", obs.T, b.pk.lastSt, b.pk.st, obs.Me.Pos, b.pk.hide, b.pk.peek, obs.Me.Hp)
	}
	b.pk.lastSt = b.pk.st
	if b.pk.st == LaserPkStOff {
		return
	}
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	lsp := b.laserSlot(obs)
	if lsp == nil {
		b.pk.st = LaserPkStOff
		return
	}
	ls := *lsp
	w := me.Weapons[ls]
	if w == nil {
		return
	}
	var tr *Track
	for i := range b.mem.tracks {
		if b.mem.tracks[i].id == b.pk.tid {
			c := b.mem.tracks[i]
			tr = &c
			break
		}
	}
	if tr == nil {
		b.pk.st = LaserPkStOff
		return
	}
	tp := tr.posAt(t)
	at := b.aimTarget(tr)
	c := up(tp, at.height*0.5)
	vpos := add(pos, scale(V3{me.Vel[0], me.Vel[1], 0.0}, 0.05))
	muzzle := up(vpos, me.Stats.MuzzleHeight)
	line := world.ray(muzzle, c) == nil && world.ray(up(pos, me.Stats.MuzzleHeight), c) == nil
	direct := dirToAim(sub(c, up(pos, me.Stats.MuzzleHeight)))
	full := fmax(w.Stats.Charge, 0.05)
	charging := w.Charge > 0.0
	pred := slew(me.Aim, direct, me.Stats.TurnRate*0.05, me.Stats.PitchRate*0.05)
	d := fmax(len_(sub(c, muzzle)), 1.0)
	tol := math.Atan2(at.radius*0.8, d)
	aligned := math.Abs(adiff(pred[0], direct[0])) <= tol && math.Abs(pred[1]-direct[1]) <= math.Atan2(at.height*0.4, d)
	to := func(a V3, bp V3) (V2, float64) {
		dx := bp[0] - a[0]
		dy := bp[1] - a[1]
		l := math.Hypot(dx, dy)
		if l < 1e-6 {
			return V2{0.0, 0.0}, 0.0
		}
		return V2{dx / l, dy / l}, l
	}
	gunFire := fc.fire
	switch b.pk.st {
	case LaserPkStHide:
		dir, dh := to(pos, b.pk.hide)
		if dh > 0.7 {
			mv.dir = V2{dir[0] * fmin(dh/2.0, 1.0), dir[1] * fmin(dh/2.0, 1.0)}
			mv.boost = dh > 6.0 && me.Energy > 30.0 && !charging
		} else {
			mv.dir = V2{0.0, 0.0}
			mv.boost = false
		}
		mv.jump = false
		fc.aim = direct
		te := up(tp, b.cfg.Mech.EyeHeight)
		exposed := world.ray(te, up(pos, fmax(at.height, 1.0)*0.5)) == nil || world.ray(te, up(pos, b.cfg.Mech.Height*0.95)) == nil
		if exposed && p.lzExpRush > 0.5 && dh > 0.7 {
			mv.dir = dir
			mv.boost = me.Energy > 10.0 && !charging
		}
		near := dh < p.lzChargeD && !(exposed && p.lzExpRush > 0.5)
		if charging {
			if line && aligned && w.Charge >= full-0.06 {
				fc.fire[ls] = false
				b.pk.shots += 1
				b.pk.st = LaserPkStBack
				b.pk.stT = t
			} else {
				fc.fire[ls] = true
				since := t - b.pk.ebeamT
				heldS := fmax(w.Charge-full, 0.0)
				wait := p.lzSync > 0.0 && tr.has(WeaponKindLaser) && since > 1.3 && heldS < p.lzSync
				if w.Charge >= full-0.12 && dh < 1.5 && !wait && dist(b.pk.peek, b.pk.hide) > 0.5 {
					b.pk.st = LaserPkStOut
					b.pk.stT = t
				}
			}
		} else {
			fc.fire[ls] = near && w.Cooldown <= 0.05
		}
	case LaserPkStOut:
		dir, dp := to(pos, b.pk.peek)
		if dp > 0.3 {
			mv.dir = dir
		} else {
			mv.dir = V2{0.0, 0.0}
		}
		mv.boost = false
		mv.jump = false
		fc.aim = direct
		if !charging {
			b.pk.st = LaserPkStBack
			b.pk.stT = t
		} else if line && aligned {
			fc.fire[ls] = false
			b.pk.shots += 1
			b.pk.st = LaserPkStBack
			b.pk.stT = t
		} else {
			fc.fire[ls] = true
			if t-b.pk.stT > p.lzOutMax {
				b.pk.st = LaserPkStHide
				b.pk.stT = t
				b.pk.planT = -1e9
			}
		}
		if line && p.lzGun > 0.5 {
			for i, wo := range me.Weapons {
				if i != ls && wo != nil && wo.Kind == WeaponKindGun {
					fc.fire[i] = gunFire[i] || ((wo != nil && wo.Ammo > 0.0 && wo.Reload <= 0.0) && aligned)
				}
			}
		}
	case LaserPkStBack:
		dir, dh := to(pos, b.pk.hide)
		if dh > 0.4 {
			mv.dir = dir
		} else {
			mv.dir = V2{0.0, 0.0}
		}
		mv.boost = dh > 1.0 && me.Energy > 15.0 && p.lzBoost > 0.5
		mv.jump = false
		fc.fire[ls] = false
		if dh < 0.9 || t-b.pk.stT > 1.5 {
			b.pk.st = LaserPkStHide
			b.pk.stT = t
			b.pk.planT = -1e9
		}
	case LaserPkStOff:
	}
	fc.charging = fc.fire[ls]
}

func (b *Brain) counterLaser(obs *Obs, world *World, mv *PlanMoveOut) {
	p := b.p
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	if b.pk.dodge != nil {
		q, until := b.pk.dodge.f0, b.pk.dodge.f1
		if t > until {
			b.pk.dodge = nil
		} else {
			dx := q[0] - pos[0]
			dy := q[1] - pos[1]
			l := math.Hypot(dx, dy)
			if l > 0.5 {
				mv.dir = V2{dx / l, dy / l}
				mv.boost = me.Energy > 8.0
				mv.jump = false
			}
			return
		}
	}
	full := fmax(b.cfg.Weapons.Laser.Charge, 0.1)
	h := b.cfg.Mech.Height
	var thr *struct {
		f0 V3
		f1 float64
	}
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || !tr.charging || t-tr.lastSeen > 0.15 || !tr.has(WeaponKindLaser) {
			continue
		}
		tp := tr.posAt(t)
		d := dist(tp, pos)
		if d < 15.0 {
			continue
		}
		brg := math.Atan2(pos[1]-tp[1], pos[0]-tp[0])
		if math.Abs(adiff(tr.aim[0], brg)) > p.lzClCone {
			continue
		}
		left := full - (t - tr.chargeSince)
		if left < 0.05 || left > p.lzClLeft {
			continue
		}
		if thr == nil || left < thr.f1 {
			thr = &struct {
				f0 V3
				f1 float64
			}{tp, left}
		}
	}
	if b.pk.dbg {
		n := 0
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind == ActorKindMech && tr.charging && t-tr.lastSeen < 0.15 {
				n++
			}
		}
		if n > 0 {
			logf("LZC charging_seen=%d thr=%v\n", n, thr != nil)
		}
	}
	if thr == nil {
		return
	}
	tp, left := thr.f0, thr.f1
	te := up(tp, b.cfg.Mech.EyeHeight)
	if world.ray(te, up(pos, h*0.5)) != nil && world.ray(te, up(pos, h*0.95)) != nil {
		return
	}
	nav := &b.shared.nav
	speed := fmax(fmax(me.Stats.BoostSpeed, me.Stats.WalkSpeed), 1.0)
	r := fmin(left*speed+1.0, p.lzClR)
	g := nav.grid()
	cnx, cny, o := g.f0, g.f1, g.f2
	cnxm := 0
	if cnx > 0 {
		cnxm = cnx - 1
	}
	cnym := 0
	if cny > 0 {
		cnym = cny - 1
	}
	i0 := f2u(fmax(math.Floor((pos[0]-r-o[0])/navCELL), 0.0))
	j0 := f2u(fmax(math.Floor((pos[1]-r-o[1])/navCELL), 0.0))
	i1 := min(f2u(fmax(math.Ceil((pos[0]+r-o[0])/navCELL), 0.0)), cnxm)
	j1 := min(f2u(fmax(math.Ceil((pos[1]+r-o[1])/navCELL), 0.0)), cnym)
	var best *struct {
		f0 float64
		f1 V3
	}
	for cj := j0; cj <= j1; cj++ {
		for ci := i0; ci <= i1; ci++ {
			lo, hi := nav.cellNodeIds(ci, cj)
			for nd := lo; nd < hi; nd++ {
				q := nav.nodePos(nd)
				d := dist(q, pos)
				if d > r || math.Abs(q[2]-pos[2]) > 2.5 || (best != nil && d >= best.f0) {
					continue
				}
				if world.ray(te, up(q, h*0.5)) != nil && world.ray(te, up(q, h*0.95)) != nil && nav.clear(pos, q) {
					best = &struct {
						f0 float64
						f1 V3
					}{d, q}
				}
			}
		}
		if b.spentMs() > p.lzMs+4.0 {
			break
		}
	}
	if b.pk.dbg {
		logf("LZC2 found=%v\n", best != nil)
	}
	if best != nil {
		q := best.f1
		b.pk.dodge = &struct {
			f0 V3
			f1 float64
		}{q, t + left + 0.3}
		dx := q[0] - pos[0]
		dy := q[1] - pos[1]
		l := fmax(math.Hypot(dx, dy), 1e-6)
		mv.dir = V2{dx / l, dy / l}
		mv.boost = me.Energy > 8.0
		mv.jump = false
	}
}
