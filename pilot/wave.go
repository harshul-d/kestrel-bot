package pilot

import "math"

type WaveWaveState struct {
	kills uint32
	prep  *struct {
		f0 V3
		f1 float64
	}
	nextEval float64
	k        uint32
}

func waveWaveStateNew() WaveWaveState {
	return WaveWaveState{kills: 0, prep: nil, nextEval: 0.0, k: 0}
}

func (b *Brain) waveCount(obs *Obs) {
	for i := range obs.Events {
		if obs.Events[i].kind == EventKindKill {
			b.wave.kills += 1
		}
	}
}

func (b *Brain) waveArmed() bool {
	return b.p.readyKill >= 1.0 && float64(b.wave.kills) >= b.p.readyKill-1.0 && (b.p.readyOnly < 0.5 || float64(b.wave.kills) < b.p.readyKill)
}

func (b *Brain) ready(obs *Obs) bool {
	p := b.p
	me := &obs.Me
	if obs.T >= p.readyDeadline {
		return true
	}
	if me.Cubes < fmin(p.readyCubes, fmax(b.cfg.Cubes.StatCap, 1.0)) || frac(me.Hp, me.MaxHp) < p.readyHp {
		return false
	}
	if p.readyLaser > 0.5 && !brainHasKind(obs, WeaponKindLaser) {
		return false
	}
	if p.readySecond > 0.0 {
		for _, w := range me.Weapons {
			if w == nil {
				return false
			}
		}
	}
	if p.readyCover > 0.0 {
		if !(b.wave.prep != nil && dist(b.wave.prep.f0, me.Pos) <= p.readyCover) {
			return false
		}
	}
	return true
}

func (b *Brain) gateClosed(obs *Obs) bool {
	return b.waveArmed() && !b.ready(obs)
}

func (b *Brain) spare(hp float64, maxHp float64) bool {
	return frac(hp, maxHp) < b.p.readySpareFrac || hp < b.p.readySpareHp
}

func (b *Brain) prepGoal() *V3 {
	if b.p.prepPos <= 0.0 || !(b.p.readyKill < 1.0 || b.waveArmed()) {
		return nil
	}
	if b.wave.prep == nil {
		return nil
	}
	return ptr(b.wave.prep.f0)
}

func (b *Brain) exposure(world *World, q V3, h float64) float64 {
	n := 16
	tau := 2 * math.Pi
	seen := 0.0
	c := up(q, h*0.6)
	rw := [2]struct {
		f0 float64
		f1 float64
	}{{45.0, 1.0}, {110.0, 0.7}}
	for k := 0; k < n; k++ {
		a := float64(k) * tau / float64(n)
		for _, x0 := range rw {
			r, w := x0.f0, x0.f1
			x := q[0] + r*math.Cos(a)
			y := q[1] + r*math.Sin(a)
			z := fin(world.terrainZ(x, y), q[2])
			e := V3{x, y, z + b.cfg.Mech.EyeHeight}
			if world.ray(e, c) == nil {
				seen += w
			}
		}
	}
	return seen / (float64(n) * 1.7)
}

func (b *Brain) waveUpdate(obs *Obs, world *World, nav *Nav) {
	if b.p.prepPos <= 0.0 {
		return
	}
	t := obs.T
	z := &obs.Zone
	if !(isFinite(z.NextRadius) && isFinite(z.NextCenter[0])) {
		return
	}
	me := &obs.Me
	keepR := fmax(z.NextRadius-b.p.zoneMargin-10.0, z.NextRadius*0.4)
	if b.wave.prep != nil {
		q := b.wave.prep.f0
		if dist2v(q, z.NextCenter) > keepR+2.0 {
			b.wave.prep = nil
		}
	}
	if t < b.wave.nextEval {
		return
	}
	b.wave.nextEval = t + 0.3
	h := fmax(me.Stats.Height, 1.0)
	tau := 2 * math.Pi
	for it := 0; it < 3; it++ {
		if b.spentMs() > 6.0 {
			break
		}
		b.wave.k = b.wave.k + 1
		var cx, cy, rr float64
		if b.wave.k%2 == 0 {
			cx, cy, rr = me.Pos[0], me.Pos[1], 120.0
		} else {
			cx, cy, rr = z.NextCenter[0], z.NextCenter[1], keepR
		}
		a := b.rng.f64() * tau
		r := rr * math.Sqrt(b.rng.f64())
		x := cx + r*math.Cos(a)
		y := cy + r*math.Sin(a)
		if math.Hypot(x-z.NextCenter[0], y-z.NextCenter[1]) > keepR {
			continue
		}
		q0 := V3{x, y, fin(world.terrainZ(x, y), me.Pos[2])}
		qp := nav.snap(q0)
		if qp == nil {
			continue
		}
		q := *qp
		if dist(q, q0) > 4.0 || !(isFinite(q[0]) && isFinite(q[1]) && isFinite(q[2])) || q[2] > q0[2]+2.0 {
			continue
		}
		ex := b.exposure(world, q, h)
		dz := dist2v(q, z.NextCenter) / fmax(z.NextRadius, 1.0)
		s := ex + 0.15*dz + 0.0015*dist(q, me.Pos)
		if b.wave.prep == nil || s < b.wave.prep.f1-0.03 {
			b.wave.prep = &struct {
				f0 V3
				f1 float64
			}{q, s}
		}
	}
}
