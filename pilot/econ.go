package pilot

type EconEcon struct {
	sites []struct {
		f0 V3
		f1 float64
	}
	resting bool
}

func (b *Brain) econUpdate(obs *Obs) {
	t := obs.T
	for i := range obs.Events {
		ev := &obs.Events[i]
		if ev.kind == EventKindKill {
			k := &ev.kill0
			for j := range b.mem.tracks {
				tr := &b.mem.tracks[j]
				if tr.kind == ActorKindMech && tr.id == k.Target {
					b.econ.sites = append(b.econ.sites, struct {
						f0 V3
						f1 float64
					}{tr.pos, t})
					break
				}
			}
		}
	}
	keep := b.p.harvestSiteS
	me := obs.Me.Pos
	kept := b.econ.sites[:0]
	for _, s := range b.econ.sites {
		if t-s.f1 < keep || t-s.f1 > 1e8 && t-(s.f1+1e9) < keep {
			kept = append(kept, s)
		}
	}
	b.econ.sites = kept
	for i := range b.econ.sites {
		s := &b.econ.sites[i]
		if dist2(s.f0, me) < 5.0 && t-s.f1 < 1e8 {
			s.f1 -= 1e9
		}
	}
}

func (b *Brain) cubeWanted(cubes float64) bool {
	cap_ := b.cfg.Cubes.StatCap
	return cubes < cap_ || (cubes >= 13.0 && cubes < 15.0)
}

func (b *Brain) harvest(obs *Obs, zi *StrategyZoneInfo, nAtt int, nearMech float64) *struct {
	f0 int64
	f1 V3
} {
	p := b.p
	if !p.harvestOn || !b.cubeWanted(obs.Me.Cubes) {
		return nil
	}
	t := obs.T
	me := &obs.Me
	calm := nAtt == 0 && t-b.mem.lastDamageT > p.harvestCalmS
	var best *struct {
		f0 float64
		f1 int64
		f2 V3
	}
	for i := range b.mem.cubes {
		it := &b.mem.cubes[i]
		if b.mem.blacklisted(it.id, t) {
			continue
		}
		d := dist(it.pos, me.Pos)
		site := false
		for _, s := range b.econ.sites {
			if dist2(s.f0, it.pos) < 18.0 {
				site = true
				break
			}
		}
		var lim float64
		if site && calm && nearMech > p.harvestNear {
			lim = p.harvestSiteMax
		} else if calm {
			lim = p.harvestCalmMax
		} else {
			lim = p.harvestFightMax
		}
		if d > lim || !b.inPlayPub(obs, zi, it.pos, d) {
			continue
		}
		bonus := 0.0
		if it.id == b.goalId {
			bonus = 10.0
		}
		s := d - bonus
		if best == nil || s < best.f0 {
			best = &struct {
				f0 float64
				f1 int64
				f2 V3
			}{s, it.id, it.pos}
		}
	}
	if best == nil && calm && nearMech > p.harvestNear {
		for _, s := range b.econ.sites {
			d := dist(s.f0, me.Pos)
			if t-s.f1 < p.harvestVisitS && d < p.harvestSiteMax && d > 5.0 && b.inPlayPub(obs, zi, s.f0, d) {
				if best == nil || d < best.f0 {
					best = &struct {
						f0 float64
						f1 int64
						f2 V3
					}{d, -1, s.f0}
				}
			}
		}
	}
	if best == nil {
		return nil
	}
	return &struct {
		f0 int64
		f1 V3
	}{best.f1, best.f2}
}

func (b *Brain) wantRest(obs *Obs, zi *StrategyZoneInfo, nAtt int, nearMech float64, hpf float64) bool {
	p := b.p
	t := obs.T
	ok := p.restOn && !zi.urgent && nAtt == 0 && t-b.mem.lastDamageT > 1.0 && nearMech > p.restNear
	var r bool
	if b.econ.resting {
		r = ok && hpf < p.restExitHp
	} else {
		r = ok && hpf < p.restHp
	}
	b.econ.resting = r
	return r
}
