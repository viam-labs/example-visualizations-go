package exampleviz

import (
	"context"
	"testing"

	"go.viam.com/rdk/resource"

	"exampleviz/visuals"
)

// Smoke test: BuildGeometry returns a non-nil geometry for each
// primitive type in the hardcoded scene.
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

// The remaining hooks are all trivial — verify each returns the
// expected default for a static, asset-free, no-preset scene.

func TestSimpleSceneExample_ReadAssetReturnsError(t *testing.T) {
	s := &simpleScene{}
	if _, err := s.ReadAsset("anything"); err == nil {
		t.Error("expected ReadAsset to return an error (no assets supported)")
	}
}

func TestSimpleSceneExample_ComputeTickReturnsBasePose(t *testing.T) {
	s := &simpleScene{}
	base := visuals.PoseAt(10, 20, 30, 0, 0, 1, 45)
	result := s.ComputeTick(visuals.Item{}, base, visuals.BaseGeom{}, 0.5)
	if result.Pose != base {
		t.Errorf("ComputeTick should return base pose unchanged, got %+v", result.Pose)
	}
}

func TestSimpleSceneExample_IsAnimatedAlwaysFalse(t *testing.T) {
	s := &simpleScene{}
	if s.IsAnimated(visuals.Item{}) {
		t.Error("IsAnimated should always return false")
	}
}

func TestSimpleSceneExample_LoadPresetAlwaysErrors(t *testing.T) {
	s := &simpleScene{}
	if _, err := s.LoadPreset("anything"); err == nil {
		t.Error("LoadPreset should return an error (no presets)")
	}
}

func TestSimpleSceneExample_HandleCustomCommandFallsThrough(t *testing.T) {
	s := &simpleScene{}
	resp, handled, err := s.HandleCustomCommand(context.Background(), map[string]any{"command": "x"})
	if resp != nil || handled || err != nil {
		t.Errorf("HandleCustomCommand default should be (nil, false, nil), got (%v, %v, %v)",
			resp, handled, err)
	}
}

// ---- SceneTick (the new per-frame animation API) ---------------------

// bareScene constructs a simpleScene with hooks wired and the scene
// populated via Reconfigure, ready for SceneTick exercise without a
// real framework Config.
func bareScene(t *testing.T) *simpleScene {
	t.Helper()
	s := &simpleScene{}
	s.SceneServiceBase.Hooks = s
	if err := s.Reconfigure(nil, nil, resource.Config{}); err != nil {
		t.Fatalf("Reconfigure: %v", err)
	}
	return s
}

func TestSimpleSceneExample_SceneTickReturnsEventsForMovingBox(t *testing.T) {
	s := bareScene(t)
	events := s.SceneTick(s.Scene, 0.5)
	if len(events) < 1 {
		t.Fatalf("expected at least one SceneTick event, got %d", len(events))
	}
	if events[0].Label != "moving_box" {
		t.Errorf("expected event for moving_box, got %q", events[0].Label)
	}
	if events[0].Kind != visuals.EventUpdated {
		t.Errorf("expected EventUpdated, got %v", events[0].Kind)
	}
}

func TestSimpleSceneExample_SceneTickEmitsPoseAndDimsPaths(t *testing.T) {
	s := bareScene(t)
	events := s.SceneTick(s.Scene, 0.5)
	paths := events[0].Paths
	hasPose := false
	hasDims := false
	for _, p := range paths {
		if len(p) >= 19 && p[:19] == "poseInObserverFrame" {
			hasPose = true
		}
		if len(p) >= 14 && p[:14] == "physicalObject" {
			hasDims = true
		}
	}
	if !hasPose {
		t.Errorf("expected at least one pose path; got %v", paths)
	}
	if !hasDims {
		t.Errorf("expected at least one physicalObject path; got %v", paths)
	}
}

func TestSimpleSceneExample_BaseGeomForItemExtractsShapeFields(t *testing.T) {
	s := &simpleScene{}
	box := visuals.Box{
		Label:  "b",
		DimsMM: visuals.BoxDims{X: 1, Y: 2, Z: 3},
	}.ToItem()
	bg := s.BaseGeomForItem(box)
	if !bg.HasDims || bg.Dims.X != 1 || bg.Dims.Y != 2 || bg.Dims.Z != 3 {
		t.Errorf("BaseGeomForItem(box) should populate Dims, got %+v", bg)
	}
	sphere := visuals.Sphere{Label: "s", RadiusMM: 42}.ToItem()
	if got := s.BaseGeomForItem(sphere).RadiusMM; got != 42 {
		t.Errorf("BaseGeomForItem(sphere).RadiusMM = %v, want 42", got)
	}
	capsule := visuals.Capsule{Label: "c", RadiusMM: 5, LengthMM: 50}.ToItem()
	bg = s.BaseGeomForItem(capsule)
	if bg.RadiusMM != 5 || bg.LengthMM != 50 {
		t.Errorf("BaseGeomForItem(capsule) = %+v, want RadiusMM=5 LengthMM=50", bg)
	}
}
