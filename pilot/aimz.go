package pilot

import "math"

type AimzSmp struct {
	t      float64
	pos    V3
	s      float64
	e      *V2
	ground bool
	sp     float64
}

type AimzDisp = [12][2]float32

type AimzHist struct {
	id      int64
	lastRev float64
	refE    *V2
	smp     []AimzSmp
	table   [][]AimzDisp
	shot    []struct {
		f0 float64
		f1 AimzDisp
	}
}

const aimzNB int = 44

type AimzAimZ struct {
	fired []struct {
		f0 int64
		f1 float64
	}
	gshot []struct {
		f0 float64
		f1 AimzDisp
	}
	hist  []AimzHist
	table [][]AimzDisp
}

func aimzBin(s *AimzSmp) int {
	sp := 0
	if s.sp > 15.0 {
		sp = 1
	}
	return f2u(fclamp(math.Floor(s.s/0.1), 0.0, 21.0))*2 + sp
}

func aimzAimZNew() AimzAimZ {
	return AimzAimZ{table: make([][]AimzDisp, aimzNB)}
}

func (a *AimzAimZ) update(obs *Obs) {
	t := obs.T
	for mi := range obs.Mechs {
		m := &obs.Mechs[mi]
		if m.Kind != ActorKindMech {
			continue
		}
		allFin := true
		for _, x := range m.Pos {
			if !isFinite(x) {
				allFin = false
				break
			}
		}
		if allFin {
			for _, x := range m.Vel {
				if !isFinite(x) {
					allFin = false
					break
				}
			}
		}
		if !allFin {
			continue
		}
		hi := -1
		for i := range a.hist {
			if a.hist[i].id == m.Id {
				hi = i
				break
			}
		}
		if hi < 0 {
			if len(a.hist) >= 24 {
				old := 0
				oldT := 0.0
				for i := range a.hist {
					ti := 0.0
					if n := len(a.hist[i].smp); n > 0 {
						ti = a.hist[i].smp[n-1].t
					}
					if i == 0 || totalCmp(ti, oldT) < 0 {
						old = i
						oldT = ti
					}
				}
				a.hist = append(a.hist[:old], a.hist[old+1:]...)
			}
			a.hist = append(a.hist, AimzHist{id: m.Id, lastRev: t - 2.0, refE: nil, shot: nil, smp: nil, table: make([][]AimzDisp, aimzNB)})
			hi = len(a.hist) - 1
		}
		h := &a.hist[hi]
		if n := len(h.smp); n > 0 {
			l := &h.smp[n-1]
			if t-l.t < 0.085 {
				continue
			}
			if t-l.t > 0.5 {
				h.smp = h.smp[:0]
			}
		}
		sp := math.Hypot(m.Vel[0], m.Vel[1])
		if sp > 3.0 {
			e := V2{m.Vel[0] / sp, m.Vel[1] / sp}
			if h.refE != nil {
				r := *h.refE
				c := r[0]*e[0] + r[1]*e[1]
				if c < 0.34 {
					h.lastRev = t
					h.refE = ptr(e)
				} else if c > 0.87 {
					h.refE = ptr(e)
				}
			} else {
				h.refE = ptr(e)
			}
		}
		var e *V2
		if sp > 2.0 {
			e = &V2{m.Vel[0] / sp, m.Vel[1] / sp}
		}
		ground := math.Abs(m.Vel[2]) < 0.3
		h.smp = append(h.smp, AimzSmp{t: t, pos: m.Pos, s: fmin(t-h.lastRev, 2.1), e: e, ground: ground, sp: sp})
		for len(h.smp) > 14 {
			h.smp = h.smp[1:]
		}
		if len(h.smp) >= 13 {
			base := h.smp[len(h.smp)-13]
			if base.e != nil && base.ground {
				e := *base.e
				ok := true
				var d AimzDisp
				for j := 1; j <= 12; j++ {
					q := h.smp[len(h.smp)-13+j]
					if math.Abs(q.t-base.t-0.1*float64(j)) > 0.04 {
						ok = false
						break
					}
					dx := q.pos[0] - base.pos[0]
					dy := q.pos[1] - base.pos[1]
					d[j-1] = [2]float32{float32(dx*e[0] + dy*e[1]), float32(-dx*e[1] + dy*e[0])}
				}
				if ok {
					id := h.id
					fired := false
					for _, f := range a.fired {
						if f.f0 == id && math.Abs(f.f1-base.t) < 0.06 {
							fired = true
							break
						}
					}
					if fired {
						h.shot = append(h.shot, struct {
							f0 float64
							f1 AimzDisp
						}{base.sp, d})
						if len(h.shot) > 40 {
							h.shot = h.shot[1:]
						}
						a.gshot = append(a.gshot, struct {
							f0 float64
							f1 AimzDisp
						}{base.sp, d})
						if len(a.gshot) > 80 {
							a.gshot = a.gshot[1:]
						}
					}
					bb := aimzBin(&base)
					h.table[bb] = append(h.table[bb], d)
					if len(h.table[bb]) > 60 {
						h.table[bb] = h.table[bb][1:]
					}
					a.table[bb] = append(a.table[bb], d)
					if len(a.table[bb]) > 160 {
						a.table[bb] = a.table[bb][1:]
					}
				}
			}
		}
	}
}

// findHist returns the first history whose latest sample matches feet at time now.
func (a *AimzAimZ) findHist(now float64, feet V3) *AimzHist {
	for i := range a.hist {
		h := &a.hist[i]
		if n := len(h.smp); n > 0 {
			s := &h.smp[n-1]
			if now-s.t < 0.15 && dist2(s.pos, feet) < 0.5 {
				return h
			}
		}
	}
	return nil
}

func (a *AimzAimZ) noteFire(now float64, feet V3) {
	if h := a.findHist(now, feet); h != nil {
		st := now
		if n := len(h.smp); n > 0 {
			st = h.smp[n-1].t
		}
		push := true
		if n := len(a.fired); n > 0 {
			f := a.fired[n-1]
			push = f.f0 != h.id || math.Abs(f.f1-st) > 0.01
		}
		if push {
			a.fired = append(a.fired, struct {
				f0 int64
				f1 float64
			}{h.id, st})
		}
	}
	for len(a.fired) > 0 && now-a.fired[0].f1 > 2.0 {
		a.fired = a.fired[1:]
	}
}

// query returns the lateral offset (along nn) of the best aim point after tt seconds, and the share of samples it covers.
func (a *AimzAimZ) query(now float64, feet V3, tt float64, nn V2, hw float64, shrink float64, minN int, shotMin int, gshot bool) *struct {
	f0 float64
	f1 float64
} {
	if !(tt > 0.0) || tt > 1.2 {
		return nil
	}
	h := a.findHist(now, feet)
	if h == nil {
		return nil
	}
	if len(h.smp) == 0 {
		return nil
	}
	s := h.smp[len(h.smp)-1]
	if s.e == nil {
		return nil
	}
	e := *s.e
	if !s.ground {
		return nil
	}
	bb := aimzBin(&s)
	var pool []AimzDisp
	if shotMin > 0 {
		for _, x := range h.shot {
			if (x.f0 > 15.0) == (s.sp > 15.0) {
				pool = append(pool, x.f1)
			}
		}
		if len(pool) < shotMin && gshot {
			pool = pool[:0]
			for _, x := range a.gshot {
				if (x.f0 > 15.0) == (s.sp > 15.0) {
					pool = append(pool, x.f1)
				}
			}
		}
	}
	if len(pool) < max(shotMin, 1) {
		pool = pool[:0]
		for _, src := range [2][][]AimzDisp{h.table, a.table} {
			pool = pool[:0]
			pool = append(pool, src[bb]...)
			for _, k := range [2]int{2, 4} {
				if len(pool) >= minN {
					break
				}
				if bb >= k {
					pool = append(pool, src[bb-k]...)
				}
				if bb+k < aimzNB {
					pool = append(pool, src[bb+k]...)
				}
			}
			if len(pool) >= minN {
				break
			}
		}
	}
	if len(pool) < minN {
		return nil
	}
	j := fclamp(tt/0.1, 1.0, 12.0)
	j0 := uclamp(f2u(math.Floor(j)), 1, 12)
	fr := j - float64(j0)
	j1 := min(j0+1, 12)
	lat := make([]float64, 0, len(pool))
	for pi := range pool {
		d := &pool[pi]
		av := d[j0-1]
		cv := d[j1-1]
		x := float64(av[0])*(1.0-fr) + float64(cv[0])*fr
		y := float64(av[1])*(1.0-fr) + float64(cv[1])*fr
		wx := x*e[0] - y*e[1]
		wy := x*e[1] + y*e[0]
		lat = append(lat, wx*nn[0]+wy*nn[1])
	}
	sortStable(lat, func(x, y *float64) int { return totalCmp(*x, *y) })
	w := 2.0 * hw * shrink
	best := 0
	bx := 0.0
	lo := 0
	for i := 0; i < len(lat); i++ {
		for lat[i]-lat[lo] > w {
			lo += 1
		}
		c := i - lo + 1
		if c > best {
			best = c
			bx = 0.5 * (lat[i] + lat[lo])
		}
	}
	n := float64(len(lat))
	cnt := 0
	for _, x := range lat {
		if math.Abs(x-bx) < hw {
			cnt++
		}
	}
	cover := float64(cnt) / n
	return &struct {
		f0 float64
		f1 float64
	}{bx, cover}
}

func aimzPhi(x float64) float64 {
	t := 1.0 / (1.0 + 0.2316419*math.Abs(x))
	d := 0.3989423 * math.Exp(-x*x/2.0)
	p := d * t * (0.3193815 + t*(-0.3565638+t*(1.781478+t*(-1.821256+t*1.330274))))
	if x >= 0.0 {
		return 1.0 - p
	}
	return p
}

// aimzAntilead takes the shooters as (eye, aim yaw) pairs, in iteration order.
func aimzAntilead(pts []V3, dt float64, pos V3, vel V3, shooters []struct {
	f0 V3
	f1 float64
}, hw float64, speed float64, lat float64, sigma float64, dmg float64) float64 {
	n := len(pts)
	if n < 2 {
		return 0.0
	}
	cost := 0.0
	for _, sh := range shooters {
		eye, aimYaw := sh.f0, sh.f1
		dx := pos[0] - eye[0]
		dy := pos[1] - eye[1]
		d := math.Hypot(dx, dy)
		if !(d > 30.0 && d < 230.0) {
			continue
		}
		bearing := math.Atan2(dy, dx)
		if math.Abs(adiff(aimYaw, bearing)) > 0.6 {
			continue
		}
		tt := lat + d/speed
		kf := tt / dt
		k := f2u(math.Floor(kf))
		var q V2
		if k+1 < n {
			f := kf - float64(k)
			q = V2{pts[k][0]*(1.0-f) + pts[k+1][0]*f, pts[k][1]*(1.0-f) + pts[k+1][1]*f}
		} else {
			av := pts[n-2]
			bv := pts[n-1]
			ex := tt - float64(n-1)*dt
			q = V2{bv[0] + (bv[0]-av[0])/dt*ex, bv[1] + (bv[1]-av[1])/dt*ex}
		}
		pl := V2{pos[0] + vel[0]*tt, pos[1] + vel[1]*tt}
		nn := V2{-dy / fmax(d, 0.1), dx / fmax(d, 0.1)}
		sep := (q[0]-pl[0])*nn[0] + (q[1]-pl[1])*nn[1]
		sg := fmax(sigma*d, 0.2)
		ph := aimzPhi((hw-sep)/sg) - aimzPhi((-hw-sep)/sg)
		cost += ph * dmg
	}
	return cost
}

type AimzCadence struct {
	known []int64
	last  []struct {
		f0 int64
		f1 float64
		f2 float64
		f3 uint32
	}
}

func aimzCadenceNew() AimzCadence {
	return AimzCadence{known: nil, last: nil}
}

func (c *AimzCadence) update(obs *Obs, period float64, snap float64) {
	t := obs.T
	me := obs.Me.Pos
	mc := V3{me[0], me[1], me[2] + obs.Me.Stats.Height*0.5}
	for pi := range obs.Projectiles {
		pr := &obs.Projectiles[pi]
		if pr.Kind != ProjKindGun {
			continue
		}
		known := false
		for _, k := range c.known {
			if k == pr.Id {
				known = true
				break
			}
		}
		if known {
			continue
		}
		allFin := true
		for _, x := range pr.Pos {
			if !isFinite(x) {
				allFin = false
				break
			}
		}
		if allFin {
			for _, x := range pr.Vel {
				if !isFinite(x) {
					allFin = false
					break
				}
			}
		}
		if !allFin {
			continue
		}
		c.known = append(c.known, pr.Id)
		if len(c.known) > 160 {
			c.known = c.known[1:]
		}
		sp := len_(pr.Vel)
		if sp < 50.0 {
			continue
		}
		u := scale(pr.Vel, 1.0/sp)
		rel := sub(mc, pr.Pos)
		along := dot(rel, u)
		if along < 0.0 || len_(sub(rel, scale(u, along))) > 12.0 {
			continue
		}
		var best *struct {
			f0 float64
			f1 int64
			f2 float64
		}
		for mi := range obs.Mechs {
			m := &obs.Mechs[mi]
			if m.Kind != ActorKindMech {
				continue
			}
			w := sub(V3{m.Pos[0], m.Pos[1], m.Pos[2] + obs.Me.Stats.MuzzleHeight}, pr.Pos)
			s := -dot(w, u)
			if s < 0.0 || s > 400.0 {
				continue
			}
			perp := len_(add(w, scale(u, s)))
			if perp < 4.0 && (best == nil || perp < best.f0) {
				best = &struct {
					f0 float64
					f1 int64
					f2 float64
				}{perp, m.Id, s / sp}
			}
		}
		if best != nil {
			id, age := best.f1, best.f2
			raw := t - age
			var ft float64
			if snap > 0.0 {
				ft = math.Round(raw/snap) * snap
			} else {
				ft = raw
			}
			xi := -1
			for i := range c.last {
				if c.last[i].f0 == id {
					xi = i
					break
				}
			}
			if xi >= 0 {
				x := &c.last[xi]
				iv := raw - x.f2
				if iv > 0.05 {
					k := math.Round(iv / period)
					if k >= 1.0 && k <= 3.0 && math.Abs(iv-k*period) < 0.025 {
						x.f3 += 1
					} else {
						x.f3 = 0
					}
					x.f1 = ft
					x.f2 = raw
				}
			} else {
				c.last = append(c.last, struct {
					f0 int64
					f1 float64
					f2 float64
					f3 uint32
				}{id, ft, raw, 0})
			}
		}
	}
	kept := c.last[:0]
	for _, x := range c.last {
		if t-x.f1 < 3.0+period {
			kept = append(kept, x)
		}
	}
	c.last = kept
}

func (c *AimzCadence) due(id int64, t float64, period float64, win float64, keep float64, off float64, reg uint32) bool {
	xi := -1
	for i := range c.last {
		if c.last[i].f0 == id {
			xi = i
			break
		}
	}
	if xi < 0 {
		return false
	}
	x := &c.last[xi]
	if x.f3 < reg {
		return false
	}
	since := t - (x.f1 - off)
	if since < period*0.5 || since > keep {
		return false
	}
	k := math.Round(since / period)
	return math.Abs(since-k*period) <= win
}
