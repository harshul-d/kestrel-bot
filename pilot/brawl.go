package pilot

import "math"

// Brawl: with only a gun (no laser), win duels up close. Gun bullets need lead and spread
// grows with range, while a laser hits for full damage at any range, so the gun's edge is
// inside ~35 m. In a fight with a gun as the main weapon, boost to close the gap instead of
// trading shots across open ground.
func (b *Brain) brawlBoost(obs *Obs) bool {
	p := b.p
	if p.brawlOn < 0.5 || b.mode != ModeFight || b.target.kind != TgtKindMech {
		return false
	}
	me := &obs.Me
	if !me.Grounded || me.Energy < me.MaxEnergy*p.brawlBoostE || brainHasKind(obs, WeaponKindLaser) {
		return false
	}
	t := obs.T
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || tr.id != b.target.mech0 {
			continue
		}
		if t-tr.lastSeen > 2.0 || tr.has(WeaponKindLightning) {
			return false
		}
		d := dist(tr.posAt(t), me.Pos)
		return d > p.brawlBoostD && d < 260.0 && !math.IsNaN(d)
	}
	return false
}
