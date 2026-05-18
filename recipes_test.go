package exampleviz

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"exampleviz/visuals"
)

// ---- registry ---------------------------------------------------------

func TestRecipes_ContainsAllTen(t *testing.T) {
	want := []string{
		"all", "all_primitives", "breathing_shapes",
		"coordinate_frames_arm", "detections_overlay", "force_vector",
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

func TestTrajectoryRunner_InitialInstallsPlanAndRunner(t *testing.T) {
	scene := visuals.NewScene("world")
	events := TrajectoryRunner{}.Initial(scene)
	// TrajectoryPlan expands: 4 line segments + 5 CoordinateFrames
	// (each = anchor + 3 axes) = 4 + 20 = 24. Plus runner sphere = 25.
	if len(events) != 25 {
		t.Errorf("expected 25 events, got %d", len(events))
	}
	if scene.Get("trajectory_runner") == nil {
		t.Error("trajectory_runner missing")
	}
	// Each waypoint should be a full CoordinateFrame triad.
	for i := 0; i < 5; i++ {
		anchor := fmt.Sprintf("trajectory_wp_%d", i)
		if scene.Get(anchor) == nil {
			t.Errorf("missing %q", anchor)
		}
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

// ---- trajectory_runner orientation -----------------------------------

func TestTrajectoryRunner_TickEmitsOrientationPaths(t *testing.T) {
	scene := visuals.NewScene("world")
	TrajectoryRunner{}.Initial(scene)
	events := TrajectoryRunner{}.Tick(scene, 0.5)
	hasOrientPath := false
	for _, p := range events[0].Paths {
		if strings.HasPrefix(p, "poseInObserverFrame.pose.o") {
			hasOrientPath = true
			break
		}
	}
	if !hasOrientPath {
		t.Errorf("expected orientation path in tick output: %v", events[0].Paths)
	}
}

func TestTrajectoryRunner_OrientationLerpsFromWaypoints(t *testing.T) {
	scene := visuals.NewScene("world")
	tr := TrajectoryRunner{}
	tr.Initial(scene)
	tr.Tick(scene, 0.0)
	v := scene.Get("trajectory_runner")
	runner, ok := v.(*visuals.Sphere)
	if !ok {
		t.Fatalf("runner is %T", v)
	}
	wp0 := tr.waypoints()[0]
	// At t=0 the runner should exactly match wp 0's orientation.
	if math.Abs(runner.Pose.OX-wp0.OX) > 1e-6 ||
		math.Abs(runner.Pose.OY-wp0.OY) > 1e-6 ||
		math.Abs(runner.Pose.OZ-wp0.OZ) > 1e-6 ||
		math.Abs(runner.Pose.Theta-wp0.Theta) > 1e-6 {
		t.Errorf("runner orient @ t=0 should match wp0: got (%v,%v,%v,θ=%v) want (%v,%v,%v,θ=%v)",
			runner.Pose.OX, runner.Pose.OY, runner.Pose.OZ, runner.Pose.Theta,
			wp0.OX, wp0.OY, wp0.OZ, wp0.Theta)
	}
}

// ---- force_vector recipe ---------------------------------------------

func TestForceVector_InitialInstallsArrow(t *testing.T) {
	scene := visuals.NewScene("world")
	events := ForceVectorRecipe{}.Initial(scene)
	if len(events) != 1 || events[0].Item.Type != "arrow" {
		t.Errorf("expected 1 arrow ADDED, got %+v", events)
	}
}

func TestForceVector_TickEmitsLengthRadiusOrientationPaths(t *testing.T) {
	scene := visuals.NewScene("world")
	ForceVectorRecipe{}.Initial(scene)
	events := ForceVectorRecipe{}.Tick(scene, 1.0)
	paths := strings.Join(events[0].Paths, " ")
	if !strings.Contains(paths, "lengthMm") {
		t.Errorf("missing length path: %v", events[0].Paths)
	}
	if !strings.Contains(paths, "radiusMm") {
		t.Errorf("missing radius path: %v", events[0].Paths)
	}
	if !strings.Contains(paths, "poseInObserverFrame.pose.o") {
		t.Errorf("missing orientation path: %v", events[0].Paths)
	}
}

// ---- breathing_shapes ------------------------------------------------

func TestBreathingShapes_InitialIsEmpty(t *testing.T) {
	scene := visuals.NewScene("world")
	bs := &BreathingShapes{}
	if got := bs.Initial(scene); len(got) != 0 {
		t.Errorf("expected empty initial, got %d events", len(got))
	}
}

func TestBreathingShapes_FirstTickAddsShapes(t *testing.T) {
	scene := visuals.NewScene("world")
	bs := &BreathingShapes{}
	events := bs.Tick(scene, 0.0)
	added := 0
	for _, e := range events {
		if e.Kind == visuals.EventAdded {
			added++
		}
	}
	if added != bsN {
		t.Errorf("expected %d ADDs on first tick, got %d", bsN, added)
	}
}

func TestBreathingShapes_StepChangeRotatesLabels(t *testing.T) {
	scene := visuals.NewScene("world")
	bs := &BreathingShapes{}
	bs.Tick(scene, 0.0)
	stepDt := bsPeriodS / float64(bsStepsPerPeriod)
	events := bs.Tick(scene, stepDt*1.01)
	added := 0
	removed := 0
	for _, e := range events {
		switch e.Kind {
		case visuals.EventAdded:
			added++
		case visuals.EventRemoved:
			removed++
		}
	}
	if added == 0 || added != removed {
		t.Errorf("expected matched ADD/REMOVE pairs, got %d ADD %d REMOVE",
			added, removed)
	}
}

// ---- all recipe includes new recipes ---------------------------------

func TestAllRecipe_IncludesForceVectorAndBreathing(t *testing.T) {
	ar := newAllRecipe()
	hasFV, hasBS := false, false
	for _, sub := range ar.subs {
		switch sub.(type) {
		case ForceVectorRecipe:
			hasFV = true
		case *BreathingShapes:
			hasBS = true
		}
	}
	if !hasFV {
		t.Error("AllRecipe missing ForceVectorRecipe")
	}
	if !hasBS {
		t.Error("AllRecipe missing BreathingShapes")
	}
}

// ---- y_origin parameter ----------------------------------------------

func TestYOrigin_ShiftsMarchingBoxes(t *testing.T) {
	scene := visuals.NewScene("world")
	MarchingBoxes{YOrigin: -1500}.Initial(scene)
	v := scene.Get("march_0")
	box, ok := v.(*visuals.Box)
	if !ok {
		t.Fatalf("expected *Box, got %T", v)
	}
	if box.Pose.Y != -1500 {
		t.Errorf("expected y=-1500, got %v", box.Pose.Y)
	}
}

func TestYOrigin_ShiftsCoordinateFramesArm(t *testing.T) {
	scene := visuals.NewScene("world")
	CoordinateFramesArm{YOrigin: 1000}.Initial(scene)
	// Frame anchor at YOrigin + cfFrameY (= 600).
	frame := scene.Get("frame_0")
	if frame == nil {
		t.Fatal("frame_0 missing")
	}
	// Composite-stored as value-typed Sphere.
	if sphere, ok := frame.(visuals.Sphere); ok {
		if sphere.Pose.Y != 1600 {
			t.Errorf("frame_0.y = %v, want 1600", sphere.Pose.Y)
		}
	} else if psphere, ok := frame.(*visuals.Sphere); ok {
		if psphere.Pose.Y != 1600 {
			t.Errorf("frame_0.y = %v, want 1600", psphere.Pose.Y)
		}
	} else {
		t.Fatalf("frame_0 wrong type: %T", frame)
	}
}

// ---- all recipe -------------------------------------------------------

func TestAllRecipe_RunsEverySubRecipe(t *testing.T) {
	scene := visuals.NewScene("world")
	ar := newAllRecipe()
	ar.Initial(scene)
	labels := scene.Labels()
	expected := []string{
		"march_0", "pulse_0", "demo_box",
		"frame_0", "arm_shoulder", "trajectory_wp_0", "trajectory_runner",
	}
	for _, want := range expected {
		found := false
		for _, l := range labels {
			if l == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("`all` scene missing %q (have %v)", want, labels)
		}
	}
}

func TestAllRecipe_SubRecipesDontOverlapInY(t *testing.T) {
	scene := visuals.NewScene("world")
	ar := newAllRecipe()
	ar.Initial(scene)
	march := scene.Get("march_0").(*visuals.Box)
	pulse := scene.Get("pulse_0").(*visuals.Sphere)
	diff := march.Pose.Y - pulse.Pose.Y
	if diff < 0 {
		diff = -diff
	}
	if diff < 500 {
		t.Errorf("expected sub-recipes spaced >= 500mm apart, got |Δy|=%v", diff)
	}
}

func TestAllRecipe_TickAnimatesEverySubRecipe(t *testing.T) {
	scene := visuals.NewScene("world")
	ar := newAllRecipe()
	ar.Initial(scene)
	events := ar.Tick(scene, 0.5)
	hasPrefix := func(prefix string) bool {
		for _, e := range events {
			if strings.HasPrefix(e.Label, prefix) {
				return true
			}
		}
		return false
	}
	for _, prefix := range []string{"march_", "pulse_", "det_", "trajectory_runner"} {
		if !hasPrefix(prefix) {
			t.Errorf("expected an event with prefix %q in tick output", prefix)
		}
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
