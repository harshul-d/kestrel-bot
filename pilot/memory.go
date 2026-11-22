package pilot

import (
	"math"
	"slices"
)

type Track struct {
	id          int64
	kind        ActorKind
	pos         V3
	vel         V3
	acc         V3
	aim         V2
	hp          float64
	maxHp       float64
	cubes       float64
	weapons     []WeaponKind
	charging    bool
	chargeSince float64
	lastSeen    float64
	dmgToUs     float64
	lastDmgT    float64
}

func (tr *Track) has(k WeaponKind) bool {
	return slices.Contains(tr.weapons, k)
}

func (tr *Track) posAt(t float64) V3 {
	dt := fclamp(t-tr.lastSeen, 0.0, 1.0)
	return add(tr.pos, scale(V3{tr.vel[0], tr.vel[1], 0.0}, dt))
}

type Item struct {
	id       int64
	pos      V3
	hp       float64
	kind     WeaponKind
	lastSeen float64
	misses   uint32
}

type Heard struct {
	kind    string
	bearing float64
	dist    SoundDist
	t       float64
}

type Unseen struct {
	bearing float64
	t       float64
	from    V3
	laser   bool
}

type Memory struct {
	tracks      []Track
	crates      []Item
	cubes       []Item
	pickups     []Item
	sounds      []Heard
	lastDamageT float64
	lastDealtT  float64
	unseen      *Unseen
	hist        []struct {
		f0 float64
		f1 V3
	}
	blacklist []struct {
		f0 int64
		f1 float64
	}
	lastT      float64
	rangeFails []struct {
		f0 int64
		f1 uint32
	}
	rangeBan float64
}

func memoryAllFinite3(v V3) bool {
	return isFinite(v[0]) && isFinite(v[1]) && isFinite(v[2])
}

func memoryShouldSee(obs *Obs, world *World, cfg *Config, p V3) bool {
	me := &obs.Me
	eye := up(me.Pos, me.Stats.EyeHeight)
	d := dist(eye, p)
	near := cfg.Vision.NearRange + me.Stats.NearRangeBonus - 3.0
	far := cfg.Vision.Range + me.Stats.VisionBonus - 8.0
	var inRange bool
	if d <= near {
		inRange = true
	} else if d <= far {
		yaw := math.Atan2(p[1]-eye[1], p[0]-eye[0])
		pi := math.Pi
		inRange = math.Abs(adiff(yaw, me.Aim[0])) < (cfg.Vision.FovDeg*0.5)*(pi/180.0)-0.12
	} else {
		inRange = false
	}
	return inRange && world.sight(eye, up(p, 1.0))
}

func memoryMergeItems(list *[]Item, seen []struct {
	f0 int64
	f1 V3
	f2 float64
	f3 WeaponKind
}, obs *Obs, world *World, cfg *Config, checkBudget *int32) {
	t := obs.T
	for _, s := range seen {
		id, pos, hp, kind := s.f0, s.f1, s.f2, s.f3
		found := false
		for i := range *list {
			it := &(*list)[i]
			if it.id == id {
				it.pos = pos
				it.hp = hp
				it.kind = kind
				it.lastSeen = t
				it.misses = 0
				found = true
				break
			}
		}
		if !found {
			*list = append(*list, Item{id: id, pos: pos, hp: hp, kind: kind, lastSeen: t, misses: 0})
		}
	}
	for i := range *list {
		it := &(*list)[i]
		if it.lastSeen < t && *checkBudget > 0 {
			*checkBudget -= 1
			if memoryShouldSee(obs, world, cfg, it.pos) {
				it.misses += 1
			}
		}
	}
	*list = slices.DeleteFunc(*list, func(i Item) bool { return !(i.misses < 2 && memoryAllFinite3(i.pos)) })
	if len(*list) > 200 {
		cut := len(*list) - 200
		*list = slices.Delete(*list, 0, cut)
	}
}

func memoryNew() Memory {
	return Memory{
		tracks:      nil,
		crates:      nil,
		cubes:       nil,
		pickups:     nil,
		sounds:      nil,
		lastDamageT: -1e9,
		lastDealtT:  -1e9,
		unseen:      nil,
		hist:        nil,
		blacklist:   nil,
		lastT:       -1.0,
		rangeFails:  nil,
		rangeBan:    0.0,
	}
}

func (m *Memory) blacklisted(id int64, t float64) bool {
	for _, x := range m.blacklist {
		if x.f0 == id && x.f1 > t {
			return true
		}
	}
	return false
}

func (m *Memory) ban(id int64, until float64) {
	m.blacklist = slices.DeleteFunc(m.blacklist, func(x struct {
		f0 int64
		f1 float64
	}) bool {
		return x.f0 == id
	})
	m.blacklist = append(m.blacklist, struct {
		f0 int64
		f1 float64
	}{id, until})
	if len(m.blacklist) > 64 {
		m.blacklist = slices.Delete(m.blacklist, 0, 1)
	}
}

func (m *Memory) update(obs *Obs, world *World, cfg *Config) {
	t := obs.T
	me := &obs.Me
	var dt float64
	if m.lastT >= 0.0 {
		dt = fclamp(t-m.lastT, 0.02, 2.0)
	} else {
		dt = 0.1
	}
	m.lastT = t

	decay := math.Exp(-dt / 4.0)
	for i := range m.tracks {
		m.tracks[i].dmgToUs *= decay
	}

	for ai := range obs.Mechs {
		a := &obs.Mechs[ai]
		if a.Id == me.Id || a.Kind == ActorKindUnknown || !memoryAllFinite3(a.Pos) {
			continue
		}
		var hp, maxHp float64
		if a.Hp != nil {
			hp = *a.Hp
		} else if a.Kind == ActorKindNpc {
			hp = cfg.Npc.Hp
		} else {
			hp = cfg.Mech.Hp
		}
		if a.MaxHp != nil {
			maxHp = *a.MaxHp
		} else if a.Kind == ActorKindNpc {
			maxHp = cfg.Npc.Hp
		} else {
			maxHp = cfg.Mech.Hp
		}
		var tr *Track
		for i := range m.tracks {
			if m.tracks[i].id == a.Id {
				tr = &m.tracks[i]
				break
			}
		}
		if tr != nil {
			dtr := t - tr.lastSeen
			if dtr > 1e-6 && dtr <= 0.25 {
				na := scale(sub(a.Vel, tr.vel), 1.0/dtr)
				tr.acc = add(scale(tr.acc, 0.5), scale(na, 0.5))
			} else {
				tr.acc = V3{}
			}
			if a.Charging && !tr.charging {
				tr.chargeSince = t
			}
			tr.kind = a.Kind
			tr.pos = a.Pos
			tr.vel = a.Vel
			tr.aim = a.Aim
			tr.hp = hp
			tr.maxHp = maxHp
			tr.cubes = a.Cubes
			tr.weapons = slices.Clone(a.Weapons)
			tr.charging = a.Charging
			tr.lastSeen = t
		} else {
			m.tracks = append(m.tracks, Track{
				id:          a.Id,
				kind:        a.Kind,
				pos:         a.Pos,
				vel:         a.Vel,
				acc:         V3{},
				aim:         a.Aim,
				hp:          hp,
				maxHp:       maxHp,
				cubes:       a.Cubes,
				weapons:     slices.Clone(a.Weapons),
				charging:    a.Charging,
				chargeSince: t,
				lastSeen:    t,
				dmgToUs:     0.0,
				lastDmgT:    -1e9,
			})
		}
	}

	for ei := range obs.Events {
		ev := &obs.Events[ei]
		switch ev.kind {
		case EventKindDamaged:
			d := &ev.damaged0
			if d.Weapon == "zone" || d.By == nil {
				continue
			}
			m.lastDamageT = t
			by := *d.By
			fresh := false
			for i := range m.tracks {
				tr := &m.tracks[i]
				if tr.id == by {
					tr.dmgToUs += fmax(d.Amount, 0.0)
					tr.lastDmgT = t
					fresh = t-tr.lastSeen < 0.3
					break
				}
			}
			if !fresh {
				if d.Bearing != nil {
					bb := *d.Bearing
					if isFinite(bb) {
						m.unseen = &Unseen{bearing: bb, t: t, from: me.Pos, laser: d.Weapon == "laser"}
					}
				}
			}
		case EventKindHit:
			h := &ev.hit0
			m.lastDealtT = t
			for i := range m.crates {
				if m.crates[i].id == h.Target {
					m.crates[i].hp -= h.Amount
					break
				}
			}
		case EventKindKill:
			k := &ev.kill0
			m.tracks = slices.DeleteFunc(m.tracks, func(x Track) bool { return x.id == k.Target })
		case EventKindTook:
			k := &ev.took0
			m.pickups = slices.DeleteFunc(m.pickups, func(x Item) bool { return x.id == k.Id })
		case EventKindTakeFailed:
			f := &ev.takeFailed0
			switch f.Reason {
			case TakeFailReasonGone:
				m.pickups = slices.DeleteFunc(m.pickups, func(x Item) bool { return x.id == f.Id })
			case TakeFailReasonRange:
				if m.rangeBan > 0.0 {
					var n uint32
					idx := -1
					for i := range m.rangeFails {
						if m.rangeFails[i].f0 == f.Id {
							idx = i
							break
						}
					}
					if idx >= 0 {
						m.rangeFails[idx].f1 += 1
						n = m.rangeFails[idx].f1
					} else {
						m.rangeFails = append(m.rangeFails, struct {
							f0 int64
							f1 uint32
						}{f.Id, 1})
						if len(m.rangeFails) > 32 {
							m.rangeFails = slices.Delete(m.rangeFails, 0, 1)
						}
						n = 1
					}
					if float64(n) >= m.rangeBan {
						m.rangeFails = slices.DeleteFunc(m.rangeFails, func(x struct {
							f0 int64
							f1 uint32
						}) bool {
							return x.f0 == f.Id
						})
						m.ban(f.Id, t+8.0)
					}
				}
			default:
				m.ban(f.Id, t+8.0)
			}
		}
	}

	m.tracks = slices.DeleteFunc(m.tracks, func(x Track) bool {
		return !(t-x.lastSeen < 25.0 && !(x.kind == ActorKindNpc && x.hp <= 0.0))
	})
	sortStable(m.tracks, func(a, b *Track) int {
		switch {
		case a.id < b.id:
			return -1
		case a.id > b.id:
			return 1
		}
		return 0
	})

	budget := int32(60)
	crates := make([]struct {
		f0 int64
		f1 V3
		f2 float64
		f3 WeaponKind
	}, 0, len(obs.Crates))
	for _, c := range obs.Crates {
		hp := cfg.Crates.Hp
		if c.Hp != nil {
			hp = *c.Hp
		}
		crates = append(crates, struct {
			f0 int64
			f1 V3
			f2 float64
			f3 WeaponKind
		}{c.Id, c.Pos, hp, WeaponKindUnknown})
	}
	memoryMergeItems(&m.crates, crates, obs, world, cfg, &budget)
	cubes := make([]struct {
		f0 int64
		f1 V3
		f2 float64
		f3 WeaponKind
	}, 0, len(obs.Cubes))
	for _, c := range obs.Cubes {
		cubes = append(cubes, struct {
			f0 int64
			f1 V3
			f2 float64
			f3 WeaponKind
		}{c.Id, c.Pos, 0.0, WeaponKindUnknown})
	}
	memoryMergeItems(&m.cubes, cubes, obs, world, cfg, &budget)
	picks := make([]struct {
		f0 int64
		f1 V3
		f2 float64
		f3 WeaponKind
	}, 0, len(obs.Pickups))
	for _, c := range obs.Pickups {
		picks = append(picks, struct {
			f0 int64
			f1 V3
			f2 float64
			f3 WeaponKind
		}{c.Id, c.Pos, 0.0, c.Kind})
	}
	memoryMergeItems(&m.pickups, picks, obs, world, cfg, &budget)

	for _, s := range obs.Sounds {
		if isFinite(s.Bearing) {
			m.sounds = append(m.sounds, Heard{kind: s.Kind, bearing: s.Bearing, dist: s.Dist, t: t - fmax(s.Age, 0.0)})
		}
	}
	m.sounds = slices.DeleteFunc(m.sounds, func(s Heard) bool { return !(t-s.t < 6.0) })
	if len(m.sounds) > 64 {
		cut := len(m.sounds) - 64
		m.sounds = slices.Delete(m.sounds, 0, cut)
	}
	if m.unseen != nil {
		if t-m.unseen.t > 6.0 {
			m.unseen = nil
		}
	}
	m.blacklist = slices.DeleteFunc(m.blacklist, func(x struct {
		f0 int64
		f1 float64
	}) bool {
		return !(x.f1 > t)
	})

	m.hist = append(m.hist, struct {
		f0 float64
		f1 V3
	}{t, me.Pos})
	for len(m.hist) > 0 && t-m.hist[0].f0 > 3.0 {
		m.hist = slices.Delete(m.hist, 0, 1)
	}
}

// moved is the displacement over the last w seconds, or nil when history is shorter.
func (m *Memory) moved(t, w float64) *float64 {
	if len(m.hist) == 0 {
		return nil
	}
	first := m.hist[0]
	if t-first.f0 < w*0.9 {
		return nil
	}
	oi := -1
	for i := range m.hist {
		if t-m.hist[i].f0 <= w {
			oi = i
			break
		}
	}
	if oi < 0 {
		return nil
	}
	old := m.hist[oi]
	cur := m.hist[len(m.hist)-1]
	return ptr(dist(old.f1, cur.f1))
}
