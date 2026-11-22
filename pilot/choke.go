package pilot

import (
	"math"
	"time"
)

// Choke: when outnumbered, fight from a walled pocket that attackers can only reach through
// one opening, so they come at us one or two at a time.
//
// Candidates are precomputed once at startup from the nav grid: a ground node whose
// surroundings are walled within chokeWallR in all directions but one contiguous arc, and
// whose opening has a clear lane out to chokeLaneR (so we can shoot through it).

const (
	chokeDirs   = 16
	chokeWallR  = 20.0 // a direction counts as walled if a wall is this close
	chokeLaneR  = 40.0 // the middle of the opening must be clear this far
	chokeRayH   = 5.0  // ray height above the feet
	chokeGrid   = 1    // sample every nav cell (2 m)
	chokeSep    = 12.0 // keep one spot per 12 m
	chokeOpenLo = 2    // opening width in directions (2..7 of 16 = 45..157 degrees)
	chokeOpenHi = 7
)

type ChokeSpot struct {
	pos  V3
	yaw  float64 // centre of the opening
	half float64 // half width of the opening (rad)
}

type ChokeState struct {
	spots     []ChokeSpot
	active    bool
	spot      int
	since     float64 // when the current spot was chosen
	arrived   float64 // when we first reached it (<0 = travelling)
	lastFoeT  float64 // last time a hunter was in line of sight or approaching while active
	outnumT   float64 // last time we were outnumbered
	peekUntil float64
	peekNext  float64
	ban       []struct {
		i int
		t float64
	}
}

// chokeBuild scans the nav grid for pocket positions. It stops at the deadline and keeps what it found.
func chokeBuild(world *World, nav *Nav, deadline time.Time) []ChokeSpot {
	var out []ChokeSpot
	n := nav.nodeCount()
	var dirs [chokeDirs]V2
	for k := 0; k < chokeDirs; k++ {
		a := float64(k) * 2 * math.Pi / chokeDirs
		dirs[k] = V2{math.Cos(a), math.Sin(a)}
	}
	for i := 0; i < n; i++ {
		if i&1023 == 0 && time.Now().After(deadline) {
			break
		}
		node := uint32(i)
		ci, cj := nav.cellIj(node)
		if ci%chokeGrid != 0 || cj%chokeGrid != 0 || nav.nodeRoofFn(node) {
			continue
		}
		p := nav.nodePos(node)
		if !(isFinite(p[0]) && isFinite(p[1]) && isFinite(p[2])) {
			continue
		}
		o := V3{p[0], p[1], p[2] + chokeRayH}
		var open [chokeDirs]bool
		nOpen := 0
		for k := 0; k < chokeDirs; k++ {
			e := V3{o[0] + dirs[k][0]*chokeWallR, o[1] + dirs[k][1]*chokeWallR, o[2]}
			if _, hit := world.rayT(o, e); !hit {
				open[k] = true
				nOpen++
			}
		}
		if nOpen < chokeOpenLo || nOpen > chokeOpenHi {
			continue
		}
		// exactly one contiguous open run (circular)
		runs, start := 0, -1
		for k := 0; k < chokeDirs; k++ {
			prev := open[(k+chokeDirs-1)%chokeDirs]
			if open[k] && !prev {
				runs++
				start = k
			}
		}
		if runs != 1 {
			continue
		}
		mid := float64(start) + float64(nOpen-1)/2.0
		yaw := wrap(mid * 2 * math.Pi / chokeDirs)
		e := V3{o[0] + math.Cos(yaw)*chokeLaneR, o[1] + math.Sin(yaw)*chokeLaneR, o[2]}
		if _, hit := world.rayT(o, e); hit {
			continue
		}
		dup := false
		for j := len(out) - 1; j >= 0 && j >= len(out)-64; j-- {
			if dist(out[j].pos, p) < chokeSep {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, ChokeSpot{pos: p, yaw: yaw, half: float64(nOpen) * math.Pi / chokeDirs})
		}
	}
	return out
}

// chokeHunters counts mechs that have line on us or are closing in on us.
func (b *Brain) chokeHunters(obs *Obs, world *World) (int, V2) {
	t := obs.T
	me := &obs.Me
	body := up(me.Pos, fmax(me.Stats.Height, 1.0)*0.6)
	n := 0
	var c V2
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || t-tr.lastSeen > 3.0 {
			continue
		}
		q := tr.posAt(t)
		d := dist(q, me.Pos)
		if d > 200.0 {
			continue
		}
		line := t-tr.lastDmgT < 1.0 || (t-tr.lastSeen < 0.5 && world.sight(up(q, b.cfg.Mech.EyeHeight), body))
		closing := false
		if d > 1.0 {
			// velocity component toward us
			vr := (tr.vel[0]*(me.Pos[0]-q[0]) + tr.vel[1]*(me.Pos[1]-q[1])) / d
			closing = vr > 2.0 && d < 150.0
		}
		if line || closing {
			n++
			c[0] += q[0]
			c[1] += q[1]
		}
	}
	if n > 0 {
		c[0] /= float64(n)
		c[1] /= float64(n)
	}
	return n, c
}

func (b *Brain) chokeSpotOk(obs *Obs, sp *ChokeSpot) bool {
	z := &obs.Zone
	margin := fmax(10.0, z.NextRadius*0.15)
	if z.NextRadius <= 25.0 || dist2v(sp.pos, z.NextCenter) > z.NextRadius-margin {
		return false
	}
	za := strategyZoneAt(z, obs.T, obs.T, false)
	return dist2v(sp.pos, za.f0) <= za.f1-10.0
}

func (b *Brain) chokeBanned(i int, t float64) bool {
	for _, x := range b.choke.ban {
		if x.i == i && x.t > t {
			return true
		}
	}
	return false
}

func (b *Brain) chokeBan(i int, t float64) {
	kept := b.choke.ban[:0]
	for _, x := range b.choke.ban {
		if x.t > t {
			kept = append(kept, x)
		}
	}
	b.choke.ban = append(kept, struct {
		i int
		t float64
	}{i, t + 40.0})
}

func (b *Brain) chokePick(obs *Obs, foes V2) int {
	me := &obs.Me
	walk := fmax(me.Stats.WalkSpeed, 1.0)
	best, bestS := -1, math.Inf(1)
	toFoe := unit2(foes[0]-me.Pos[0], foes[1]-me.Pos[1])
	for i := range b.choke.spots {
		sp := &b.choke.spots[i]
		d := dist(sp.pos, me.Pos)
		if d > b.p.chokeMaxD || math.Abs(sp.pos[2]-me.Pos[2]) > 6.0 || b.chokeBanned(i, obs.T) || !b.chokeSpotOk(obs, sp) {
			continue
		}
		s := d / walk
		// The opening should face the attackers so we can shoot through it.
		fy := math.Atan2(foes[1]-sp.pos[1], foes[0]-sp.pos[0])
		if math.Abs(wrap(fy-sp.yaw)) > sp.half+0.5 {
			s += 6.0
		}
		// Do not run through the attackers to get there.
		if d > 1.0 {
			dir := unit2(sp.pos[0]-me.Pos[0], sp.pos[1]-me.Pos[1])
			if dir[0]*toFoe[0]+dir[1]*toFoe[1] > 0.5 {
				s += 4.0
			}
		}
		if s < bestS {
			best, bestS = i, s
		}
	}
	return best
}

// chokeDecide runs after decide and may take over the mode and goal.
func (b *Brain) chokeDecide(obs *Obs, world *World, unreachable bool) {
	p := b.p
	if p.chokeOn < 0.5 || len(b.choke.spots) == 0 {
		return
	}
	t := obs.T
	me := &obs.Me
	if b.mode == ModeZone || !me.InZone {
		b.choke.active = false
		return
	}
	n, foes := b.chokeHunters(obs, world)
	if n >= 2 {
		b.choke.outnumT = t
	}
	cs := &b.choke
	if cs.active {
		sp := &cs.spots[cs.spot]
		if n > 0 {
			cs.lastFoeT = t
		}
		leave := !b.chokeSpotOk(obs, sp) ||
			t-cs.lastFoeT > p.chokeQuietS ||
			(cs.arrived < 0.0 && t-cs.since > p.chokeTravelS)
		if unreachable || (cs.arrived < 0.0 && t-cs.since > p.chokeTravelS) {
			b.chokeBan(cs.spot, t)
		}
		if leave {
			cs.active = false
			return
		}
	} else {
		if n < 2 {
			return
		}
		i := b.chokePick(obs, foes)
		if i < 0 {
			return
		}
		cs.active, cs.spot, cs.since, cs.arrived, cs.lastFoeT = true, i, t, -1.0, t
		cs.peekUntil, cs.peekNext = -1.0, t
	}
	sp := &cs.spots[cs.spot]
	d := dist2(sp.pos, me.Pos)
	if cs.arrived < 0.0 && d < 2.5 {
		cs.arrived = t
	}
	// Fight, do not hide: regen rest is off while holding the pocket.
	b.vx.regroup = false
	b.vx.hold = false
	if cs.arrived < 0.0 {
		b.mode = ModeRetreat
		b.goal = Goal{kind: GoalKindNav, navPos: sp.pos, navArrive: 1.5}
		b.goalId = -1
		return
	}
	// In the pocket. If an attacker is in the opening, hold the spot and shoot (fire control
	// keeps the target). If none is visible from inside, peek out along the opening for a
	// moment, then step back.
	eye := up(sp.pos, fmax(me.Stats.EyeHeight, 1.0))
	visible := false
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 0.5 && dist(tr.posAt(t), sp.pos) < 230.0 &&
			world.sight(eye, up(tr.posAt(t), b.cfg.Mech.Height*0.5)) {
			visible = true
			if b.target.kind != TgtKindMech {
				b.target = Tgt{kind: TgtKindMech, mech0: tr.id}
			}
			break
		}
	}
	goal := sp.pos
	if !visible && t-cs.lastFoeT < p.chokeQuietS {
		if t >= cs.peekNext {
			cs.peekUntil = t + p.chokePeekS
			cs.peekNext = t + p.chokePeekS + p.chokeBackS
		}
		if t < cs.peekUntil {
			goal = V3{sp.pos[0] + math.Cos(sp.yaw)*p.chokePeekD, sp.pos[1] + math.Sin(sp.yaw)*p.chokePeekD, sp.pos[2]}
		}
	}
	b.mode = ModeFight
	b.goal = Goal{kind: GoalKindNav, navPos: goal, navArrive: 1.0}
	b.goalId = -1
}
