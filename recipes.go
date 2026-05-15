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
	"math"

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

// ---- registry ----------------------------------------------------------

var Recipes = map[string]Recipe{
	(MarchingBoxes{}).Name():     MarchingBoxes{},
	(PulsingSpheres{}).Name():    PulsingSpheres{},
	(AllPrimitives{}).Name():     AllPrimitives{},
	(DetectionsOverlay{}).Name(): DetectionsOverlay{},
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
