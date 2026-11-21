package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"sort"
)

var debugLog = os.Getenv("MECH_DEBUG") != ""

type Track struct {
	A        Actor
	LastSeen float64
}

type Brain struct {
	w   *World
	cfg Config
	rng *rand.Rand
	me  int

	t      float64
	tracks map[int]*Track

	// combat state
	targetID     int
	lastDamagedT float64
	npcDamagedT  float64
	peekPos      [2]float64
	peekT        float64
	spot         *[2]float64
	spotT        float64
	lastDealtT   float64
	hitBearing   float64
	hitBy        int
	hitT         float64
	soundBearing float64
	soundT       float64

	// movement state
	astar       astarBuf
	path        [][2]float64
	pathGoal    [2]float64
	pathT       float64
	strafeSign  float64
	strafeT     float64
	stuckT      float64
	jetUntil    float64
	lastMoveMag float64

	laserHoldT float64
	takeFail   map[int]float64

	mode string
	dbg  Debug
}

func NewBrain(h *Hello) *Brain {
	b := &Brain{
		cfg:          h.Config,
		rng:          rand.New(rand.NewSource(h.Seed + 7)),
		me:           h.Slot,
		tracks:       map[int]*Track{},
		targetID:     -1,
		hitBy:        -1,
		strafeSign:   1,
		takeFail:     map[int]float64{},
		lastDamagedT: -100,
		npcDamagedT:  -100,
		lastDealtT:   -100,
		hitT:         -100,
		soundT:       -100,
	}
	dir := h.Map.Dir
	if dir == "" {
		dir = "/arena"
	}
	b.w = LoadWorld(dir)
	return b
}

// ---------- helpers ----------

func wrap(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func hyp(ax, ay, bx, by float64) float64 { return math.Hypot(bx-ax, by-ay) }

func (b *Brain) wstat(o *Obs, kind, key string, def float64) float64 {
	for _, wp := range o.Self.Weapons {
		if wp != nil && wp.Kind == kind {
			if v, ok := wp.Stats[key]; ok {
				return v
			}
		}
	}
	if m, ok := b.cfg.Weapons[kind]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return def
}

func (b *Brain) stat(o *Obs, key string, def float64) float64 {
	if v, ok := o.Self.Stats[key]; ok {
		return v
	}
	if v, ok := b.cfg.Mech[key]; ok {
		return v
	}
	return def
}

func slotOf(o *Obs, kind string) int {
	for i, wp := range o.Self.Weapons {
		if wp != nil && wp.Kind == kind {
			return i
		}
	}
	return -1
}

func weaponPrio(kind string) int {
	switch kind {
	case "laser":
		return 3
	case "lightning":
		return 2
	case "mortar":
		return 1
	}
	return 0
}

// zone circle at time t
func zoneAt(z Zone, t float64) (cx, cy, r float64) {
	if t <= z.ShrinkStart {
		return z.Center[0], z.Center[1], z.Radius
	}
	if t >= z.ShrinkEnd || z.ShrinkEnd <= z.ShrinkStart {
		return z.NextCenter[0], z.NextCenter[1], z.NextRadius
	}
	f := (t - z.ShrinkStart) / (z.ShrinkEnd - z.ShrinkStart)
	return z.Center[0] + (z.NextCenter[0]-z.Center[0])*f,
		z.Center[1] + (z.NextCenter[1]-z.Center[1])*f,
		z.Radius + (z.NextRadius-z.Radius)*f
}

// ---------- main decision ----------

func (b *Brain) Act(o *Obs) *Action {
	b.t = o.T
	b.dbg = Debug{}
	me := &o.Self
	x, y, z := me.Pos[0], me.Pos[1], me.Pos[2]
	act := &Action{Tick: o.Tick, Aim: me.Aim}

	// ---- events ----
	for _, e := range o.Events {
		switch e.Type {
		case "damaged":
			if e.Weapon == "npc" {
				b.npcDamagedT = b.t
			}
			if e.Weapon != "zone" {
				b.lastDamagedT = b.t
				if e.Bearing != nil {
					b.hitBearing = *e.Bearing
					b.hitT = b.t
				}
				if e.By != nil {
					b.hitBy = *e.By
				}
			}
		case "hit":
			b.lastDealtT = b.t
		case "take_failed":
			var id int
			if json.Unmarshal(e.ID, &id) == nil {
				b.takeFail[id] = b.t
			}
		}
	}
	for _, s := range o.Sounds {
		switch s.Kind {
		case "gun", "laser", "laser_charge", "lightning", "step", "boost", "jet", "mortar", "reload":
			if s.Dist != "far" || b.t-b.soundT > 2 {
				b.soundBearing = s.Bearing
				b.soundT = b.t - s.Age
			}
		}
	}

	// ---- upgrades ----
	if o.Offer != nil && len(o.Offer.Choices) > 0 {
		best, bs := 0, -1
		for i, c := range o.Offer.Choices {
			s := upgradeScore(c.ID)
			if s > bs {
				best, bs = i, s
			}
		}
		act.Pick = &best
	}

	// ---- perception ----
	var enemies, npcs []Actor
	for _, m := range o.Mechs {
		if m.Kind == "mech" {
			if m.ID == me.ID {
				continue
			}
			enemies = append(enemies, m)
			b.tracks[m.ID] = &Track{A: m, LastSeen: b.t}
		} else {
			npcs = append(npcs, m)
		}
	}
	_ = b.stat(o, "eye_height", 8)
	muzzleH := b.stat(o, "muzzle_height", 7)
	muzzle := [3]float64{x, y, z + muzzleH}

	los := func(a Actor) bool {
		h := 9.0 * (1 + 0.05*float64(a.Cubes))
		if a.Kind == "npc" {
			h = 3.5
		}
		c := [3]float64{a.Pos[0], a.Pos[1], a.Pos[2] + h*0.5}
		return b.w.Clear(muzzle, c, 3.0, 1.5)
	}

	// ---- target selection ----
	var target *Actor
	tLOS := false
	bestScore := math.Inf(1)
	for i := range enemies {
		e := &enemies[i]
		d := hyp(x, y, e.Pos[0], e.Pos[1])
		l := los(*e)
		s := e.HP + d*4
		if !l {
			s += 600
		}
		if e.ID == b.targetID {
			s -= 250
		}
		if s < bestScore {
			bestScore, target, tLOS = s, e, l
		}
	}
	targetIsMech := target != nil
	hpFrac := me.HP / math.Max(1, me.MaxHP)
	npcHitMe := b.t-b.npcDamagedT < 2.5
	// regen discipline: with no mech in sight and HP down, stop dealing damage so the
	// out-of-combat regen timer can run (it needs regen_delay s without dealing or taking damage)
	regen := len(enemies) == 0 && hpFrac < 0.85 && !npcHitMe

	// NPCs as targets when no mechs around (or when one is shooting us)
	if target == nil && ((hpFrac > 0.45 && !regen) || npcHitMe) {
		for i := range npcs {
			n := &npcs[i]
			d := hyp(x, y, n.Pos[0], n.Pos[1])
			if d > 110 {
				continue
			}
			l := los(*n)
			if !l {
				continue
			}
			s := n.HP + d*3
			if s < bestScore {
				bestScore, target, tLOS = s, n, l
			}
		}
	}
	if target != nil {
		b.targetID = target.ID
	} else {
		b.targetID = -1
	}

	// ---- zone ----
	zcx, zcy, zr := zoneAt(o.Zone, b.t)
	ncx, ncy, nr := o.Zone.NextCenter[0], o.Zone.NextCenter[1], o.Zone.NextRadius
	margin := math.Max(4, math.Min(35, nr*0.3))
	dNext := hyp(x, y, ncx, ncy)
	walk := b.stat(o, "walk_speed", 11.7)
	zoneUrgent := false
	var zoneGoal *[2]float64
	if dNext > nr-margin {
		g := [2]float64{ncx, ncy}
		if dNext > 1e-3 && nr-margin > 0 {
			k := (nr - margin) / dNext
			g = [2]float64{ncx + (x-ncx)*k, ncy + (y-ncy)*k}
		}
		zoneGoal = &g
		need := (dNext - (nr - margin)) / walk
		left := o.Zone.ShrinkEnd - b.t
		if !me.InZone || hyp(x, y, zcx, zcy) > zr-8 || need > left-12 {
			zoneUrgent = true
		}
	}
	// also: close to the edge of the current (shrinking) circle
	if c2x, c2y, r2 := zoneAt(o.Zone, b.t+4); hyp(x, y, c2x, c2y) > r2-5 {
		zoneUrgent = true
		if zoneGoal == nil {
			g := [2]float64{ncx, ncy}
			zoneGoal = &g
		}
	}
	safeInZone := func(px, py float64) bool {
		c2x, c2y, r2 := zoneAt(o.Zone, b.t+8)
		return hyp(px, py, c2x, c2y) < r2-6
	}

	// ---- weapons and aim ----
	aimYaw, aimPitch := me.Aim[0], 0.0
	haveAim := false
	gunSlot := slotOf(o, "gun")
	laserSlot := slotOf(o, "laser")
	ltgSlot := slotOf(o, "lightning")
	mortarSlot := slotOf(o, "mortar")
	var fire [2]bool
	targetDist := 0.0

	missOK := func(yaw, pitch, dist, rad float64) bool {
		dy := wrap(yaw - me.Aim[0])
		dp := pitch - me.Aim[1]
		miss := dist * math.Hypot(dy*math.Cos(pitch), dp)
		return miss < rad
	}

	var crateT *Crate
	if target != nil {
		tp := target.Pos
		th := 9.0 * (1 + 0.05*float64(target.Cubes))
		trad := 2.5 * (1 + 0.05*float64(target.Cubes))
		if target.Kind == "npc" {
			th, trad = 3.5, 1.8
		}
		targetDist = hyp(x, y, tp[0], tp[1])
		center := [3]float64{tp[0], tp[1], tp[2] + th*0.5}

		// gun lead
		gspeed := b.wstat(o, "gun", "speed", 252)
		airborne := math.Abs(target.Vel[2]) > 0.5
		g := b.stat(o, "gravity", 22)
		pred := center
		for it := 0; it < 4; it++ {
			dd := math.Sqrt(sq(pred[0]-muzzle[0]) + sq(pred[1]-muzzle[1]) + sq(pred[2]-muzzle[2]))
			tt := dd/gspeed + 0.05
			pred[0] = center[0] + target.Vel[0]*tt
			pred[1] = center[1] + target.Vel[1]*tt
			pred[2] = center[2]
			if airborne {
				pred[2] = center[2] + target.Vel[2]*tt - 0.5*g*tt*tt
				floor := b.w.Surf(pred[0], pred[1]) + th*0.5
				if pred[2] < floor {
					pred[2] = floor
				}
			}
		}
		gunYaw := math.Atan2(pred[1]-muzzle[1], pred[0]-muzzle[0])
		gunPitch := math.Atan2(pred[2]-muzzle[2], hyp(muzzle[0], muzzle[1], pred[0], pred[1]))
		directYaw := math.Atan2(center[1]-muzzle[1], center[0]-muzzle[0])
		directPitch := math.Atan2(center[2]-muzzle[2], targetDist)

		aimYaw, aimPitch, haveAim = gunYaw, gunPitch, true

		// laser
		if laserSlot >= 0 {
			lw := o.Self.Weapons[laserSlot]
			full := b.wstat(o, "laser", "charge", 0.75)
			rng := b.wstat(o, "laser", "range", 280)
			if lw.Charge > 0 {
				if lw.Charge >= full-0.25 {
					aimYaw, aimPitch = directYaw, directPitch
				}
				hold := true
				if lw.Charge >= full-0.02 && tLOS && missOK(directYaw, directPitch, targetDist, trad*0.8) {
					hold = false // release: fire
				}
				if !tLOS && lw.Charge >= full && b.t-b.laserHoldT > 1.0 {
					hold = true // keep holding, auto-fires after hold_s
				}
				fire[laserSlot] = hold
			} else if lw.Cooldown <= 0.05 && (tLOS || targetIsMech) && targetDist < rng-10 &&
				(targetDist > 35 || (gunSlot < 0 && ltgSlot < 0)) {
				// (without LOS: charge behind cover, release when we peek out)
				// charging halves our speed: not worth it point-blank when we have another weapon
				fire[laserSlot] = true // start charging
				b.laserHoldT = b.t
			}
		}
		// gun
		if gunSlot >= 0 {
			gw := o.Self.Weapons[gunSlot]
			grange := b.wstat(o, "gun", "range", 230)
			if gw.Reload <= 0 && gw.Ammo > 0 && tLOS && targetDist < grange &&
				missOK(gunYaw, gunPitch, targetDist, trad*0.9+targetDist*0.004) &&
				b.w.Clear(muzzle, pred, 3.0, 2.0) {
				fire[gunSlot] = true
			}
		}
		// lightning
		if ltgSlot >= 0 {
			lr := b.wstat(o, "lightning", "range", 45)
			cone := b.wstat(o, "lightning", "cone_deg", 30) * math.Pi / 180
			cost := b.wstat(o, "lightning", "energy", 3)
			if targetDist < lr*0.95 && tLOS && math.Abs(wrap(directYaw-me.Aim[0])) < cone*0.4 && me.Energy > cost+8 {
				fire[ltgSlot] = true
			}
			if targetDist < lr*1.3 && (laserSlot < 0 || o.Self.Weapons[laserSlot].Charge == 0) {
				aimYaw, aimPitch = directYaw, directPitch
			}
		}
		// mortar: lob at targets we can't shoot directly, or alongside at mid range
		if mortarSlot >= 0 && target.Kind == "mech" {
			mw := o.Self.Weapons[mortarSlot]
			if mw.Cooldown <= 0.05 && targetDist > 25 {
				mspeed := b.wstat(o, "mortar", "speed", 72)
				mg := b.wstat(o, "mortar", "gravity", 22)
				tube := [3]float64{x, y, z + b.stat(o, "height", 9) + 0.5}
				useHigh := !tLOS
				if yaw, pitch, ok := ballistic(tube, [3]float64{tp[0], tp[1], tp[2] + 1}, target.Vel, mspeed, mg, useHigh); ok {
					gunBusy := gunSlot < 0 || o.Self.Weapons[gunSlot].Reload > 0 || o.Self.Weapons[gunSlot].Ammo <= 0 || !tLOS
					laserBusy := laserSlot < 0 || o.Self.Weapons[laserSlot].Charge == 0
					if gunBusy && laserBusy {
						aimYaw, aimPitch = yaw, pitch
						if missOK(yaw, pitch, targetDist, 3.0) {
							fire[mortarSlot] = true
						}
					}
				}
			}
		}
		b.dbg.Lines = append(b.dbg.Lines, DbgLine{A: muzzle, B: center, Color: "#ff5040"})
	} else {
		// no target: crates for cubes/loot when calm
		if laserSlot >= 0 && o.Self.Weapons[laserSlot].Charge > 0 {
			// keep the charge briefly in case the target pops back out
			fire[laserSlot] = b.hasRecentTrack(1.5)
		}
		if b.t-b.lastDamagedT > 3 && !regen {
			bestD := 140.0
			for i := range o.Crates {
				c := &o.Crates[i]
				d := hyp(x, y, c.Pos[0], c.Pos[1])
				if d < bestD && safeInZone(c.Pos[0], c.Pos[1]) {
					cc := [3]float64{c.Pos[0], c.Pos[1], c.Pos[2] + 1.2}
					if b.w.Clear(muzzle, cc, 3.0, 1.0) {
						bestD, crateT = d, c
					}
				}
			}
		}
		if crateT != nil && gunSlot >= 0 {
			cc := [3]float64{crateT.Pos[0], crateT.Pos[1], crateT.Pos[2] + 1.2}
			d := hyp(muzzle[0], muzzle[1], cc[0], cc[1])
			aimYaw = math.Atan2(cc[1]-muzzle[1], cc[0]-muzzle[0])
			aimPitch = math.Atan2(cc[2]-muzzle[2], d)
			haveAim = true
			gw := o.Self.Weapons[gunSlot]
			if gw.Reload <= 0 && gw.Ammo > 0 && missOK(aimYaw, aimPitch, d, 1.0) {
				fire[gunSlot] = true
			}
		}
	}

	// reload when idle
	if gunSlot >= 0 {
		gw := o.Self.Weapons[gunSlot]
		mag := b.wstat(o, "gun", "mag", 12)
		if gw.Reload <= 0 && !fire[gunSlot] && ((target == nil && gw.Ammo < mag && crateT == nil) || gw.Ammo <= 0) {
			act.Reload = true
		}
	}

	// ---- threats: incoming projectiles and laser lines ----
	dodge := [2]float64{}
	urgentDodge := false
	cz := z + 4.5
	for _, p := range o.Projectiles {
		vx, vy, vz := p.Vel[0], p.Vel[1], p.Vel[2]
		if p.Kind == "mortar" {
			// estimate landing point
			lx, ly, ok := shellLanding(b.w, p.Pos, p.Vel, b.wstat(o, "mortar", "gravity", 22))
			if ok {
				d := hyp(x, y, lx, ly)
				if d < 12 {
					ax, ay := x-lx, y-ly
					if d < 0.5 {
						ax, ay = math.Cos(b.rng.Float64()*6.28), math.Sin(b.rng.Float64()*6.28)
						d = 1
					}
					dodge[0] += ax / d * 2
					dodge[1] += ay / d * 2
					urgentDodge = true
				}
			}
			continue
		}
		rx, ry, rz := x-p.Pos[0], y-p.Pos[1], cz-p.Pos[2]
		v2 := vx*vx + vy*vy + vz*vz
		if v2 < 1 {
			continue
		}
		tc := (rx*vx + ry*vy + rz*vz) / v2
		if tc < 0 || tc > 1.2 {
			continue
		}
		mx, my := p.Pos[0]+vx*tc-x, p.Pos[1]+vy*tc-y
		md := math.Hypot(mx, my)
		if md < 4.5 {
			// move perpendicular to the bullet, away from its line
			px, py := -vy, vx
			n := math.Hypot(px, py)
			px, py = px/n, py/n
			if px*mx+py*my > 0 { // closest point is on +perp side: move to -perp
				px, py = -px, -py
			}
			dodge[0] += px
			dodge[1] += py
		}
	}
	laserThreat := false
	for _, e := range enemies {
		if !e.Charging {
			continue
		}
		d := hyp(x, y, e.Pos[0], e.Pos[1])
		toMe := math.Atan2(y-e.Pos[1], x-e.Pos[0])
		off := math.Abs(wrap(e.Aim[0] - toMe))
		if d*math.Sin(math.Min(off, 1.5)) < 8 && d < 300 {
			if !los(e) {
				continue
			}
			laserThreat = true
			px, py := -math.Sin(toMe), math.Cos(toMe)
			dodge[0] += px * b.strafeSign * 2
			dodge[1] += py * b.strafeSign * 2
			urgentDodge = true
		}
	}

	// ---- movement goal ----
	var goal *[2]float64
	moveScale := 1.0
	boost := false
	b.mode = "roam"
	strafe := false
	prefRange := 80.0
	if laserSlot >= 0 {
		prefRange = 130
	}
	if ltgSlot >= 0 && laserSlot < 0 {
		prefRange = 28
	}

	// fight evaluation: compare time-to-kill both ways
	var threats [][3]float64
	nLOS := 0
	for _, e := range enemies {
		if hyp(x, y, e.Pos[0], e.Pos[1]) < 220 && los(e) {
			nLOS++
			threats = append(threats, e.Pos)
		}
	}
	retreat := false
	if targetIsMech {
		ours := 0.0
		for _, wp := range o.Self.Weapons {
			if wp != nil {
				ours += weaponDPS(wp.Kind, targetDist)
			}
		}
		ours *= 1 + 0.04*math.Min(float64(me.Cubes), 10)
		theirs := 0.0
		kinds := target.Weapons
		if len(kinds) == 0 {
			kinds = []string{"gun"}
		}
		for _, k := range kinds {
			theirs += weaponDPS(k, targetDist)
		}
		theirs *= 1 + 0.04*math.Min(float64(target.Cubes), 10)
		myTTL := me.HP / math.Max(1, theirs)
		theirTTL := target.HP / math.Max(1, ours)
		losing := myTTL < theirTTL*0.85
		retreat = (losing && hpFrac < 0.55) || hpFrac < 0.22 || (nLOS >= 2 && hpFrac < 0.75)
		// finishing blow: don't run from a nearly dead target
		if target.HP < 0.2*target.MaxHP && theirTTL < myTTL {
			retreat = false
		}
	}
	if len(threats) == 0 && target != nil {
		threats = append(threats, target.Pos)
	}
	if zoneUrgent && zoneGoal != nil {
		goal = zoneGoal
		b.mode = "zone"
		boost = !me.InZone || me.Energy > 50
		if target != nil {
			strafe = true
		}
	} else if retreat {
		b.mode = "retreat"
		if c := b.findCover(o, threats, safeInZone, nil); c != nil {
			goal = c
		}
		boost = me.Energy > 30
		strafe = goal == nil
	} else if target != nil && targetIsMech {
		b.mode = "fight"
		strafe = true
		tp := target.Pos
		// peek-and-hide: duck behind nearby cover while a laser is aimed at us or our guns are down,
		// step back out to the last firing spot when it's safe
		ourDown := true
		if gunSlot >= 0 {
			gw := o.Self.Weapons[gunSlot]
			if gw.Reload <= 0 && gw.Ammo > 0 {
				ourDown = false
			}
		}
		if laserSlot >= 0 {
			lw := o.Self.Weapons[laserSlot]
			if lw.Charge > 0 || lw.Cooldown < 0.4 {
				ourDown = false
			}
		}
		if ltgSlot >= 0 && targetDist < 45 {
			ourDown = false
		}
		enemyCharging := false
		for _, e := range enemies {
			if e.Charging {
				enemyCharging = true
			}
		}
		if tLOS {
			b.peekPos = [2]float64{x, y}
			b.peekT = b.t
		}
		hidden := false
		if debugLog {
			c := b.findCover(o, threats, safeInZone, []float64{4, 7, 10, 13})
			logf("t=%.1f los=%v lt=%v down=%v ech=%v dist=%.0f cover=%v", b.t, tLOS, laserThreat, ourDown, enemyCharging, targetDist, c != nil)
		}
		if tLOS && (laserThreat || ourDown) && targetDist > 30 {
			if c := b.findCover(o, threats, safeInZone, []float64{4, 7, 10, 13}); c != nil {
				goal = c
				b.mode = "hide"
				strafe = false
				boost = me.Energy > 20
				hidden = true
			}
		} else if !tLOS && (enemyCharging || ourDown) && b.t-b.peekT < 6 {
			// stay behind cover until their charge is spent and ours is ready
			b.mode = "cover"
			strafe = false
			hidden = true
			moveScale = 0.3
		} else if !tLOS && b.t-b.peekT < 6 && hyp(x, y, b.peekPos[0], b.peekPos[1]) < 25 {
			g := b.peekPos
			goal = &g
			b.mode = "peek"
			strafe = false
			hidden = true
		}
		if !hidden && tLOS && targetDist > 40 {
			if b.t-b.spotT > 1.0 {
				b.spot = b.fightSpot(o, target, threats, safeInZone, prefRange)
				b.spotT = b.t
			}
			if b.spot != nil && hyp(x, y, b.spot[0], b.spot[1]) > 3 {
				g := *b.spot
				goal = &g
				b.mode = "position"
				hidden = true
			}
		}
		if hidden {
			// handled above
		} else if !tLOS || targetDist > prefRange+35 {
			g := [2]float64{tp[0], tp[1]}
			goal = &g
		} else if targetDist < prefRange-30 && prefRange > 40 {
			// back away
			ax, ay := x-tp[0], y-tp[1]
			n := math.Max(1e-3, math.Hypot(ax, ay))
			g := [2]float64{x + ax/n*20, y + ay/n*20}
			if safeInZone(g[0], g[1]) {
				goal = &g
			}
		}
	} else if target != nil { // npc
		b.mode = "npc"
		strafe = true
		if targetDist > 70 {
			g := [2]float64{target.Pos[0], target.Pos[1]}
			goal = &g
		}
	}

	// loot when not fighting
	var takeID *int
	if b.mode == "roam" && regen {
		// heal up: stay quiet, behind cover from the last mech we saw, only grab loot right next to us
		b.mode = "regen"
		moveScale = 0.42
		if g, tid := b.lootGoal(o, safeInZone, true); g != nil && hyp(x, y, g[0], g[1]) < 15 {
			goal, takeID = g, tid
		} else if tr := b.recentTrack(); tr != nil && b.t-tr.LastSeen < 20 {
			teye := [3]float64{tr.A.Pos[0], tr.A.Pos[1], tr.A.Pos[2] + 8}
			if b.w.Clear(teye, [3]float64{x, y, z + 6}, 1, 1) {
				if c := b.findCover(o, [][3]float64{tr.A.Pos}, safeInZone, nil); c != nil {
					goal = c
					moveScale = 1
				}
			}
		} else if zoneGoal != nil {
			goal = zoneGoal
		}
	}
	if b.mode == "roam" || b.mode == "npc" {
		if g, tid := b.lootGoal(o, safeInZone, len(enemies) > 0); g != nil {
			goal = g
			b.mode = "loot"
			takeID = tid
		} else if dg := b.dropGoal(o, len(enemies) > 0); dg != nil {
			goal = dg
			b.mode = "drop"
		} else if zoneGoal != nil && goal == nil {
			goal = zoneGoal
			b.mode = "zone"
		} else if goal == nil && crateT == nil && target == nil {
			// drift to a point near the next zone centre, quietly
			g := b.idleSpot(o)
			if g != nil && hyp(x, y, g[0], g[1]) > 12 {
				goal = g
			}
			moveScale = 0.42 // below step speed: silent
		}
	}
	// take any good pickup in reach regardless of mode
	if t := b.reachablePickup(o); t != nil {
		takeID = t
	}
	if takeID != nil {
		pk := *takeID
		act.Take = &pk
		// both slots full: replace the worse one
		if o.Self.Weapons[0] != nil && len(o.Self.Weapons) > 1 && o.Self.Weapons[1] != nil {
			var kind string
			for _, p := range o.Pickups {
				if p.ID == pk {
					kind = p.Kind
				}
			}
			s := 1
			if weaponPrio(o.Self.Weapons[1].Kind) >= weaponPrio(kind) {
				act.Take = nil
			} else if o.Self.Weapons[1].Charge > 0 {
				act.Take = nil
			} else {
				act.Drop = &s
			}
		}
	}

	// ---- compute move direction ----
	mv := [2]float64{}
	if goal != nil {
		mv = b.follow(o, *goal)
	}
	if strafe && target != nil {
		if b.t > b.strafeT {
			b.strafeSign = -b.strafeSign
			b.strafeT = b.t + 0.35 + b.rng.Float64()*0.8
		}
		toT := math.Atan2(target.Pos[1]-y, target.Pos[0]-x)
		sx, sy := -math.Sin(toT)*b.strafeSign, math.Cos(toT)*b.strafeSign
		w := 0.9
		if goal != nil {
			w = 0.6
		}
		mv[0] += sx * w
		mv[1] += sy * w
	}
	mv[0] += dodge[0] * 1.5
	mv[1] += dodge[1] * 1.5
	mag := math.Hypot(mv[0], mv[1])
	if mag > 1e-6 {
		mv[0], mv[1] = mv[0]/mag, mv[1]/mag
		mv = b.safeDir(x, y, mv)
		mv[0] *= moveScale
		mv[1] *= moveScale
	}
	if urgentDodge && me.Energy > 15 {
		boost = true
		mv[0], mv[1] = mv[0]/math.Max(1e-6, math.Hypot(mv[0], mv[1])), mv[1]/math.Max(1e-6, math.Hypot(mv[0], mv[1]))
	}
	if laserSlot >= 0 && o.Self.Weapons[laserSlot].Charge > 0 {
		boost = false // can't sprint usefully while charging anyway
	}

	// ---- stuck handling ----
	hs := math.Hypot(me.Vel[0], me.Vel[1])
	want := math.Hypot(mv[0], mv[1])
	if want > 0.3 && hs < 1.5 && me.Grounded {
		b.stuckT += 0.1
	} else if hs > 3 {
		b.stuckT = 0
	}
	if b.stuckT > 0.8 {
		b.jetUntil = b.t + 0.7
		b.stuckT = 0
		b.path = nil
		b.strafeSign = -b.strafeSign
	}
	if b.t < b.jetUntil {
		if me.Grounded {
			act.Jump = true
		} else {
			boost = me.Energy > 10
		}
	}

	// ---- aim when idle ----
	if !haveAim {
		switch {
		case b.t-b.hitT < 3:
			aimYaw = b.hitBearing
		case b.t-b.soundT < 2.5:
			aimYaw = b.soundBearing
		case b.hasRecentTrack(4):
			tr := b.recentTrack()
			aimYaw = math.Atan2(tr.A.Pos[1]-y, tr.A.Pos[0]-x)
		case want > 0.2:
			base := math.Atan2(mv[1], mv[0])
			ph := math.Mod(b.t, 4.0)
			switch {
			case ph < 2.0:
				aimYaw = base
			case ph < 3.0:
				aimYaw = base + 1.6
			default:
				aimYaw = base - 1.6
			}
		default:
			aimYaw = me.Aim[0] + 1.2
		}
		aimPitch = 0
	}

	act.Move = [2]float64{round3(mv[0]), round3(mv[1])}
	act.Boost = boost
	act.Aim = [2]float64{round4(wrap(aimYaw)), round4(clamp(aimPitch, b.stat(o, "min_pitch", -0.7), b.stat(o, "max_pitch", 1.3)))}
	act.Fire = fire
	b.dbg.Label = b.mode
	if len(b.dbg.Lines) > 0 || b.dbg.Label != "" {
		d := b.dbg
		act.Debug = &d
	}
	return act
}

func sq(v float64) float64 { return v * v }
func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

func (b *Brain) hasRecentTrack(s float64) bool {
	return b.recentTrack() != nil && b.t-b.recentTrack().LastSeen < s
}

func (b *Brain) recentTrack() *Track {
	var best *Track
	for _, t := range b.tracks {
		if best == nil || t.LastSeen > best.LastSeen {
			best = t
		}
	}
	return best
}

func upgradeScore(id string) int {
	order := []string{"plating", "armor", "laser_damage", "nanites", "gun_damage", "laser_charge", "gun_rate",
		"lightning_efficiency", "gun_mag", "servos", "gun_velocity", "mortar_reload", "reactor", "capacitor",
		"lightning_range", "jets", "gyros", "sensors", "lightning_chain", "mortar_velocity", "mortar_splash", "laser_pierce"}
	for i, s := range order {
		if s == id {
			return 100 - i
		}
	}
	return 0
}

// ballistic solves the launch yaw/pitch to hit tgt (leading with vel) from origin.
func ballistic(org, tgt, vel [3]float64, v, g float64, high bool) (float64, float64, bool) {
	p := tgt
	var yaw, pitch float64
	ok := false
	for it := 0; it < 4; it++ {
		dx, dy := p[0]-org[0], p[1]-org[1]
		d := math.Hypot(dx, dy)
		h := p[2] - org[2]
		disc := v*v*v*v - g*(g*d*d+2*h*v*v)
		if disc < 0 || d < 1 {
			return 0, 0, false
		}
		sq := math.Sqrt(disc)
		var th float64
		if high {
			th = math.Atan((v*v + sq) / (g * d))
		} else {
			th = math.Atan((v*v - sq) / (g * d))
		}
		tt := d / (v * math.Cos(th))
		yaw, pitch, ok = math.Atan2(dy, dx), th, true
		p = [3]float64{tgt[0] + vel[0]*tt, tgt[1] + vel[1]*tt, tgt[2]}
	}
	return yaw, pitch, ok
}

func shellLanding(w *World, pos, vel [3]float64, g float64) (float64, float64, bool) {
	p, v := pos, vel
	dt := 0.05
	for i := 0; i < 200; i++ {
		p[0] += v[0] * dt
		p[1] += v[1] * dt
		p[2] += v[2] * dt
		v[2] -= g * dt
		if v[2] < 0 && w.Solid(p[0], p[1], p[2]) {
			return p[0], p[1], true
		}
		if p[2] < -60 {
			return p[0], p[1], true
		}
	}
	return 0, 0, false
}

// follow returns a unit direction along a path to goal.
func (b *Brain) follow(o *Obs, goal [2]float64) [2]float64 {
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	if hyp(x, y, goal[0], goal[1]) < 2 {
		return [2]float64{}
	}
	w := b.w
	if !w.ok {
		d := hyp(x, y, goal[0], goal[1])
		return [2]float64{(goal[0] - x) / d, (goal[1] - y) / d}
	}
	// straight line if walkable and short
	if hyp(x, y, goal[0], goal[1]) < 40 && w.WalkLine(x, y, goal[0], goal[1]) {
		d := hyp(x, y, goal[0], goal[1])
		b.path = nil
		return [2]float64{(goal[0] - x) / d, (goal[1] - y) / d}
	}
	replan := b.path == nil || hyp(goal[0], goal[1], b.pathGoal[0], b.pathGoal[1]) > 10 || b.t-b.pathT > 2.5
	if !replan && len(b.path) > 0 {
		// deviated?
		if hyp(x, y, b.path[0][0], b.path[0][1]) > 12 {
			replan = true
		}
	}
	if replan {
		s := w.NearestStand(x, y, 6, false)
		t := w.NearestStand(goal[0], goal[1], 15, true)
		if t < 0 {
			t = w.NearestStand(goal[0], goal[1], 15, false)
		}
		b.path = w.Path(&b.astar, s, t, 10000, nil)
		b.pathGoal = goal
		b.pathT = b.t
	}
	// consume reached waypoints
	for len(b.path) > 0 && hyp(x, y, b.path[0][0], b.path[0][1]) < 3 {
		b.path = b.path[1:]
	}
	if len(b.path) == 0 {
		d := hyp(x, y, goal[0], goal[1])
		return [2]float64{(goal[0] - x) / d, (goal[1] - y) / d}
	}
	// look ahead
	pick := b.path[0]
	for k := 1; k < len(b.path) && k < 10; k++ {
		if w.WalkLine(x, y, b.path[k][0], b.path[k][1]) {
			pick = b.path[k]
		} else {
			break
		}
	}
	d := math.Max(1e-6, hyp(x, y, pick[0], pick[1]))
	return [2]float64{(pick[0] - x) / d, (pick[1] - y) / d}
}

// safeDir rotates a move direction away from walls/drops into unwalkable cells.
func (b *Brain) safeDir(x, y float64, mv [2]float64) [2]float64 {
	w := b.w
	if !w.ok || !w.Standable(x, y) {
		return mv
	}
	base := math.Atan2(mv[1], mv[0])
	for _, da := range []float64{0, 0.4, -0.4, 0.8, -0.8, 1.2, -1.2, 1.6, -1.6, 2.2, -2.2, 3.0} {
		a := base + da
		if w.WalkLine(x, y, x+math.Cos(a)*4, y+math.Sin(a)*4) {
			return [2]float64{math.Cos(a), math.Sin(a)}
		}
	}
	return mv
}

func (b *Brain) findCover(o *Obs, threats [][3]float64, safe func(x, y float64) bool, radii []float64) *[2]float64 {
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	if len(threats) == 0 {
		return nil
	}
	var best *[2]float64
	bd := math.Inf(1)
	if radii == nil {
		radii = []float64{12, 22, 34, 48}
	}
	for _, r := range radii {
		for k := 0; k < 16; k++ {
			a := float64(k) * math.Pi / 8
			px, py := x+math.Cos(a)*r, y+math.Sin(a)*r
			if !b.w.Standable(px, py) || !safe(px, py) {
				continue
			}
			pz := b.w.Surf(px, py)
			seen := false
			near := math.Inf(1)
			for _, th := range threats {
				teye := [3]float64{th[0], th[1], th[2] + 8}
				if b.w.Clear(teye, [3]float64{px, py, pz + 6}, 1, 1) {
					seen = true
					break
				}
				near = math.Min(near, hyp(px, py, th[0], th[1]))
			}
			if seen {
				continue
			}
			// prefer short moves that end away from the threats
			s := r + 0.3*(60-near)
			if s < bd {
				bd = s
				g := [2]float64{px, py}
				best = &g
			}
		}
		if best != nil {
			return best
		}
	}
	return best
}

// lootGoal picks a nearby weapon pickup or cube worth walking to.
func (b *Brain) lootGoal(o *Obs, safe func(x, y float64) bool, enemySeen bool) (*[2]float64, *int) {
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	maxD := 90.0
	if enemySeen {
		maxD = 30
	}
	haveLaser := slotOf(o, "laser") >= 0
	cur := 0
	empty := len(o.Self.Weapons) > 1 && o.Self.Weapons[1] == nil
	if !empty && len(o.Self.Weapons) > 1 && o.Self.Weapons[1] != nil {
		cur = weaponPrio(o.Self.Weapons[1].Kind)
	}
	var best *[2]float64
	var bid *int
	bs := math.Inf(1)
	for _, p := range o.Pickups {
		pr := weaponPrio(p.Kind)
		if pr == 0 || slotOf(o, p.Kind) >= 0 {
			continue
		}
		if !empty && pr <= cur {
			continue
		}
		if ft, ok := b.takeFail[p.ID]; ok && b.t-ft < 5 {
			continue
		}
		d := hyp(x, y, p.Pos[0], p.Pos[1])
		lim := maxD
		if p.Kind == "laser" && !haveLaser {
			lim = maxD * 1.8
		}
		if d > lim || !safe(p.Pos[0], p.Pos[1]) {
			continue
		}
		if math.Abs(p.Pos[2]-b.w.Ground(p.Pos[0], p.Pos[1])) > 2 && math.Abs(p.Pos[2]-o.Self.Pos[2]) > 3 {
			continue // on a roof we're not on
		}
		s := d - float64(pr)*25
		if p.Kind == "laser" && !haveLaser {
			s -= 60
		}
		if s < bs {
			bs = s
			g := [2]float64{p.Pos[0], p.Pos[1]}
			best = &g
			id := p.ID
			bid = &id
		}
	}
	for _, c := range o.Cubes {
		d := hyp(x, y, c.Pos[0], c.Pos[1])
		if d > maxD || !safe(c.Pos[0], c.Pos[1]) {
			continue
		}
		if math.Abs(c.Pos[2]-b.w.Ground(c.Pos[0], c.Pos[1])) > 2 && math.Abs(c.Pos[2]-o.Self.Pos[2]) > 3 {
			continue
		}
		s := d - 15
		if s < bs {
			bs = s
			g := [2]float64{c.Pos[0], c.Pos[1]}
			best = &g
			bid = nil
		}
	}
	return best, bid
}

func (b *Brain) reachablePickup(o *Obs) *int {
	x, y, z := o.Self.Pos[0], o.Self.Pos[1], o.Self.Pos[2]
	r := 4.0
	empty := len(o.Self.Weapons) > 1 && o.Self.Weapons[1] == nil
	cur := 0
	if !empty && len(o.Self.Weapons) > 1 && o.Self.Weapons[1] != nil {
		cur = weaponPrio(o.Self.Weapons[1].Kind)
	}
	type c struct {
		id, pr int
		d      float64
	}
	var cs []c
	for _, p := range o.Pickups {
		pr := weaponPrio(p.Kind)
		if pr == 0 || slotOf(o, p.Kind) >= 0 || (!empty && pr <= cur) {
			continue
		}
		d := hyp(x, y, p.Pos[0], p.Pos[1])
		if d < r-0.3 && math.Abs(p.Pos[2]-z) < 3.8 {
			cs = append(cs, c{p.ID, pr, d})
		}
	}
	if len(cs) == 0 {
		return nil
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].pr > cs[j].pr })
	id := cs[0].id
	return &id
}

func (b *Brain) dropGoal(o *Obs, enemySeen bool) *[2]float64 {
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	walk := b.stat(o, "walk_speed", 11.7)
	for _, d := range o.Drops {
		if d.Holder != nil && *d.Holder != o.Self.ID {
			continue
		}
		dist := hyp(x, y, d.Pos[0], d.Pos[1])
		if dist > 220 {
			continue
		}
		if enemySeen && dist > 40 {
			continue
		}
		if dist/walk > d.LandsIn+30 {
			continue
		}
		g := [2]float64{d.Pos[0], d.Pos[1]}
		if dist < d.Radius*0.6 {
			// stay inside the ring
			return &g
		}
		return &g
	}
	return nil
}

func (b *Brain) idleSpot(o *Obs) *[2]float64 {
	ncx, ncy, nr := o.Zone.NextCenter[0], o.Zone.NextCenter[1], o.Zone.NextRadius
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	d := hyp(x, y, ncx, ncy)
	target := math.Min(nr*0.45, 60)
	if d <= target {
		return nil
	}
	k := target / d
	g := [2]float64{ncx + (x-ncx)*k, ncy + (y-ncy)*k}
	return &g
}

// weaponDPS is a rough expected damage rate at distance d, including typical misses.
func weaponDPS(kind string, d float64) float64 {
	switch kind {
	case "gun":
		if d < 230 {
			return 150 * math.Max(0.25, 1-d/260)
		}
	case "laser":
		if d < 280 {
			return 115
		}
	case "lightning":
		if d < 45 {
			return 190
		}
	case "mortar":
		if d > 20 && d < 235 {
			return 55
		}
	}
	return 0
}

// fightSpot finds a nearby standable point that can shoot the target and has a hiding place within ~8 m.
func (b *Brain) fightSpot(o *Obs, target *Actor, threats [][3]float64, safe func(x, y float64) bool, pref float64) *[2]float64 {
	w := b.w
	if !w.ok {
		return nil
	}
	x, y := o.Self.Pos[0], o.Self.Pos[1]
	th := 9.0 * (1 + 0.05*float64(target.Cubes))
	tc := [3]float64{target.Pos[0], target.Pos[1], target.Pos[2] + th*0.5}
	teye := [3]float64{target.Pos[0], target.Pos[1], target.Pos[2] + 8}
	hideAt := func(px, py float64) bool {
		for _, r := range []float64{5, 8} {
			for k := 0; k < 8; k++ {
				a := float64(k) * math.Pi / 4
				hx, hy := px+math.Cos(a)*r, py+math.Sin(a)*r
				if !w.Standable(hx, hy) || !w.WalkLine(px, py, hx, hy) {
					continue
				}
				hz := w.Surf(hx, hy)
				blocked := !w.Clear(teye, [3]float64{hx, hy, hz + 6}, 1, 1)
				for _, t := range threats {
					if !blocked {
						break
					}
					blocked = !w.Clear([3]float64{t[0], t[1], t[2] + 8}, [3]float64{hx, hy, hz + 6}, 1, 1)
				}
				if blocked {
					return true
				}
			}
		}
		return false
	}
	var best *[2]float64
	bs := math.Inf(1)
	try := func(px, py, r float64) {
		if !w.Standable(px, py) || !safe(px, py) {
			return
		}
		pz := w.Surf(px, py)
		if !w.Clear([3]float64{px, py, pz + 7}, tc, 3, 1.5) {
			return
		}
		if !hideAt(px, py) {
			return
		}
		d := hyp(px, py, target.Pos[0], target.Pos[1])
		s := r + 0.25*math.Abs(d-pref)
		if s < bs {
			bs = s
			g := [2]float64{px, py}
			best = &g
		}
	}
	try(x, y, 0)
	if best != nil {
		return best
	}
	for _, r := range []float64{8, 16, 26} {
		for k := 0; k < 12; k++ {
			a := float64(k) * math.Pi / 6
			try(x+math.Cos(a)*r, y+math.Sin(a)*r, r)
		}
	}
	return best
}
