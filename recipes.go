// Driver recipes — domain-logic generators that mutate a Scene.
//
// A recipe pairs Initial (seed the scene) with Tick (mutate at the
// driver's cadence). The driver serializes the returned events
// and ships them to its visualizer.
//
// Recipes are the "what to draw" side of the split. They're written
// against the visuals.Scene API — no proto, no gRPC, no field-mask
// paths. The library handles everything below the scene.Add /
// scene.Update calls.
package exampleviz

import (
	"fmt"
	"math"
	"strings"

	"exampleviz/visuals"
)

// Recipe is the two-method contract every recipe satisfies.
type Recipe interface {
	Name() string
	Initial(scene *visuals.Scene) []visuals.SceneEvent
	Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent
}

// MarchingBoxes — five boxes in a row, each bobbing in Y on a sine wave.
// Simplest possible recipe; proves the pipeline end-to-end.
//
// YOrigin shifts the row along Y for use in the "all" recipe.
type MarchingBoxes struct {
	YOrigin float64
}

func (MarchingBoxes) Name() string { return "marching_boxes" }

const (
	mbCount     = 5
	mbSpacing   = 250.0
	mbAmplitude = 150.0
	mbPeriodS   = 3.0
)

func (mb MarchingBoxes) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < mbCount; i++ {
		x := (float64(i) - float64(mbCount-1)/2) * mbSpacing
		c := rainbow(float64(i) / mbCount)
		box := &visuals.Box{
			Label:  marchingLabel(i),
			Pose:   visuals.Pose{X: x, Y: mb.YOrigin, Z: 100},
			DimsMM: visuals.BoxDims{X: 120, Y: 120, Z: 120},
			Color:  &c,
		}
		events, err := scene.Add(box)
		if err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func (mb MarchingBoxes) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < mbCount; i++ {
		v := scene.Get(marchingLabel(i))
		if v == nil {
			continue
		}
		box, ok := v.(*visuals.Box)
		if !ok {
			continue
		}
		x := (float64(i) - float64(mbCount-1)/2) * mbSpacing
		phase := 2 * math.Pi * float64(i) / float64(mbCount)
		y := mbAmplitude * math.Sin(2*math.Pi*t/mbPeriodS+phase)
		box.Pose = visuals.Pose{X: x, Y: mb.YOrigin + y, Z: 100}
		events, err := scene.Update(box)
		if err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func marchingLabel(i int) string {
	return "march_" + string(rune('0'+i))
}

// PulsingSpheres — three spheres pulsing radius on phase-offset
// sine waves. Exercises a different field-mask path
// (physicalObject.geometryType.value.radiusMm) and confirms the
// visualizer rebuilds the geometry proto, not just the pose.
type PulsingSpheres struct {
	YOrigin float64
}

func (PulsingSpheres) Name() string { return "pulsing_spheres" }

const (
	psCount   = 3
	psSpacing = 400.0
	psRBase   = 80.0
	psRAmp    = 30.0
	psPeriodS = 2.5
)

func (ps PulsingSpheres) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < psCount; i++ {
		x := (float64(i) - float64(psCount-1)/2) * psSpacing
		c := rainbow(0.7 * float64(i) / float64(max1(psCount-1)))
		sp := &visuals.Sphere{
			Label:    pulsingLabel(i),
			Pose:     visuals.Pose{X: x, Y: ps.YOrigin, Z: 120},
			RadiusMM: psRBase,
			Color:    &c,
		}
		events, err := scene.Add(sp)
		if err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func (ps PulsingSpheres) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < psCount; i++ {
		v := scene.Get(pulsingLabel(i))
		if v == nil {
			continue
		}
		sp, ok := v.(*visuals.Sphere)
		if !ok {
			continue
		}
		phase := 2 * math.Pi * float64(i) / float64(psCount)
		sp.RadiusMM = psRBase + psRAmp*math.Sin(2*math.Pi*t/psPeriodS+phase)
		events, err := scene.Update(sp)
		if err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func pulsingLabel(i int) string {
	return "pulse_" + string(rune('0'+i))
}

// ---- all_primitives ----------------------------------------------------

// AllPrimitives — one of every supported shape, static.
//
// Driver-side equivalent of the standalone-playground "primitives"
// preset. Useful as the "what can I put in a Scene" reference: each
// shape type appears in a row along X. Static — the driver pushes
// ADDED events on startup and nothing thereafter.
//
// Note that Mesh and PointCloud items reference asset paths that
// the *visualizer* resolves at install time. As long as the driver
// and visualizer ship from the same module binary (the default with
// the in-process registry), the visualizer's ReadAsset hook finds
// the assets in the module's installed directory.
type AllPrimitives struct {
	YOrigin float64
}

func (AllPrimitives) Name() string { return "all_primitives" }

const apSpacing = 280.0

func (ap AllPrimitives) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	z := 100.0
	y := ap.YOrigin
	red := visuals.Color{R: 230, G: 25, B: 75}
	green := visuals.Color{R: 60, G: 180, B: 75}
	blue := visuals.Color{R: 0, G: 130, B: 200}
	orange := visuals.Color{R: 245, G: 130, B: 48}
	purple := visuals.Color{R: 145, G: 30, B: 180}
	cyan := visuals.Color{R: 70, G: 240, B: 240}

	items := []interface{}{
		&visuals.Box{
			Label:  "demo_box",
			Pose:   visuals.Pose{X: -3 * apSpacing, Y: y, Z: z},
			DimsMM: visuals.BoxDims{X: 140, Y: 140, Z: 140},
			Color:  &red,
		},
		&visuals.Sphere{
			Label:    "demo_sphere",
			Pose:     visuals.Pose{X: -2 * apSpacing, Y: y, Z: z},
			RadiusMM: 80,
			Color:    &green,
		},
		&visuals.Capsule{
			Label:    "demo_capsule",
			Pose:     visuals.Pose{X: -1 * apSpacing, Y: y, Z: z},
			RadiusMM: 40,
			LengthMM: 200,
			Color:    &blue,
		},
		&visuals.Point{
			Label: "demo_point",
			Pose:  visuals.Pose{Y: y, Z: z},
			Color: &orange,
		},
		&visuals.Arrow{
			Label:    "demo_arrow",
			Pose:     visuals.Pose{X: 1 * apSpacing, Y: y, Z: z},
			LengthMM: 240,
			RadiusMM: 20,
			Color:    &purple,
		},
		&visuals.Mesh{
			Label:    "demo_bunny",
			Pose:     visuals.Pose{X: 2 * apSpacing, Y: y, Z: z},
			MeshPath: "assets/bunny.stl",
			Color:    &cyan,
		},
		&visuals.PointCloud{
			Label:          "demo_pcd",
			Pose:           visuals.Pose{X: 3 * apSpacing, Y: y, Z: z},
			PointcloudPath: "assets/helix.pcd",
		},
	}
	events, _ := scene.Add(items...)
	return events
}

func (AllPrimitives) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	return nil // static
}

// ---- detections_overlay ------------------------------------------------

// DetectionsOverlay — simulated object-detection overlay: N solid
// translucent bounding boxes drifting around the origin.
//
// This is the canonical driver-shaped use case — a perception
// module produces detections every tick and the visualizer
// publishes them to the renderer. The recipe stands in for the
// real detector by walking N synthetic detections on phase-offset
// circular paths.
//
// Uses visuals.BoundingBox with Wireframe=false (a single solid
// Box per detection). The translucent opacity keeps the underlying
// scene visible through the overlay, mirroring how most perception
// output is rendered (YOLO, Google Cloud Vision, etc.).
//
// Note: BoundingBox{Wireframe: true} expands into 12 capsule edges
// positioned via ParentFrame chaining, but doesn't honor the
// composite's Pose directly today. Solid is the right choice here;
// wireframe is a future direction once an anchor-frame pattern is
// wired into the recipe.
type DetectionsOverlay struct {
	YOrigin float64
}

func (DetectionsOverlay) Name() string { return "detections_overlay" }

const (
	doDetections    = 4
	doOrbitRadiusMM = 500.0
	doOrbitPeriodS  = 8.0
)

var doBoxDims = visuals.BoxDims{X: 140, Y: 100, Z: 120}

func (DetectionsOverlay) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	// Detections appear on first tick — returning nothing here keeps
	// the initial-burst broadcast clean.
	return nil
}

func (do DetectionsOverlay) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < doDetections; i++ {
		phase := 2 * math.Pi * float64(i) / float64(doDetections)
		angle := 2*math.Pi*t/doOrbitPeriodS + phase
		x := doOrbitRadiusMM * math.Cos(angle)
		y := do.YOrigin + doOrbitRadiusMM*math.Sin(angle)
		z := 200.0
		c := rainbow(float64(i) / float64(doDetections))
		opacity := 0.4
		bbox := visuals.BoundingBox{
			Label:   detectionLabel(i),
			Pose:    visuals.Pose{X: x, Y: y, Z: z},
			DimsMM:  doBoxDims,
			Color:   &c,
			Opacity: &opacity,
		}
		events, err := scene.AddOrUpdate(bbox)
		if err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func detectionLabel(i int) string {
	return "det_" + string(rune('0'+i))
}

// ---- coordinate_frames_arm --------------------------------------------

// CoordinateFramesArm — three spinning coordinate-frame triads + an
// articulated arm. Demonstrates composite expansion (CoordinateFrame
// → 4 visuals) and chained parent_frame propagation (each arm link
// parents to the previous link's label).
//
// All animation is computed client-side: only joint angles change
// per tick. The static parent-frame offsets stay fixed.
type CoordinateFramesArm struct {
	YOrigin float64
}

func (CoordinateFramesArm) Name() string { return "coordinate_frames_arm" }

const (
	cfFrameY      = 600.0
	cfFrameSizeMM = 120.0
	cfArmBaseX    = -800.0
	cfArmBaseY    = -400.0
	cfArmBaseZ    = 0.0
	cfLinkLength  = 200.0
	cfShoulderAmp = 50.0
	cfShoulderPer = 4.5
	cfElbowAmp    = 60.0
	cfElbowPer    = 3.2
	cfWristAmp    = 90.0
	cfWristPer    = 2.6
)

var cfFrameXs = []float64{-400, 0, 400}
var cfFramePeriods = []float64{4.0, 5.5, 7.0}

func (cf CoordinateFramesArm) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}

	// Three coordinate-frame triads.
	for i, x := range cfFrameXs {
		frame := visuals.CoordinateFrame{
			Label:  fmt.Sprintf("frame_%d", i),
			Pose:   visuals.Pose{X: x, Y: cf.YOrigin + cfFrameY, Z: 200},
			SizeMM: cfFrameSizeMM,
		}
		events, err := scene.Add(frame)
		if err == nil {
			out = append(out, events...)
		}
	}

	// Articulated arm: each link parents to the prior link's label.
	gray := visuals.Color{R: 120, G: 120, B: 120}
	red := visuals.Color{R: 230, G: 100, B: 100}
	green := visuals.Color{R: 100, G: 230, B: 100}
	blue := visuals.Color{R: 100, G: 100, B: 230}
	yellow := visuals.Color{R: 230, G: 230, B: 100}
	L := cfLinkLength

	armItems := []interface{}{
		&visuals.Sphere{
			Label:    "arm_shoulder",
			Pose:     visuals.Pose{X: cfArmBaseX, Y: cf.YOrigin + cfArmBaseY, Z: cfArmBaseZ},
			RadiusMM: 45, Color: &gray,
		},
		&visuals.Capsule{
			Label:       "arm_upper",
			ParentFrame: "arm_shoulder",
			Pose:        visuals.Pose{Z: L / 2},
			RadiusMM:    28, LengthMM: L, Color: &red,
		},
		&visuals.Sphere{
			Label:       "arm_elbow",
			ParentFrame: "arm_upper",
			Pose:        visuals.Pose{Z: L / 2},
			RadiusMM:    36, Color: &green,
		},
		&visuals.Capsule{
			Label:       "arm_forearm",
			ParentFrame: "arm_elbow",
			Pose:        visuals.Pose{Z: L / 2},
			RadiusMM:    22, LengthMM: L, Color: &blue,
		},
		&visuals.Sphere{
			Label:       "arm_wrist",
			ParentFrame: "arm_forearm",
			Pose:        visuals.Pose{Z: L / 2},
			RadiusMM:    28, Color: &yellow,
		},
	}
	events, err := scene.Add(armItems...)
	if err == nil {
		out = append(out, events...)
	}
	return out
}

func (cf CoordinateFramesArm) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	gray := visuals.Color{R: 120, G: 120, B: 120}
	green := visuals.Color{R: 100, G: 230, B: 100}
	yellow := visuals.Color{R: 230, G: 230, B: 100}

	// Rebuild each composite/joint with the new pose every tick.
	// scene.AddOrUpdate handles composite expansion and per-label
	// diffing — only the changed labels produce UPDATED events.
	// (Scene.Get + type-assert against the stored type doesn't
	// generalize across value-stored composites vs pointer-stored
	// arm pointers; AddOrUpdate keeps the code uniform.)

	for i, x := range cfFrameXs {
		theta := math.Mod(360.0*t/cfFramePeriods[i], 360.0)
		frame := visuals.CoordinateFrame{
			Label:  fmt.Sprintf("frame_%d", i),
			Pose:   visuals.Pose{X: x, Y: cf.YOrigin + cfFrameY, Z: 200, Theta: theta},
			SizeMM: cfFrameSizeMM,
		}
		if events, err := scene.AddOrUpdate(frame); err == nil {
			out = append(out, events...)
		}
	}

	shoulderTheta := cfShoulderAmp * math.Sin(2*math.Pi*t/cfShoulderPer)
	shoulder := &visuals.Sphere{
		Label: "arm_shoulder",
		Pose: visuals.Pose{
			X: cfArmBaseX, Y: cf.YOrigin + cfArmBaseY, Z: cfArmBaseZ,
			OY: 1, Theta: shoulderTheta,
		},
		RadiusMM: 45, Color: &gray,
	}
	if events, err := scene.AddOrUpdate(shoulder); err == nil {
		out = append(out, events...)
	}

	elbowTheta := cfElbowAmp * math.Sin(2*math.Pi*t/cfElbowPer+0.7)
	elbow := &visuals.Sphere{
		Label:       "arm_elbow",
		ParentFrame: "arm_upper",
		Pose:        visuals.Pose{Z: cfLinkLength / 2, OY: 1, Theta: elbowTheta},
		RadiusMM:    36, Color: &green,
	}
	if events, err := scene.AddOrUpdate(elbow); err == nil {
		out = append(out, events...)
	}

	wristTheta := cfWristAmp * math.Sin(2*math.Pi*t/cfWristPer)
	wrist := &visuals.Sphere{
		Label:       "arm_wrist",
		ParentFrame: "arm_forearm",
		Pose:        visuals.Pose{Z: cfLinkLength / 2, Theta: wristTheta},
		RadiusMM:    28, Color: &yellow,
	}
	if events, err := scene.AddOrUpdate(wrist); err == nil {
		out = append(out, events...)
	}

	return out
}

// ---- trajectory_runner ------------------------------------------------

// TrajectoryRunner — a "runner" sphere walking through a list of
// waypoints, each with its own orientation, with smooth
// interpolation between. Mirrors the standalone-playground's
// trajectory_preview preset, and — more importantly — the typical
// output of a motion planner (CBiRRT, RRT*, motion-service plans):
// a sequence of Cartesian poses produced by forward-kinematics on
// the planner's joint output.
//
// Built on the visuals.TrajectoryPlan composite (static line +
// per-waypoint CoordinateFrame triads) plus visuals.LerpPose for
// the runner's between-waypoint interpolation. To preview a real
// motion plan, swap trWaypoints for the planner's pose list — the
// rest of the recipe is plan-agnostic.
type TrajectoryRunner struct {
	YOrigin float64
}

func (TrajectoryRunner) Name() string { return "trajectory_runner" }

const (
	trLapPeriodS  = 12.0
	trLabelPrefix = "trajectory"
)

// Each waypoint carries position AND orientation. The runner's
// orientation interpolates between adjacent waypoints — the result
// is visibly different at every t value, not constant along a
// segment.
var trWaypoints = []visuals.Pose{
	visuals.PoseAt(-400, -300, 100, 0, 0, 1, 0), // identity (Z up)
	visuals.PoseAt(-200, -150, 200, 1, 0, 0, 0), // tipped (X up)
	visuals.PoseAt(0, 0, 300, 0, 0, 1, 90),      // identity + 90° roll
	visuals.PoseAt(200, 150, 200, 0, 1, 0, 0),   // tipped (Y up)
	visuals.PoseAt(400, 300, 100, 0, 0, 1, 0),   // back to identity
}

func (tr TrajectoryRunner) waypoints() []visuals.Pose {
	out := make([]visuals.Pose, len(trWaypoints))
	for i, wp := range trWaypoints {
		// Shift Y by YOrigin; preserve orientation verbatim.
		out[i] = visuals.PoseAt(
			wp.X, wp.Y+tr.YOrigin, wp.Z,
			wp.OX, wp.OY, wp.OZ, wp.Theta,
		)
	}
	return out
}

func (tr TrajectoryRunner) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	wps := tr.waypoints()

	// The whole static plan visualization — line + per-waypoint
	// coordinate-frame triads — from one composite.
	lineColor := visuals.Color{R: 120, G: 180, B: 220}
	plan := visuals.TrajectoryPlan{
		LabelPrefix:         trLabelPrefix,
		Waypoints:           wps,
		LineColor:           &lineColor,
		LineWidthMM:         6,
		LineOpacity:         ptr(0.5),
		FrameSizeMM:         80,
		FrameAxisRadiusMM:   4,
		FrameAnchorRadiusMM: 8,
		FrameAnchorOpacity:  ptr(0.5),
		FrameAxisOpacity:    ptr(0.8),
	}
	if events, err := scene.Add(plan); err == nil {
		out = append(out, events...)
	}

	// The runner — brighter and larger, with its own axes helper so
	// its orientation through the arc is unmistakable.
	runnerColor := visuals.Color{R: 255, G: 200, B: 50}
	runner := &visuals.Sphere{
		Label:          "trajectory_runner",
		Pose:           wps[0],
		RadiusMM:       55,
		Color:          &runnerColor,
		ShowAxesHelper: true,
	}
	if events, err := scene.Add(runner); err == nil {
		out = append(out, events...)
	}
	return out
}

func (tr TrajectoryRunner) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	wps := tr.waypoints()
	n := len(wps)
	// nSegs = n-1 so the runner walks wp0→wp1→…→wp[n-1] and then
	// snaps back to wp0 to start the next lap. Matches the
	// all-in-one playground's trajectory_preview; avoids a phantom
	// interpolated wp[-1]→wp[0] segment that visually overshoots the
	// last waypoint and makes the runner stop matching its frame.
	nSegs := n - 1
	progress := math.Mod(t/trLapPeriodS*float64(nSegs), float64(nSegs))
	segIdx := int(progress)
	if segIdx >= nSegs {
		segIdx = nSegs - 1
	}
	local := progress - float64(segIdx)
	a := wps[segIdx]
	b := wps[segIdx+1]

	// Lerp position + orientation between adjacent waypoints.
	// LerpPose handles the lerp-and-normalize on the orientation
	// vector — close enough to SLERP for visual playback.
	runnerPose := visuals.LerpPose(a, b, local)

	runnerColor := visuals.Color{R: 255, G: 200, B: 50}
	runner := &visuals.Sphere{
		Label:          "trajectory_runner",
		Pose:           runnerPose,
		RadiusMM:       55,
		Color:          &runnerColor,
		ShowAxesHelper: true,
	}
	events, _ := scene.AddOrUpdate(runner)
	return events
}

// ---- lifecycle_garden -------------------------------------------------

// LifecycleGarden — N "plots" cycling through appear → alive →
// disappear → gone phases at staggered offsets. Demonstrates
// scene-graph mutation from a recipe (ADD / UPDATE / REMOVE) and
// the renderer's REMOVED-UUID cache workaround (each cycle uses a
// fresh label, so the visualizer's stable-strategy UUID differs
// from prior cycles).
//
// Uses a pointer receiver because the per-plot version counter
// needs to mutate across tick calls.
type LifecycleGarden struct {
	YOrigin float64
	version [lgNPlots]int
}

func (lg *LifecycleGarden) Name() string { return "lifecycle_garden" }

const (
	lgNPlots        = 5
	lgPlotSpacingMM = 250.0
	lgAppearS       = 0.8
	lgAliveS        = 1.6
	lgDisappearS    = 0.8
	lgGoneS         = 0.8
	lgCycleS        = lgAppearS + lgAliveS + lgDisappearS + lgGoneS
)

var (
	lgColorAppear    = visuals.Color{R: 50, G: 110, B: 220}
	lgColorAlive     = visuals.Color{R: 255, G: 165, B: 0}
	lgColorDisappear = visuals.Color{R: 220, G: 60, B: 60}
)

func (lg *LifecycleGarden) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	return nil // all plots start gone; tick handles add/update.
}

func (lg *LifecycleGarden) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < lgNPlots; i++ {
		phaseOffset := (lgCycleS / float64(lgNPlots)) * float64(i)
		localT := math.Mod(t+phaseOffset, lgCycleS)
		phase, _ := lg.phaseFor(localT)

		// Find current label for this plot (any version) in the scene.
		prefix := fmt.Sprintf("garden_%d_v", i)
		var current string
		for _, lab := range scene.Labels() {
			if strings.HasPrefix(lab, prefix) {
				current = lab
				break
			}
		}

		if phase == "gone" {
			if current != "" {
				if events := scene.Remove(current); len(events) > 0 {
					out = append(out, events...)
				}
			}
			continue
		}

		color := lg.colorFor(phase)
		opacity := lg.opacityFor(phase)
		x := (float64(i) - float64(lgNPlots-1)/2.0) * lgPlotSpacingMM

		if current == "" {
			// Fresh cycle — bump version, use a new label.
			lg.version[i]++
			box := &visuals.Box{
				Label:   fmt.Sprintf("garden_%d_v%d", i, lg.version[i]),
				Pose:    visuals.Pose{X: x, Y: lg.YOrigin, Z: 100},
				DimsMM:  visuals.BoxDims{X: 140, Y: 140, Z: 140},
				Color:   &color,
				Opacity: &opacity,
			}
			if events, err := scene.Add(box); err == nil {
				out = append(out, events...)
			}
			continue
		}

		// Update existing plot — color/opacity change.
		v := scene.Get(current)
		if v == nil {
			continue
		}
		box, ok := v.(*visuals.Box)
		if !ok {
			continue
		}
		box.Color = &color
		box.Opacity = &opacity
		if events, err := scene.Update(box); err == nil {
			out = append(out, events...)
		}
	}
	return out
}

func (lg *LifecycleGarden) phaseFor(localT float64) (string, float64) {
	if localT < lgAppearS {
		return "appear", localT / lgAppearS
	}
	localT -= lgAppearS
	if localT < lgAliveS {
		return "alive", localT / lgAliveS
	}
	localT -= lgAliveS
	if localT < lgDisappearS {
		return "disappear", localT / lgDisappearS
	}
	localT -= lgDisappearS
	return "gone", localT / lgGoneS
}

func (lg *LifecycleGarden) colorFor(phase string) visuals.Color {
	switch phase {
	case "appear":
		return lgColorAppear
	case "alive":
		return lgColorAlive
	case "disappear":
		return lgColorDisappear
	}
	return visuals.Color{R: 255, G: 255, B: 255}
}

func (lg *LifecycleGarden) opacityFor(phase string) float64 {
	switch phase {
	case "appear":
		return 0.5
	case "alive":
		return 1.0
	case "disappear":
		return 0.5
	}
	return 0.0
}

// ---- force_vector -----------------------------------------------------

// ForceVector — animated force-vector arrow: length, radius, and
// orientation all cycling simultaneously.
//
// Mirrors the standalone-playground's force_vector_demo preset.
// The arrow's length and radius oscillate on phase-offset sine
// waves; orientation precesses around world +Z at a fixed tilt.
// Color cycling (the standalone preset's hue sweep) is not included
// because metadata updates don't propagate via UPDATED — only at
// spawn time. To add color cycling, use the label-rotation pattern
// from BreathingShapes.
// (Named ForceVectorRecipe — the unqualified “ForceVector“ is
// already taken by the visuals AnimationSpec alias in aliases.go.)
type ForceVectorRecipe struct {
	YOrigin float64
}

func (ForceVectorRecipe) Name() string { return "force_vector" }

const (
	fvBaseLength      = 200.0
	fvLengthAmplitude = 80.0
	fvLengthPeriodS   = 3.2
	fvBaseRadius      = 16.0
	fvRadiusAmplitude = 6.0
	fvRadiusPeriodS   = 2.0
	fvTiltDeg         = 45.0
	fvPrecessionPerS  = 5.0
)

func (fv ForceVectorRecipe) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	color := visuals.Color{R: 255, G: 140, B: 30}
	arrow := &visuals.Arrow{
		Label:    "force_vector",
		Pose:     visuals.Pose{Y: fv.YOrigin, OZ: 1},
		LengthMM: fvBaseLength,
		RadiusMM: fvBaseRadius,
		Color:    &color,
	}
	events, _ := scene.Add(arrow)
	return events
}

func (fv ForceVectorRecipe) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	length := fvBaseLength + fvLengthAmplitude*math.Sin(2*math.Pi*t/fvLengthPeriodS)
	radius := fvBaseRadius + fvRadiusAmplitude*math.Sin(2*math.Pi*t/fvRadiusPeriodS+math.Pi/2)

	tiltRad := fvTiltDeg * math.Pi / 180.0
	phi := 2 * math.Pi * t / fvPrecessionPerS
	ox := math.Sin(tiltRad) * math.Cos(phi)
	oy := math.Sin(tiltRad) * math.Sin(phi)
	oz := math.Cos(tiltRad)

	color := visuals.Color{R: 255, G: 140, B: 30}
	arrow := &visuals.Arrow{
		Label:    "force_vector",
		Pose:     visuals.Pose{Y: fv.YOrigin, OX: ox, OY: oy, OZ: oz},
		LengthMM: length,
		RadiusMM: radius,
		Color:    &color,
	}
	events, _ := scene.AddOrUpdate(arrow)
	return events
}

// ---- breathing_shapes -------------------------------------------------

// BreathingShapes — N spheres whose opacity smoothly cycles in
// [0, 1] via label rotation. Demonstrates the only working pattern
// for live opacity changes given the renderer's UPDATED handler
// ignores metadata.* paths: REMOVE the current label, ADD with a
// fresh label so the renderer re-reads metadata at spawn.
//
// Opacity is snapped to STEPS_PER_PERIOD discrete values per
// oscillation period so label-rotation rate stays bounded.
//
// Pointer receiver because the per-slot version + last-step
// counters mutate across tick calls.
type BreathingShapes struct {
	YOrigin      float64
	version      [bsN]int
	lastStep     [bsN]int
	initLastStep bool
}

func (*BreathingShapes) Name() string { return "breathing_shapes" }

const (
	bsN              = 4
	bsSpacingMM      = 300.0
	bsRadiusMM       = 70.0
	bsPeriodS        = 3.5
	bsStepsPerPeriod = 16
	bsOpacityMin     = 0.10
	bsOpacityMax     = 1.0
)

func (bs *BreathingShapes) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	if !bs.initLastStep {
		for i := range bs.lastStep {
			bs.lastStep[i] = -1
		}
		bs.initLastStep = true
	}
	return nil
}

func (bs *BreathingShapes) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	if !bs.initLastStep {
		for i := range bs.lastStep {
			bs.lastStep[i] = -1
		}
		bs.initLastStep = true
	}
	out := []visuals.SceneEvent{}
	for i := 0; i < bsN; i++ {
		phase := 2 * math.Pi * float64(i) / float64(bsN)
		theta := 2*math.Pi*t/bsPeriodS + phase
		opacity := bsOpacityMin + (bsOpacityMax-bsOpacityMin)*0.5*(1+math.Sin(theta))
		step := int(theta/(2*math.Pi/float64(bsStepsPerPeriod))) % bsStepsPerPeriod
		if step == bs.lastStep[i] {
			continue
		}
		bs.lastStep[i] = step
		bs.version[i]++

		prefix := fmt.Sprintf("breathe_%d_v", i)
		var current string
		for _, lab := range scene.Labels() {
			if strings.HasPrefix(lab, prefix) {
				current = lab
				break
			}
		}
		if current != "" {
			out = append(out, scene.Remove(current)...)
		}

		x := (float64(i) - float64(bsN-1)/2.0) * bsSpacingMM
		c := rainbow(float64(i) / float64(bsN))
		op := opacity
		sphere := &visuals.Sphere{
			Label:    fmt.Sprintf("breathe_%d_v%d", i, bs.version[i]),
			Pose:     visuals.Pose{X: x, Y: bs.YOrigin, Z: 120},
			RadiusMM: bsRadiusMM,
			Color:    &c,
			Opacity:  &op,
		}
		if events, err := scene.Add(sphere); err == nil {
			out = append(out, events...)
		}
	}
	return out
}

// ---- all (every recipe, stacked along Y) ------------------------------

// AllRecipe — run every other recipe simultaneously, stacked along Y.
//
// Driver-side equivalent of the standalone-playground's "all" preset.
// Each sub-recipe gets a unique YOrigin so their visuals don't
// collide. Label uniqueness is already enforced across recipes (each
// uses its own prefix — march_*, pulse_*, demo_*, det_*, frame_* /
// arm_*, wp_* / trajectory_*, garden_*), so simply running them in
// sequence is collision-free.
type AllRecipe struct {
	subs []Recipe
}

func (*AllRecipe) Name() string { return "all" }

func newAllRecipe() *AllRecipe {
	return &AllRecipe{
		subs: []Recipe{
			MarchingBoxes{YOrigin: -2600},
			PulsingSpheres{YOrigin: -2000},
			&BreathingShapes{YOrigin: -1400},
			AllPrimitives{YOrigin: -800},
			DetectionsOverlay{YOrigin: 0},
			ForceVectorRecipe{YOrigin: 600},
			&LifecycleGarden{YOrigin: 1200},
			TrajectoryRunner{YOrigin: 1900},
			CoordinateFramesArm{YOrigin: 2800},
		},
	}
}

func (ar *AllRecipe) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for _, sub := range ar.subs {
		out = append(out, sub.Initial(scene)...)
	}
	return out
}

func (ar *AllRecipe) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for _, sub := range ar.subs {
		out = append(out, sub.Tick(scene, t)...)
	}
	return out
}

// ---- registry ----------------------------------------------------------

var Recipes = map[string]Recipe{
	(MarchingBoxes{}).Name():       MarchingBoxes{},
	(PulsingSpheres{}).Name():      PulsingSpheres{},
	(AllPrimitives{}).Name():       AllPrimitives{},
	(DetectionsOverlay{}).Name():   DetectionsOverlay{},
	(CoordinateFramesArm{}).Name(): CoordinateFramesArm{},
	(TrajectoryRunner{}).Name():    TrajectoryRunner{},
	(&LifecycleGarden{}).Name():    &LifecycleGarden{},
	(ForceVectorRecipe{}).Name():   ForceVectorRecipe{},
	(&BreathingShapes{}).Name():    &BreathingShapes{},
	(&AllRecipe{}).Name():          newAllRecipe(),
}

// ---- helpers -----------------------------------------------------------

func rainbow(u float64) visuals.Color {
	if u < 0 {
		u = 0
	}
	if u > 1 {
		u = 1
	}
	h := u * 6.0
	i := int(h) % 6
	f := h - float64(int(h))
	var r, g, b float64
	switch i {
	case 0:
		r, g, b = 1.0, f, 0.0
	case 1:
		r, g, b = 1.0-f, 1.0, 0.0
	case 2:
		r, g, b = 0.0, 1.0, f
	case 3:
		r, g, b = 0.0, 1.0-f, 1.0
	case 4:
		r, g, b = f, 0.0, 1.0
	default:
		r, g, b = 1.0, 0.0, 1.0-f
	}
	return visuals.Color{R: int(r * 255), G: int(g * 255), B: int(b * 255)}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}
