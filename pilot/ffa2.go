package pilot

// Free-for-all play. Every switch here is off in the practice settings and turned on only by
// the free-for-all overlay (ffa_overlay.go), so practice play is unchanged.

// ffaGunVsLaser: we hold no laser and the target does. A gun loses that duel at range, so
// close in to brawl range instead of trading shots.
func (b *Brain) ffaGunVsLaser(obs *Obs, tr *Track) bool {
	return b.p.ffaCloseLaser >= 0.5 && tr != nil && tr.has(WeaponKindLaser) && !brainHasKind(obs, WeaponKindLaser)
}

// ffaLaserBoost boosts toward a laser-carrying target while it is beyond brawl range.
func (b *Brain) ffaLaserBoost(obs *Obs) bool {
	p := b.p
	if p.ffaCloseLaser < 0.5 || b.mode != ModeFight || b.target.kind != TgtKindMech {
		return false
	}
	me := &obs.Me
	if !me.Grounded || me.Energy < me.MaxEnergy*0.35 {
		return false
	}
	t := obs.T
	for i := range b.mem.tracks {
		tr := &b.mem.tracks[i]
		if tr.kind != ActorKindMech || tr.id != b.target.mech0 {
			continue
		}
		if t-tr.lastSeen > 2.0 || !b.ffaGunVsLaser(obs, tr) {
			return false
		}
		d := dist(tr.posAt(t), me.Pos)
		return d > p.ffaCloseRange+15.0 && d < 260.0
	}
	return false
}

// ffaOppBonus: target score bonus for a mech that is already hurt or was just hit by someone else.
func (b *Brain) ffaOppBonus(tr *Track, t float64) float64 {
	p := b.p
	if p.ffaOppBonus <= 0.0 {
		return 0.0
	}
	s := 0.0
	if t-tr.lastSeen < 1.0 && frac(tr.hp, tr.maxHp) < p.ffaOppHp {
		s += p.ffaOppBonus
	}
	if t-b.ffa.foreignT(tr.id) < 4.0 {
		s += p.ffaOppBonus
	}
	return s
}

// ffaBanking: late in a free-for-all, keep HP for the final collapse.
func (b *Brain) ffaBanking(t float64) bool {
	return b.p.ffaBankS > 0.0 && t >= b.p.ffaBankS
}

// ffaBankHold: while banking, do not start fights: hold fire on a mech unless it attacked us
// recently or is nearly dead.
func (b *Brain) ffaBankHold(obs *Obs, tr *Track) bool {
	t := obs.T
	if !b.ffaBanking(t) || tr.kind != ActorKindMech {
		return false
	}
	if t-tr.lastDmgT < 6.0 {
		return false
	}
	return frac(tr.hp, tr.maxHp) >= b.p.ffaBankHp
}
