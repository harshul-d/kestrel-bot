package pilot

import "math"

// Mech body kinematics, one tick at a time.

const physicsTICK float64 = 0.05

// physicsSLIDE: sideways slide per tick off a roof edge or out from under an eave.
const physicsSLIDE float64 = 0.6
const physicsEPS float64 = 1e-6

const physicsTAU float64 = 2 * math.Pi

var physicsRadsPerDeg = func() float64 { p := float64(math.Pi); return p / 180.0 }()

type Body struct {
	// feet
	pos      V3
	vel      V3
	grounded bool
	energy   float64
	// seconds before energy regeneration resumes
	energyWait float64
}

func bodyNew(pos V3, energy float64) Body {
	return Body{pos: pos, vel: V3{}, grounded: true, energy: energy, energyWait: 0.0}
}

// spend: spend energy outside of movement (restarts the regeneration delay).
func (bd *Body) spend(st *MechCfg, amount, dt float64) {
	bd.energy = fmax(bd.energy-amount, 0.0)
	bd.energyWait = st.EnergyRegenDelay + dt
}

type Drive struct {
	mv        V2
	boost     bool
	jump      bool
	speedMult float64
}

func driveDefault() Drive {
	return Drive{mv: V2{0.0, 0.0}, boost: false, jump: false, speedMult: 1.0}
}

type StepInfo struct {
	boosted bool
	jetted  bool
	jumped  bool
	landed  bool
	blocked bool
}

func physicsFin(v, d float64) float64 {
	if isFinite(v) {
		return v
	}
	return d
}

func physicsWrapAngle(a float64) float64 {
	if !isFinite(a) {
		return 0.0
	}
	a = math.Mod(a, physicsTAU)
	if a > math.Pi {
		a -= physicsTAU
	}
	if a < -math.Pi {
		a += physicsTAU
	}
	return a
}

func physicsAimDir(aim V2) V3 {
	cp := math.Cos(aim[1])
	return V3{cp * math.Cos(aim[0]), cp * math.Sin(aim[0]), math.Sin(aim[1])}
}

// physicsSlewAim: torso aim after one tick of slewing toward want.
func physicsSlewAim(aim, want V2, st *MechCfg, turnMult, dt float64) V2 {
	wy := physicsFin(want[0], aim[0])
	wp := fclamp(physicsFin(want[1], aim[1]), fmin(st.MinPitch, st.MaxPitch), fmax(st.MaxPitch, st.MinPitch))
	my := fmax(st.TurnRate*turnMult*dt, 0.0)
	mp := fmax(st.PitchRate*turnMult*dt, 0.0)
	dy := physicsWrapAngle(wy - aim[0])
	yaw := physicsWrapAngle(aim[0] + fclamp(dy, -my, my))
	dp := wp - aim[1]
	pitch := fclamp(aim[1]+fclamp(dp, -mp, mp), fmin(st.MinPitch, st.MaxPitch), fmax(st.MaxPitch, st.MinPitch))
	return V2{yaw, pitch}
}

// physicsShotOrigin: where a shot leaves: out along the aim yaw at the given height above the feet,
// pulled back if a wall is closer.
func physicsShotOrigin(world *World, pos V3, yaw, up, out float64) V3 {
	c := V3{pos[0], pos[1], pos[2] + up}
	o := V3{c[0] + out*math.Cos(yaw), c[1] + out*math.Sin(yaw), c[2]}
	if t, ok := world.rayT(c, o); ok {
		t = fmax(t-0.02, 0.0)
		return V3{c[0] + (o[0]-c[0])*t, c[1] + (o[1]-c[1])*t, c[2]}
	}
	return o
}

func physicsBlocks(base, top, z, hgt, step float64) bool {
	return top > z+step && base < z+hgt
}

// Near holds the indices of the prisms close to a body, gathered once per step.
type Near struct {
	n     int
	idx   [24]uint32
	extra []uint32
}

func nearGather(world *World, x, y, r float64) Near {
	var s Near
	world.prismsNear(x, y, r, func(i int, _ *Prism) {
		if s.n < len(s.idx) {
			s.idx[s.n] = uint32(i)
			s.n++
		} else {
			s.extra = append(s.extra, uint32(i))
		}
	})
	return s
}

func (s *Near) each(world *World, f func(*Prism)) {
	for k := 0; k < s.n; k++ {
		if i := int(s.idx[k]); i < len(world.prisms) {
			f(&world.prisms[i])
		}
	}
	for _, i := range s.extra {
		if int(i) < len(world.prisms) {
			f(&world.prisms[i])
		}
	}
}

func (s *Near) isEmpty() bool {
	return s.n == 0
}

func physicsResolveIn(world *World, near *Near, p *V2, z, r, hgt, step float64, vel *V2) bool {
	if near.isEmpty() {
		return true
	}
	for it := 0; it < 4; it++ {
		moved := false
		x, y := p[0], p[1]
		v := *vel
		near.each(world, func(pr *Prism) {
			if !physicsBlocks(pr.base, pr.top, z, hgt, step) {
				return
			}
			if x < pr.minX-r || x > pr.maxX+r || y < pr.minY-r || y > pr.maxY+r {
				return
			}
			dr := pr.distXy(x, y)
			d := math.Abs(dr.f0)
			cp := dr.f1
			if !isFinite(d) {
				return
			}
			inside := pr.containsXy(x, y)
			if !inside && d >= r {
				return
			}
			var n V2
			if d > 1e-9 {
				n = V2{(x - cp[0]) / d, (y - cp[1]) / d}
			} else {
				s := math.Hypot(v[0], v[1])
				if s > 1e-9 {
					n = V2{-v[0] / s, -v[1] / s}
				} else {
					n = V2{1.0, 0.0}
				}
			}
			if inside && d > 1e-9 {
				n = V2{-n[0], -n[1]}
			}
			x = cp[0] + n[0]*(r+1e-4)
			y = cp[1] + n[1]*(r+1e-4)
			vn := v[0]*n[0] + v[1]*n[1]
			if vn < 0.0 {
				v[0] -= vn * n[0]
				v[1] -= vn * n[1]
			}
			moved = true
		})
		*p = V2{x, y}
		*vel = v
		if !moved {
			return true
		}
	}
	return physicsFreeIn(world, near, *p, z, r, hgt, step)
}

func physicsFreeIn(world *World, near *Near, p V2, z, r, hgt, step float64) bool {
	free := true
	near.each(world, func(pr *Prism) {
		if !free || !physicsBlocks(pr.base, pr.top, z, hgt, step) {
			return
		}
		if p[0] < pr.minX-r || p[0] > pr.maxX+r || p[1] < pr.minY-r || p[1] > pr.maxY+r {
			return
		}
		if pr.containsXy(p[0], p[1]) || math.Abs(pr.distXy(p[0], p[1]).f0) < r-1e-3 {
			free = false
		}
	})
	return free
}

// physicsResolveWalls pushes the cylinder centre p out of every wall. False if it is still inside one afterwards.
func physicsResolveWalls(world *World, p *V2, z, r, hgt, step float64, vel *V2) bool {
	near := nearGather(world, p[0], p[1], 2.0*r+1.0)
	return physicsResolveIn(world, &near, p, z, r, hgt, step, vel)
}

// physicsIsFree: true if a cylinder at p touches no wall.
func physicsIsFree(world *World, p V2, z, r, hgt, step float64) bool {
	near := nearGather(world, p[0], p[1], r)
	return physicsFreeIn(world, &near, p, z, r, hgt, step)
}

type Support struct {
	// surface under the centre
	s float64
	// roof edge the cylinder overhangs (top at or below the feet), or -inf
	edge   float64
	edgeCp V2
	edgeD  float64
}

func physicsSupport(world *World, near *Near, x, y, z, r, step float64) Support {
	out := Support{s: world.terrainZ(x, y), edge: math.Inf(-1), edgeCp: V2{x, y}, edgeD: 0.0}
	near.each(world, func(p *Prism) {
		if p.top > z+step+physicsEPS {
			return
		}
		if p.top <= out.s && p.top <= out.edge {
			return
		}
		if x < p.minX-r || x > p.maxX+r || y < p.minY-r || y > p.maxY+r {
			return
		}
		if p.containsXy(x, y) {
			if p.top > out.s {
				out.s = p.top
			}
		} else if p.top <= z+physicsEPS && p.top > out.edge {
			dr := p.distXy(x, y)
			d := math.Abs(dr.f0)
			if d < r-1e-3 {
				out.edge = p.top
				out.edgeCp = dr.f1
				out.edgeD = d
			}
		}
	})
	return out
}

func physicsSlideFrom(world *World, near *Near, pos *V3, cp V2, d, r, hgt float64) {
	if d < 1e-9 {
		return
	}
	n := V2{(pos[0] - cp[0]) / d, (pos[1] - cp[1]) / d}
	push := fclamp(r-d+1e-3, 0.0, physicsSLIDE)
	q := V2{pos[0] + n[0]*push, pos[1] + n[1]*push}
	v := V2{0.0, 0.0}
	if physicsResolveIn(world, near, &q, pos[2], r, hgt, physicsEPS, &v) {
		moved := (q[0]-pos[0])*n[0] + (q[1]-pos[1])*n[1]
		if moved > 0.0 {
			pos[0] = q[0]
			pos[1] = q[1]
		}
	}
}

// physicsAirTerrainBlocks: terrain at (x, y) is above feet height zt and steeper than lim there.
func physicsAirTerrainBlocks(world *World, x, y, zt, lim float64) bool {
	if world.terrainZ(x, y) <= zt+physicsEPS {
		return false
	}
	gx := world.terrainZ(x+0.5, y) - world.terrainZ(x-0.5, y)
	gy := world.terrainZ(x, y+0.5) - world.terrainZ(x, y-0.5)
	return math.Hypot(gx, gy) > lim
}

// physicsStep advances one mech body by one tick.
func physicsStep(world *World, st *MechCfg, body *Body, d *Drive, dt float64) StepInfo {
	prev := *body
	var info StepInfo
	if !(isFinite(dt) && dt > 0.0) {
		dt = physicsTICK
	}
	r := fmax(physicsFin(st.Radius, 2.5), 0.05)
	hgt := fmax(physicsFin(st.Height, 9.0), 0.05)
	mv := V2{physicsFin(d.mv[0], 0.0), physicsFin(d.mv[1], 0.0)}
	ml := math.Hypot(mv[0], mv[1])
	if ml > 1.0 {
		mv = V2{mv[0] / ml, mv[1] / ml}
	}
	moving := ml > 1e-9
	mult := fmax(physicsFin(d.speedMult, 1.0), 0.0)
	wasG := body.grounded
	boostOk := d.boost && body.energy > 0.0
	gboost := wasG && boostOk && moving

	// horizontal velocity (boost held raises the target speed in the air too)
	speed := st.WalkSpeed
	if boostOk && moving {
		speed = st.BoostSpeed
	}
	speed *= mult
	acc := st.AirAccel
	if wasG {
		acc = st.GroundAccel
	}
	acc *= dt
	dx, dy := mv[0]*speed-body.vel[0], mv[1]*speed-body.vel[1]
	dl := math.Hypot(dx, dy)
	if dl <= acc || dl < 1e-12 {
		body.vel[0] = mv[0] * speed
		body.vel[1] = mv[1] * speed
	} else {
		body.vel[0] += dx / dl * acc
		body.vel[1] += dy / dl * acc
	}

	// vertical velocity and energy
	drained := false
	jumped := d.jump && wasG
	air := !wasG || jumped
	if jumped {
		body.vel[2] = st.JumpSpeed
		info.jumped = true
	}
	if air {
		if boostOk {
			body.vel[2] = fmin(body.vel[2]+st.JetAccel*dt, st.MaxRiseSpeed)
			body.energy -= st.JetDrain * dt
			drained = true
			info.jetted = true
		}
		body.vel[2] -= st.Gravity * dt
	} else if gboost {
		body.energy -= st.BoostDrain * dt
		drained = true
		info.boosted = true
	}
	if body.energy < 0.0 {
		body.energy = 0.0
	}
	if drained {
		body.energyWait = st.EnergyRegenDelay + dt
	} else if body.energyWait > 1e-9 {
		body.energyWait -= dt
	} else {
		body.energy = fmin(body.energy+st.EnergyRegen*dt, fmax(st.Energy, body.energy))
	}

	// horizontal move
	z := body.pos[2]
	disp := V2{body.vel[0] * dt, body.vel[1] * dt}
	lim := math.Tan(fclamp(st.MaxSlopeDeg, 1.0, 89.0) * physicsRadsPerDeg)
	if wasG {
		tz := world.terrainZ(body.pos[0], body.pos[1])
		dlen := math.Hypot(disp[0], disp[1])
		if z-tz < 0.3 && dlen > 1e-9 {
			nx, ny := body.pos[0]+disp[0], body.pos[1]+disp[1]
			rise := world.terrainZ(nx, ny) - tz
			if rise > 0.0 && rise/dlen > lim {
				gx := world.terrainZ(nx+0.5, ny) - world.terrainZ(nx-0.5, ny)
				gy := world.terrainZ(nx, ny+0.5) - world.terrainZ(nx, ny-0.5)
				g2 := gx*gx + gy*gy
				if g2 > 1e-12 {
					k := (disp[0]*gx + disp[1]*gy) / g2
					if k > 0.0 {
						disp = V2{disp[0] - gx*k, disp[1] - gy*k}
						kv := (body.vel[0]*gx + body.vel[1]*gy) / g2
						body.vel[0] -= gx * kv
						body.vel[1] -= gy * kv
					}
				}
				d2 := math.Hypot(disp[0], disp[1])
				rise2 := world.terrainZ(body.pos[0]+disp[0], body.pos[1]+disp[1]) - tz
				if d2 < 1e-9 || (rise2 > 0.0 && rise2/d2 > lim) {
					disp = V2{0.0, 0.0}
					body.vel[0] = 0.0
					body.vel[1] = 0.0
				}
				info.blocked = true
			}
		}
	}
	if air {
		// steep terrain above the feet is a wall in the air
		zt := fmax(z, z+body.vel[2]*dt)
		dlen := math.Hypot(disp[0], disp[1])
		if dlen > 1e-9 && physicsAirTerrainBlocks(world, body.pos[0]+disp[0], body.pos[1]+disp[1], zt, lim) {
			nx, ny := body.pos[0]+disp[0], body.pos[1]+disp[1]
			gx := world.terrainZ(nx+0.5, ny) - world.terrainZ(nx-0.5, ny)
			gy := world.terrainZ(nx, ny+0.5) - world.terrainZ(nx, ny-0.5)
			g2 := gx*gx + gy*gy
			if g2 > 1e-12 {
				k := (disp[0]*gx + disp[1]*gy) / g2
				if k > 0.0 {
					disp = V2{disp[0] - gx*k, disp[1] - gy*k}
					kv := (body.vel[0]*gx + body.vel[1]*gy) / g2
					if kv > 0.0 {
						body.vel[0] -= gx * kv
						body.vel[1] -= gy * kv
					}
				}
			}
			if math.Hypot(disp[0], disp[1]) < 1e-9 || physicsAirTerrainBlocks(world, body.pos[0]+disp[0], body.pos[1]+disp[1], zt, lim) {
				disp = V2{0.0, 0.0}
				body.vel[0] = 0.0
				body.vel[1] = 0.0
			}
			info.blocked = true
		}
	}
	stepH := physicsEPS
	if !air {
		stepH = fmax(st.StepUp, 0.0)
	}
	dlen := math.Hypot(disp[0], disp[1])
	near := nearGather(world, body.pos[0], body.pos[1], 2.0*r+dlen+2.0*physicsSLIDE+0.5)
	p := V2{body.pos[0], body.pos[1]}
	v2 := V2{body.vel[0], body.vel[1]}
	if dlen > 1e-12 {
		n := uclamp(f2u(math.Ceil(dlen)), 1, 8)
		for it := 0; it < n; it++ {
			q := V2{p[0] + disp[0]/float64(n), p[1] + disp[1]/float64(n)}
			if physicsResolveIn(world, &near, &q, z, r, hgt, stepH, &v2) {
				p = q
			} else {
				v2 = V2{0.0, 0.0}
				info.blocked = true
				break
			}
		}
	}
	b := &world.bounds
	if b.maxX-b.minX > 2.0*r && b.maxY-b.minY > 2.0*r {
		if p[0] < b.minX+r {
			p[0] = b.minX + r
			v2[0] = fmax(v2[0], 0.0)
		}
		if p[0] > b.maxX-r {
			p[0] = b.maxX - r
			v2[0] = fmin(v2[0], 0.0)
		}
		if p[1] < b.minY+r {
			p[1] = b.minY + r
			v2[1] = fmax(v2[1], 0.0)
		}
		if p[1] > b.maxY-r {
			p[1] = b.maxY - r
			v2[1] = fmin(v2[1], 0.0)
		}
	}
	if math.Abs(v2[0]-body.vel[0])+math.Abs(v2[1]-body.vel[1]) > 1e-9 {
		info.blocked = true
	}
	body.vel[0] = v2[0]
	body.vel[1] = v2[1]

	// vertical move
	if !air {
		sup := physicsSupport(world, &near, p[0], p[1], z, r, stepH)
		if sup.s > z+physicsEPS && world.ceilingAbove(p[0], p[1], sup.s) < sup.s+hgt {
			// no head room on the step up: stay
			p = V2{prev.pos[0], prev.pos[1]}
			body.vel[0] = 0.0
			body.vel[1] = 0.0
			info.blocked = true
			sup = physicsSupport(world, &near, p[0], p[1], z, r, stepH)
		}
		body.pos[0] = p[0]
		body.pos[1] = p[1]
		if sup.edge > sup.s && sup.edge >= z-stepH {
			body.pos[2] = sup.edge
			body.vel[2] = 0.0
			toward := mv[0]*(sup.edgeCp[0]-p[0]) + mv[1]*(sup.edgeCp[1]-p[1])
			if toward <= 1e-9 {
				physicsSlideFrom(world, &near, &body.pos, sup.edgeCp, sup.edgeD, r, hgt)
			}
		} else if sup.s >= z-stepH {
			body.pos[2] = sup.s
			body.vel[2] = 0.0
		} else {
			body.grounded = false
			body.vel[2] = 0.0
		}
	} else {
		body.pos[0] = p[0]
		body.pos[1] = p[1]
		zn := z + body.vel[2]*dt
		if body.vel[2] > 0.0 {
			h0, h1 := z+hgt, zn+hgt
			ceil := math.Inf(1)
			hasEave := false
			var eaveCp V2
			var eaveD float64
			near.each(world, func(pr *Prism) {
				if pr.base < h0-physicsEPS || pr.base >= h1 || pr.base >= ceil {
					return
				}
				if p[0] < pr.minX-r || p[0] > pr.maxX+r || p[1] < pr.minY-r || p[1] > pr.maxY+r {
					return
				}
				if pr.containsXy(p[0], p[1]) {
					ceil = pr.base
					hasEave = false
				} else {
					dr := pr.distXy(p[0], p[1])
					if math.Abs(dr.f0) < r-1e-3 {
						ceil = pr.base
						hasEave = true
						eaveCp = dr.f1
						eaveD = math.Abs(dr.f0)
					}
				}
			})
			if isFinite(ceil) {
				zn = fmax(ceil-hgt, z)
				body.vel[2] = 0.0
				body.pos[2] = zn
				if hasEave {
					physicsSlideFrom(world, &near, &body.pos, eaveCp, eaveD, r, hgt)
				}
			}
		}
		sup := physicsSupport(world, &near, body.pos[0], body.pos[1], z, r, physicsEPS)
		floor := fmax(sup.s, sup.edge)
		if zn <= floor && (body.vel[2] <= 0.0 || zn < floor-physicsEPS) {
			body.pos[2] = floor
			body.vel[2] = 0.0
			body.grounded = true
			info.landed = !wasG
			if sup.edge > sup.s {
				toward := mv[0]*(sup.edgeCp[0]-body.pos[0]) + mv[1]*(sup.edgeCp[1]-body.pos[1])
				if toward <= 1e-9 {
					physicsSlideFrom(world, &near, &body.pos, sup.edgeCp, sup.edgeD, r, hgt)
				}
			}
		} else {
			body.pos[2] = zn
			body.grounded = false
		}
	}
	tz := world.terrainZ(body.pos[0], body.pos[1])
	if body.pos[2] < tz {
		body.pos[2] = tz
		body.vel[2] = 0.0
		body.grounded = true
	}
	ok := true
	for _, v := range body.pos {
		if !isFinite(v) {
			ok = false
			break
		}
	}
	if ok {
		for _, v := range body.vel {
			if !isFinite(v) {
				ok = false
				break
			}
		}
	}
	ok = ok && isFinite(body.energy) && isFinite(body.energyWait)
	if !ok {
		*body = prev
		body.vel = V3{}
		posOk := true
		for _, v := range body.pos {
			if !isFinite(v) {
				posOk = false
				break
			}
		}
		if !posOk {
			body.pos = V3{}
		}
		if !isFinite(body.energy) {
			body.energy = 0.0
		}
		if !isFinite(body.energyWait) {
			body.energyWait = 0.0
		}
	}
	return info
}
