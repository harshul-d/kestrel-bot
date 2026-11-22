package pilot

import (
	"math"
	"os"
)

type FfaFfa struct {
	on     bool
	onT    float64
	why    uint8
	alive0 int64
	kills  int64
	sig    uint32
	third  uint32 // beams by other mechs aimed away from us and not at a drone or crate
	offSnd uint32 // observations with mid/far combat sounds from no mech we track
	seenHp []struct {
		f0 int64
		f1 float64
		f2 float64
	}
	ourHits []struct {
		f0 int64
		f1 float64
	}
	foreign []struct {
		f0 int64
		f1 float64
	}
	beamLast []struct {
		f0 int64
		f1 float64
	}
	seed int64
}

func ffaFfaNew(seed int64) FfaFfa {
	return FfaFfa{seed: seed}
}

func (f *FfaFfa) foreignT(id int64) float64 {
	for _, x := range f.foreign {
		if x.f0 == id {
			return x.f1
		}
	}
	return -1e9
}

func (f *FfaFfa) mark(id int64, t float64) {
	for i := range f.foreign {
		if f.foreign[i].f0 == id {
			f.foreign[i].f1 = t
			return
		}
	}
	f.foreign = append(f.foreign, struct {
		f0 int64
		f1 float64
	}{id, t})
}

func (f *FfaFfa) set(t float64, why uint8) {
	if !f.on {
		f.on = true
		f.onT = t
		f.why = why
		if _, ok := os.LookupEnv("FFA_LOG"); ok {
			logf("FFA seed=%d t=%.1f why=%d\n", f.seed, t, why)
		}
	}
}

func (f *FfaFfa) update(obs *Obs, tracks []Track, enabled bool, twoSig bool) {
	t := obs.T
	me := &obs.Me
	if f.alive0 <= 0 {
		f.alive0 = max(obs.Alive, 1)
	}
	for i := range obs.Events {
		ev := &obs.Events[i]
		switch ev.kind {
		case EventKindKill:
			f.kills += 1
		case EventKindHit:
			h := &ev.hit0
			found := false
			for j := range f.ourHits {
				if f.ourHits[j].f0 == h.Target {
					f.ourHits[j].f1 = t
					found = true
					break
				}
			}
			if !found {
				f.ourHits = append(f.ourHits, struct {
					f0 int64
					f1 float64
				}{h.Target, t})
			}
		}
	}
	if len(f.ourHits) > 64 {
		f.ourHits = append(f.ourHits[:0], f.ourHits[1:]...)
	}
	if !enabled {
		return
	}
	deaths := f.alive0 - obs.Alive
	if !twoSig && t < 75.0 && deaths >= f.kills+2 {
		f.set(t, 1)
	}
	isNpc := func(id int64) bool {
		for i := range tracks {
			if tracks[i].id == id && tracks[i].kind == ActorKindNpc {
				return true
			}
		}
		for i := range obs.Mechs {
			if obs.Mechs[i].Id == id && obs.Mechs[i].Kind == ActorKindNpc {
				return true
			}
		}
		return false
	}
	for bi := range obs.Beams {
		bm := &obs.Beams[bi]
		if bm.By == me.Id || isNpc(bm.By) {
			continue
		}
		if len(bm.Path) == 0 {
			continue
		}
		end := bm.Path[len(bm.Path)-1]
		if !(isFinite(end[0]) && isFinite(end[1]) && isFinite(end[2])) || dist(end, me.Pos) < 9.0 {
			continue
		}
		var hit3 *ActorObs
		for mi := range obs.Mechs {
			m := &obs.Mechs[mi]
			if m.Kind == ActorKindMech && m.Id != bm.By && m.Id != me.Id && dist(up(m.Pos, 4.5), end) < 5.5 {
				hit3 = m
				break
			}
		}
		if hit3 != nil {
			m := hit3
			last := -1e9
			for _, x := range f.beamLast {
				if x.f0 == bm.By {
					last = x.f1
					break
				}
			}
			if t-last > 1.0 {
				kept := f.beamLast[:0]
				for _, x := range f.beamLast {
					if x.f0 != bm.By {
						kept = append(kept, x)
					}
				}
				f.beamLast = kept
				f.beamLast = append(f.beamLast, struct {
					f0 int64
					f1 float64
				}{bm.By, t})
				f.sig += 1
			}
			id := m.Id
			f.mark(id, t)
		}
	}
	var npcs []V3
	for i := range obs.Mechs {
		if obs.Mechs[i].Kind == ActorKindNpc {
			npcs = append(npcs, obs.Mechs[i].Pos)
		}
	}
	zc := obs.Zone.Center
	zr := obs.Zone.Radius
	for mi := range obs.Mechs {
		m := &obs.Mechs[mi]
		if m.Kind != ActorKindMech || m.Id == me.Id {
			continue
		}
		if m.Hp == nil {
			continue
		}
		hp := *m.Hp
		prevOk := false
		var php, pt float64
		for _, x := range f.seenHp {
			if x.f0 == m.Id {
				prevOk = true
				php, pt = x.f1, x.f2
				break
			}
		}
		if prevOk {
			drop := php - hp
			ours := false
			for _, x := range f.ourHits {
				if x.f0 == m.Id && t-x.f1 < 1.2 {
					ours = true
					break
				}
			}
			inZone := dist2v(m.Pos, zc) < zr-2.0
			npcNear := false
			for _, q := range npcs {
				if dist(q, m.Pos) < 150.0 {
					npcNear = true
					break
				}
			}
			if t-pt < 0.6 && drop >= 45.0 && !ours && inZone && !npcNear {
				f.sig += 1
				f.mark(m.Id, t)
			}
		}
		found := false
		for j := range f.seenHp {
			if f.seenHp[j].f0 == m.Id {
				f.seenHp[j].f1 = hp
				f.seenHp[j].f2 = t
				found = true
				break
			}
		}
		if !found {
			f.seenHp = append(f.seenHp, struct {
				f0 int64
				f1 float64
				f2 float64
			}{m.Id, hp, t})
		}
	}
	if len(f.seenHp) > 32 {
		f.seenHp = append(f.seenHp[:0], f.seenHp[1:]...)
	}
	if twoSig {
		for bi := range obs.Beams {
			bm := &obs.Beams[bi]
			if bm.By == me.Id || isNpc(bm.By) || len(bm.Path) < 2 {
				continue
			}
			s0, e := bm.Path[0], bm.Path[len(bm.Path)-1]
			if !(memoryAllFinite3(s0) && memoryAllFinite3(e)) || dist(e, me.Pos) < 30.0 {
				continue
			}
			toMe := unit2(me.Pos[0]-s0[0], me.Pos[1]-s0[1])
			toEnd := unit2(e[0]-s0[0], e[1]-s0[1])
			if toMe[0]*toEnd[0]+toMe[1]*toEnd[1] > math.Cos(0.25) {
				continue // a shot at us that stopped short (wall, range)
			}
			atLoot := false
			for i := range tracks {
				if tracks[i].kind == ActorKindNpc && dist(tracks[i].pos, e) < 12.0 {
					atLoot = true
					break
				}
			}
			for i := range obs.Mechs {
				if obs.Mechs[i].Kind == ActorKindNpc && dist(obs.Mechs[i].Pos, e) < 12.0 {
					atLoot = true
					break
				}
			}
			for i := range obs.Crates {
				if dist(obs.Crates[i].Pos, e) < 12.0 {
					atLoot = true
					break
				}
			}
			if !atLoot {
				f.third++
			}
		}
		for si := range obs.Sounds {
			s := &obs.Sounds[si]
			if s.Dist == SoundDistNear || !(s.Kind == "gun" || s.Kind == "explosion" || s.Kind == "laser" || s.Kind == "mortar") {
				continue
			}
			near := false
			for i := range tracks {
				tr := &tracks[i]
				if tr.kind != ActorKindMech || t-tr.lastSeen > 3.0 {
					continue
				}
				q := tr.posAt(t)
				by := math.Atan2(q[1]-me.Pos[1], q[0]-me.Pos[0])
				if math.Abs(wrap(s.Bearing-by)) < 0.6 {
					near = true
					break
				}
			}
			if !near {
				f.offSnd++
				break
			}
		}
		// Two independent signals, early in the match: at least two mechs died that we did not
		// kill, and fighting we are not part of (other mechs seen taking damage that was not
		// ours, a beam fired away from us at no drone or crate, or sustained combat sounds from
		// where no tracked mech is). House bots in practice attack only us.
		if t < 150.0 && deaths >= f.kills+2 && (f.sig >= 2 || f.third >= 1 || f.offSnd >= 10) {
			f.set(t, 3)
		}
	} else if f.sig >= 3 {
		f.set(t, 2)
	}
}

func (b *Brain) ffaPassive(obs *Obs) bool {
	return b.ffa.on && b.p.ffaOn > 0.5 && float64(obs.Alive) > b.p.ffaFairAlive && obs.T < b.p.ffaEndT
}

func (b *Brain) ffaEndRival(obs *Obs, tr *Track) bool {
	if b.p.ffaEndBonus <= 0.0 || obs.T < b.p.ffaEndT || obs.Zone.NextRadius > 30.0 {
		return false
	}
	c := obs.Zone.NextCenter
	mine := dist2v(obs.Me.Pos, c) - obs.Me.Hp/260.0
	theirs := dist2v(tr.posAt(obs.T), c) - tr.hp/260.0
	return theirs < mine
}

func (b *Brain) ffaAllow(obs *Obs, tr *Track, attacker bool, d float64) bool {
	if !b.ffaPassive(obs) {
		return true
	}
	t := obs.T
	p := b.p
	hpf := frac(obs.Me.Hp, obs.Me.MaxHp)
	if attacker && (d < p.ffaDefendRange || hpf > 0.5) {
		return true
	}
	others := false
	for i := range b.mem.tracks {
		o := &b.mem.tracks[i]
		if o.kind == ActorKindMech && o.id != tr.id && t-o.lastSeen < 4.0 && dist(o.posAt(t), obs.Me.Pos) < p.ffaCrowdRange {
			others = true
			break
		}
	}
	tf := frac(tr.hp, tr.maxHp)
	if tf < p.ffaSureHp && hpf > p.ffaMinHp && !others && d < 150.0 {
		return true
	}
	if t-b.ffa.foreignT(tr.id) < 3.0 && tf < p.ffaVultureHp && hpf > p.ffaMinHp-0.1 && d < 140.0 {
		return true
	}
	return false
}

func (b *Brain) ffaFireOk(obs *Obs, tr *Track, attacker bool, d float64, clear_ bool) bool {
	if attacker || b.target == (Tgt{kind: TgtKindMech, mech0: tr.id}) || d < 60.0 {
		return true
	}
	if !b.ffa.on {
		return b.p.ffaEarlyQuietRange <= 0.0 || obs.T >= 75.0 || d < b.p.ffaEarlyQuietRange
	}
	if !b.ffaPassive(obs) {
		return true
	}
	toUs := math.Atan2(obs.Me.Pos[1]-tr.pos[1], obs.Me.Pos[0]-tr.pos[0])
	return clear_ && d < 110.0 && math.Abs(adiff(tr.aim[0], toUs)) < 0.4
}

func (b *Brain) ffaAvoid(obs *Obs) bool {
	if !b.ffaPassive(obs) || b.p.ffaAvoidRange <= 0.0 {
		return false
	}
	t := obs.T
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 1.0 && dist(tr.posAt(t), obs.Me.Pos) < b.p.ffaAvoidRange {
			return true
		}
	}
	return false
}

func (b *Brain) ffaQuiet(obs *Obs) float64 {
	if !b.ffaPassive(obs) || !obs.Me.InZone || obs.T-b.mem.lastDamageT < 4.0 {
		return 1.0
	}
	t := obs.T
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind == ActorKindMech && t-tr.lastSeen < 1.0 {
			return 1.0
		}
	}
	heard := false
	for i := range b.mem.sounds {
		s := &b.mem.sounds[i]
		if t-s.t < 4.0 && s.kind == "step" {
			heard = true
			break
		}
	}
	if !heard {
		return 1.0
	}
	walk := fmax(obs.Me.Stats.WalkSpeed, 1.0)
	return fclamp(b.cfg.Sound.StepSpeed*0.92/walk, 0.2, 1.0)
}
