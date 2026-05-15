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
type MarchingBoxes struct{}

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
			Pose:   visuals.Pose{X: x, Z: 100},
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
		box.Pose = visuals.Pose{X: x, Y: y, Z: 100}
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
type PulsingSpheres struct{}

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
			Pose:     visuals.Pose{X: x, Z: 120},
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
type AllPrimitives struct{}

func (AllPrimitives) Name() string { return "all_primitives" }

const apSpacing = 280.0

func (AllPrimitives) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	z := 100.0
	red := visuals.Color{R: 230, G: 25, B: 75}
	green := visuals.Color{R: 60, G: 180, B: 75}
	blue := visuals.Color{R: 0, G: 130, B: 200}
	orange := visuals.Color{R: 245, G: 130, B: 48}
	purple := visuals.Color{R: 145, G: 30, B: 180}
	cyan := visuals.Color{R: 70, G: 240, B: 240}

	items := []interface{}{
		&visuals.Box{
			Label:  "demo_box",
			Pose:   visuals.Pose{X: -3 * apSpacing, Z: z},
			DimsMM: visuals.BoxDims{X: 140, Y: 140, Z: 140},
			Color:  &red,
		},
		&visuals.Sphere{
			Label:    "demo_sphere",
			Pose:     visuals.Pose{X: -2 * apSpacing, Z: z},
			RadiusMM: 80,
			Color:    &green,
		},
		&visuals.Capsule{
			Label:    "demo_capsule",
			Pose:     visuals.Pose{X: -1 * apSpacing, Z: z},
			RadiusMM: 40,
			LengthMM: 200,
			Color:    &blue,
		},
		&visuals.Point{
			Label: "demo_point",
			Pose:  visuals.Pose{Z: z},
			Color: &orange,
		},
		&visuals.Arrow{
			Label:    "demo_arrow",
			Pose:     visuals.Pose{X: 1 * apSpacing, Z: z},
			LengthMM: 240,
			RadiusMM: 20,
			Color:    &purple,
		},
		&visuals.Mesh{
			Label:    "demo_bunny",
			Pose:     visuals.Pose{X: 2 * apSpacing, Z: z},
			MeshPath: "assets/bunny.stl",
			Color:    &cyan,
		},
		&visuals.PointCloud{
			Label:          "demo_pcd",
			Pose:           visuals.Pose{X: 3 * apSpacing, Z: z},
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
type DetectionsOverlay struct{}

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

func (DetectionsOverlay) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}
	for i := 0; i < doDetections; i++ {
		phase := 2 * math.Pi * float64(i) / float64(doDetections)
		angle := 2*math.Pi*t/doOrbitPeriodS + phase
		x := doOrbitRadiusMM * math.Cos(angle)
		y := doOrbitRadiusMM * math.Sin(angle)
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
type CoordinateFramesArm struct{}

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

func (CoordinateFramesArm) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}

	// Three coordinate-frame triads.
	for i, x := range cfFrameXs {
		frame := visuals.CoordinateFrame{
			Label:  fmt.Sprintf("frame_%d", i),
			Pose:   visuals.Pose{X: x, Y: cfFrameY, Z: 200},
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
			Pose:     visuals.Pose{X: cfArmBaseX, Y: cfArmBaseY, Z: cfArmBaseZ},
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

func (CoordinateFramesArm) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
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
			Pose:   visuals.Pose{X: x, Y: cfFrameY, Z: 200, Theta: theta},
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
			X: cfArmBaseX, Y: cfArmBaseY, Z: cfArmBaseZ,
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
// waypoints with linear interpolation. The waypoints and the line
// connecting them are static; only the runner's pose changes per tick.
type TrajectoryRunner struct{}

func (TrajectoryRunner) Name() string { return "trajectory_runner" }

const (
	trLapPeriodS = 8.0
)

var trWaypoints = []visuals.Pose{
	{X: -400, Y: -300, Z: 100, OZ: 1},
	{X: -200, Y: -150, Z: 200, OZ: 1},
	{X: 0, Y: 0, Z: 300, OZ: 1},
	{X: 200, Y: 150, Z: 200, OZ: 1},
	{X: 400, Y: 300, Z: 100, OZ: 1},
}

func (TrajectoryRunner) Initial(scene *visuals.Scene) []visuals.SceneEvent {
	out := []visuals.SceneEvent{}

	// Translucent static waypoint markers.
	wpColor := visuals.Color{R: 120, G: 180, B: 220}
	wpOpacity := 0.4
	for i, wp := range trWaypoints {
		marker := &visuals.Sphere{
			Label:    fmt.Sprintf("wp_%d", i),
			Pose:     wp,
			RadiusMM: 30, Color: &wpColor, Opacity: &wpOpacity,
		}
		if events, err := scene.Add(marker); err == nil {
			out = append(out, events...)
		}
	}

	// Path line connecting the waypoints.
	lineColor := visuals.Color{R: 120, G: 180, B: 220}
	lineOpacity := 0.5
	line := visuals.Line{
		LabelPrefix: "trajectory",
		Points:      append([]visuals.Pose(nil), trWaypoints...),
		WidthMM:     6,
		Color:       &lineColor,
		Opacity:     &lineOpacity,
	}
	if events, err := scene.Add(line); err == nil {
		out = append(out, events...)
	}

	// The runner — brighter and larger, with axes helper.
	runnerColor := visuals.Color{R: 255, G: 200, B: 50}
	runner := &visuals.Sphere{
		Label:          "trajectory_runner",
		Pose:           trWaypoints[0],
		RadiusMM:       55,
		Color:          &runnerColor,
		ShowAxesHelper: true,
	}
	if events, err := scene.Add(runner); err == nil {
		out = append(out, events...)
	}
	return out
}

func (TrajectoryRunner) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	n := len(trWaypoints)
	nSegs := n // LOOP=true: wrap back to wp 0
	progress := math.Mod(t/trLapPeriodS*float64(nSegs), float64(nSegs))
	segIdx := int(progress)
	local := progress - float64(segIdx)
	a := trWaypoints[segIdx]
	b := trWaypoints[(segIdx+1)%n]

	runnerColor := visuals.Color{R: 255, G: 200, B: 50}
	runner := &visuals.Sphere{
		Label: "trajectory_runner",
		Pose: visuals.Pose{
			X:  a.X + (b.X-a.X)*local,
			Y:  a.Y + (b.Y-a.Y)*local,
			Z:  a.Z + (b.Z-a.Z)*local,
			OZ: 1,
		},
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
				Pose:    visuals.Pose{X: x, Z: 100},
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

// ---- registry ----------------------------------------------------------

var Recipes = map[string]Recipe{
	(MarchingBoxes{}).Name():       MarchingBoxes{},
	(PulsingSpheres{}).Name():      PulsingSpheres{},
	(AllPrimitives{}).Name():       AllPrimitives{},
	(DetectionsOverlay{}).Name():   DetectionsOverlay{},
	(CoordinateFramesArm{}).Name(): CoordinateFramesArm{},
	(TrajectoryRunner{}).Name():    TrajectoryRunner{},
	(&LifecycleGarden{}).Name():    &LifecycleGarden{},
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
