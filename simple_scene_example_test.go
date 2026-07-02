package exampleviz

import (
	"testing"

	"go.viam.com/rdk/resource"

	"github.com/viam-labs/viam-viz-helpers-go"
)

// SimpleScene exercises the library's default geometry / base-geom
// path: it implements NO SceneHooks methods. Only SceneTicker is on
// the type; every other capability falls through to the built-in
// defaults (visuals.BuildBasicGeometry + visuals.DefaultBaseGeomForItem).
func TestSimpleSceneExample_OmitsOptionalInterfaces(t *testing.T) {
	s := &simpleScene{}
	var hooks visuals.SceneHooks = s
	if _, ok := hooks.(visuals.GeometryBuilder); ok {
		t.Error("simpleScene should NOT implement GeometryBuilder (uses library default)")
	}
	if _, ok := hooks.(visuals.BaseGeomProvider); ok {
		t.Error("simpleScene should NOT implement BaseGeomProvider (uses library default)")
	}
	if _, ok := hooks.(visuals.AssetReader); ok {
		t.Error("simpleScene should NOT implement AssetReader")
	}
	if _, ok := hooks.(visuals.PresetLoader); ok {
		t.Error("simpleScene should NOT implement PresetLoader")
	}
	if _, ok := hooks.(visuals.CustomCommandHandler); ok {
		t.Error("simpleScene should NOT implement CustomCommandHandler")
	}
	if _, ok := hooks.(visuals.LegacyAnimator); ok {
		t.Error("simpleScene should NOT implement LegacyAnimator")
	}
	if _, ok := hooks.(visuals.SceneTicker); !ok {
		t.Error("simpleScene should implement SceneTicker")
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

func TestSimpleSceneExample_SceneTickEmitsMetadataPathsForMovingBox(t *testing.T) {
	// Moving box mutates color + opacity every step in addition to
	// pose + dims. Scene emits a single UPDATED with the full
	// field-mask path list — pose subfields + physicalObject dims +
	// metadata.colors + metadata.opacities. (Pre-fix behavior was
	// an empty-Paths respawn signal; the viewer fix removed the
	// need for that escalation.)
	s := bareScene(t)
	events := s.SceneTick(s.Scene, 0.5)
	var movingEvent *visuals.SceneEvent
	for i := range events {
		if events[i].Label == "moving_box" {
			movingEvent = &events[i]
			break
		}
	}
	if movingEvent == nil {
		t.Fatal("expected an event for moving_box")
	}
	if movingEvent.Kind != visuals.EventUpdated {
		t.Errorf("expected EventUpdated, got %v", movingEvent.Kind)
	}
	pathsSeen := map[string]bool{}
	for _, p := range movingEvent.Paths {
		pathsSeen[p] = true
	}
	for _, required := range []string{"metadata.colors", "metadata.opacities"} {
		if !pathsSeen[required] {
			t.Errorf("expected %q in Paths, got %v", required, movingEvent.Paths)
		}
	}
}

// SimpleScene relies on the library's DefaultBaseGeomForItem; this
// test pins that the default still extracts the standard primitive
// shape fields. (Lives next to simpleScene because the library
// guarantee is what this service depends on.)
func TestSimpleSceneExample_DefaultBaseGeomExtractsShapeFields(t *testing.T) {
	box := visuals.Box{
		Label:  "b",
		DimsMM: visuals.BoxDims{X: 1, Y: 2, Z: 3},
	}.ToItem()
	bg := visuals.DefaultBaseGeomForItem(box)
	if !bg.HasDims || bg.Dims.X != 1 || bg.Dims.Y != 2 || bg.Dims.Z != 3 {
		t.Errorf("DefaultBaseGeomForItem(box) should populate Dims, got %+v", bg)
	}
	sphere := visuals.Sphere{Label: "s", RadiusMM: 42}.ToItem()
	if got := visuals.DefaultBaseGeomForItem(sphere).RadiusMM; got != 42 {
		t.Errorf("DefaultBaseGeomForItem(sphere).RadiusMM = %v, want 42", got)
	}
	capsule := visuals.Capsule{Label: "c", RadiusMM: 5, LengthMM: 50}.ToItem()
	bg = visuals.DefaultBaseGeomForItem(capsule)
	if bg.RadiusMM != 5 || bg.LengthMM != 50 {
		t.Errorf("DefaultBaseGeomForItem(capsule) = %+v, want RadiusMM=5 LengthMM=50", bg)
	}
}
