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

// ---- registry ----------------------------------------------------------

var Recipes = map[string]Recipe{
	(MarchingBoxes{}).Name():  MarchingBoxes{},
	(PulsingSpheres{}).Name(): PulsingSpheres{},
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
