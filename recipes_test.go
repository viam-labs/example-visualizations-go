package exampleviz

import (
	"math"
	"sort"
	"testing"

	"exampleviz/visuals"
)

// ---- registry ---------------------------------------------------------

func TestRecipes_ContainsAllFour(t *testing.T) {
	want := []string{
		"all_primitives", "detections_overlay",
		"marching_boxes", "pulsing_spheres",
	}
	got := make([]string, 0, len(Recipes))
	for k := range Recipes {
		got = append(got, k)
	}
	sort.Strings(got)
	if !sliceEq(got, want) {
		t.Errorf("recipes = %v, want %v", got, want)
	}
}

func TestRecipes_NamesMatchRegistryKeys(t *testing.T) {
	for name, r := range Recipes {
		if r.Name() != name {
			t.Errorf("registry key %q maps to recipe.Name() %q", name, r.Name())
		}
	}
}

// ---- all_primitives ---------------------------------------------------

func TestAllPrimitives_InitialAddsOneOfEachShape(t *testing.T) {
	scene := visuals.NewScene("world")
	events := (AllPrimitives{}).Initial(scene)
	if len(events) != 7 {
		t.Fatalf("expected 7 events, got %d", len(events))
	}
	for _, e := range events {
		if e.Kind != visuals.EventAdded {
			t.Errorf("expected all ADDED, got kind %q", e.Kind)
		}
	}
	types := map[string]bool{}
	for _, e := range events {
		types[e.Item.Type] = true
	}
	want := map[string]bool{
		"box": true, "sphere": true, "capsule": true, "point": true,
		"arrow": true, "mesh": true, "pointcloud": true,
	}
	for k := range want {
		if !types[k] {
			t.Errorf("missing shape type %q in initial events", k)
		}
	}
}

func TestAllPrimitives_LabelsUniqueAndPrefixed(t *testing.T) {
	scene := visuals.NewScene("world")
	(AllPrimitives{}).Initial(scene)
	got := scene.Labels()
	want := []string{
		"demo_arrow", "demo_box", "demo_bunny", "demo_capsule",
		"demo_pcd", "demo_point", "demo_sphere",
	}
	sort.Strings(got)
	if !sliceEq(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

func TestAllPrimitives_TickIsStatic(t *testing.T) {
	scene := visuals.NewScene("world")
	(AllPrimitives{}).Initial(scene)
	ap := AllPrimitives{}
	for _, tt := range []float64{0.0, 1.5, 100.0} {
		if got := ap.Tick(scene, tt); len(got) != 0 {
			t.Errorf("expected 0 events at t=%v, got %d", tt, len(got))
		}
	}
}

func TestAllPrimitives_MeshCarriesAssetPath(t *testing.T) {
	scene := visuals.NewScene("world")
	events := (AllPrimitives{}).Initial(scene)
	for _, e := range events {
		if e.Item.Type == "mesh" {
			if e.Item.MeshPath != "assets/bunny.stl" {
				t.Errorf("mesh path = %q, want assets/bunny.stl", e.Item.MeshPath)
			}
			return
		}
	}
	t.Error("no mesh event found")
}

func TestAllPrimitives_PointCloudCarriesAssetPath(t *testing.T) {
	scene := visuals.NewScene("world")
	events := (AllPrimitives{}).Initial(scene)
	for _, e := range events {
		if e.Item.Type == "pointcloud" {
			if e.Item.PointcloudPath != "assets/helix.pcd" {
				t.Errorf("pcd path = %q, want assets/helix.pcd", e.Item.PointcloudPath)
			}
			return
		}
	}
	t.Error("no pointcloud event found")
}

// ---- detections_overlay ----------------------------------------------

func TestDetectionsOverlay_InitialIsEmpty(t *testing.T) {
	scene := visuals.NewScene("world")
	if got := (DetectionsOverlay{}).Initial(scene); len(got) != 0 {
		t.Errorf("expected 0 initial events, got %d", len(got))
	}
	if scene.Len() != 0 {
		t.Errorf("expected empty scene, got %d items", scene.Len())
	}
}

func TestDetectionsOverlay_FirstTickAddsNDetections(t *testing.T) {
	scene := visuals.NewScene("world")
	(DetectionsOverlay{}).Initial(scene)
	events := (DetectionsOverlay{}).Tick(scene, 0.5)
	if len(events) != doDetections {
		t.Errorf("expected %d events, got %d", doDetections, len(events))
	}
	for _, e := range events {
		if e.Kind != visuals.EventAdded {
			t.Errorf("expected ADDED, got %q", e.Kind)
		}
	}
	if scene.Len() != doDetections {
		t.Errorf("scene size = %d, want %d", scene.Len(), doDetections)
	}
}

func TestDetectionsOverlay_SubsequentTickUpdatesInPlace(t *testing.T) {
	scene := visuals.NewScene("world")
	(DetectionsOverlay{}).Tick(scene, 0.0)
	events := (DetectionsOverlay{}).Tick(scene, 0.5)
	if len(events) != doDetections {
		t.Errorf("expected %d UPDATED events, got %d", doDetections, len(events))
	}
	for _, e := range events {
		if e.Kind != visuals.EventUpdated {
			t.Errorf("expected UPDATED, got %q", e.Kind)
		}
		hasPosePath := false
		for _, p := range e.Paths {
			if len(p) > 21 && p[:21] == "poseInObserverFrame.p" {
				hasPosePath = true
				break
			}
		}
		if !hasPosePath {
			t.Errorf("expected at least one pose path, got %v", e.Paths)
		}
	}
}

func TestDetectionsOverlay_NoEventWhenPoseUnchanged(t *testing.T) {
	scene := visuals.NewScene("world")
	(DetectionsOverlay{}).Tick(scene, 0.5)
	got := (DetectionsOverlay{}).Tick(scene, 0.5)
	if len(got) != 0 {
		t.Errorf("expected 0 events on repeated t, got %d", len(got))
	}
}

func TestDetectionsOverlay_LabelsAreSequential(t *testing.T) {
	scene := visuals.NewScene("world")
	(DetectionsOverlay{}).Tick(scene, 0.0)
	got := scene.Labels()
	want := make([]string, doDetections)
	for i := 0; i < doDetections; i++ {
		want[i] = detectionLabel(i)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !sliceEq(got, want) {
		t.Errorf("labels = %v, want %v", got, want)
	}
}

func TestDetectionsOverlay_OrbitCirclesOrigin(t *testing.T) {
	// At t = T/4 with phase i=0 (=0), angle = π/2 → (0, +R).
	scene := visuals.NewScene("world")
	(DetectionsOverlay{}).Tick(scene, doOrbitPeriodS*0.25)
	v := scene.Get("det_0")
	if v == nil {
		t.Fatal("det_0 not in scene")
	}
	// BoundingBox (solid variant) expands into a single Box;
	// composites return Box as a value (vs. *Box for direct
	// driver-side usage), so the scene stores it by value here.
	b, ok := v.(visuals.Box)
	if !ok {
		t.Fatalf("scene.Get returned unexpected type %T", v)
	}
	if math.Abs(b.Pose.X) > 1.0 {
		t.Errorf("at T/4 expected x≈0, got %v", b.Pose.X)
	}
	if b.Pose.Y < doOrbitRadiusMM*0.99 {
		t.Errorf("at T/4 expected y≈+R, got %v (R=%v)", b.Pose.Y, doOrbitRadiusMM)
	}
}

// ---- helper ----------------------------------------------------------

func sliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
