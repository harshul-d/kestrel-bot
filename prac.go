package main

// Practice profile: against the house squad, play greedier for cubes and weapons, the way the
// top practice bots do (crates all match, wide loot radius, long trips for a laser).
// It is switched off the moment a free-for-all is detected, restoring the FFA-tested settings.

var (
	optPrac     = envBool("MECH_PRAC", false)
	optIdleLoot = envFloat("MECH_IDLE_LOOT", 90) // weapon/cube pickup radius when no enemy is in sight (m)
)

func setPracProfile(on bool) {
	if on {
		optCrateHunt = envFloat("MECH_PRAC_CRATE", 150)
		optLootR = envFloat("MECH_PRAC_LOOT_R", 70)
		optIdleLoot = envFloat("MECH_PRAC_IDLE_LOOT", 150)
		optRushRange = envFloat("MECH_PRAC_RUSH", 240)
		return
	}
	optCrateHunt, optLootR, optIdleLoot, optRushRange = 0, 35, 90, 140
}
