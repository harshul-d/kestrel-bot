package pilot

import (
	"fmt"
	"math"
	"os"
	"time"
)

type Mode int

const (
	ModeIdle Mode = iota
	ModeZone
	ModeFight
	ModeRetreat
	ModeLoot
	ModeDrop
	ModeHunt
	ModeRoof
)

type GoalKind int

const (
	GoalKindNone GoalKind = iota
	GoalKindNav
	GoalKindField
	GoalKindRange
)

type Goal struct {
	kind        GoalKind
	navPos      V3
	navArrive   float64
	fieldCenter V2
	fieldArrive float64
	rangePos    V3
	rangePref   float64
}

type TgtKind int

const (
	TgtKindNone TgtKind = iota
	TgtKindMech
	TgtKindNpc
	TgtKindCrate
)

type Tgt struct {
	kind   TgtKind
	mech0  int64
	npc0   int64
	crate0 int64
}

type Brain struct {
	cfg     Config
	p       *Params
	pFfa    *Params // free-for-all settings, swapped in on detection (nil = none)
	shared  *Shared
	rng     Rng
	mem     Memory
	mode    Mode
	goal    Goal
	goalId  int64
	pursuit struct {
		f0 int64
		f1 float64
	}
	target      Tgt
	allowCrates bool
	field       *struct {
		f0 V3
		f1 *Field
		f2 float64
	}
	zoneField *struct {
		f0 V2
		f1 float64
		f2 *Field
	}
	fsvc            FieldsvcFieldSvc
	budget          Budget
	goalUnreachable bool
	cover           *struct {
		f0 V3
		f1 float64
	}
	lastDir      V2
	jukeSide     float64
	jukeNext     float64
	unstickUntil float64
	unstickDir   V2
	lastTargetT  float64
	zoneB        bool
	unstickCount uint32
	unstickLevel uint32
	cmdSince     float64
	wantsSince   float64
	movingSince  float64
	tAct         time.Time
	noLos        struct {
		f0 int64
		f1 float64
	}
	jet *struct {
		f0 V3
		f1 float64
	}
	sq   SquadBook
	econ EconEcon
	gear GearGear
	sf   SafetySafety
	wave WaveWaveState
	vx   VxVx
	ffa  FfaFfa
	spot *struct {
		f0 V3
		f1 float64
		f2 bool
	}
	order           OrderOrder
	orderHasPassive bool
	roof            *RoofRoofMap
	roofSel         *int
	roofSelT        float64
	roofOn          *int
	roofRest        bool
	roofHold        bool
	roofJet         bool
	roofBan         []struct {
		f0 int
		f1 float64
	}
	roofStep *struct {
		f0 int64
		f1 *V3
		f2 float64
	}
	roofFirstT float64
	kills      uint32
	pk         LaserPeek
	tele       TeleTele
	bu         BurstBurstSt
	aimz       AimzAimZ
	cad        AimzCadence
	cadUntil   float64
	cadAxis    V2
}

func brainNew(hello *Hello, shared *Shared) *Brain {
	return brainWithParams(hello, shared, paramsLoad())
}

func brainWithParams(hello *Hello, shared *Shared, params Params) *Brain {
	rng := rngNew(uint64(hello.Seed) ^ (uint64(hello.Slot) << 40))
	side := 1.0
	if rng.f64() < 0.5 {
		side = -1.0
	}
	var roof *RoofRoofMap
	if params.roofMode > 0.0 {
		mn, ms := f2u(fmax(params.roofMinNodes, 1.0)), params.roofMinSetback*0.5
		roof = func() (r *RoofRoofMap) {
			defer func() {
				if recover() != nil {
					r = nil
				}
			}()
			m := roofRoofMapBuild(&shared.world, &shared.nav, mn, ms, hello.Config.Mech.Height)
			return &m
		}()
	}
	return &Brain{
		cfg:    hello.Config,
		p:      &params,
		pFfa:   paramsLoadFfa(),
		shared: shared,
		rng:    rng,
		mem:    memoryNew(),
		mode:   ModeIdle,
		goal:   Goal{},
		goalId: -1,
		pursuit: struct {
			f0 int64
			f1 float64
		}{-1, 0.0},
		target:          Tgt{},
		allowCrates:     false,
		field:           nil,
		zoneField:       nil,
		fsvc:            FieldsvcFieldSvc{},
		budget:          budgetUnlimited(),
		goalUnreachable: false,
		cover:           nil,
		lastDir:         V2{0.0, 0.0},
		jukeSide:        side,
		jukeNext:        0.0,
		unstickUntil:    -1.0,
		unstickDir:      V2{0.0, 0.0},
		lastTargetT:     -1e9,
		zoneB:           false,
		unstickCount:    0,
		unstickLevel:    0,
		cmdSince:        0.0,
		wantsSince:      0.0,
		movingSince:     0.0,
		tAct:            time.Now(),
		noLos: struct {
			f0 int64
			f1 float64
		}{-1, 0.0},
		jet:             nil,
		sq:              squadBookNew(),
		econ:            EconEcon{},
		gear:            GearGear{},
		sf:              SafetySafety{},
		wave:            waveWaveStateNew(),
		vx:              VxVx{},
		ffa:             ffaFfaNew(hello.Seed),
		spot:            nil,
		order:           OrderOrder{},
		orderHasPassive: false,
		roof:            roof,
		roofSel:         nil,
		roofSelT:        -1e9,
		roofOn:          nil,
		roofRest:        false,
		roofHold:        false,
		roofJet:         false,
		roofBan:         nil,
		roofStep:        nil,
		roofFirstT:      -1.0,
		kills:           0,
		pk:              laserPeekNew(),
		tele:            TeleTele{},
		bu:              BurstBurstSt{},
		aimz:            aimzAimZNew(),
		cad:             aimzCadenceNew(),
		cadUntil:        -1.0,
		cadAxis:         V2{1.0, 0.0},
	}
}

func (b *Brain) act(obs *Obs, budget *Budget) Action {
	b.tAct = time.Now()
	act := b.actInner(obs, budget)
	return act
}

func (b *Brain) actInner(obs *Obs, budget *Budget) Action {
	b.budget = *budget
	act := actionNew(obs.Tick)
	shared := b.shared
	world := &shared.world
	nav := &shared.nav
	b.econUpdate(obs)
	b.waveCount(obs)
	b.mem.rangeBan = b.p.takeRangeBan
	b.mem.update(obs, world, &b.cfg)
	b.sfUpdate(obs, world)
	b.vxUpdate(obs, world)
	if b.p.aimzOn > 0.0 {
		b.aimz.update(obs)
	}
	if b.p.wCad > 0.0 {
		b.cad.update(obs, b.p.cadPeriod, b.p.cadSnap)
	}
	b.waveUpdate(obs, world, nav)
	b.ffa.update(obs, b.mem.tracks, b.p.ffaOn > 0.5 || b.pFfa != nil)
	if b.ffa.on && b.pFfa != nil && b.p != b.pFfa {
		b.p = b.pFfa
		if _, ok := os.LookupEnv("FFA_LOG"); ok {
			logf("FFA settings on at t=%.1f why=%d", obs.T, b.ffa.why)
		}
	}
	b.order.update(obs)
	b.teleUpdate(obs)
	if b.p.haltMulti >= 1.0 && !b.order.halted && float64(b.order.onUs(obs.T)) >= b.p.haltMulti {
		b.order.halted = true
	}
	if obs.Offer != nil {
		act.Pick = ptr(b.pickUpgrade(obs.Offer))
	}
	b.decide(obs, world, nav)
	b.laserPre(obs, world, nav)
	fc := b.fireControl(obs, world)
	if y := b.gearSweep(obs, fc.target != nil || fc.fire[0] || fc.fire[1] || fc.charging); y != nil {
		fc.aim = V2{*y, 0.0}
	}
	mv := b.planMove(obs, world, nav, budget, &fc)
	if !mv.boost && b.gearBoost(obs) && math.Hypot(mv.dir[0], mv.dir[1]) > 0.5 {
		mv.boost = true
	}
	if b.vx.seeking && b.p.skBoost >= 0.5 && obs.Me.Grounded && obs.Me.Energy > obs.Me.MaxEnergy*0.5 && math.Hypot(mv.dir[0], mv.dir[1]) > 0.5 {
		mv.boost = true
	}
	if jb := b.gearJet(obs); jb != nil {
		mv.jump = mv.jump || jb.f0
		mv.boost = mv.boost || jb.f1
	}
	b.laserPost(obs, world, &fc, &mv)
	b.esFilter(obs, &fc)
	if b.vx.hold {
		fc.fire = [2]bool{false, false}
	}
	b.burstPost(obs, world, &fc, &mv)
	b.lootAction(obs, &act)
	b.sfDrop(obs, &act)
	var q float64
	if mv.boost || b.mode != ModeIdle {
		q = 1.0
	} else {
		q = b.ffaQuiet(obs)
	}
	act.Move = ptr(V2{mv.dir[0] * q, mv.dir[1] * q})
	act.Boost = ptr(mv.boost)
	act.Jump = ptr(mv.jump)
	act.Aim = ptr(fc.aim)
	act.Fire = ptr(fc.fire)
	act.Reload = ptr(fc.reload)
	if (b.p.haltAfter >= 0.0 && obs.T >= b.p.haltAfter) || b.order.halted {
		act.Move = ptr(V2{0.0, 0.0})
		act.Boost = ptr(false)
		act.Jump = ptr(false)
		act.Fire = ptr([2]bool{false, false})
		act.Reload = ptr(false)
		act.Take = nil
		act.Drop = nil
	}
	brainSanitize(&act, &obs.Me.Stats)
	return act
}

func (b *Brain) held(t float64) bool {
	return b.p.holdUntil >= 0.0 && t < b.p.holdUntil
}

// state is a short state line for logs and tools.
func (b *Brain) state() string {
	sel, on := "None", "None"
	if b.roofSel != nil {
		sel = fmt.Sprintf("Some(%d)", *b.roofSel)
	}
	if b.roofOn != nil {
		on = fmt.Sprintf("Some(%d)", *b.roofOn)
	}
	return fmt.Sprintf("%v %+v %+v tracks=%d items=%d/%d/%d roof=%s/%s/%v first=%.1f", b.mode, b.goal, b.target, len(b.mem.tracks), len(b.mem.crates), len(b.mem.cubes), len(b.mem.pickups), sel, on, b.roofRest, b.roofFirstT)
}

func (b *Brain) spentMs() float64 {
	return time.Since(b.tAct).Seconds() * 1000.0
}

func (b *Brain) unsticks() uint32 {
	return b.unstickCount
}

func (b *Brain) pickUpgrade(of *Offer) int64 {
	for _, id := range b.p.upgradePriority {
		for i := 0; i < len(of.Choices) && i < 3; i++ {
			if of.Choices[i].Id == id {
				return int64(i)
			}
		}
	}
	return 0
}

func (b *Brain) prefOf(k WeaponKind) float64 {
	switch k {
	case WeaponKindGun:
		return b.p.prefGun
	case WeaponKindLaser:
		return b.p.prefLaser
	case WeaponKindLightning:
		return b.p.prefLightning
	case WeaponKindMortar:
		return b.p.prefMortar
	}
	return -1.0
}

// want reports whether kind is wanted; slot nil means an empty slot, else the slot to swap out.
func (b *Brain) want(me *SelfObs, kind WeaponKind) (wanted bool, slot *int64) {
	if kind == WeaponKindMortar && b.sfNoMortar() {
		return false, nil
	}
	if kind == WeaponKindUnknown {
		return false, nil
	}
	for _, w := range me.Weapons {
		if w != nil && w.Kind == kind {
			return false, nil
		}
	}
	for _, w := range me.Weapons {
		if w == nil {
			return true, nil
		}
	}
	var worst *struct {
		f0 float64
		f1 int
	}
	for i, w := range me.Weapons {
		if w == nil {
			continue
		}
		if w.Kind == WeaponKindLaser && w.Charge > 0.0 {
			continue
		}
		pv := b.prefOf(w.Kind)
		if worst == nil || pv < worst.f0 {
			worst = &struct {
				f0 float64
				f1 int
			}{pv, i}
		}
	}
	if worst != nil && worst.f0+0.5 < b.prefOf(kind) {
		return true, ptr(int64(worst.f1))
	}
	return false, nil
}

func (b *Brain) lootAction(obs *Obs, act *Action) {
	me := &obs.Me
	reach := fmax(b.cfg.Loot.PickupRadius-0.3, 0.5)
	var best *struct {
		f0 float64
		f1 int64
		f2 *int64
	}
	for i := range obs.Pickups {
		pk := &obs.Pickups[i]
		if dist2(pk.Pos, me.Pos) > reach || math.Abs(pk.Pos[2]-me.Pos[2]) > 3.7 {
			continue
		}
		if b.mem.blacklisted(pk.Id, obs.T) {
			continue
		}
		if wanted, slot := b.want(me, pk.Kind); wanted {
			v := b.prefOf(pk.Kind)
			if best == nil || v > best.f0 {
				best = &struct {
					f0 float64
					f1 int64
					f2 *int64
				}{v, pk.Id, slot}
			}
		}
	}
	if best != nil {
		act.Take = ptr(best.f1)
		act.Drop = best.f2
	}
}

func brainSanitize(act *Action, st *MechCfg) {
	m := V2{0.0, 0.0}
	if act.Move != nil {
		m = *act.Move
	}
	if !isFinite(m[0]) || !isFinite(m[1]) {
		m = V2{0.0, 0.0}
	}
	l := math.Hypot(m[0], m[1])
	if l > 1.0 {
		m = V2{m[0] / l * 0.999_999, m[1] / l * 0.999_999}
	}
	act.Move = ptr(m)
	a := V2{0.0, 0.0}
	if act.Aim != nil {
		a = *act.Aim
	}
	var lo, hi float64
	if st.MinPitch < st.MaxPitch {
		lo, hi = st.MinPitch, st.MaxPitch
	} else {
		lo, hi = -0.7, 1.3
	}
	act.Aim = ptr(V2{wrap(a[0]), fclamp(fin(a[1], 0.0), lo, hi)})
	if act.Fire == nil {
		act.Fire = ptr([2]bool{false, false})
	}
	if act.Pick != nil {
		act.Pick = ptr(iclamp(*act.Pick, 0, 2))
	}
}
