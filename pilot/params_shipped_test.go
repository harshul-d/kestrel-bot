package pilot

import "testing"

// The shipped settings must parse; a broken document would silently fall back to defaults.
func TestShippedParamsParse(t *testing.T) {
	if _, err := paramsFromJSON(paramsShipped); err != nil {
		t.Fatalf("shipped params: %v", err)
	}
	if paramsLoadFfa() == nil {
		t.Fatal("free-for-all overlay failed to load")
	}
}
