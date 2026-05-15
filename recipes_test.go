package exampleviz

import (
	"math"
	"sort"
	"strings"
	"testing"

	"exampleviz/visuals"
)

// ---- registry ---------------------------------------------------------

func TestRecipes_ContainsAllSeven(t *testing.T) {
	want := []string{
		"all_primitives", "coordinate_frames_arm", "detections_overlay",
		"lifecycle_garden", "marching_boxes", "pulsing_spheres",
		"trajectory_runner",
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

// ---- coordinate_frames_arm -------------------------------------------

func TestCoordinateFramesArm_InitialInstallsFramesAndArm(t *testing.T) {
	scene := visuals.NewScene("world")
	events := CoordinateFramesArm{}.Initial(scene)
	// 3 frames × 4 visuals + 5 arm parts = 17.
	if len(events) != 17 {
		t.Errorf("expected 17 events, got %d", len(events))
	}
	for i := 0; i < 3; i++ {
		for _, suffix := range []string{"", "_axis_x", "_axis_y", "_axis_z"} {
			label := "frame_" + string(rune('0'+i)) + suffix
			if scene.Get(label) == nil {
				t.Errorf("missing %q in scene", label)
			}
		}
	}
	for _, part := range []string{"arm_shoulder", "arm_upper", "arm_elbow", "arm_forearm", "arm_wrist"} {
		if scene.Get(part) == nil {
			t.Errorf("missing %q in scene", part)
		}
	}
}

func TestCoordinateFramesArm_TickDrivesAnchorsAndJoints(t *testing.T) {
	scene := visuals.NewScene("world")
	CoordinateFramesArm{}.Initial(scene)
	events := CoordinateFramesArm{}.Tick(scene, 0.5)
	// 3 frame anchors + 3 arm joints (shoulder/elbow/wrist) = 6.
	if len(events) != 6 {
		t.Errorf("expected 6 UPDATED events, got %d", len(events))
	}
	for _, e := range events {
		if e.Kind != visuals.EventUpdated {
			t.Errorf("expected UPDATED, got %q", e.Kind)
		}
	}
}

func TestCoordinateFramesArm_NoEventWhenTUnchanged(t *testing.T) {
	scene := visuals.NewScene("world")
	CoordinateFramesArm{}.Initial(scene)
	cf := CoordinateFramesArm{}
	cf.Tick(scene, 0.5)
	if got := cf.Tick(scene, 0.5); len(got) != 0 {
		t.Errorf("expected 0 events on repeated t, got %d", len(got))
	}
}

// ---- trajectory_runner -----------------------------------------------

func TestTrajectoryRunner_InitialInstallsPathAndRunner(t *testing.T) {
	scene := visuals.NewScene("world")
	events := TrajectoryRunner{}.Initial(scene)
	// 5 waypoints + 4 line segments + 1 runner = 10.
	if len(events) != 10 {
		t.Errorf("expected 10 events, got %d", len(events))
	}
	if scene.Get("trajectory_runner") == nil {
		t.Error("trajectory_runner missing")
	}
}

func TestTrajectoryRunner_StartsAtFirstWaypointAtTZero(t *testing.T) {
	scene := visuals.NewScene("world")
	TrajectoryRunner{}.Initial(scene)
	TrajectoryRunner{}.Tick(scene, 0.0)
	v := scene.Get("trajectory_runner")
	runner, ok := v.(*visuals.Sphere)
	if !ok {
		t.Fatalf("runner is wrong type: %T", v)
	}
	wp0 := trWaypoints[0]
	if math.Abs(runner.Pose.X-wp0.X) > 1e-6 ||
		math.Abs(runner.Pose.Y-wp0.Y) > 1e-6 ||
		math.Abs(runner.Pose.Z-wp0.Z) > 1e-6 {
		t.Errorf("runner not at wp0: got pose=%v, wp0=%v", runner.Pose, wp0)
	}
}

func TestTrajectoryRunner_OnlyRunnerMovesDuringTick(t *testing.T) {
	scene := visuals.NewScene("world")
	TrajectoryRunner{}.Initial(scene)
	events := TrajectoryRunner{}.Tick(scene, 1.0)
	if len(events) != 1 {
		t.Errorf("expected 1 event (runner only), got %d", len(events))
	}
	if len(events) > 0 && events[0].Label != "trajectory_runner" {
		t.Errorf("expected runner update, got label %q", events[0].Label)
	}
}

// ---- lifecycle_garden ------------------------------------------------

func TestLifecycleGarden_InitialIsEmpty(t *testing.T) {
	scene := visuals.NewScene("world")
	lg := &LifecycleGarden{}
	if got := lg.Initial(scene); len(got) != 0 {
		t.Errorf("expected 0 initial events, got %d", len(got))
	}
	if scene.Len() != 0 {
		t.Errorf("scene should be empty, got %d items", scene.Len())
	}
}

func TestLifecycleGarden_FirstTickAddsActivePlots(t *testing.T) {
	scene := visuals.NewScene("world")
	lg := &LifecycleGarden{}
	events := lg.Tick(scene, 0.0)
	added := 0
	for _, e := range events {
		if e.Kind == visuals.EventAdded {
			added++
		}
	}
	if added < 3 {
		t.Errorf("expected at least 3 plots to ADD, got %d (events=%+v)", added, events)
	}
}

func TestLifecycleGarden_LabelVersionsBumpAcrossCycles(t *testing.T) {
	scene := visuals.NewScene("world")
	lg := &LifecycleGarden{}
	lg.Tick(scene, 0.1)
	v1 := lg.version[0]
	// Walk through the gone phase explicitly, then back into appear.
	goneT := lgAppearS + lgAliveS + lgDisappearS + 0.1
	lg.Tick(scene, goneT)
	lg.Tick(scene, lgCycleS+0.1)
	if lg.version[0] <= v1 {
		t.Errorf("expected version bump on plot 0; got %d after %d", lg.version[0], v1)
	}
}

func TestLifecycleGarden_ReAddUsesFreshLabel(t *testing.T) {
	scene := visuals.NewScene("world")
	lg := &LifecycleGarden{}
	lg.Tick(scene, 0.1)
	firstLabels := []string{}
	for _, lab := range scene.Labels() {
		if strings.HasPrefix(lab, "garden_0_v") {
			firstLabels = append(firstLabels, lab)
		}
	}
	goneT := lgAppearS + lgAliveS + lgDisappearS + 0.1
	lg.Tick(scene, goneT)
	lg.Tick(scene, lgCycleS+0.1)
	for _, lab := range scene.Labels() {
		if !strings.HasPrefix(lab, "garden_0_v") {
			continue
		}
		for _, prior := range firstLabels {
			if lab == prior {
				t.Errorf("reused stale label %q after cycle", lab)
			}
		}
	}
}

func TestLifecycleGarden_ColorChangesThroughPhases(t *testing.T) {
	scene := visuals.NewScene("world")
	lg := &LifecycleGarden{}
	lg.Tick(scene, 0.0) // plot 0: appear phase
	var firstColor *visuals.Color
	for _, lab := range scene.Labels() {
		if strings.HasPrefix(lab, "garden_0_v") {
			if b, ok := scene.Get(lab).(*visuals.Box); ok {
				firstColor = b.Color
				break
			}
		}
	}
	if firstColor == nil || *firstColor != lgColorAppear {
		t.Errorf("expected appear color %v, got %v", lgColorAppear, firstColor)
	}
	// Advance into the alive phase.
	lg.Tick(scene, lgAppearS+0.1)
	var aliveColor *visuals.Color
	for _, lab := range scene.Labels() {
		if strings.HasPrefix(lab, "garden_0_v") {
			if b, ok := scene.Get(lab).(*visuals.Box); ok {
				aliveColor = b.Color
				break
			}
		}
	}
	if aliveColor == nil || *aliveColor != lgColorAlive {
		t.Errorf("expected alive color %v, got %v", lgColorAlive, aliveColor)
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
