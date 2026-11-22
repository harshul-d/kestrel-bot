package pilot

import "math"

type BurstBurstSt struct {
	dodge *struct {
		f0 V3
		f1 float64
		f2 int64
	}
	side    float64
	sideT   float64
	hiddenT float64
}

type BurstThr struct {
	id   int64
	eye  V3
	tp   V3
	held float64
}

func (b *Brain) buThreats(obs *Obs) []BurstThr {
	t := obs.T
	pos := obs.Me.Pos
	var v []BurstThr
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || !tr.charging || t-tr.lastSeen > 0.25 || !tr.has(WeaponKindLaser) {
			continue
		}
		tp := tr.posAt(t)
		d := dist(tp, pos)
		if d < 12.0 || d > b.cfg.Weapons.Laser.Range+20.0 {
			continue
		}
		brg := math.Atan2(pos[1]-tp[1], pos[0]-tp[0])
		if math.Abs(adiff(tr.aim[0], brg)) > b.p.buCone+math.Atan2(5.0, d) {
			continue
		}
		s := 1.0 + b.cfg.Cubes.SizePerCube*fmax(tr.cubes, 0.0)
		v = append(v, BurstThr{id: tr.id, eye: up(tp, b.cfg.Mech.EyeHeight*s), tp: tp, held: t - tr.chargeSince})
	}
	return v
}

func brainBuSeen(world *World, eyes []V3, q V3, h float64) int {
	n := 0
	for _, e := range eyes {
		if world.ray(e, up(q, h*0.5)) == nil || world.ray(e, up(q, h*0.95)) == nil {
			n++
		}
	}
	return n
}

func (b *Brain) burstPost(obs *Obs, world *World, fc *AimFireOut, mv *PlanMoveOut) {
	p := b.p
	t := obs.T
	me := &obs.Me
	pos := me.Pos
	if p.buRegenHp > 0.0 {
		hpf := frac(me.Hp, me.MaxHp)
		charging := false
		for _, w := range me.Weapons {
			if w != nil && w.Charge > 0.0 {
				charging = true
				break
			}
		}
		mechTgt := b.target.kind == TgtKindMech
		recent := false
		for i := range b.mem.tracks {
			tr := &b.mem.tracks[i]
			if tr.kind == ActorKindMech && (t-tr.lastSeen < p.buRegenSeenS || t-tr.lastDmgT < p.buRegenSeenS) {
				recent = true
				break
			}
		}
		if hpf < p.buRegenHp && !mechTgt && !recent && !charging {
			fc.fire = [2]bool{false, false}
		}
	}
	if p.buBreak <= 0.0 {
		return
	}
	h := b.cfg.Mech.Height
	thr := b.buThreats(obs)
	if b.bu.dodge != nil {
		q, until, id := b.bu.dodge.f0, b.bu.dodge.f1, b.bu.dodge.f2
		still := false
		for i := range thr {
			if thr[i].id == id {
				still = true
				break
			}
		}
		if t > until || !still {
			b.bu.dodge = nil
		} else {
			dx := q[0] - pos[0]
			dy := q[1] - pos[1]
			l := math.Hypot(dx, dy)
			if l > 0.7 {
				mv.dir = V2{dx / l, dy / l}
				mv.boost = me.Energy > 6.0
				mv.jump = false
			}
			return
		}
	}
	if len(thr) < f2u(p.buMinThr) || len(thr) == 0 {
		return
	}
	eyes := make([]V3, 0, len(thr))
	for i := range thr {
		eyes = append(eyes, thr[i].eye)
	}
	seenNow := brainBuSeen(world, eyes, pos, h)
	if seenNow == 0 {
		return
	}
	mi := 0
	for i := 1; i < len(thr); i++ {
		if totalCmp(thr[mi].held, thr[i].held) <= 0 {
			mi = i
		}
	}
	mainId, mainTp := thr[mi].id, thr[mi].tp
	nav := &b.shared.nav
	speed := fmax(fmax(me.Stats.BoostSpeed, me.Stats.WalkSpeed), 1.0)
	r := fmax(fmin(p.buReachS*speed, p.buR), 2.0)
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
outer:
	for cj := j0; cj <= j1; cj++ {
		for ci := i0; ci <= i1; ci++ {
			lo, hi := nav.cellNodeIds(ci, cj)
			for nd := lo; nd < hi; nd++ {
				q := nav.nodePos(nd)
				d := dist(q, pos)
				if d > r || math.Abs(q[2]-pos[2]) > 2.5 || (best != nil && d >= best.f0) {
					continue
				}
				if brainBuSeen(world, eyes, q, h) == 0 && nav.clear(pos, q) {
					best = &struct {
						f0 float64
						f1 V3
					}{d, q}
				}
			}
			if b.spentMs() > p.buMs {
				break outer
			}
		}
	}
	if best != nil {
		q := best.f1
		b.bu.dodge = &struct {
			f0 V3
			f1 float64
			f2 int64
		}{q, t + p.buHoldS, mainId}
		dx := q[0] - pos[0]
		dy := q[1] - pos[1]
		l := fmax(math.Hypot(dx, dy), 1e-6)
		mv.dir = V2{dx / l, dy / l}
		mv.boost = me.Energy > 6.0
		mv.jump = false
		return
	}
	if p.buDodge > 0.0 && me.Energy > p.buDodgeEnergy {
		if t > b.bu.sideT {
			if b.bu.side > 0.0 {
				b.bu.side = -1.0
			} else {
				b.bu.side = 1.0
			}
			b.bu.sideT = t + b.rng.range_(0.35, 0.9)
		}
		tp := mainTp
		bx := pos[0] - tp[0]
		by := pos[1] - tp[1]
		l := fmax(math.Hypot(bx, by), 1e-6)
		perp := V2{-by / l * b.bu.side, bx / l * b.bu.side}
		probe := V3{pos[0] + perp[0]*6.0, pos[1] + perp[1]*6.0, pos[2]}
		ok := false
		if s := nav.snap(probe); s != nil {
			ok = dist(*s, probe) < 3.0 && nav.clear(pos, *s)
		}
		var dir V2
		if ok {
			dir = perp
		} else {
			dir = V2{-perp[0], -perp[1]}
		}
		if !ok {
			b.bu.side = -b.bu.side
		}
		mv.dir = dir
		mv.boost = true
	}
	_ = fc
}
