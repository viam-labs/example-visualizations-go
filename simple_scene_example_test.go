package exampleviz

import (
	"testing"

	"exampleviz/visuals"
)

// Smoke test for SimpleSceneExample: confirm the hardcoded scene
// has the expected three labels and the basic-geometry dispatch
// works for each.
func TestSimpleSceneExample_BuildGeometryForEachPrimitive(t *testing.T) {
	tests := []struct {
		name string
		item visuals.Item
	}{
		{"box", visuals.Box{
			Label:  "demo_box",
			DimsMM: visuals.BoxDims{X: 150, Y: 150, Z: 150},
		}.ToItem()},
		{"sphere", visuals.Sphere{
			Label:    "demo_sphere",
			RadiusMM: 90,
		}.ToItem()},
		{"capsule", visuals.Capsule{
			Label:    "demo_capsule",
			RadiusMM: 50,
			LengthMM: 200,
		}.ToItem()},
	}
	s := &simpleScene{}
	for _, tc := range tests {
		geom, err := s.BuildGeometry(tc.item, visuals.BaseGeom{})
		if err != nil {
			t.Errorf("%s: BuildGeometry returned error: %v", tc.name, err)
			continue
		}
		if geom == nil {
			t.Errorf("%s: BuildGeometry returned nil geometry", tc.name)
		}
		if geom.Label != tc.item.Label {
			t.Errorf("%s: label mismatch: got %q want %q", tc.name, geom.Label, tc.item.Label)
		}
	}
}

func TestSimpleSceneExample_ReadAssetReturnsError(t *testing.T) {
	s := &simpleScene{}
	if _, err := s.ReadAsset("anything"); err == nil {
		t.Error("expected ReadAsset to return an error (no assets supported)")
	}
}

// Confirm the BasicSceneHooks defaults pass through transparently:
// no animation, no presets, no custom commands.
func TestSimpleSceneExample_DefaultsFromBasicHooks(t *testing.T) {
	s := &simpleScene{}
	if s.IsAnimated(visuals.Item{}) {
		t.Error("IsAnimated should return false by default")
	}
	if _, err := s.LoadPreset("anything"); err == nil {
		t.Error("LoadPreset should return an error by default")
	}
	if _, handled, err := s.HandleCustomCommand(nil, nil); handled || err != nil {
		t.Errorf("HandleCustomCommand default should be (nil, false, nil), got handled=%v err=%v", handled, err)
	}
}
