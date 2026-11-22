package pilot

import (
	"bytes"
	"encoding/json"
	"os"
	"unicode/utf8"
)

// paramsFfaJSON holds the settings that replace the playing settings once a free-for-all is
// detected (same keys as paramsShippedJSON; only the keys that differ are needed).
// Empty means no free-for-all settings: detection stays off and play is unchanged.
const paramsFfaJSON = ``

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
