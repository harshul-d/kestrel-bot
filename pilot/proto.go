package pilot

// Protocol message types.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
)

// ---------------------------------------------------------------- enums

type WeaponKind int

const (
	WeaponKindGun WeaponKind = iota
	WeaponKindMortar
	WeaponKindLaser
	WeaponKindLightning
	WeaponKindUnknown
)

func (k WeaponKind) asStr() string {
	switch k {
	case WeaponKindGun:
		return "gun"
	case WeaponKindMortar:
		return "mortar"
	case WeaponKindLaser:
		return "laser"
	case WeaponKindLightning:
		return "lightning"
	}
	return "unknown"
}

func (k WeaponKind) MarshalText() ([]byte, error) { return []byte(k.asStr()), nil }

func (k *WeaponKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "gun":
		*k = WeaponKindGun
	case "mortar":
		*k = WeaponKindMortar
	case "laser":
		*k = WeaponKindLaser
	case "lightning":
		*k = WeaponKindLightning
	default:
		*k = WeaponKindUnknown
	}
	return nil
}

type ActorKind int

const (
	ActorKindMech ActorKind = iota
	ActorKindNpc
	ActorKindUnknown
)

func (k ActorKind) MarshalText() ([]byte, error) {
	switch k {
	case ActorKindMech:
		return []byte("mech"), nil
	case ActorKindNpc:
		return []byte("npc"), nil
	}
	return []byte("unknown"), nil
}

func (k *ActorKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "mech":
		*k = ActorKindMech
	case "npc":
		*k = ActorKindNpc
	default:
		*k = ActorKindUnknown
	}
	return nil
}

type ProjKind int

const (
	ProjKindGun ProjKind = iota
	ProjKindMortar
	ProjKindNpc
	ProjKindUnknown
)

func (k ProjKind) MarshalText() ([]byte, error) {
	switch k {
	case ProjKindGun:
		return []byte("gun"), nil
	case ProjKindMortar:
		return []byte("mortar"), nil
	case ProjKindNpc:
		return []byte("npc"), nil
	}
	return []byte("unknown"), nil
}

func (k *ProjKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "gun":
		*k = ProjKindGun
	case "mortar":
		*k = ProjKindMortar
	case "npc":
		*k = ProjKindNpc
	default:
		*k = ProjKindUnknown
	}
	return nil
}

type BeamKind int

const (
	BeamKindLaser BeamKind = iota
	BeamKindLightning
	BeamKindUnknown
)

func (k BeamKind) MarshalText() ([]byte, error) {
	switch k {
	case BeamKindLaser:
		return []byte("laser"), nil
	case BeamKindLightning:
		return []byte("lightning"), nil
	}
	return []byte("unknown"), nil
}

func (k *BeamKind) UnmarshalText(b []byte) error {
	switch string(b) {
	case "laser":
		*k = BeamKindLaser
	case "lightning":
		*k = BeamKindLightning
	default:
		*k = BeamKindUnknown
	}
	return nil
}

type SoundDist int

const (
	SoundDistNear SoundDist = iota
	SoundDistMid
	SoundDistFar
	SoundDistUnknown
)

func (k SoundDist) MarshalText() ([]byte, error) {
	switch k {
	case SoundDistNear:
		return []byte("near"), nil
	case SoundDistMid:
		return []byte("mid"), nil
	case SoundDistFar:
		return []byte("far"), nil
	}
	return []byte("unknown"), nil
}

func (k *SoundDist) UnmarshalText(b []byte) error {
	switch string(b) {
	case "near":
		*k = SoundDistNear
	case "mid":
		*k = SoundDistMid
	case "far":
		*k = SoundDistFar
	default:
		*k = SoundDistUnknown
	}
	return nil
}

type EffectName int

const (
	EffectNameCookoff EffectName = iota
	EffectNameShort
	EffectNameConcuss
	EffectNameUnknown
)

func (k EffectName) MarshalText() ([]byte, error) {
	switch k {
	case EffectNameCookoff:
		return []byte("cookoff"), nil
	case EffectNameShort:
		return []byte("short"), nil
	case EffectNameConcuss:
		return []byte("concuss"), nil
	}
	return []byte("unknown"), nil
}

func (k *EffectName) UnmarshalText(b []byte) error {
	switch string(b) {
	case "cookoff":
		*k = EffectNameCookoff
	case "short":
		*k = EffectNameShort
	case "concuss":
		*k = EffectNameConcuss
	default:
		*k = EffectNameUnknown
	}
	return nil
}

type TakeFailReason int

const (
	TakeFailReasonGone TakeFailReason = iota
	TakeFailReasonRange
	TakeFailReasonHave
	TakeFailReasonFull
	TakeFailReasonBusy
	TakeFailReasonUnknown
)

func (k TakeFailReason) MarshalText() ([]byte, error) {
	switch k {
	case TakeFailReasonGone:
		return []byte("gone"), nil
	case TakeFailReasonRange:
		return []byte("range"), nil
	case TakeFailReasonHave:
		return []byte("have"), nil
	case TakeFailReasonFull:
		return []byte("full"), nil
	case TakeFailReasonBusy:
		return []byte("busy"), nil
	}
	return []byte("unknown"), nil
}

func (k *TakeFailReason) UnmarshalText(b []byte) error {
	switch string(b) {
	case "gone":
		*k = TakeFailReasonGone
	case "range":
		*k = TakeFailReasonRange
	case "have":
		*k = TakeFailReasonHave
	case "full":
		*k = TakeFailReasonFull
	case "busy":
		*k = TakeFailReasonBusy
	default:
		*k = TakeFailReasonUnknown
	}
	return nil
}

// ---------------------------------------------------------------- hello / config

type Hello struct {
	Protocol string  `json:"protocol"`
	Slot     int64   `json:"slot"`
	Seed     int64   `json:"seed"`
	Map      MapInfo `json:"map"`
	Config   Config  `json:"config"`
}

func helloDefault() Hello {
	return Hello{Protocol: "mechfight/1", Slot: 0, Seed: 0, Map: mapInfoDefault(), Config: configDefault()}
}

func (x *Hello) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Hello
	a := struct {
		raw
		Slot json.RawMessage `json:"slot"`
		Seed json.RawMessage `json:"seed"`
	}{raw: raw(helloDefault())}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Slot, err = protoDeI64(a.Slot, a.raw.Slot); err != nil {
		return err
	}
	if a.raw.Seed, err = protoDeI64(a.Seed, a.raw.Seed); err != nil {
		return err
	}
	*x = Hello(a.raw)
	return nil
}

type MapInfo struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

func mapInfoDefault() MapInfo {
	return MapInfo{Dir: "/arena", Files: []string{"v2/world.json", "v2/terrain.i16"}}
}

func (x *MapInfo) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw MapInfo
	r := raw(mapInfoDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = MapInfo(r)
	return nil
}

type Config struct {
	Match      MatchCfg      `json:"match"`
	Bot        BotCfg        `json:"bot"`
	Mech       MechCfg       `json:"mech"`
	Vision     VisionCfg     `json:"vision"`
	Sound      SoundCfg      `json:"sound"`
	Weapons    WeaponsCfg    `json:"weapons"`
	Projectile ProjectileCfg `json:"projectile"`
	Cubes      CubesCfg      `json:"cubes"`
	Crates     CratesCfg     `json:"crates"`
	Loot       LootCfg       `json:"loot"`
	Npc        NpcCfg        `json:"npc"`
	Drops      *DropsCfg     `json:"drops,omitempty"`
	Zone       ZoneCfg       `json:"zone"`
	Replay     ReplayCfg     `json:"replay"`
	Debug      DebugCaps     `json:"debug"`
	Upgrades   []UpgradeDef  `json:"upgrades"`
}

// configDefault is Config::default() (every section at its own default).
func configDefault() Config {
	return Config{
		Match:      matchCfgDefault(),
		Bot:        botCfgDefault(),
		Mech:       mechCfgDefault(),
		Vision:     visionCfgDefault(),
		Sound:      soundCfgDefault(),
		Weapons:    weaponsCfgDefault(),
		Projectile: projectileCfgDefault(),
		Cubes:      cubesCfgDefault(),
		Crates:     cratesCfgDefault(),
		Loot:       lootCfgDefault(),
		Npc:        npcCfgDefault(),
		Drops:      nil,
		Zone:       zoneCfgDefault(),
		Replay:     replayCfgDefault(),
		Debug:      debugCapsDefault(),
		Upgrades:   nil,
	}
}

func (x *Config) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Config
	r := raw(configDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = Config(r)
	return nil
}

type MatchCfg struct {
	TickHz             float64 `json:"tick_hz"`
	DecideEvery        float64 `json:"decide_every"`
	DurationS          float64 `json:"duration_s"`
	Mechs              float64 `json:"mechs"`
	SpawnMinSeparation float64 `json:"spawn_min_separation"`
	SpawnZoneFraction  float64 `json:"spawn_zone_fraction"`
	ArenaMargin        float64 `json:"arena_margin"`
}

func matchCfgDefault() MatchCfg {
	return MatchCfg{TickHz: 20.0, DecideEvery: 2.0, DurationS: 480.0, Mechs: 16.0, SpawnMinSeparation: 130.0, SpawnZoneFraction: 0.8, ArenaMargin: 30.0}
}

func (x *MatchCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw MatchCfg
	r := raw(matchCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = MatchCfg(r)
	return nil
}

type BotCfg struct {
	TickBudgetMs  float64 `json:"tick_budget_ms"`
	HelloBudgetMs float64 `json:"hello_budget_ms"`
	CoastS        float64 `json:"coast_s"`
	OfferTimeoutS float64 `json:"offer_timeout_s"`
	MaxLineBytes  float64 `json:"max_line_bytes"`
}

func botCfgDefault() BotCfg {
	return BotCfg{TickBudgetMs: 50.0, HelloBudgetMs: 20000.0, CoastS: 1.0, OfferTimeoutS: 5.0, MaxLineBytes: 65536.0}
}

func (x *BotCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw BotCfg
	r := raw(botCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = BotCfg(r)
	return nil
}

// MechCfg is config.mech and obs.self.stats.
type MechCfg struct {
	Radius           float64 `json:"radius"`
	Height           float64 `json:"height"`
	EyeHeight        float64 `json:"eye_height"`
	MuzzleHeight     float64 `json:"muzzle_height"`
	Hp               float64 `json:"hp"`
	WalkSpeed        float64 `json:"walk_speed"`
	BoostSpeed       float64 `json:"boost_speed"`
	GroundAccel      float64 `json:"ground_accel"`
	AirAccel         float64 `json:"air_accel"`
	Gravity          float64 `json:"gravity"`
	JumpSpeed        float64 `json:"jump_speed"`
	JetAccel         float64 `json:"jet_accel"`
	MaxRiseSpeed     float64 `json:"max_rise_speed"`
	StepUp           float64 `json:"step_up"`
	MaxSlopeDeg      float64 `json:"max_slope_deg"`
	Energy           float64 `json:"energy"`
	EnergyRegen      float64 `json:"energy_regen"`
	EnergyRegenDelay float64 `json:"energy_regen_delay"`
	BoostDrain       float64 `json:"boost_drain"`
	JetDrain         float64 `json:"jet_drain"`
	TurnRate         float64 `json:"turn_rate"`
	PitchRate        float64 `json:"pitch_rate"`
	MinPitch         float64 `json:"min_pitch"`
	MaxPitch         float64 `json:"max_pitch"`
	RegenDelay       float64 `json:"regen_delay"`
	RegenPerS        float64 `json:"regen_per_s"`
	ChargeMoveMult   float64 `json:"charge_move_mult"`
	SelfDamageMult   float64 `json:"self_damage_mult"`
	Armor            float64 `json:"armor"`
	RearMult         float64 `json:"rear_mult"`
	FrontArcDeg      float64 `json:"front_arc_deg"`
	NearRangeBonus   float64 `json:"near_range_bonus"`
	VisionBonus      float64 `json:"vision_bonus"`
	Hearing          float64 `json:"hearing"`
}

func mechCfgDefault() MechCfg {
	return MechCfg{
		Radius:           2.5,
		Height:           9.0,
		EyeHeight:        8.0,
		MuzzleHeight:     7.0,
		Hp:               1400.0,
		WalkSpeed:        11.7,
		BoostSpeed:       19.5,
		GroundAccel:      58.5,
		AirAccel:         12.0,
		Gravity:          22.0,
		JumpSpeed:        11.0,
		JetAccel:         34.0,
		MaxRiseSpeed:     10.0,
		StepUp:           1.2,
		MaxSlopeDeg:      50.0,
		Energy:           100.0,
		EnergyRegen:      18.0,
		EnergyRegenDelay: 0.6,
		BoostDrain:       17.0,
		JetDrain:         15.0,
		TurnRate:         4.0,
		PitchRate:        3.0,
		MinPitch:         -0.7,
		MaxPitch:         1.3,
		RegenDelay:       5.0,
		RegenPerS:        45.0,
		ChargeMoveMult:   0.5,
		SelfDamageMult:   0.5,
		Armor:            0.0,
		RearMult:         1.5,
		FrontArcDeg:      180.0,
		NearRangeBonus:   0.0,
		VisionBonus:      0.0,
		Hearing:          1.0,
	}
}

func (x *MechCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw MechCfg
	r := raw(mechCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = MechCfg(r)
	return nil
}

type VisionCfg struct {
	Range        float64 `json:"range"`
	FovDeg       float64 `json:"fov_deg"`
	NearRange    float64 `json:"near_range"`
	CanopyBlocks bool    `json:"canopy_blocks"`
	ShowEnemyHp  bool    `json:"show_enemy_hp"`
	MemoryS      float64 `json:"memory_s"`
}

func visionCfgDefault() VisionCfg {
	return VisionCfg{Range: 200.0, FovDeg: 120.0, NearRange: 45.0, CanopyBlocks: true, ShowEnemyHp: true, MemoryS: 5.0}
}

func (x *VisionCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw VisionCfg
	r := raw(visionCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = VisionCfg(r)
	return nil
}

type SoundCfg struct {
	BearingNoiseDeg float64    `json:"bearing_noise_deg"`
	Near            float64    `json:"near"`
	Mid             float64    `json:"mid"`
	StepSpeed       float64    `json:"step_speed"`
	StepEvery       float64    `json:"step_every"`
	Range           SoundRange `json:"range"`
}

func soundCfgDefault() SoundCfg {
	return SoundCfg{BearingNoiseDeg: 7.0, Near: 60.0, Mid: 160.0, StepSpeed: 5.0, StepEvery: 0.6, Range: soundRangeDefault()}
}

func (x *SoundCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw SoundCfg
	r := raw(soundCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = SoundCfg(r)
	return nil
}

type SoundRange struct {
	Step        float64 `json:"step"`
	Boost       float64 `json:"boost"`
	Reload      float64 `json:"reload"`
	Gun         float64 `json:"gun"`
	Mortar      float64 `json:"mortar"`
	Explosion   float64 `json:"explosion"`
	LaserCharge float64 `json:"laser_charge"`
	Laser       float64 `json:"laser"`
	Lightning   float64 `json:"lightning"`
	Jet         float64 `json:"jet"`
	Death       float64 `json:"death"`
	Crate       float64 `json:"crate"`
	NpcGun      float64 `json:"npc_gun"`
	Drop        float64 `json:"drop"`
}

func soundRangeDefault() SoundRange {
	return SoundRange{
		Step:        40.0,
		Boost:       90.0,
		Reload:      60.0,
		Gun:         320.0,
		Mortar:      420.0,
		Explosion:   420.0,
		LaserCharge: 140.0,
		Laser:       380.0,
		Lightning:   160.0,
		Jet:         90.0,
		Death:       600.0,
		Crate:       220.0,
		NpcGun:      200.0,
		Drop:        600.0,
	}
}

func (x *SoundRange) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw SoundRange
	r := raw(soundRangeDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = SoundRange(r)
	return nil
}

func (x *SoundRange) get(kind string) *float64 {
	var v float64
	switch kind {
	case "step":
		v = x.Step
	case "boost":
		v = x.Boost
	case "reload":
		v = x.Reload
	case "gun":
		v = x.Gun
	case "mortar":
		v = x.Mortar
	case "explosion":
		v = x.Explosion
	case "laser_charge":
		v = x.LaserCharge
	case "laser":
		v = x.Laser
	case "lightning":
		v = x.Lightning
	case "jet":
		v = x.Jet
	case "death":
		v = x.Death
	case "crate":
		v = x.Crate
	case "npc_gun":
		v = x.NpcGun
	case "drop":
		v = x.Drop
	default:
		return nil
	}
	return &v
}

// WeaponStats holds the stats of any weapon (config.weapons.* and obs.self.weapons[i].stats).
// Fields that do not apply are 0.
type WeaponStats struct {
	Damage         float64 `json:"damage"`
	Cooldown       float64 `json:"cooldown"`
	Speed          float64 `json:"speed"`
	Range          float64 `json:"range"`
	SpreadDeg      float64 `json:"spread_deg"`
	Mag            float64 `json:"mag"`
	Reload         float64 `json:"reload"`
	Splash         float64 `json:"splash"`
	SplashRadius   float64 `json:"splash_radius"`
	SplashEdge     float64 `json:"splash_edge"`
	Gravity        float64 `json:"gravity"`
	Ttl            float64 `json:"ttl"`
	Knock          float64 `json:"knock"`
	KnockUp        float64 `json:"knock_up"`
	Drain          float64 `json:"drain"`
	Charge         float64 `json:"charge"`
	MinCharge      float64 `json:"min_charge"`
	MinFrac        float64 `json:"min_frac"`
	HoldS          float64 `json:"hold_s"`
	Pierce         float64 `json:"pierce"`
	ChargeTurnMult float64 `json:"charge_turn_mult"`
	Cookoff        float64 `json:"cookoff"`
	ConeDeg        float64 `json:"cone_deg"`
	Chain          float64 `json:"chain"`
	ChainRange     float64 `json:"chain_range"`
	ChainFalloff   float64 `json:"chain_falloff"`
	Energy         float64 `json:"energy"`
	ShortS         float64 `json:"short_s"`
}

func (x *WeaponStats) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw WeaponStats
	r := raw(WeaponStats{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = WeaponStats(r)
	return nil
}

func weaponStatsDefaultGun() WeaponStats {
	return WeaponStats{Damage: 45.0, Cooldown: 0.24, Speed: 252.0, Range: 230.0, SpreadDeg: 0.6, Mag: 12.0, Reload: 2.4}
}

func weaponStatsDefaultMortar() WeaponStats {
	return WeaponStats{
		Damage:       200.0,
		Splash:       160.0,
		SplashRadius: 7.0,
		SplashEdge:   0.3,
		Cooldown:     2.6,
		Speed:        72.0,
		Gravity:      22.0,
		Ttl:          7.0,
		Knock:        16.0,
		KnockUp:      9.0,
		Drain:        45.0,
	}
}

func weaponStatsDefaultLaser() WeaponStats {
	return WeaponStats{
		Damage:         315.0,
		Charge:         0.75,
		MinCharge:      0.15,
		MinFrac:        0.35,
		HoldS:          2.0,
		Cooldown:       1.65,
		Range:          280.0,
		Pierce:         0.0,
		ChargeTurnMult: 1.0,
		Cookoff:        0.6,
	}
}

func weaponStatsDefaultLightning() WeaponStats {
	return WeaponStats{
		Damage:       60.0,
		Cooldown:     0.25,
		Range:        45.0,
		ConeDeg:      30.0,
		Chain:        2.0,
		ChainRange:   14.0,
		ChainFalloff: 0.7,
		Energy:       3.0,
		ShortS:       2.0,
	}
}

func weaponStatsDefaultFor(kind WeaponKind) WeaponStats {
	switch kind {
	case WeaponKindGun:
		return weaponStatsDefaultGun()
	case WeaponKindMortar:
		return weaponStatsDefaultMortar()
	case WeaponKindLaser:
		return weaponStatsDefaultLaser()
	case WeaponKindLightning:
		return weaponStatsDefaultLightning()
	}
	return WeaponStats{}
}

// weaponStatsMerged: numeric entries of raw override base; everything else keeps base.
func weaponStatsMerged(base WeaponStats, raw map[string]json.RawMessage) WeaponStats {
	fields := protoFields(protoWeaponStatsType)
	m := make(map[string]json.RawMessage, len(raw))
	for k, x := range raw {
		s := bytes.TrimSpace(x)
		if !protoIsNumber(s) {
			continue
		}
		if _, ok := fields[k]; !ok {
			continue
		}
		m[k] = s
	}
	enc, err := json.Marshal(m)
	if err != nil {
		return base
	}
	type rawWS WeaponStats
	r := rawWS(base)
	if err := json.Unmarshal(enc, &r); err != nil {
		return base
	}
	return WeaponStats(r)
}

// protoDeWeapon decodes a config.weapons.* entry on top of base (absent keeps cur).
func protoDeWeapon(raw json.RawMessage, cur, base WeaponStats) (WeaponStats, error) {
	if raw == nil {
		return cur, nil
	}
	if protoIsNull(raw) {
		return weaponStatsMerged(base, nil), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return cur, err
	}
	return weaponStatsMerged(base, m), nil
}

type WeaponsCfg struct {
	Gun       WeaponStats `json:"gun" rs:"weapon"`
	Mortar    WeaponStats `json:"mortar" rs:"weapon"`
	Laser     WeaponStats `json:"laser" rs:"weapon"`
	Lightning WeaponStats `json:"lightning" rs:"weapon"`
}

func weaponsCfgDefault() WeaponsCfg {
	return WeaponsCfg{
		Gun:       weaponStatsDefaultGun(),
		Mortar:    weaponStatsDefaultMortar(),
		Laser:     weaponStatsDefaultLaser(),
		Lightning: weaponStatsDefaultLightning(),
	}
}

func (x *WeaponsCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw WeaponsCfg
	a := struct {
		raw
		Gun       json.RawMessage `json:"gun"`
		Mortar    json.RawMessage `json:"mortar"`
		Laser     json.RawMessage `json:"laser"`
		Lightning json.RawMessage `json:"lightning"`
	}{raw: raw(weaponsCfgDefault())}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Gun, err = protoDeWeapon(a.Gun, a.raw.Gun, weaponStatsDefaultGun()); err != nil {
		return err
	}
	if a.raw.Mortar, err = protoDeWeapon(a.Mortar, a.raw.Mortar, weaponStatsDefaultMortar()); err != nil {
		return err
	}
	if a.raw.Laser, err = protoDeWeapon(a.Laser, a.raw.Laser, weaponStatsDefaultLaser()); err != nil {
		return err
	}
	if a.raw.Lightning, err = protoDeWeapon(a.Lightning, a.raw.Lightning, weaponStatsDefaultLightning()); err != nil {
		return err
	}
	*x = WeaponsCfg(a.raw)
	return nil
}

func (x *WeaponsCfg) get(kind WeaponKind) *WeaponStats {
	switch kind {
	case WeaponKindGun:
		return &x.Gun
	case WeaponKindMortar:
		return &x.Mortar
	case WeaponKindLaser:
		return &x.Laser
	case WeaponKindLightning:
		return &x.Lightning
	}
	return nil
}

type ProjectileCfg struct {
	GunRadius float64 `json:"gun_radius"`
	NpcRadius float64 `json:"npc_radius"`
}

func projectileCfgDefault() ProjectileCfg {
	return ProjectileCfg{GunRadius: 0.3, NpcRadius: 0.3}
}

func (x *ProjectileCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ProjectileCfg
	r := raw(projectileCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = ProjectileCfg(r)
	return nil
}

type CubesCfg struct {
	PickupRadius      float64   `json:"pickup_radius"`
	HpPerCube         float64   `json:"hp_per_cube"`
	DamagePerCube     float64   `json:"damage_per_cube"`
	StatCap           float64   `json:"stat_cap"`
	DeathDropExtra    float64   `json:"death_drop_extra"`
	SizePerCube       float64   `json:"size_per_cube"`
	UpgradeThresholds []float64 `json:"upgrade_thresholds"`
}

func cubesCfgDefault() CubesCfg {
	return CubesCfg{
		PickupRadius:      3.5,
		HpPerCube:         0.04,
		DamagePerCube:     0.04,
		StatCap:           10.0,
		DeathDropExtra:    2.0,
		SizePerCube:       0.05,
		UpgradeThresholds: []float64{1.0, 3.0, 6.0, 10.0, 15.0, 21.0, 28.0, 36.0, 45.0},
	}
}

func (x *CubesCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw CubesCfg
	r := raw(cubesCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = CubesCfg(r)
	return nil
}

type CratesCfg struct {
	Count        float64 `json:"count"`
	RoofFraction float64 `json:"roof_fraction"`
	Hp           float64 `json:"hp"`
	Radius       float64 `json:"radius"`
	Height       float64 `json:"height"`
	Cubes        float64 `json:"cubes"`
}

func cratesCfgDefault() CratesCfg {
	return CratesCfg{Count: 36.0, RoofFraction: 0.25, Hp: 300.0, Radius: 1.4, Height: 2.4, Cubes: 1.0}
}

func (x *CratesCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw CratesCfg
	r := raw(cratesCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = CratesCfg(r)
	return nil
}

type LootWeights struct {
	Gun       float64 `json:"gun"`
	Mortar    float64 `json:"mortar"`
	Laser     float64 `json:"laser"`
	Lightning float64 `json:"lightning"`
}

func lootWeightsDefault() LootWeights {
	return LootWeights{Gun: 0.0, Mortar: 1.0, Laser: 1.0, Lightning: 1.0}
}

func (x *LootWeights) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw LootWeights
	r := raw(lootWeightsDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = LootWeights(r)
	return nil
}

type LootCfg struct {
	PickupRadius      float64     `json:"pickup_radius"`
	EquipS            float64     `json:"equip_s"`
	FloorCount        float64     `json:"floor_count"`
	FloorRoofFraction float64     `json:"floor_roof_fraction"`
	CrateWeaponChance float64     `json:"crate_weapon_chance"`
	DeathDrop         float64     `json:"death_drop"`
	Weights           LootWeights `json:"weights"`
}

func lootCfgDefault() LootCfg {
	return LootCfg{
		PickupRadius:      4.0,
		EquipS:            1.0,
		FloorCount:        40.0,
		FloorRoofFraction: 0.25,
		CrateWeaponChance: 0.5,
		DeathDrop:         1.0,
		Weights:           lootWeightsDefault(),
	}
}

func (x *LootCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw LootCfg
	r := raw(lootCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = LootCfg(r)
	return nil
}

type NpcCfg struct {
	Count           float64 `json:"count"`
	Hp              float64 `json:"hp"`
	Radius          float64 `json:"radius"`
	Height          float64 `json:"height"`
	Speed           float64 `json:"speed"`
	Leash           float64 `json:"leash"`
	ChaseLeash      float64 `json:"chase_leash"`
	AggroRange      float64 `json:"aggro_range"`
	DeaggroRange    float64 `json:"deaggro_range"`
	SpawnClear      float64 `json:"spawn_clear"`
	Damage          float64 `json:"damage"`
	Cooldown        float64 `json:"cooldown"`
	ProjectileSpeed float64 `json:"projectile_speed"`
	SpreadDeg       float64 `json:"spread_deg"`
	Cubes           float64 `json:"cubes"`
}

func npcCfgDefault() NpcCfg {
	return NpcCfg{
		Count:           12.0,
		Hp:              350.0,
		Radius:          1.8,
		Height:          3.5,
		Speed:           5.0,
		Leash:           30.0,
		ChaseLeash:      60.0,
		AggroRange:      45.0,
		DeaggroRange:    90.0,
		SpawnClear:      0.0,
		Damage:          25.0,
		Cooldown:        1.0,
		ProjectileSpeed: 90.0,
		SpreadDeg:       2.5,
		Cubes:           2.0,
	}
}

func (x *NpcCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw NpcCfg
	r := raw(npcCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = NpcCfg(r)
	return nil
}

type DropsCfg struct {
	Times    []float64 `json:"times"`
	FallS    float64   `json:"fall_s"`
	Radius   float64   `json:"radius"`
	CaptureS float64   `json:"capture_s"`
	Cubes    float64   `json:"cubes"`
	Repair   float64   `json:"repair"`
	Spread   float64   `json:"spread"`
}

func dropsCfgDefault() DropsCfg {
	return DropsCfg{Times: []float64{70.0, 140.0, 210.0}, FallS: 25.0, Radius: 8.0, CaptureS: 4.0, Cubes: 3.0, Repair: 1.0, Spread: 0.7}
}

func (x *DropsCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DropsCfg
	r := raw(dropsCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DropsCfg(r)
	return nil
}

type ZonePhase struct {
	HoldS   float64 `json:"hold_s"`
	ShrinkS float64 `json:"shrink_s"`
	Radius  float64 `json:"radius"`
	Dps     float64 `json:"dps"`
}

func (x *ZonePhase) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ZonePhase
	r := raw(ZonePhase{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = ZonePhase(r)
	return nil
}

type ZoneCfg struct {
	StartCenter V2          `json:"start_center"`
	StartRadius float64     `json:"start_radius"`
	Phases      []ZonePhase `json:"phases"`
}

func zoneCfgDefault() ZoneCfg {
	p := func(holdS, shrinkS, radius, dps float64) ZonePhase {
		return ZonePhase{HoldS: holdS, ShrinkS: shrinkS, Radius: radius, Dps: dps}
	}
	return ZoneCfg{
		StartCenter: V2{41.3, -72.8},
		StartRadius: 560.0,
		Phases: []ZonePhase{
			p(60.0, 50.0, 380.0, 20.0),
			p(45.0, 40.0, 240.0, 35.0),
			p(35.0, 35.0, 140.0, 60.0),
			p(30.0, 30.0, 70.0, 100.0),
			p(25.0, 25.0, 25.0, 160.0),
			p(15.0, 25.0, 0.0, 260.0),
		},
	}
}

func (x *ZoneCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ZoneCfg
	r := raw(zoneCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = ZoneCfg(r)
	return nil
}

type ReplayCfg struct {
	FrameEvery float64 `json:"frame_every"`
}

func replayCfgDefault() ReplayCfg {
	return ReplayCfg{FrameEvery: 2.0}
}

func (x *ReplayCfg) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ReplayCfg
	r := raw(replayCfgDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = ReplayCfg(r)
	return nil
}

type DebugCaps struct {
	MaxLines      float64 `json:"max_lines"`
	MaxPoints     float64 `json:"max_points"`
	MaxText       float64 `json:"max_text"`
	MaxTextChars  float64 `json:"max_text_chars"`
	MaxBytes      float64 `json:"max_bytes"`
	MaxMatchBytes float64 `json:"max_match_bytes"`
	HoldS         float64 `json:"hold_s"`
}

func debugCapsDefault() DebugCaps {
	return DebugCaps{MaxLines: 64.0, MaxPoints: 64.0, MaxText: 16.0, MaxTextChars: 80.0, MaxBytes: 8192.0, MaxMatchBytes: 8_000_000.0, HoldS: 0.5}
}

func (x *DebugCaps) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DebugCaps
	r := raw(debugCapsDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DebugCaps(r)
	return nil
}

type UpgradeDef struct {
	Id       string          `json:"id"`
	Name     string          `json:"name"`
	Desc     string          `json:"desc"`
	Requires *WeaponKind     `json:"requires,omitempty"`
	Max      float64         `json:"max"`
	Effects  []UpgradeEffect `json:"effects"`
}

func (x *UpgradeDef) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw UpgradeDef
	r := raw(UpgradeDef{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = UpgradeDef(r)
	return nil
}

type UpgradeEffect struct {
	// e.g. "mech.hp" or "weapons.gun.damage"
	Path string   `json:"path"`
	Mul  *float64 `json:"mul,omitempty"`
	Add  *float64 `json:"add,omitempty"`
}

func (x *UpgradeEffect) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw UpgradeEffect
	r := raw(UpgradeEffect{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = UpgradeEffect(r)
	return nil
}

// ---------------------------------------------------------------- observation

type Obs struct {
	Tick        int64           `json:"tick"`
	T           float64         `json:"t" rs:"null"`
	Alive       int64           `json:"alive"`
	Me          SelfObs         `json:"self"`
	Mechs       []ActorObs      `json:"mechs"`
	Projectiles []ProjectileObs `json:"projectiles"`
	Crates      []ItemObs       `json:"crates"`
	Cubes       []ItemObs       `json:"cubes"`
	Pickups     []PickupObs     `json:"pickups"`
	Sounds      []SoundObs      `json:"sounds"`
	Beams       []BeamObs       `json:"beams"`
	Events      []Event         `json:"events"`
	Zone        ZoneObs         `json:"zone"`
	Offer       *Offer          `json:"offer"`
	Drops       []DropObs       `json:"drops"`
}

// obsDefault is Obs::default() (self.stats at the MechCfg defaults).
func obsDefault() Obs {
	return Obs{Me: selfObsDefault()}
}

func (x *Obs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Obs
	a := struct {
		raw
		Tick  json.RawMessage `json:"tick"`
		Alive json.RawMessage `json:"alive"`
	}{raw: raw(obsDefault())}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Tick, err = protoDeI64(a.Tick, a.raw.Tick); err != nil {
		return err
	}
	if a.raw.Alive, err = protoDeI64(a.Alive, a.raw.Alive); err != nil {
		return err
	}
	*x = Obs(a.raw)
	return nil
}

type SelfObs struct {
	Id        int64         `json:"id"`
	Pos       V3            `json:"pos"`
	Vel       V3            `json:"vel"`
	Aim       V2            `json:"aim"`
	Hp        float64       `json:"hp"`
	MaxHp     float64       `json:"max_hp"`
	Energy    float64       `json:"energy"`
	MaxEnergy float64       `json:"max_energy"`
	Grounded  bool          `json:"grounded"`
	Cubes     float64       `json:"cubes"`
	Upgrades  []string      `json:"upgrades"`
	Weapons   [2]*WeaponObs `json:"weapons"`
	InZone    bool          `json:"in_zone"`
	Stats     MechCfg       `json:"stats"`
}

// selfObsDefault is SelfObs::default() (stats at the MechCfg defaults).
func selfObsDefault() SelfObs {
	return SelfObs{Stats: mechCfgDefault()}
}

func (x *SelfObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw SelfObs
	a := struct {
		raw
		Id      json.RawMessage `json:"id"`
		Weapons json.RawMessage `json:"weapons"`
	}{raw: raw(selfObsDefault())}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	if a.Weapons != nil && !protoIsNull(a.Weapons) {
		var ws []*WeaponObs
		if err := json.Unmarshal(a.Weapons, &ws); err != nil {
			return err
		}
		if len(ws) != 2 {
			return fmt.Errorf("invalid length %d, expected an array of length 2", len(ws))
		}
		a.raw.Weapons = [2]*WeaponObs{ws[0], ws[1]}
	}
	*x = SelfObs(a.raw)
	return nil
}

type WeaponObs struct {
	Kind     WeaponKind `json:"kind"`
	Cooldown float64    `json:"cooldown"`
	// gun only
	Ammo float64 `json:"ammo"`
	// gun only: seconds of reload left
	Reload float64 `json:"reload"`
	// laser only: seconds charged so far
	Charge float64     `json:"charge"`
	Stats  WeaponStats `json:"stats"`
}

func (x *WeaponObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw WeaponObs
	r := raw(WeaponObs{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = WeaponObs(r)
	return nil
}

type ActorObs struct {
	Id       int64        `json:"id"`
	Kind     ActorKind    `json:"kind"`
	Pos      V3           `json:"pos"`
	Vel      V3           `json:"vel"`
	Aim      V2           `json:"aim"`
	Hp       *float64     `json:"hp,omitempty"`
	MaxHp    *float64     `json:"max_hp,omitempty"`
	Cubes    float64      `json:"cubes"`
	Weapons  []WeaponKind `json:"weapons,omitempty"`
	Charging bool         `json:"charging,omitempty"`
}

func (x *ActorObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ActorObs
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(ActorObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = ActorObs(a.raw)
	return nil
}

type ProjectileObs struct {
	Id   int64    `json:"id"`
	Kind ProjKind `json:"kind"`
	Pos  V3       `json:"pos"`
	Vel  V3       `json:"vel"`
}

func (x *ProjectileObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ProjectileObs
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(ProjectileObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = ProjectileObs(a.raw)
	return nil
}

// ItemObs is a crate (with hp) or a cube (no hp).
type ItemObs struct {
	Id  int64    `json:"id"`
	Pos V3       `json:"pos"`
	Hp  *float64 `json:"hp,omitempty"`
}

func (x *ItemObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ItemObs
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(ItemObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = ItemObs(a.raw)
	return nil
}

type PickupObs struct {
	Id   int64      `json:"id"`
	Kind WeaponKind `json:"kind"`
	Pos  V3         `json:"pos"`
}

func (x *PickupObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw PickupObs
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(PickupObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = PickupObs(a.raw)
	return nil
}

type SoundObs struct {
	Kind    string    `json:"kind"`
	Bearing float64   `json:"bearing"`
	Dist    SoundDist `json:"dist"`
	Age     float64   `json:"age"`
}

func (x *SoundObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw SoundObs
	r := raw(SoundObs{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = SoundObs(r)
	return nil
}

type BeamObs struct {
	By   int64    `json:"by"`
	Kind BeamKind `json:"kind"`
	Path []V3     `json:"path"`
	Age  float64  `json:"age"`
}

func (x *BeamObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw BeamObs
	a := struct {
		raw
		By json.RawMessage `json:"by"`
	}{raw: raw(BeamObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.By, err = protoDeI64(a.By, a.raw.By); err != nil {
		return err
	}
	*x = BeamObs(a.raw)
	return nil
}

type DropObs struct {
	Id       int64   `json:"id"`
	Pos      V3      `json:"pos"`
	Radius   float64 `json:"radius"`
	LandsIn  float64 `json:"lands_in"`
	CaptureS float64 `json:"capture_s"`
	Progress float64 `json:"progress"`
	Holder   *int64  `json:"holder"`
}

func (x *DropObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DropObs
	a := struct {
		raw
		Id     json.RawMessage `json:"id"`
		Holder json.RawMessage `json:"holder"`
	}{raw: raw(DropObs{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	if a.raw.Holder, err = protoDeOptI64(a.Holder, a.raw.Holder); err != nil {
		return err
	}
	*x = DropObs(a.raw)
	return nil
}

type ZoneObs struct {
	Center      V2      `json:"center"`
	Radius      float64 `json:"radius"`
	NextCenter  V2      `json:"next_center"`
	NextRadius  float64 `json:"next_radius"`
	ShrinkStart float64 `json:"shrink_start"`
	ShrinkEnd   float64 `json:"shrink_end"`
	Dps         float64 `json:"dps"`
}

func (x *ZoneObs) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw ZoneObs
	r := raw(ZoneObs{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = ZoneObs(r)
	return nil
}

type Offer struct {
	Id      int64         `json:"id"`
	Choices []OfferChoice `json:"choices"`
	Expires float64       `json:"expires"`
}

func (x *Offer) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Offer
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(Offer{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = Offer(a.raw)
	return nil
}

type OfferChoice struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

func (x *OfferChoice) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw OfferChoice
	r := raw(OfferChoice{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = OfferChoice(r)
	return nil
}

// ---------------------------------------------------------------- events

type EventKind int

const (
	EventKindDamaged EventKind = iota
	EventKindHit
	EventKindEffect
	EventKindDrop
	EventKindKill
	EventKindPickup
	EventKindUpgrade
	EventKindTook
	EventKindDropped
	EventKindTakeFailed
	EventKindUnknown
)

// Event is tagged by "type" in JSON.
type Event struct {
	kind        EventKind
	damaged0    DamagedEv
	hit0        HitEv
	effect0     EffectEv
	drop0       DropEv
	kill0       KillEv
	pickup0     PickupEv
	upgrade0    UpgradeEv
	took0       TookEv
	dropped0    DroppedEv
	takeFailed0 TakeFailedEv
}

var protoEventTypes = map[string]reflect.Type{
	"damaged":     reflect.TypeOf(DamagedEv{}),
	"hit":         reflect.TypeOf(HitEv{}),
	"effect":      reflect.TypeOf(EffectEv{}),
	"drop":        reflect.TypeOf(DropEv{}),
	"kill":        reflect.TypeOf(KillEv{}),
	"pickup":      reflect.TypeOf(PickupEv{}),
	"upgrade":     reflect.TypeOf(UpgradeEv{}),
	"took":        reflect.TypeOf(TookEv{}),
	"dropped":     reflect.TypeOf(DroppedEv{}),
	"take_failed": reflect.TypeOf(TakeFailedEv{}),
}

// protoEventNames lists the Event tags in variant order: a non-negative integer tag selects
// the variant with that index (Unknown when out of range).
var protoEventNames = []string{"damaged", "hit", "effect", "drop", "kill", "pickup", "upgrade", "took", "dropped", "take_failed"}

func (x *Event) UnmarshalJSON(b []byte) error {
	name, err := protoTag(b, "Event", protoEventNames)
	if err != nil {
		return err
	}
	var e Event
	switch name {
	case "damaged":
		e.kind = EventKindDamaged
		err = json.Unmarshal(b, &e.damaged0)
	case "hit":
		e.kind = EventKindHit
		err = json.Unmarshal(b, &e.hit0)
	case "effect":
		e.kind = EventKindEffect
		err = json.Unmarshal(b, &e.effect0)
	case "drop":
		e.kind = EventKindDrop
		err = json.Unmarshal(b, &e.drop0)
	case "kill":
		e.kind = EventKindKill
		err = json.Unmarshal(b, &e.kill0)
	case "pickup":
		e.kind = EventKindPickup
		err = json.Unmarshal(b, &e.pickup0)
	case "upgrade":
		e.kind = EventKindUpgrade
		err = json.Unmarshal(b, &e.upgrade0)
	case "took":
		e.kind = EventKindTook
		err = json.Unmarshal(b, &e.took0)
	case "dropped":
		e.kind = EventKindDropped
		err = json.Unmarshal(b, &e.dropped0)
	case "take_failed":
		e.kind = EventKindTakeFailed
		err = json.Unmarshal(b, &e.takeFailed0)
	default:
		e.kind = EventKindUnknown
	}
	if err != nil {
		return err
	}
	*x = e
	return nil
}

type DamagedEv struct {
	Amount float64 `json:"amount"`
	// gun, mortar, laser, lightning, npc or zone
	Weapon string `json:"weapon"`
	// null for the zone
	Bearing *float64 `json:"bearing"`
	// null for the zone
	By   *int64 `json:"by"`
	Rear bool   `json:"rear,omitempty"`
}

func (x *DamagedEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DamagedEv
	a := struct {
		raw
		By json.RawMessage `json:"by"`
	}{raw: raw(DamagedEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.By, err = protoDeOptI64(a.By, a.raw.By); err != nil {
		return err
	}
	*x = DamagedEv(a.raw)
	return nil
}

type HitEv struct {
	Target int64       `json:"target"`
	Amount float64     `json:"amount"`
	Weapon string      `json:"weapon"`
	Effect *EffectName `json:"effect,omitempty"`
	Rear   bool        `json:"rear,omitempty"`
}

func (x *HitEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw HitEv
	a := struct {
		raw
		Target json.RawMessage `json:"target"`
	}{raw: raw(HitEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Target, err = protoDeI64(a.Target, a.raw.Target); err != nil {
		return err
	}
	*x = HitEv(a.raw)
	return nil
}

type EffectEv struct {
	Effect EffectName `json:"effect"`
	By     *int64     `json:"by"`
}

func effectEvDefault() EffectEv {
	return EffectEv{Effect: EffectNameUnknown, By: nil}
}

func (x *EffectEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw EffectEv
	a := struct {
		raw
		By json.RawMessage `json:"by"`
	}{raw: raw(effectEvDefault())}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.By, err = protoDeOptI64(a.By, a.raw.By); err != nil {
		return err
	}
	*x = EffectEv(a.raw)
	return nil
}

type DropEv struct {
	Id    int64   `json:"id"`
	Cubes float64 `json:"cubes"`
}

func (x *DropEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DropEv
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(DropEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = DropEv(a.raw)
	return nil
}

type KillEv struct {
	Target int64 `json:"target"`
}

func (x *KillEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw KillEv
	a := struct {
		raw
		Target json.RawMessage `json:"target"`
	}{raw: raw(KillEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Target, err = protoDeI64(a.Target, a.raw.Target); err != nil {
		return err
	}
	*x = KillEv(a.raw)
	return nil
}

type PickupEv struct {
	Cubes float64 `json:"cubes"`
}

func (x *PickupEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw PickupEv
	r := raw(PickupEv{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = PickupEv(r)
	return nil
}

type UpgradeEv struct {
	Id string `json:"id"`
}

func (x *UpgradeEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw UpgradeEv
	r := raw(UpgradeEv{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = UpgradeEv(r)
	return nil
}

type TookEv struct {
	Id   int64      `json:"id"`
	Kind WeaponKind `json:"kind"`
	Slot int64      `json:"slot"`
}

func (x *TookEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw TookEv
	a := struct {
		raw
		Id   json.RawMessage `json:"id"`
		Slot json.RawMessage `json:"slot"`
	}{raw: raw(TookEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	if a.raw.Slot, err = protoDeI64(a.Slot, a.raw.Slot); err != nil {
		return err
	}
	*x = TookEv(a.raw)
	return nil
}

type DroppedEv struct {
	Id   int64      `json:"id"`
	Kind WeaponKind `json:"kind"`
	Slot int64      `json:"slot"`
}

func (x *DroppedEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DroppedEv
	a := struct {
		raw
		Id   json.RawMessage `json:"id"`
		Slot json.RawMessage `json:"slot"`
	}{raw: raw(DroppedEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	if a.raw.Slot, err = protoDeI64(a.Slot, a.raw.Slot); err != nil {
		return err
	}
	*x = DroppedEv(a.raw)
	return nil
}

type TakeFailedEv struct {
	Id     int64          `json:"id"`
	Reason TakeFailReason `json:"reason"`
}

func (x *TakeFailedEv) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw TakeFailedEv
	a := struct {
		raw
		Id json.RawMessage `json:"id"`
	}{raw: raw(TakeFailedEv{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Id, err = protoDeI64(a.Id, a.raw.Id); err != nil {
		return err
	}
	*x = TakeFailedEv(a.raw)
	return nil
}

// ---------------------------------------------------------------- end

type End struct {
	Place int64 `json:"place"`
	Kills int64 `json:"kills"`
}

func (x *End) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw End
	a := struct {
		raw
		Place json.RawMessage `json:"place"`
		Kills json.RawMessage `json:"kills"`
	}{raw: raw(End{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Place, err = protoDeI64(a.Place, a.raw.Place); err != nil {
		return err
	}
	if a.raw.Kills, err = protoDeI64(a.Kills, a.raw.Kills); err != nil {
		return err
	}
	*x = End(a.raw)
	return nil
}

type ServerMsgKind int

const (
	ServerMsgKindHello ServerMsgKind = iota
	ServerMsgKindObs
	ServerMsgKindEnd
	ServerMsgKindUnknown
)

// ServerMsg is every server message, tagged by "type" in JSON.
type ServerMsg struct {
	kind   ServerMsgKind
	hello0 *Hello
	obs0   *Obs
	end0   End
}

func (x *ServerMsg) UnmarshalJSON(b []byte) error {
	name, err := protoTag(b, "ServerMsg", nil)
	if err != nil {
		return err
	}
	var m ServerMsg
	switch name {
	case "hello":
		m.kind = ServerMsgKindHello
		m.hello0 = new(Hello)
		err = json.Unmarshal(b, m.hello0)
	case "obs":
		m.kind = ServerMsgKindObs
		m.obs0 = new(Obs)
		err = json.Unmarshal(b, m.obs0)
	case "end":
		m.kind = ServerMsgKindEnd
		err = json.Unmarshal(b, &m.end0)
	default:
		m.kind = ServerMsgKindUnknown
	}
	if err != nil {
		return err
	}
	*x = m
	return nil
}

// protoParseServerMsg parses one server line. A line that only fails because of null values
// is accepted with those nulls removed (null array elements dropped, except inside "weapons").
// kind is "hello", "obs", "end" or "unknown" (any other "type"); exactly one of hello/obs/end
// is set for the first three.
func protoParseServerMsg(line []byte) (kind string, hello *Hello, obs *Obs, end *End, err error) {
	v, err := protoParseTree(line)
	if err != nil {
		return "", nil, nil, nil, err
	}
	if v, err = protoCanon(v, "", true); err != nil {
		return "", nil, nil, nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return "", nil, nil, nil, errors.New("invalid type: expected internally tagged enum ServerMsg")
	}
	tv, ok := m["type"]
	if !ok {
		return "", nil, nil, nil, errors.New("missing field `type`")
	}
	name, ok := tv.(string)
	if !ok {
		return "", nil, nil, nil, errors.New("invalid type for `type`, expected a string")
	}
	var out any
	switch name {
	case "hello":
		hello = new(Hello)
		out = hello
	case "obs":
		obs = new(Obs)
		out = obs
	case "end":
		end = new(End)
		out = end
	default:
		return "unknown", nil, nil, nil, nil
	}
	if err := protoDecodeTree(v, out); err != nil {
		return "", nil, nil, nil, err
	}
	return name, hello, obs, end, nil
}

// protoDecodeStrict decodes data into out (a pointer) the way a direct typed parse does:
// null is an error for every field that is not optional.
func protoDecodeStrict(data []byte, out any) error {
	v, err := protoParseTree(data)
	if err != nil {
		return err
	}
	if v, err = protoCanon(v, "", false); err != nil {
		return err
	}
	return protoDecodeTree(v, out)
}

// ---------------------------------------------------------------- bot messages

type Ready struct {
	Ty   string `json:"type"`
	Name string `json:"name"`
}

func readyNew(name string) Ready {
	return Ready{Ty: "ready", Name: name}
}

func readyDefault() Ready {
	return readyNew("")
}

func (x *Ready) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Ready
	r := raw(readyDefault())
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = Ready(r)
	return nil
}

// MarshalJSON writes {"type":..,"name":..}. Call it directly (json.Marshal would HTML-escape).
func (x Ready) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, 48)
	b = append(b, '{')
	b = protoPutStr(b, "type")
	b = append(b, ':')
	b = protoPutStr(b, x.Ty)
	b = append(b, ',')
	b = protoPutStr(b, "name")
	b = append(b, ':')
	b = protoPutStr(b, x.Name)
	return append(b, '}'), nil
}

type Action struct {
	Tick   int64      `json:"tick"`
	Move   *V2        `json:"move,omitempty"`
	Boost  *bool      `json:"boost,omitempty"`
	Jump   *bool      `json:"jump,omitempty"`
	Aim    *V2        `json:"aim,omitempty"`
	Fire   *[2]bool   `json:"fire,omitempty"`
	Reload *bool      `json:"reload,omitempty"`
	Pick   *int64     `json:"pick,omitempty"`
	Take   *int64     `json:"take,omitempty"`
	Drop   *int64     `json:"drop,omitempty"`
	Debug  *DebugDraw `json:"debug,omitempty"`
}

func actionNew(tick int64) Action {
	return Action{Tick: tick}
}

func (x *Action) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw Action
	a := struct {
		raw
		Tick json.RawMessage `json:"tick"`
		Fire json.RawMessage `json:"fire"`
		Pick json.RawMessage `json:"pick"`
		Take json.RawMessage `json:"take"`
		Drop json.RawMessage `json:"drop"`
	}{raw: raw(Action{})}
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	var err error
	if a.raw.Tick, err = protoDeI64(a.Tick, a.raw.Tick); err != nil {
		return err
	}
	if a.Fire != nil {
		if protoIsNull(a.Fire) {
			a.raw.Fire = nil
		} else {
			var f []bool
			if err := json.Unmarshal(a.Fire, &f); err != nil {
				return err
			}
			if len(f) != 2 {
				return fmt.Errorf("invalid length %d, expected an array of length 2", len(f))
			}
			a.raw.Fire = &[2]bool{f[0], f[1]}
		}
	}
	if a.raw.Pick, err = protoDeOptI64(a.Pick, a.raw.Pick); err != nil {
		return err
	}
	if a.raw.Take, err = protoDeOptI64(a.Take, a.raw.Take); err != nil {
		return err
	}
	if a.raw.Drop, err = protoDeOptI64(a.Drop, a.raw.Drop); err != nil {
		return err
	}
	*x = Action(a.raw)
	return nil
}

// MarshalJSON writes the action with None fields omitted, NaN/inf as null.
// Call it directly (json.Marshal would HTML-escape strings).
func (x Action) MarshalJSON() ([]byte, error) {
	b := make([]byte, 0, 128)
	b = append(b, '{')
	first := true
	b = protoPutKey(b, &first, "tick")
	b = strconv.AppendInt(b, x.Tick, 10)
	if x.Move != nil {
		b = protoPutKey(b, &first, "move")
		b = protoPutF64s(b, x.Move[:])
	}
	if x.Boost != nil {
		b = protoPutKey(b, &first, "boost")
		b = strconv.AppendBool(b, *x.Boost)
	}
	if x.Jump != nil {
		b = protoPutKey(b, &first, "jump")
		b = strconv.AppendBool(b, *x.Jump)
	}
	if x.Aim != nil {
		b = protoPutKey(b, &first, "aim")
		b = protoPutF64s(b, x.Aim[:])
	}
	if x.Fire != nil {
		b = protoPutKey(b, &first, "fire")
		b = append(b, '[')
		b = strconv.AppendBool(b, x.Fire[0])
		b = append(b, ',')
		b = strconv.AppendBool(b, x.Fire[1])
		b = append(b, ']')
	}
	if x.Reload != nil {
		b = protoPutKey(b, &first, "reload")
		b = strconv.AppendBool(b, *x.Reload)
	}
	if x.Pick != nil {
		b = protoPutKey(b, &first, "pick")
		b = strconv.AppendInt(b, *x.Pick, 10)
	}
	if x.Take != nil {
		b = protoPutKey(b, &first, "take")
		b = strconv.AppendInt(b, *x.Take, 10)
	}
	if x.Drop != nil {
		b = protoPutKey(b, &first, "drop")
		b = strconv.AppendInt(b, *x.Drop, 10)
	}
	if x.Debug != nil {
		b = protoPutKey(b, &first, "debug")
		b = protoPutDebugDraw(b, x.Debug)
	}
	return append(b, '}'), nil
}

type DebugDraw struct {
	Lines  []DebugLine  `json:"lines,omitempty"`
	Points []DebugPoint `json:"points,omitempty"`
	Text   []DebugText  `json:"text,omitempty"`
	Label  *string      `json:"label,omitempty"`
}

func (x *DebugDraw) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DebugDraw
	r := raw(DebugDraw{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DebugDraw(r)
	return nil
}

func (x DebugDraw) MarshalJSON() ([]byte, error) {
	return protoPutDebugDraw(nil, &x), nil
}

type DebugLine struct {
	A     V3      `json:"a"`
	B     V3      `json:"b"`
	Color *string `json:"color,omitempty"`
}

func (x *DebugLine) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DebugLine
	r := raw(DebugLine{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DebugLine(r)
	return nil
}

type DebugPoint struct {
	At    V3       `json:"at"`
	Color *string  `json:"color,omitempty"`
	Size  *float64 `json:"size,omitempty"`
}

func (x *DebugPoint) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DebugPoint
	r := raw(DebugPoint{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DebugPoint(r)
	return nil
}

type DebugText struct {
	Text  string  `json:"text"`
	At    *V3     `json:"at,omitempty"`
	Color *string `json:"color,omitempty"`
}

func (x *DebugText) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	type raw DebugText
	r := raw(DebugText{})
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	*x = DebugText(r)
	return nil
}

func protoPutDebugDraw(b []byte, d *DebugDraw) []byte {
	b = append(b, '{')
	first := true
	if len(d.Lines) > 0 {
		b = protoPutKey(b, &first, "lines")
		b = append(b, '[')
		for i := range d.Lines {
			if i > 0 {
				b = append(b, ',')
			}
			l := &d.Lines[i]
			b = append(b, '{')
			f := true
			b = protoPutKey(b, &f, "a")
			b = protoPutF64s(b, l.A[:])
			b = protoPutKey(b, &f, "b")
			b = protoPutF64s(b, l.B[:])
			if l.Color != nil {
				b = protoPutKey(b, &f, "color")
				b = protoPutStr(b, *l.Color)
			}
			b = append(b, '}')
		}
		b = append(b, ']')
	}
	if len(d.Points) > 0 {
		b = protoPutKey(b, &first, "points")
		b = append(b, '[')
		for i := range d.Points {
			if i > 0 {
				b = append(b, ',')
			}
			p := &d.Points[i]
			b = append(b, '{')
			f := true
			b = protoPutKey(b, &f, "at")
			b = protoPutF64s(b, p.At[:])
			if p.Color != nil {
				b = protoPutKey(b, &f, "color")
				b = protoPutStr(b, *p.Color)
			}
			if p.Size != nil {
				b = protoPutKey(b, &f, "size")
				b = protoPutF64(b, *p.Size)
			}
			b = append(b, '}')
		}
		b = append(b, ']')
	}
	if len(d.Text) > 0 {
		b = protoPutKey(b, &first, "text")
		b = append(b, '[')
		for i := range d.Text {
			if i > 0 {
				b = append(b, ',')
			}
			t := &d.Text[i]
			b = append(b, '{')
			f := true
			b = protoPutKey(b, &f, "text")
			b = protoPutStr(b, t.Text)
			if t.At != nil {
				b = protoPutKey(b, &f, "at")
				b = protoPutF64s(b, t.At[:])
			}
			if t.Color != nil {
				b = protoPutKey(b, &f, "color")
				b = protoPutStr(b, *t.Color)
			}
			b = append(b, '}')
		}
		b = append(b, ']')
	}
	if d.Label != nil {
		b = protoPutKey(b, &first, "label")
		b = protoPutStr(b, *d.Label)
	}
	return append(b, '}')
}

// ---------------------------------------------------------------- JSON helpers

// V2 and V3 decode only from arrays of exactly 2 / 3 numbers.
func (v *V2) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	return protoFloats(b, v[:])
}

func (v *V3) UnmarshalJSON(b []byte) error {
	if protoIsNull(b) {
		return nil
	}
	return protoFloats(b, v[:])
}

func protoFloats(b []byte, out []float64) error {
	s := bytes.TrimSpace(b)
	if len(s) < 2 || s[0] != '[' || s[len(s)-1] != ']' {
		return errors.New("invalid type: expected an array")
	}
	inner := bytes.TrimSpace(s[1 : len(s)-1])
	var parts [][]byte
	if len(inner) > 0 {
		parts = bytes.Split(inner, []byte{','})
	}
	if len(parts) != len(out) {
		return fmt.Errorf("invalid length %d, expected an array of length %d", len(parts), len(out))
	}
	var tmp [3]float64
	for i, p := range parts {
		f, isNull, err := protoRawF64(p)
		if err != nil {
			return err
		}
		if isNull {
			return errors.New("invalid type: null, expected f64")
		}
		tmp[i] = f
	}
	copy(out, tmp[:len(out)])
	return nil
}

func protoIsNull(b []byte) bool {
	return string(bytes.TrimSpace(b)) == "null"
}

func protoIsNumber(s []byte) bool {
	return len(s) > 0 && (s[0] == '-' || (s[0] >= '0' && s[0] <= '9'))
}

// protoRawF64 reads a JSON number (or null) as f64.
func protoRawF64(raw []byte) (f float64, isNull bool, err error) {
	s := bytes.TrimSpace(raw)
	if string(s) == "null" {
		return 0, true, nil
	}
	if !protoIsNumber(s) {
		return 0, false, errors.New("invalid type: expected f64")
	}
	f, err = strconv.ParseFloat(string(s), 64)
	if err != nil {
		return 0, false, err
	}
	return f, false, nil
}

// protoDeI64 is the lenient i64 field: any number truncates, null is 0, absent keeps cur.
func protoDeI64(raw json.RawMessage, cur int64) (int64, error) {
	if raw == nil {
		return cur, nil
	}
	f, isNull, err := protoRawF64(raw)
	if err != nil {
		return cur, err
	}
	if isNull {
		return 0, nil
	}
	return f2i64(f), nil
}

// protoDeOptI64 is the lenient Option<i64> field: any number truncates, null is None, absent keeps cur.
func protoDeOptI64(raw json.RawMessage, cur *int64) (*int64, error) {
	if raw == nil {
		return cur, nil
	}
	f, isNull, err := protoRawF64(raw)
	if err != nil {
		return cur, err
	}
	if isNull {
		return nil, nil
	}
	return ptr(f2i64(f)), nil
}

// protoTag reads the "type" tag of a tagged object. With byIndex, a non-negative integer tag
// names the variant with that index ("" when out of range).
func protoTag(b []byte, what string, byIndex []string) (string, error) {
	if protoIsNull(b) {
		return "", fmt.Errorf("invalid type: null, expected internally tagged enum %s", what)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return "", err
	}
	t, ok := m["type"]
	if !ok {
		return "", errors.New("missing field `type`")
	}
	t = bytes.TrimSpace(t)
	if byIndex != nil && len(t) > 0 && t[0] >= '0' && t[0] <= '9' {
		name, ok := protoTagIndex(string(t), byIndex)
		if !ok {
			return "", errors.New("invalid type: expected variant identifier")
		}
		return name, nil
	}
	if protoIsNull(t) {
		return "", errors.New("invalid type: null, expected variant identifier")
	}
	var name string
	if err := json.Unmarshal(t, &name); err != nil {
		return "", err
	}
	return name, nil
}

// protoTagIndex maps an integer tag token (u64) to a variant name; ok is false for other numbers.
func protoTagIndex(tok string, names []string) (string, bool) {
	i, err := strconv.ParseUint(tok, 10, 64)
	if err != nil {
		return "", false
	}
	if i < uint64(len(names)) {
		return names[i], true
	}
	return "", true
}

// protoParseTree parses one JSON document into a generic tree (numbers kept as text).
func protoParseTree(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			err = errors.New("trailing characters")
		}
		return nil, err
	}
	return v, nil
}

// protoCanon replaces every number by text that decodes to the value the reference parser
// produces, and with strip removes null object entries and null array elements (except in
// arrays under the key "weapons").
func protoCanon(v any, key string, strip bool) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		if strip {
			for k, x := range t {
				if x == nil {
					delete(t, k)
				}
			}
		}
		for k, x := range t {
			nx, err := protoCanon(x, k, strip)
			if err != nil {
				return nil, err
			}
			t[k] = nx
		}
		return t, nil
	case []any:
		if strip && key != "weapons" {
			n := 0
			for _, x := range t {
				if x != nil {
					t[n] = x
					n++
				}
			}
			t = t[:n]
		}
		for i, x := range t {
			nx, err := protoCanon(x, "", strip)
			if err != nil {
				return nil, err
			}
			t[i] = nx
		}
		return t, nil
	case json.Number:
		f, err := protoSerdeF64(string(t))
		if err != nil {
			return nil, err
		}
		if g, e := strconv.ParseFloat(string(t), 64); e == nil && math.Float64bits(g) == math.Float64bits(f) {
			return t, nil
		}
		return json.Number(strconv.FormatFloat(f, 'g', -1, 64)), nil
	}
	return v, nil
}

// protoDecodeTree checks a canonical tree against out's type (null only where allowed, unknown
// keys dropped so that only exact key matches decode) and decodes it into out.
func protoDecodeTree(v any, out any) error {
	if err := protoWalk(reflect.TypeOf(out).Elem(), v); err != nil {
		return err
	}
	enc, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(enc, out)
}

type protoField struct {
	typ     reflect.Type
	lenient bool // accepts null
	weapon  bool // config.weapons.* entry: only numeric members count
}

var protoFieldCache sync.Map

var (
	protoEventType       = reflect.TypeOf(Event{})
	protoServerMsgType   = reflect.TypeOf(ServerMsg{})
	protoWeaponStatsType = reflect.TypeOf(WeaponStats{})
	protoServerMsgTypes  = map[string]reflect.Type{
		"hello": reflect.TypeOf(Hello{}),
		"obs":   reflect.TypeOf(Obs{}),
		"end":   reflect.TypeOf(End{}),
	}
)

func protoFields(t reflect.Type) map[string]protoField {
	if m, ok := protoFieldCache.Load(t); ok {
		return m.(map[string]protoField)
	}
	m := make(map[string]protoField)
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := f.Tag.Get("json")
		if c := strings.IndexByte(name, ','); c >= 0 {
			name = name[:c]
		}
		if name == "-" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		rs := f.Tag.Get("rs")
		m[name] = protoField{typ: f.Type, lenient: rs == "null" || rs == "weapon", weapon: rs == "weapon"}
	}
	protoFieldCache.Store(t, m)
	return m
}

func protoWalk(t reflect.Type, v any) error {
	if v == nil {
		if t.Kind() == reflect.Pointer || t.Kind() == reflect.Int64 {
			return nil
		}
		return fmt.Errorf("invalid type: null, expected %s", t)
	}
	switch t.Kind() {
	case reflect.Pointer:
		return protoWalk(t.Elem(), v)
	case reflect.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		switch t {
		case protoEventType:
			return protoWalkTagged(m, protoEventTypes, protoEventNames)
		case protoServerMsgType:
			return protoWalkTagged(m, protoServerMsgTypes, nil)
		}
		return protoWalkStruct(t, m)
	case reflect.Slice, reflect.Array:
		a, ok := v.([]any)
		if !ok {
			return nil
		}
		for _, e := range a {
			if err := protoWalk(t.Elem(), e); err != nil {
				return err
			}
		}
	}
	return nil
}

func protoWalkTagged(m map[string]any, types map[string]reflect.Type, byIndex []string) error {
	tag := m["type"]
	var name string
	switch t := tag.(type) {
	case string:
		name = t
	case json.Number:
		if byIndex == nil {
			return nil
		}
		name, _ = protoTagIndex(string(t), byIndex)
	default:
		return nil
	}
	pt, ok := types[name]
	if !ok {
		return nil
	}
	err := protoWalkStruct(pt, m)
	m["type"] = tag
	return err
}

func protoWalkStruct(t reflect.Type, m map[string]any) error {
	fs := protoFields(t)
	for k, x := range m {
		f, ok := fs[k]
		if !ok {
			delete(m, k)
			continue
		}
		if f.weapon {
			if wm, ok := x.(map[string]any); ok {
				wfs := protoFields(protoWeaponStatsType)
				for wk, wx := range wm {
					if _, isNum := wx.(json.Number); !isNum {
						delete(wm, wk)
					} else if _, known := wfs[wk]; !known {
						delete(wm, wk)
					}
				}
			}
			continue
		}
		if x == nil && f.lenient {
			continue
		}
		if err := protoWalk(f.typ, x); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------- number parsing

var errProtoNumber = errors.New("invalid number")
var errProtoRange = errors.New("number out of range")

var protoPow10 = func() [309]float64 {
	var t [309]float64
	for i := range t {
		t[i], _ = strconv.ParseFloat("1e"+strconv.Itoa(i), 64)
	}
	return t
}()

func protoIsDigit(c byte) bool { return c >= '0' && c <= '9' }

// protoOverflowU64 reports whether sig*10+d overflows u64.
func protoOverflowU64(sig, d uint64) bool {
	const c = math.MaxUint64
	return sig >= c/10 && (sig > c/10 || d > c%10)
}

// protoSerdeF64 converts a JSON number token to f64 with the reference parser's arithmetic:
// up to 19-20 significant digits accumulated in a u64, then scaled by one power of ten.
func protoSerdeF64(s string) (float64, error) {
	i, n := 0, len(s)
	positive := true
	if i < n && s[i] == '-' {
		positive = false
		i++
	}
	if i >= n {
		return 0, errProtoNumber
	}
	var sig uint64
	switch {
	case s[i] == '0':
		i++
		if i < n && protoIsDigit(s[i]) {
			return 0, errProtoNumber
		}
	case s[i] >= '1' && s[i] <= '9':
		sig = uint64(s[i] - '0')
		i++
		for i < n && protoIsDigit(s[i]) {
			d := uint64(s[i] - '0')
			if protoOverflowU64(sig, d) {
				return protoLongInteger(s, i, positive, sig)
			}
			sig = sig*10 + d
			i++
		}
	default:
		return 0, errProtoNumber
	}
	if i < n && s[i] == '.' {
		return protoDecimal(s, i, positive, sig, 0)
	}
	if i < n && (s[i] == 'e' || s[i] == 'E') {
		return protoExponent(s, i, positive, sig, 0)
	}
	if i != n {
		return 0, errProtoNumber
	}
	if positive {
		return float64(sig), nil
	}
	neg := -int64(sig)
	if neg >= 0 {
		return -float64(sig), nil
	}
	return float64(neg), nil
}

func protoLongInteger(s string, i int, positive bool, sig uint64) (float64, error) {
	var exponent int32
	for i < len(s) {
		switch c := s[i]; {
		case protoIsDigit(c):
			i++
			exponent++
		case c == '.':
			return protoDecimal(s, i, positive, sig, exponent)
		case c == 'e' || c == 'E':
			return protoExponent(s, i, positive, sig, exponent)
		default:
			return 0, errProtoNumber
		}
	}
	return protoFromParts(positive, sig, exponent)
}

func protoDecimal(s string, i int, positive bool, sig uint64, expBefore int32) (float64, error) {
	i++
	var expAfter int32
	for i < len(s) && protoIsDigit(s[i]) {
		d := uint64(s[i] - '0')
		if protoOverflowU64(sig, d) {
			exponent := expBefore + expAfter
			for i < len(s) && protoIsDigit(s[i]) {
				i++
			}
			if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
				return protoExponent(s, i, positive, sig, exponent)
			}
			if i != len(s) {
				return 0, errProtoNumber
			}
			return protoFromParts(positive, sig, exponent)
		}
		sig = sig*10 + d
		expAfter--
		i++
	}
	if expAfter == 0 {
		return 0, errProtoNumber
	}
	exponent := expBefore + expAfter
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		return protoExponent(s, i, positive, sig, exponent)
	}
	if i != len(s) {
		return 0, errProtoNumber
	}
	return protoFromParts(positive, sig, exponent)
}

func protoExponent(s string, i int, positive bool, sig uint64, startExp int32) (float64, error) {
	i++
	positiveExp := true
	if i < len(s) && s[i] == '+' {
		i++
	} else if i < len(s) && s[i] == '-' {
		positiveExp = false
		i++
	}
	if i >= len(s) || !protoIsDigit(s[i]) {
		return 0, errProtoNumber
	}
	exp := int32(s[i] - '0')
	i++
	for i < len(s) && protoIsDigit(s[i]) {
		d := int32(s[i] - '0')
		i++
		if exp >= math.MaxInt32/10 && (exp > math.MaxInt32/10 || d > math.MaxInt32%10) {
			if sig != 0 && positiveExp {
				return 0, errProtoRange
			}
			for i < len(s) && protoIsDigit(s[i]) {
				i++
			}
			if i != len(s) {
				return 0, errProtoNumber
			}
			if positive {
				return 0.0, nil
			}
			return math.Copysign(0, -1), nil
		}
		exp = exp*10 + d
	}
	if i != len(s) {
		return 0, errProtoNumber
	}
	var final int64
	if positiveExp {
		final = int64(startExp) + int64(exp)
	} else {
		final = int64(startExp) - int64(exp)
	}
	if final > math.MaxInt32 {
		final = math.MaxInt32
	} else if final < math.MinInt32 {
		final = math.MinInt32
	}
	return protoFromParts(positive, sig, int32(final))
}

func protoFromParts(positive bool, sig uint64, exponent int32) (float64, error) {
	f := float64(sig)
	for {
		a := exponent
		if a < 0 {
			a = -a
		}
		if a >= 0 && a < int32(len(protoPow10)) {
			pow := protoPow10[a]
			if exponent >= 0 {
				f *= pow
				if math.IsInf(f, 0) {
					return 0, errProtoRange
				}
			} else {
				f /= pow
			}
			break
		}
		if f == 0.0 {
			break
		}
		if exponent >= 0 {
			return 0, errProtoRange
		}
		f /= 1e308
		exponent += 308
	}
	if positive {
		return f, nil
	}
	return -f, nil
}

// ---------------------------------------------------------------- output formatting

func protoPutKey(b []byte, first *bool, k string) []byte {
	if !*first {
		b = append(b, ',')
	}
	*first = false
	b = protoPutStr(b, k)
	return append(b, ':')
}

const protoHex = "0123456789abcdef"

func protoPutStr(b []byte, s string) []byte {
	b = append(b, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			b = append(b, '\\', '"')
		case c == '\\':
			b = append(b, '\\', '\\')
		case c == '\b':
			b = append(b, '\\', 'b')
		case c == '\f':
			b = append(b, '\\', 'f')
		case c == '\n':
			b = append(b, '\\', 'n')
		case c == '\r':
			b = append(b, '\\', 'r')
		case c == '\t':
			b = append(b, '\\', 't')
		case c < 0x20:
			b = append(b, '\\', 'u', '0', '0', protoHex[c>>4], protoHex[c&15])
		default:
			b = append(b, c)
		}
	}
	return append(b, '"')
}

func protoPutF64s(b []byte, v []float64) []byte {
	b = append(b, '[')
	for i, f := range v {
		if i > 0 {
			b = append(b, ',')
		}
		b = protoPutF64(b, f)
	}
	return append(b, ']')
}

// protoPutF64 writes the shortest round-trip form: "1.0", "0.25", "1e+16", "1.5e-7"; null for NaN/inf.
func protoPutF64(b []byte, f float64) []byte {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return append(b, "null"...)
	}
	if f == 0 {
		if math.Signbit(f) {
			return append(b, "-0.0"...)
		}
		return append(b, "0.0"...)
	}
	var tmp [32]byte
	e := strconv.AppendFloat(tmp[:0], f, 'e', -1, 64)
	if e[0] == '-' {
		b = append(b, '-')
		e = e[1:]
	}
	ep := bytes.IndexByte(e, 'e')
	var digits [24]byte
	nd := 0
	for _, c := range e[:ep] {
		if c != '.' {
			digits[nd] = c
			nd++
		}
	}
	x, _ := strconv.Atoi(string(e[ep+1:]))
	length := nd
	kk := x + 1
	k := kk - length
	d := digits[:nd]
	switch {
	case 0 <= k && kk <= 16:
		b = append(b, d...)
		for i := length; i < kk; i++ {
			b = append(b, '0')
		}
		b = append(b, '.', '0')
	case 0 < kk && kk <= 16:
		b = append(b, d[:kk]...)
		b = append(b, '.')
		b = append(b, d[kk:]...)
	case -5 < kk && kk <= 0:
		b = append(b, '0', '.')
		for i := 0; i < -kk; i++ {
			b = append(b, '0')
		}
		b = append(b, d...)
	default:
		b = append(b, d[0])
		if length > 1 {
			b = append(b, '.')
			b = append(b, d[1:]...)
		}
		b = append(b, 'e')
		if kk-1 >= 0 {
			b = append(b, '+')
		}
		b = strconv.AppendInt(b, int64(kk-1), 10)
	}
	return b
}
