package pilot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"unicode/utf8"
)

type Params struct {
	aimzOn             float64
	aimzPmin           float64
	aimzShrink         float64
	aimzMinN           float64
	aimzShotMin        float64
	aimzGshot          float64
	wAntilead          float64
	wCad               float64
	cadMin             float64
	cadMax             float64
	cadPeriod          float64
	cadKeep            float64
	cadOff             float64
	cadWin             float64
	cadSnap            float64
	cadReg             float64
	antileadLat        float64
	antileadSigma      float64
	ffaOn              float64
	ffaAvoidRange      float64
	ffaFairAlive       float64
	ffaEndT            float64
	ffaDefendRange     float64
	ffaCrowdRange      float64
	ffaSureHp          float64
	ffaMinHp           float64
	ffaVultureHp       float64
	ffaEndBonus        float64
	ffaEarlyQuietRange float64
	// planner
	horizonSteps     float64
	nDirs            float64
	nBoostDirs       float64
	wProgressIdle    float64
	wProgressZone    float64
	wProgressRetreat float64
	wProgressLoot    float64
	wProgressDrop    float64
	wProgressFight   float64
	wNavdir          float64
	wRange           float64
	rangeBand        float64
	wProj            float64
	projMargin       float64
	wExpo            float64
	expoRetreat      float64
	expoZone         float64
	expoFightTarget  float64
	wFightLos        float64
	wCharge          float64
	wBlocked         float64
	wSmooth          float64
	wJuke            float64
	jukeMinS         float64
	jukeMaxS         float64
	jukeMinDist      float64
	wBoost           float64
	wStand           float64
	zoneCostMult     float64
	zoneCostZoneMode float64
	zoneInner        float64
	wNextZone        float64
	npcAvoid         float64
	wNpc             float64
	maxThreats       float64
	expoMinBudgetMs  float64
	fieldMinBudgetMs float64
	fieldRegoal      float64
	stuckWindowS     float64
	stuckDist        float64
	unstickS         float64
	// energy
	energyReserve          float64
	energyReserveLightning float64
	// zone
	zoneMargin  float64
	zoneExtra   float64
	zoneEdge    float64
	zoneLeadS   float64
	zoneSlackS  float64
	pathFactor  float64
	finalRadius float64
	idleFrac    float64
	// strategy
	retreatHp        float64
	retreatExitHp    float64
	retreatMultiHp   float64
	retreatPressHp   float64
	noLosDropS       float64
	finishRatio      float64
	coverPeriodS     float64
	fleeDist         float64
	engageRange      float64
	orderMode        float64
	pestDmg          float64
	orderMaxDist     float64
	orderLateS       float64
	orderPestClose   float64
	orderHostileS    float64
	orderMemoryS     float64
	orderSpare       float64
	orderAbortHp     float64
	orderSkipLaser   float64
	orderSkipCubes   float64
	haltMulti        float64
	targetMemoryS    float64
	targetSticky     float64
	chaseOutNext     float64
	rangeGun         float64
	rangeLaser       float64
	rangeLightning   float64
	rangeMortar      float64
	rangeVsLightning float64
	rangeNpc         float64
	rangeCrate       float64
	lightningRushHp  float64
	lootWeaponMax    float64
	lootSwapMax      float64
	lootCubeMax      float64
	lootQuick        float64
	lootSafeRange    float64
	huntMinHp        float64
	huntMaxDist      float64
	huntSafeRange    float64
	crateMinHp       float64
	crateMaxDist     float64
	pursuitMaxS      float64
	blacklistS       float64
	dropMaxDist      float64
	dropEarlyS       float64
	dropMinHpFar     float64
	prefGun          float64
	prefLaser        float64
	prefLightning    float64
	prefMortar       float64
	// fire control
	latency              float64
	laserLatency         float64
	accGain              float64
	accTime              float64
	aimTol               float64
	gunRangeFrac         float64
	lightningMinEnergy   float64
	lightningConeFrac    float64
	mortarMinRange       float64
	mortarMaxTargetSpeed float64
	mortarMaxFlight      float64
	reloadFrac           float64
	scanAmp              float64
	scanPeriod           float64
	// threat model
	gunDps             float64
	laserDps           float64
	lightningDps       float64
	mortarDps          float64
	ghostDps           float64
	ghostRange         float64
	freshS             float64
	sqOn               float64
	sqGateOn           float64
	sqAvoidOn          float64
	sqEtaRange         float64
	sqLosRange         float64
	sqShooterS         float64
	sqAimTol           float64
	sqVrAlpha          float64
	sqDmgMemoryS       float64
	sqHunterRange      float64
	sqHunterVr         float64
	sqHunterHoldS      float64
	sqTrackS           float64
	sqOurDps           float64
	sqIsoRange         float64
	sqLateT            float64
	sqAmbushHp         float64
	sqAmbushTgtHp      float64
	sqAmbushTtk        float64
	sqGapMargin        float64
	sqGateMaxHp        float64
	sqFinishRatio      float64
	sqAvoidRange       float64
	sqRegenOn          float64
	sqRegenHp          float64
	sqRegenFightS      float64
	sqRegenUntilT      float64
	upgradePriority    []string
	holdUntil          float64
	haltAfter          float64
	haltNl             float64
	haltWl             float64
	haltK              float64
	finishHp           float64
	finishBonus        float64
	finishChaseMult    float64
	rushSolo           float64
	rushAloneR         float64
	rushEnergyCap      float64
	rushEnergyFrac     float64
	precharge          float64
	prechargeMemoryS   float64
	gearOn             float64
	gearHarmlessDmg    float64
	gearLaserThreatR   float64
	gearLateS          float64
	gearSafeR          float64
	gearLaserMax       float64
	gearWeaponMax      float64
	gearFightKeep      float64
	gearCubeCap        float64
	gearCubeSafe       float64
	gearCubeR          float64
	gearCubeFightR     float64
	gearDrop           float64
	gearFarm           float64
	gearFarmFar        float64
	gearEasyRange      float64
	gearBoostE         float64
	gearSweepEvery     float64
	gearSweepMechR     float64
	gearSweepNolaser   float64
	gearSweepAnyR      float64
	gearJetScan        float64
	gearSiteR          float64
	gearGrabR          float64
	gearNpcFireR       float64
	gearPursuitS       float64
	gearNpcGoalR       float64
	gearNpcStand       float64
	gearGrabAlways     float64
	gearSiteS          float64
	gearJetStop        float64
	harvestOn          bool
	harvestSiteS       float64
	harvestSiteMax     float64
	harvestCalmMax     float64
	harvestFightMax    float64
	harvestCalmS       float64
	harvestNear        float64
	harvestVisitS      float64
	sfMortar           float64
	sfNofirst          float64
	sfNfClose          float64
	sfHuntS            float64
	sfHarm             float64
	sfHarmCubes        float64
	sfHarmDmg          float64
	sfSweep            float64
	sfSweepR           float64
	sfOpenR            float64
	sfLosR             float64
	sfMemS             float64
	esOn               float64
	esAttS             float64
	esUntil            float64
	esHp               float64
	esFreeD            float64
	esFreeCubes        float64
	esCone             float64
	restOn             bool
	restHp             float64
	restExitHp         float64
	restNear           float64
	readyKill          float64
	readyCubes         float64
	readyHp            float64
	readySecond        float64
	readyCover         float64
	readyDeadline      float64
	readySpareFrac     float64
	readySpareHp       float64
	readySafeRange     float64
	readyOnly          float64
	readyLaser         float64
	readySpareHarmless float64
	rgOn               float64
	rgHp               float64
	rgExit             float64
	rgQuietS           float64
	rgStrong           float64
	ktOn               float64
	ktKills            float64
	ktN                float64
	ktExitS            float64
	ktStep             float64
	dpOn               float64
	dpDist             float64
	dpLeaveHp          float64
	dpLeaveR           float64
	skOn               float64
	skDist             float64
	skMinHp            float64
	skMemS             float64
	skBoost            float64
	skDetour           float64
	tkOn               float64
	tkFocus            float64
	tkRange            float64
	tkKills            float64
	prepPos            float64
	prepFromS          float64
	svSpot             float64
	svMemoryS          float64
	svLaserMult        float64
	svPeriodS          float64
	svBudgetMs         float64
	svTargetExpo       float64
	svWCov             float64
	svWPath            float64
	svWHidden          float64
	svWLaser           float64
	svWShot            float64
	svWRange           float64
	svWEvade           float64
	svWTravel          float64
	svHyst             float64
	svWCover           float64
	svMinGain          float64
	svTorso            float64
	svHold             float64
	svHoldRange        float64
	svHoldLethal       float64
	svHoldHitS         float64
	roofMode           float64
	roofAfterKills     float64
	roofStartS         float64
	roofMinNodes       float64
	roofMinSetback     float64
	roofMinH           float64
	roofMaxH           float64
	roofZoneMargin     float64
	roofWDist          float64
	roofWExpo          float64
	roofRestHp         float64
	roofFireHp         float64
	roofLeaveHp        float64
	roofBanS           float64
	roofClimbR         float64
	roofMemoryS        float64
	roofEngage         float64
	roofLootSafe       float64
	roofLootFirst      float64
	roofTravelFight    float64
	roofTravelCommit   float64
	lzSeek             float64
	lzSeekRetreat      float64
	lzPeek             float64
	lzMemS             float64
	lzMinD             float64
	lzMaxD             float64
	lzMinHp            float64
	lzReplanS          float64
	lzR                float64
	lzCell             float64
	lzMs               float64
	lzStep             float64
	lzKeep             float64
	lzWMove            float64
	lzWStep            float64
	lzWSeen            float64
	lzChargeD          float64
	lzOutMax           float64
	lzGun              float64
	lzBoost            float64
	lzCl               float64
	lzClCone           float64
	lzClLeft           float64
	lzClR              float64
	lzHideOnly         float64
	lzAnyTgt           float64
	lzMargin           float64
	lzExpRush          float64
	lzSync             float64
	lzSeekFight        float64
	lzHotS             float64
	lzHotD             float64
	lzRetreat          float64
	lzScanS            float64
	lzScanEvery        float64
	lzScanLen          float64
	lzHoldLost         float64
	takeRangeBan       float64
	buBreak            float64
	buCone             float64
	buReachS           float64
	buR                float64
	buMs               float64
	buHoldS            float64
	buMinThr           float64
	buDodge            float64
	buDodgeEnergy      float64
	buRegenHp          float64
	buRegenSeenS       float64
	brawlOn            float64
	brawlBoostD        float64
	brawlBoostE        float64
}

func paramsDefault() Params {
	return Params{
		aimzOn:                 0.0,
		aimzPmin:               0.0,
		aimzShrink:             0.9,
		aimzMinN:               12.0,
		aimzShotMin:            0.0,
		aimzGshot:              0.0,
		wAntilead:              0.0,
		wCad:                   0.0,
		cadMin:                 45.0,
		cadMax:                 115.0,
		cadPeriod:              0.3,
		cadKeep:                1.0,
		cadOff:                 0.0,
		cadWin:                 0.045,
		cadSnap:                0.1,
		cadReg:                 0.0,
		antileadLat:            0.1,
		antileadSigma:          0.02,
		ffaOn:                  0.0,
		ffaAvoidRange:          200.0,
		ffaFairAlive:           4.00,
		ffaEndT:                360.00,
		ffaDefendRange:         90.00,
		ffaCrowdRange:          150.00,
		ffaSureHp:              0.30,
		ffaMinHp:               0.70,
		ffaVultureHp:           0.50,
		ffaEndBonus:            0.0,
		ffaEarlyQuietRange:     0.0,
		horizonSteps:           8.0,
		nDirs:                  16.0,
		nBoostDirs:             8.0,
		wProgressIdle:          2.0,
		wProgressZone:          12.0,
		wProgressRetreat:       6.0,
		wProgressLoot:          5.0,
		wProgressDrop:          6.0,
		wProgressFight:         4.0,
		wNavdir:                4.0,
		wRange:                 1.5,
		rangeBand:              8.0,
		wProj:                  1.2,
		projMargin:             1.5,
		wExpo:                  0.5,
		expoRetreat:            2.0,
		expoZone:               0.4,
		expoFightTarget:        0.2,
		wFightLos:              30.0,
		wCharge:                0.8,
		wBlocked:               12.0,
		wSmooth:                3.0,
		wJuke:                  22.0,
		jukeMinS:               0.35,
		jukeMaxS:               0.95,
		jukeMinDist:            30.0,
		wBoost:                 0.25,
		wStand:                 1.0,
		zoneCostMult:           1.5,
		zoneCostZoneMode:       0.15,
		zoneInner:              5.0,
		wNextZone:              0.3,
		npcAvoid:               52.0,
		wNpc:                   2.0,
		maxThreats:             4.0,
		expoMinBudgetMs:        6.0,
		fieldMinBudgetMs:       14.0,
		fieldRegoal:            4.0,
		stuckWindowS:           2.0,
		stuckDist:              1.5,
		unstickS:               1.2,
		energyReserve:          25.0,
		energyReserveLightning: 45.0,
		zoneMargin:             18.0,
		zoneExtra:              12.0,
		zoneEdge:               8.0,
		zoneLeadS:              9.0,
		zoneSlackS:             12.0,
		pathFactor:             1.35,
		finalRadius:            30.0,
		idleFrac:               0.5,
		retreatHp:              0.38,
		retreatExitHp:          0.8,
		retreatMultiHp:         0.65,
		retreatPressHp:         0.75,
		noLosDropS:             4.0,
		finishRatio:            0.6,
		coverPeriodS:           0.7,
		fleeDist:               55.0,
		engageRange:            170.0,
		orderMode:              0.0,
		pestDmg:                150.0,
		orderMaxDist:           200.0,
		orderLateS:             200.0,
		orderPestClose:         0.0,
		orderHostileS:          8.0,
		orderMemoryS:           8.0,
		orderSpare:             0.0,
		orderAbortHp:           0.5,
		orderSkipLaser:         1.0,
		orderSkipCubes:         1.0,
		haltMulti:              0.0,
		targetMemoryS:          4.0,
		targetSticky:           30.0,
		chaseOutNext:           60.0,
		rangeGun:               85.0,
		rangeLaser:             140.0,
		rangeLightning:         30.0,
		rangeMortar:            90.0,
		rangeVsLightning:       70.0,
		rangeNpc:               70.0,
		rangeCrate:             40.0,
		lightningRushHp:        0.5,
		lootWeaponMax:          220.0,
		lootSwapMax:            60.0,
		lootCubeMax:            110.0,
		lootQuick:              25.0,
		lootSafeRange:          120.0,
		huntMinHp:              0.6,
		huntMaxDist:            170.0,
		huntSafeRange:          150.0,
		crateMinHp:             0.85,
		crateMaxDist:           160.0,
		pursuitMaxS:            30.0,
		blacklistS:             60.0,
		dropMaxDist:            320.0,
		dropEarlyS:             12.0,
		dropMinHpFar:           0.45,
		prefGun:                10.0,
		prefLaser:              3.0,
		prefLightning:          3.0,
		prefMortar:             1.0,
		latency:                0.05,
		laserLatency:           0.05,
		accGain:                0.5,
		accTime:                0.35,
		aimTol:                 0.8,
		gunRangeFrac:           0.82,
		lightningMinEnergy:     8.0,
		lightningConeFrac:      0.85,
		mortarMinRange:         28.0,
		mortarMaxTargetSpeed:   13.0,
		mortarMaxFlight:        2.2,
		reloadFrac:             0.5,
		scanAmp:                1.1,
		scanPeriod:             3.2,
		gunDps:                 150.0,
		laserDps:               120.0,
		lightningDps:           200.0,
		mortarDps:              40.0,
		ghostDps:               100.0,
		ghostRange:             110.0,
		freshS:                 0.35,
		finishHp:               0.35,
		finishBonus:            0.0,
		finishChaseMult:        2.0,
		rushSolo:               0.0,
		rushAloneR:             70.0,
		rushEnergyCap:          72.0,
		rushEnergyFrac:         0.8,
		precharge:              0.0,
		prechargeMemoryS:       2.0,
		sqOn:                   0.0,
		sqGateOn:               1.0,
		sqAvoidOn:              0.0,
		sqEtaRange:             450.0,
		sqLosRange:             120.0,
		sqShooterS:             3.0,
		sqAimTol:               0.08,
		sqVrAlpha:              0.15,
		sqDmgMemoryS:           8.0,
		sqHunterRange:          320.0,
		sqHunterVr:             3.0,
		sqHunterHoldS:          20.0,
		sqTrackS:               8.0,
		sqOurDps:               110.0,
		sqIsoRange:             130.0,
		sqLateT:                250.0,
		sqAmbushHp:             0.75,
		sqAmbushTgtHp:          0.55,
		sqAmbushTtk:            7.0,
		sqGapMargin:            2.0,
		sqGateMaxHp:            0.8,
		sqFinishRatio:          0.6,
		sqAvoidRange:           150.0,
		sqRegenOn:              0.0,
		sqRegenHp:              0.85,
		sqRegenFightS:          2.5,
		sqRegenUntilT:          1000.0,
		upgradePriority:        []string{"plating", "armor", "nanites", "gun_damage", "gun_rate", "laser_damage", "laser_charge", "lightning_efficiency", "servos", "gyros", "gun_velocity", "gun_mag", "reactor", "capacitor", "jets", "lightning_chain", "lightning_range", "mortar_reload", "mortar_splash", "mortar_velocity", "sensors", "laser_pierce"},
		holdUntil:              -1.0,
		haltAfter:              -1.0,
		haltNl:                 -1.0,
		haltWl:                 -1.0,
		haltK:                  0.0,
		gearOn:                 0.0,
		gearHarmlessDmg:        150.0,
		gearLaserThreatR:       170.0,
		gearLateS:              1000.0,
		gearSafeR:              110.0,
		gearLaserMax:           260.0,
		gearWeaponMax:          140.0,
		gearFightKeep:          70.0,
		gearCubeCap:            10.0,
		gearCubeSafe:           60.0,
		gearCubeR:              90.0,
		gearCubeFightR:         35.0,
		gearDrop:               1.0,
		gearFarm:               1.0,
		gearFarmFar:            110.0,
		gearEasyRange:          75.0,
		gearBoostE:             60.0,
		gearSweepEvery:         15.0,
		gearSweepMechR:         150.0,
		gearSweepNolaser:       0.0,
		gearSweepAnyR:          50.0,
		gearJetScan:            0.0,
		gearSiteR:              0.0,
		gearGrabR:              0.0,
		gearNpcFireR:           0.0,
		gearPursuitS:           0.0,
		gearNpcGoalR:           0.0,
		gearNpcStand:           62.0,
		gearGrabAlways:         0.0,
		gearSiteS:              30.0,
		gearJetStop:            35.0,
		harvestOn:              false,
		harvestSiteS:           40.0,
		harvestSiteMax:         150.0,
		harvestCalmMax:         45.0,
		harvestFightMax:        18.0,
		harvestCalmS:           1.5,
		harvestNear:            45.0,
		harvestVisitS:          15.0,
		sfMortar:               0.0,
		sfNofirst:              0.0,
		sfNfClose:              55.0,
		sfHuntS:                6.0,
		sfHarm:                 0.0,
		sfHarmCubes:            3.0,
		sfHarmDmg:              150.0,
		sfSweep:                0.0,
		sfSweepR:               200.0,
		sfOpenR:                0.0,
		sfLosR:                 250.0,
		sfMemS:                 2.0,
		esOn:                   0.0,
		esAttS:                 10.0,
		esUntil:                1e9,
		esHp:                   0.0,
		esFreeD:                0.0,
		esFreeCubes:            0.0,
		esCone:                 0.08,
		restOn:                 false,
		restHp:                 0.75,
		restExitHp:             0.95,
		restNear:               70.0,
		readyKill:              0.0,
		readyCubes:             8.0,
		readyHp:                0.9,
		readySecond:            1.0,
		readyCover:             0.0,
		readyDeadline:          120.0,
		readySpareFrac:         0.3,
		readySpareHp:           200.0,
		readySafeRange:         45.0,
		readyOnly:              0.0,
		readyLaser:             0.0,
		readySpareHarmless:     0.0,
		rgOn:                   0.0,
		rgHp:                   0.5,
		rgExit:                 0.9,
		rgQuietS:               1.0,
		rgStrong:               1.0,
		ktOn:                   0.0,
		ktKills:                4.0,
		ktN:                    2.0,
		ktExitS:                1.0,
		ktStep:                 40.0,
		dpOn:                   0.0,
		dpDist:                 200.0,
		dpLeaveHp:              0.6,
		dpLeaveR:               150.0,
		skOn:                   0.0,
		skDist:                 300.0,
		skMinHp:                0.5,
		skMemS:                 30.0,
		skBoost:                1.0,
		skDetour:               40.0,
		tkOn:                   0.0,
		tkFocus:                120.0,
		tkRange:                80.0,
		tkKills:                4.0,
		prepPos:                0.0,
		prepFromS:              0.0,
		svSpot:                 0.0,
		svMemoryS:              4.0,
		svLaserMult:            1.5,
		svPeriodS:              0.3,
		svBudgetMs:             9.0,
		svTargetExpo:           0.35,
		svWCov:                 40.0,
		svWPath:                10.0,
		svWHidden:              10.0,
		svWLaser:               25.0,
		svWShot:                30.0,
		svWRange:               0.15,
		svWEvade:               40.0,
		svWTravel:              0.6,
		svHyst:                 4.0,
		svWCover:               8.0,
		svMinGain:              6.0,
		svTorso:                0.0,
		svHold:                 0.0,
		svHoldRange:            60.0,
		svHoldLethal:           250.0,
		svHoldHitS:             1.0,
		roofMode:               0.0,
		roofAfterKills:         0.0,
		roofStartS:             0.0,
		roofMinNodes:           120.0,
		roofMinSetback:         14.0,
		roofMinH:               10.0,
		roofMaxH:               36.0,
		roofZoneMargin:         12.0,
		roofWDist:              0.15,
		roofWExpo:              0.5,
		roofRestHp:             0.6,
		roofFireHp:             0.92,
		roofLeaveHp:            0.6,
		roofBanS:               45.0,
		roofClimbR:             70.0,
		roofMemoryS:            5.0,
		roofEngage:             220.0,
		roofLootSafe:           150.0,
		roofLootFirst:          30.0,
		roofTravelFight:        1.0,
		roofTravelCommit:       40.0,
		lzSeek:                 0.0,
		lzSeekRetreat:          0.0,
		lzPeek:                 0.0,
		lzMemS:                 4.5,
		lzMinD:                 30.0,
		lzMaxD:                 260.0,
		lzMinHp:                0.0,
		lzReplanS:              0.3,
		lzR:                    16.0,
		lzCell:                 1.0,
		lzMs:                   6.0,
		lzStep:                 6.0,
		lzKeep:                 3.0,
		lzWMove:                1.0,
		lzWStep:                3.0,
		lzWSeen:                25.0,
		lzChargeD:              3.0,
		lzOutMax:               1.2,
		lzGun:                  1.0,
		lzBoost:                1.0,
		lzCl:                   0.0,
		lzClCone:               0.15,
		lzClLeft:               0.7,
		lzClR:                  12.0,
		lzHideOnly:             0.0,
		lzAnyTgt:               0.0,
		lzMargin:               0.0,
		lzExpRush:              0.0,
		lzSync:                 0.0,
		lzSeekFight:            400.0,
		lzHotS:                 3.0,
		lzHotD:                 200.0,
		lzRetreat:              1.0,
		lzScanS:                0.0,
		lzScanEvery:            0.0,
		lzScanLen:              1.6,
		lzHoldLost:             0.0,
		takeRangeBan:           0.0,
		buBreak:                0.0,
		buCone:                 0.12,
		buReachS:               0.6,
		buR:                    14.0,
		buMs:                   9.0,
		buHoldS:                1.2,
		buMinThr:               1.0,
		buDodge:                0.0,
		buDodgeEnergy:          15.0,
		buRegenHp:              0.0,
		buRegenSeenS:           3.0,
		brawlOn:                0.0,
		brawlBoostD:            45.0,
		brawlBoostE:            0.35,
	}
}

// paramsFieldList lists every JSON key with the address of its field, in declaration order.
func paramsFieldList(p *Params) []struct {
	f0 string
	f1 any
} {
	return []struct {
		f0 string
		f1 any
	}{
		{"aimz_on", &p.aimzOn},
		{"aimz_pmin", &p.aimzPmin},
		{"aimz_shrink", &p.aimzShrink},
		{"aimz_min_n", &p.aimzMinN},
		{"aimz_shot_min", &p.aimzShotMin},
		{"aimz_gshot", &p.aimzGshot},
		{"w_antilead", &p.wAntilead},
		{"w_cad", &p.wCad},
		{"cad_min", &p.cadMin},
		{"cad_max", &p.cadMax},
		{"cad_period", &p.cadPeriod},
		{"cad_keep", &p.cadKeep},
		{"cad_off", &p.cadOff},
		{"cad_win", &p.cadWin},
		{"cad_snap", &p.cadSnap},
		{"cad_reg", &p.cadReg},
		{"antilead_lat", &p.antileadLat},
		{"antilead_sigma", &p.antileadSigma},
		{"ffa_on", &p.ffaOn},
		{"ffa_avoid_range", &p.ffaAvoidRange},
		{"ffa_fair_alive", &p.ffaFairAlive},
		{"ffa_end_t", &p.ffaEndT},
		{"ffa_defend_range", &p.ffaDefendRange},
		{"ffa_crowd_range", &p.ffaCrowdRange},
		{"ffa_sure_hp", &p.ffaSureHp},
		{"ffa_min_hp", &p.ffaMinHp},
		{"ffa_vulture_hp", &p.ffaVultureHp},
		{"ffa_end_bonus", &p.ffaEndBonus},
		{"ffa_early_quiet_range", &p.ffaEarlyQuietRange},
		{"horizon_steps", &p.horizonSteps},
		{"n_dirs", &p.nDirs},
		{"n_boost_dirs", &p.nBoostDirs},
		{"w_progress_idle", &p.wProgressIdle},
		{"w_progress_zone", &p.wProgressZone},
		{"w_progress_retreat", &p.wProgressRetreat},
		{"w_progress_loot", &p.wProgressLoot},
		{"w_progress_drop", &p.wProgressDrop},
		{"w_progress_fight", &p.wProgressFight},
		{"w_navdir", &p.wNavdir},
		{"w_range", &p.wRange},
		{"range_band", &p.rangeBand},
		{"w_proj", &p.wProj},
		{"proj_margin", &p.projMargin},
		{"w_expo", &p.wExpo},
		{"expo_retreat", &p.expoRetreat},
		{"expo_zone", &p.expoZone},
		{"expo_fight_target", &p.expoFightTarget},
		{"w_fight_los", &p.wFightLos},
		{"w_charge", &p.wCharge},
		{"w_blocked", &p.wBlocked},
		{"w_smooth", &p.wSmooth},
		{"w_juke", &p.wJuke},
		{"juke_min_s", &p.jukeMinS},
		{"juke_max_s", &p.jukeMaxS},
		{"juke_min_dist", &p.jukeMinDist},
		{"w_boost", &p.wBoost},
		{"w_stand", &p.wStand},
		{"zone_cost_mult", &p.zoneCostMult},
		{"zone_cost_zone_mode", &p.zoneCostZoneMode},
		{"zone_inner", &p.zoneInner},
		{"w_next_zone", &p.wNextZone},
		{"npc_avoid", &p.npcAvoid},
		{"w_npc", &p.wNpc},
		{"max_threats", &p.maxThreats},
		{"expo_min_budget_ms", &p.expoMinBudgetMs},
		{"field_min_budget_ms", &p.fieldMinBudgetMs},
		{"field_regoal", &p.fieldRegoal},
		{"stuck_window_s", &p.stuckWindowS},
		{"stuck_dist", &p.stuckDist},
		{"unstick_s", &p.unstickS},
		{"energy_reserve", &p.energyReserve},
		{"energy_reserve_lightning", &p.energyReserveLightning},
		{"zone_margin", &p.zoneMargin},
		{"zone_extra", &p.zoneExtra},
		{"zone_edge", &p.zoneEdge},
		{"zone_lead_s", &p.zoneLeadS},
		{"zone_slack_s", &p.zoneSlackS},
		{"path_factor", &p.pathFactor},
		{"final_radius", &p.finalRadius},
		{"idle_frac", &p.idleFrac},
		{"retreat_hp", &p.retreatHp},
		{"retreat_exit_hp", &p.retreatExitHp},
		{"retreat_multi_hp", &p.retreatMultiHp},
		{"retreat_press_hp", &p.retreatPressHp},
		{"no_los_drop_s", &p.noLosDropS},
		{"finish_ratio", &p.finishRatio},
		{"cover_period_s", &p.coverPeriodS},
		{"flee_dist", &p.fleeDist},
		{"engage_range", &p.engageRange},
		{"order_mode", &p.orderMode},
		{"pest_dmg", &p.pestDmg},
		{"order_max_dist", &p.orderMaxDist},
		{"order_late_s", &p.orderLateS},
		{"order_pest_close", &p.orderPestClose},
		{"order_hostile_s", &p.orderHostileS},
		{"order_memory_s", &p.orderMemoryS},
		{"order_spare", &p.orderSpare},
		{"order_abort_hp", &p.orderAbortHp},
		{"order_skip_laser", &p.orderSkipLaser},
		{"order_skip_cubes", &p.orderSkipCubes},
		{"halt_multi", &p.haltMulti},
		{"target_memory_s", &p.targetMemoryS},
		{"target_sticky", &p.targetSticky},
		{"chase_out_next", &p.chaseOutNext},
		{"range_gun", &p.rangeGun},
		{"range_laser", &p.rangeLaser},
		{"range_lightning", &p.rangeLightning},
		{"range_mortar", &p.rangeMortar},
		{"range_vs_lightning", &p.rangeVsLightning},
		{"range_npc", &p.rangeNpc},
		{"range_crate", &p.rangeCrate},
		{"lightning_rush_hp", &p.lightningRushHp},
		{"loot_weapon_max", &p.lootWeaponMax},
		{"loot_swap_max", &p.lootSwapMax},
		{"loot_cube_max", &p.lootCubeMax},
		{"loot_quick", &p.lootQuick},
		{"loot_safe_range", &p.lootSafeRange},
		{"hunt_min_hp", &p.huntMinHp},
		{"hunt_max_dist", &p.huntMaxDist},
		{"hunt_safe_range", &p.huntSafeRange},
		{"crate_min_hp", &p.crateMinHp},
		{"crate_max_dist", &p.crateMaxDist},
		{"pursuit_max_s", &p.pursuitMaxS},
		{"blacklist_s", &p.blacklistS},
		{"drop_max_dist", &p.dropMaxDist},
		{"drop_early_s", &p.dropEarlyS},
		{"drop_min_hp_far", &p.dropMinHpFar},
		{"pref_gun", &p.prefGun},
		{"pref_laser", &p.prefLaser},
		{"pref_lightning", &p.prefLightning},
		{"pref_mortar", &p.prefMortar},
		{"latency", &p.latency},
		{"laser_latency", &p.laserLatency},
		{"acc_gain", &p.accGain},
		{"acc_time", &p.accTime},
		{"aim_tol", &p.aimTol},
		{"gun_range_frac", &p.gunRangeFrac},
		{"lightning_min_energy", &p.lightningMinEnergy},
		{"lightning_cone_frac", &p.lightningConeFrac},
		{"mortar_min_range", &p.mortarMinRange},
		{"mortar_max_target_speed", &p.mortarMaxTargetSpeed},
		{"mortar_max_flight", &p.mortarMaxFlight},
		{"reload_frac", &p.reloadFrac},
		{"scan_amp", &p.scanAmp},
		{"scan_period", &p.scanPeriod},
		{"gun_dps", &p.gunDps},
		{"laser_dps", &p.laserDps},
		{"lightning_dps", &p.lightningDps},
		{"mortar_dps", &p.mortarDps},
		{"ghost_dps", &p.ghostDps},
		{"ghost_range", &p.ghostRange},
		{"fresh_s", &p.freshS},
		{"sq_on", &p.sqOn},
		{"sq_gate_on", &p.sqGateOn},
		{"sq_avoid_on", &p.sqAvoidOn},
		{"sq_eta_range", &p.sqEtaRange},
		{"sq_los_range", &p.sqLosRange},
		{"sq_shooter_s", &p.sqShooterS},
		{"sq_aim_tol", &p.sqAimTol},
		{"sq_vr_alpha", &p.sqVrAlpha},
		{"sq_dmg_memory_s", &p.sqDmgMemoryS},
		{"sq_hunter_range", &p.sqHunterRange},
		{"sq_hunter_vr", &p.sqHunterVr},
		{"sq_hunter_hold_s", &p.sqHunterHoldS},
		{"sq_track_s", &p.sqTrackS},
		{"sq_our_dps", &p.sqOurDps},
		{"sq_iso_range", &p.sqIsoRange},
		{"sq_late_t", &p.sqLateT},
		{"sq_ambush_hp", &p.sqAmbushHp},
		{"sq_ambush_tgt_hp", &p.sqAmbushTgtHp},
		{"sq_ambush_ttk", &p.sqAmbushTtk},
		{"sq_gap_margin", &p.sqGapMargin},
		{"sq_gate_max_hp", &p.sqGateMaxHp},
		{"sq_finish_ratio", &p.sqFinishRatio},
		{"sq_avoid_range", &p.sqAvoidRange},
		{"sq_regen_on", &p.sqRegenOn},
		{"sq_regen_hp", &p.sqRegenHp},
		{"sq_regen_fight_s", &p.sqRegenFightS},
		{"sq_regen_until_t", &p.sqRegenUntilT},
		{"upgrade_priority", &p.upgradePriority},
		{"hold_until", &p.holdUntil},
		{"halt_after", &p.haltAfter},
		{"halt_nl", &p.haltNl},
		{"halt_wl", &p.haltWl},
		{"halt_k", &p.haltK},
		{"finish_hp", &p.finishHp},
		{"finish_bonus", &p.finishBonus},
		{"finish_chase_mult", &p.finishChaseMult},
		{"rush_solo", &p.rushSolo},
		{"rush_alone_r", &p.rushAloneR},
		{"rush_energy_cap", &p.rushEnergyCap},
		{"rush_energy_frac", &p.rushEnergyFrac},
		{"precharge", &p.precharge},
		{"precharge_memory_s", &p.prechargeMemoryS},
		{"gear_on", &p.gearOn},
		{"gear_harmless_dmg", &p.gearHarmlessDmg},
		{"gear_laser_threat_r", &p.gearLaserThreatR},
		{"gear_late_s", &p.gearLateS},
		{"gear_safe_r", &p.gearSafeR},
		{"gear_laser_max", &p.gearLaserMax},
		{"gear_weapon_max", &p.gearWeaponMax},
		{"gear_fight_keep", &p.gearFightKeep},
		{"gear_cube_cap", &p.gearCubeCap},
		{"gear_cube_safe", &p.gearCubeSafe},
		{"gear_cube_r", &p.gearCubeR},
		{"gear_cube_fight_r", &p.gearCubeFightR},
		{"gear_drop", &p.gearDrop},
		{"gear_farm", &p.gearFarm},
		{"gear_farm_far", &p.gearFarmFar},
		{"gear_easy_range", &p.gearEasyRange},
		{"gear_boost_e", &p.gearBoostE},
		{"gear_sweep_every", &p.gearSweepEvery},
		{"gear_sweep_mech_r", &p.gearSweepMechR},
		{"gear_sweep_nolaser", &p.gearSweepNolaser},
		{"gear_sweep_any_r", &p.gearSweepAnyR},
		{"gear_jet_scan", &p.gearJetScan},
		{"gear_site_r", &p.gearSiteR},
		{"gear_grab_r", &p.gearGrabR},
		{"gear_npc_fire_r", &p.gearNpcFireR},
		{"gear_pursuit_s", &p.gearPursuitS},
		{"gear_npc_goal_r", &p.gearNpcGoalR},
		{"gear_npc_stand", &p.gearNpcStand},
		{"gear_grab_always", &p.gearGrabAlways},
		{"gear_site_s", &p.gearSiteS},
		{"gear_jet_stop", &p.gearJetStop},
		{"harvest_on", &p.harvestOn},
		{"harvest_site_s", &p.harvestSiteS},
		{"harvest_site_max", &p.harvestSiteMax},
		{"harvest_calm_max", &p.harvestCalmMax},
		{"harvest_fight_max", &p.harvestFightMax},
		{"harvest_calm_s", &p.harvestCalmS},
		{"harvest_near", &p.harvestNear},
		{"harvest_visit_s", &p.harvestVisitS},
		{"sf_mortar", &p.sfMortar},
		{"sf_nofirst", &p.sfNofirst},
		{"sf_nf_close", &p.sfNfClose},
		{"sf_hunt_s", &p.sfHuntS},
		{"sf_harm", &p.sfHarm},
		{"sf_harm_cubes", &p.sfHarmCubes},
		{"sf_harm_dmg", &p.sfHarmDmg},
		{"sf_sweep", &p.sfSweep},
		{"sf_sweep_r", &p.sfSweepR},
		{"sf_open_r", &p.sfOpenR},
		{"sf_los_r", &p.sfLosR},
		{"sf_mem_s", &p.sfMemS},
		{"es_on", &p.esOn},
		{"es_att_s", &p.esAttS},
		{"es_until", &p.esUntil},
		{"es_hp", &p.esHp},
		{"es_free_d", &p.esFreeD},
		{"es_free_cubes", &p.esFreeCubes},
		{"es_cone", &p.esCone},
		{"rest_on", &p.restOn},
		{"rest_hp", &p.restHp},
		{"rest_exit_hp", &p.restExitHp},
		{"rest_near", &p.restNear},
		{"ready_kill", &p.readyKill},
		{"ready_cubes", &p.readyCubes},
		{"ready_hp", &p.readyHp},
		{"ready_second", &p.readySecond},
		{"ready_cover", &p.readyCover},
		{"ready_deadline", &p.readyDeadline},
		{"ready_spare_frac", &p.readySpareFrac},
		{"ready_spare_hp", &p.readySpareHp},
		{"ready_safe_range", &p.readySafeRange},
		{"ready_only", &p.readyOnly},
		{"ready_laser", &p.readyLaser},
		{"ready_spare_harmless", &p.readySpareHarmless},
		{"rg_on", &p.rgOn},
		{"rg_hp", &p.rgHp},
		{"rg_exit", &p.rgExit},
		{"rg_quiet_s", &p.rgQuietS},
		{"rg_strong", &p.rgStrong},
		{"kt_on", &p.ktOn},
		{"kt_kills", &p.ktKills},
		{"kt_n", &p.ktN},
		{"kt_exit_s", &p.ktExitS},
		{"kt_step", &p.ktStep},
		{"dp_on", &p.dpOn},
		{"dp_dist", &p.dpDist},
		{"dp_leave_hp", &p.dpLeaveHp},
		{"dp_leave_r", &p.dpLeaveR},
		{"sk_on", &p.skOn},
		{"sk_dist", &p.skDist},
		{"sk_min_hp", &p.skMinHp},
		{"sk_mem_s", &p.skMemS},
		{"sk_boost", &p.skBoost},
		{"sk_detour", &p.skDetour},
		{"tk_on", &p.tkOn},
		{"tk_focus", &p.tkFocus},
		{"tk_range", &p.tkRange},
		{"tk_kills", &p.tkKills},
		{"prep_pos", &p.prepPos},
		{"sv_spot", &p.svSpot},
		{"sv_memory_s", &p.svMemoryS},
		{"sv_laser_mult", &p.svLaserMult},
		{"sv_period_s", &p.svPeriodS},
		{"sv_budget_ms", &p.svBudgetMs},
		{"sv_target_expo", &p.svTargetExpo},
		{"sv_w_cov", &p.svWCov},
		{"sv_w_path", &p.svWPath},
		{"sv_w_hidden", &p.svWHidden},
		{"sv_w_laser", &p.svWLaser},
		{"sv_w_shot", &p.svWShot},
		{"sv_w_range", &p.svWRange},
		{"sv_w_evade", &p.svWEvade},
		{"sv_w_travel", &p.svWTravel},
		{"sv_hyst", &p.svHyst},
		{"sv_w_cover", &p.svWCover},
		{"sv_min_gain", &p.svMinGain},
		{"sv_torso", &p.svTorso},
		{"sv_hold", &p.svHold},
		{"sv_hold_range", &p.svHoldRange},
		{"sv_hold_lethal", &p.svHoldLethal},
		{"sv_hold_hit_s", &p.svHoldHitS},
		{"roof_mode", &p.roofMode},
		{"roof_after_kills", &p.roofAfterKills},
		{"roof_start_s", &p.roofStartS},
		{"roof_min_nodes", &p.roofMinNodes},
		{"roof_min_setback", &p.roofMinSetback},
		{"roof_min_h", &p.roofMinH},
		{"roof_max_h", &p.roofMaxH},
		{"roof_zone_margin", &p.roofZoneMargin},
		{"roof_w_dist", &p.roofWDist},
		{"roof_w_expo", &p.roofWExpo},
		{"roof_rest_hp", &p.roofRestHp},
		{"roof_fire_hp", &p.roofFireHp},
		{"roof_leave_hp", &p.roofLeaveHp},
		{"roof_ban_s", &p.roofBanS},
		{"roof_climb_r", &p.roofClimbR},
		{"roof_memory_s", &p.roofMemoryS},
		{"roof_engage", &p.roofEngage},
		{"roof_loot_safe", &p.roofLootSafe},
		{"roof_loot_first", &p.roofLootFirst},
		{"roof_travel_fight", &p.roofTravelFight},
		{"roof_travel_commit", &p.roofTravelCommit},
		{"lz_seek", &p.lzSeek},
		{"lz_seek_retreat", &p.lzSeekRetreat},
		{"lz_peek", &p.lzPeek},
		{"lz_mem_s", &p.lzMemS},
		{"lz_min_d", &p.lzMinD},
		{"lz_max_d", &p.lzMaxD},
		{"lz_min_hp", &p.lzMinHp},
		{"lz_replan_s", &p.lzReplanS},
		{"lz_r", &p.lzR},
		{"lz_cell", &p.lzCell},
		{"lz_ms", &p.lzMs},
		{"lz_step", &p.lzStep},
		{"lz_keep", &p.lzKeep},
		{"lz_w_move", &p.lzWMove},
		{"lz_w_step", &p.lzWStep},
		{"lz_w_seen", &p.lzWSeen},
		{"lz_charge_d", &p.lzChargeD},
		{"lz_out_max", &p.lzOutMax},
		{"lz_gun", &p.lzGun},
		{"lz_boost", &p.lzBoost},
		{"lz_cl", &p.lzCl},
		{"lz_cl_cone", &p.lzClCone},
		{"lz_cl_left", &p.lzClLeft},
		{"lz_cl_r", &p.lzClR},
		{"lz_hide_only", &p.lzHideOnly},
		{"lz_any_tgt", &p.lzAnyTgt},
		{"lz_margin", &p.lzMargin},
		{"lz_exp_rush", &p.lzExpRush},
		{"lz_sync", &p.lzSync},
		{"lz_seek_fight", &p.lzSeekFight},
		{"lz_hot_s", &p.lzHotS},
		{"lz_hot_d", &p.lzHotD},
		{"lz_retreat", &p.lzRetreat},
		{"lz_scan_s", &p.lzScanS},
		{"lz_scan_every", &p.lzScanEvery},
		{"lz_scan_len", &p.lzScanLen},
		{"lz_hold_lost", &p.lzHoldLost},
		{"take_range_ban", &p.takeRangeBan},
		{"bu_break", &p.buBreak},
		{"bu_cone", &p.buCone},
		{"bu_reach_s", &p.buReachS},
		{"bu_r", &p.buR},
		{"bu_ms", &p.buMs},
		{"bu_hold_s", &p.buHoldS},
		{"bu_min_thr", &p.buMinThr},
		{"bu_dodge", &p.buDodge},
		{"bu_dodge_energy", &p.buDodgeEnergy},
		{"bu_regen_hp", &p.buRegenHp},
		{"bu_regen_seen_s", &p.buRegenSeenS},
		{"brawl_boost_e", &p.brawlBoostE},
		{"brawl_boost_d", &p.brawlBoostD},
		{"brawl_on", &p.brawlOn},
		{"prep_from_s", &p.prepFromS},
	}
}

// paramsLoad returns the defaults, overridden by the JSON file named in MECH_PARAMS when present.
// paramsShipped is the tuned parameter set the bot plays with (applied over the defaults).
var paramsShipped = []byte(paramsShippedJSON)

func paramsLoad() Params {
	base := paramsDefault()
	path, ok := os.LookupEnv("MECH_PARAMS")
	if !ok || !utf8.ValidString(path) {
		if p, err := paramsFromJSON(paramsShipped); err == nil {
			return p
		}
		return base
	}
	text, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(text) {
		return base
	}
	p, err := paramsFromJSON(text)
	if err != nil {
		logf("MECH_PARAMS ignored: %v", err)
		return base
	}
	return p
}

// paramsFromJSON decodes a params document. An object sets fields by key: missing keys keep
// their defaults, unknown keys are ignored, null or mistyped values and repeated known keys are
// errors. An array sets fields by position in declaration order (missing trailing ones keep
// their defaults).
func paramsFromJSON(data []byte) (Params, error) {
	p := paramsDefault()
	list := paramsFieldList(&p)
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return paramsDefault(), err
	}
	d, ok := tok.(json.Delim)
	if !ok || (d != '{' && d != '[') {
		return paramsDefault(), errors.New("invalid type: expected struct Params")
	}
	if d == '[' {
		i := 0
		for dec.More() {
			if i >= len(list) {
				return paramsDefault(), errors.New("trailing characters")
			}
			var v any
			if err := dec.Decode(&v); err != nil {
				return paramsDefault(), err
			}
			if err := paramsSetField(list[i].f1, list[i].f0, v); err != nil {
				return paramsDefault(), err
			}
			i++
		}
	} else {
		ptrs := make(map[string]any, len(list))
		for _, f := range list {
			ptrs[f.f0] = f.f1
		}
		seen := make(map[string]bool)
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return paramsDefault(), err
			}
			key, ok := kt.(string)
			if !ok {
				return paramsDefault(), errors.New("key must be a string")
			}
			var v any
			if err := dec.Decode(&v); err != nil {
				return paramsDefault(), err
			}
			ptr, ok := ptrs[key]
			if !ok {
				continue
			}
			if seen[key] {
				return paramsDefault(), fmt.Errorf("duplicate field `%s`", key)
			}
			seen[key] = true
			if err := paramsSetField(ptr, key, v); err != nil {
				return paramsDefault(), err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return paramsDefault(), err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("trailing characters")
		}
		return paramsDefault(), err
	}
	return p, nil
}

func paramsSetField(ptr any, key string, v any) error {
	switch f := ptr.(type) {
	case *float64:
		n, ok := v.(json.Number)
		if !ok {
			return fmt.Errorf("invalid type for `%s`, expected f64", key)
		}
		x, err := protoSerdeF64(string(n))
		if err != nil {
			return err
		}
		*f = x
	case *bool:
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("invalid type for `%s`, expected a boolean", key)
		}
		*f = b
	case *[]string:
		a, ok := v.([]any)
		if !ok {
			return fmt.Errorf("invalid type for `%s`, expected a sequence", key)
		}
		out := make([]string, 0, len(a))
		for _, e := range a {
			s, ok := e.(string)
			if !ok {
				return fmt.Errorf("invalid type in `%s`, expected a string", key)
			}
			out = append(out, s)
		}
		*f = out
	}
	return nil
}
