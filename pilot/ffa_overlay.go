package pilot

import (
	"bytes"
	"encoding/json"
	"os"
	"unicode/utf8"
)

// paramsFfaJSON holds the settings that replace the playing settings once a free-for-all is
// detected (same keys as paramsShippedJSON; only the keys that differ are needed): retreat to
// break line of sight at moderate HP, close in on laser carriers when we have none, prefer
// hurt or already-engaged targets, bank HP late, plating first, take drops when no fight is live.
// Empty means no free-for-all settings: detection stays off and play is unchanged.
const paramsFfaJSON = `{"retreat_hp":0.5,"retreat_multi_hp":0.75,"retreat_press_hp":0.8,"retreat_exit_hp":0.85,"flee_dist":30,"ffa_close_laser":1,"ffa_close_range":30,"ffa_opp_bonus":60,"ffa_opp_hp":0.5,"ffa_bank_s":270.0,"ffa_bank_hp":0.3,"ffa_bank_slack":20,"dp_on":1,"upgrade_priority":["plating","armor","nanites","gun_damage","gun_rate","laser_damage","laser_charge","lightning_efficiency","servos","gyros","gun_velocity","gun_mag","reactor","capacitor","jets","lightning_chain","lightning_range","mortar_reload","mortar_splash","mortar_velocity","sensors","laser_pierce"]}`

// paramsLoadFfa returns the free-for-all settings (playing settings with the overlay applied),
// or nil when there is no overlay. MECH_FFA_PARAMS names an overlay file that replaces the
// built-in one (for testing).
func paramsLoadFfa() *Params {
	over := []byte(paramsFfaJSON)
	if path, ok := os.LookupEnv("MECH_FFA_PARAMS"); ok && utf8.ValidString(path) {
		if text, err := os.ReadFile(path); err == nil {
			over = text
		}
	}
	if len(bytes.TrimSpace(over)) == 0 {
		return nil
	}
	base := paramsShipped
	if path, ok := os.LookupEnv("MECH_PARAMS"); ok && utf8.ValidString(path) {
		if text, err := os.ReadFile(path); err == nil {
			base = text
		}
	}
	var bm, om map[string]any
	if json.Unmarshal(base, &bm) != nil || bm == nil {
		bm = map[string]any{}
	}
	if err := json.Unmarshal(over, &om); err != nil {
		logf("MECH_FFA_PARAMS ignored: %v", err)
		return nil
	}
	for k, v := range om {
		bm[k] = v
	}
	merged, err := json.Marshal(bm)
	if err != nil {
		return nil
	}
	p, err := paramsFromJSON(merged)
	if err != nil {
		logf("MECH_FFA_PARAMS ignored: %v", err)
		return nil
	}
	return &p
}
