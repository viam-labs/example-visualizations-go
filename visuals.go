// Visual / Animation typed surface — the OO API. Mirrors
// src/visuals.py in the Python sibling repo. Each Visual subtype
// (Box, Sphere, …) is a struct that validates its params at
// construction; ToItem() converts to the existing Item struct that
// service.go consumes. AnimationSpec is the same pattern for
// per-mode animation configs.
//
// See LIBRARY_PLAN.md for the design context.
package exampleviz

import "fmt"

// ---- Visual interface --------------------------------------------------

// Visual is anything that can produce an Item. Each primitive struct
// (Box, Sphere, …) implements it; composites can too.
type Visual interface {
	ToItem() Item
}

// ToItems materializes a slice of Visuals into their Item dicts.
// Convenience for presets that build visuals positionally and flush
// to the dict format the service consumes.
func ToItems(vs ...Visual) []Item {
	out := make([]Item, 0, len(vs))
	for _, v := range vs {
		out = append(out, v.ToItem())
	}
	return out
}

// PoseAt: an alternate convenience constructor for Pose using a full
// argument list (the existing poseAt(x,y,z) helper only takes 3).
func PoseAt(x, y, z, ox, oy, oz, theta float64) Pose {
	return Pose{X: x, Y: y, Z: z, OX: ox, OY: oy, OZ: oz, Theta: theta, hasOrient: true}
}

// ---- Visual primitive types --------------------------------------------
//
// Each primitive struct carries the common Visual fields (Label,
// Pose, ParentFrame, Color, Opacity, ShowAxesHelper, Invisible,
// Animation) plus its shape-specific fields. Go doesn't inherit;
// duplicating the common fields is the most idiomatic shape because
// it preserves clean struct-literal call sites at the preset site.

// Box — solid axis-aligned box.
type Box struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	DimsMM         BoxDims
}

func (b Box) ToItem() Item {
	must(b.Label != "", "Box requires Label")
	must(b.DimsMM.X > 0 && b.DimsMM.Y > 0 && b.DimsMM.Z > 0,
		"Box.DimsMM must all be > 0; got %v", b.DimsMM)
	return Item{
		Type: "box", Label: b.Label,
		Pose: fillPose(b.Pose), ParentFrame: b.ParentFrame,
		HasDims: true, DimsMM: b.DimsMM,
		Color: b.Color, Opacity: b.Opacity,
		ShowAxesHelper: b.ShowAxesHelper, Invisible: b.Invisible,
		Animation: animOf(b.Animation),
	}
}

// Sphere — solid sphere.
type Sphere struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	RadiusMM       float64
}

func (s Sphere) ToItem() Item {
	must(s.Label != "", "Sphere requires Label")
	must(s.RadiusMM > 0, "Sphere.RadiusMM must be > 0; got %v", s.RadiusMM)
	return Item{
		Type: "sphere", Label: s.Label,
		Pose: fillPose(s.Pose), ParentFrame: s.ParentFrame,
		RadiusMM: s.RadiusMM,
		Color:    s.Color, Opacity: s.Opacity,
		ShowAxesHelper: s.ShowAxesHelper, Invisible: s.Invisible,
		Animation: animOf(s.Animation),
	}
}

// Capsule — cylinder with hemispherical end caps.
type Capsule struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	RadiusMM       float64
	LengthMM       float64
}

func (c Capsule) ToItem() Item {
	must(c.Label != "", "Capsule requires Label")
	must(c.RadiusMM > 0, "Capsule.RadiusMM must be > 0; got %v", c.RadiusMM)
	must(c.LengthMM > 0, "Capsule.LengthMM must be > 0; got %v", c.LengthMM)
	return Item{
		Type: "capsule", Label: c.Label,
		Pose: fillPose(c.Pose), ParentFrame: c.ParentFrame,
		RadiusMM: c.RadiusMM, LengthMM: c.LengthMM,
		Color: c.Color, Opacity: c.Opacity,
		ShowAxesHelper: c.ShowAxesHelper, Invisible: c.Invisible,
		Animation: animOf(c.Animation),
	}
}

// Point — marker (rendered as a small sphere because the viewer
// drops zero-radius geometries).
type Point struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
}

func (p Point) ToItem() Item {
	must(p.Label != "", "Point requires Label")
	return Item{
		Type: "point", Label: p.Label,
		Pose: fillPose(p.Pose), ParentFrame: p.ParentFrame,
		Color: p.Color, Opacity: p.Opacity,
		ShowAxesHelper: p.ShowAxesHelper, Invisible: p.Invisible,
		Animation: animOf(p.Animation),
	}
}

// Arrow — procedural cylinder-shaft + cone-tip mesh along local +Z.
type Arrow struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	LengthMM       float64
	RadiusMM       float64
}

func (a Arrow) ToItem() Item {
	must(a.Label != "", "Arrow requires Label")
	must(a.LengthMM > 0, "Arrow.LengthMM must be > 0; got %v", a.LengthMM)
	must(a.RadiusMM > 0, "Arrow.RadiusMM must be > 0; got %v", a.RadiusMM)
	return Item{
		Type: "arrow", Label: a.Label,
		Pose: fillPose(a.Pose), ParentFrame: a.ParentFrame,
		LengthMM: a.LengthMM, RadiusMM: a.RadiusMM,
		Color: a.Color, Opacity: a.Opacity,
		ShowAxesHelper: a.ShowAxesHelper, Invisible: a.Invisible,
		Animation: animOf(a.Animation),
	}
}

// Mesh — PLY/STL asset. Auto-converts STL → PLY at load time
// unless RawSTL=true (bug-demo for the silent-drop case).
type Mesh struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	MeshPath       string
	RawSTL         bool
}

func (m Mesh) ToItem() Item {
	must(m.Label != "", "Mesh requires Label")
	must(m.MeshPath != "", "Mesh requires MeshPath")
	return Item{
		Type: "mesh", Label: m.Label,
		Pose: fillPose(m.Pose), ParentFrame: m.ParentFrame,
		MeshPath: m.MeshPath, RawSTL: m.RawSTL,
		Color: m.Color, Opacity: m.Opacity,
		ShowAxesHelper: m.ShowAxesHelper, Invisible: m.Invisible,
		Animation: animOf(m.Animation),
	}
}

// PointCloud — PCD asset; experimental chunked delivery via
// Chunked + ChunkSize.
type PointCloud struct {
	Label          string
	Pose           Pose
	ParentFrame    string
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	Animation      AnimationSpec
	PointcloudPath string
	Chunked        bool
	ChunkSize      int
}

func (pc PointCloud) ToItem() Item {
	must(pc.Label != "", "PointCloud requires Label")
	must(pc.PointcloudPath != "", "PointCloud requires PointcloudPath")
	if pc.Chunked {
		must(pc.ChunkSize > 0,
			"PointCloud.ChunkSize must be > 0 when Chunked=true; got %v", pc.ChunkSize)
	}
	return Item{
		Type: "pointcloud", Label: pc.Label,
		Pose: fillPose(pc.Pose), ParentFrame: pc.ParentFrame,
		PointcloudPath: pc.PointcloudPath,
		Chunked:        pc.Chunked, ChunkSize: pc.ChunkSize,
		Color: pc.Color, Opacity: pc.Opacity,
		ShowAxesHelper: pc.ShowAxesHelper, Invisible: pc.Invisible,
		Animation: animOf(pc.Animation),
	}
}

// ---- AnimationSpec interface + concrete types --------------------------
//
// The existing Animation struct (in animation.go) is the union of all
// per-mode params — it's what gets stored in Item.Animation and what
// the tick loop reads. AnimationSpec is the new typed surface: each
// concrete spec (Spin, Pulse, …) builds the right Animation struct
// via ToAnimation().

type AnimationSpec interface {
	ToAnimation() Animation
}

type Static struct{}

func (Static) ToAnimation() Animation { return Animation{Mode: "none"} }

type Spin struct{ PeriodS float64 }

func (s Spin) ToAnimation() Animation {
	return Animation{Mode: "spin", PeriodS: s.PeriodS}
}

type Swing struct {
	AmplitudeDeg float64
	PeriodS      float64
	PhaseOffsetS float64
}

func (s Swing) ToAnimation() Animation {
	return Animation{
		Mode: "swing", AmplitudeDeg: s.AmplitudeDeg,
		PeriodS: s.PeriodS, PhaseOffsetS: s.PhaseOffsetS,
	}
}

type Oscillate struct {
	Axis         string
	AmplitudeMM  float64
	PeriodS      float64
	PhaseOffsetS float64
}

func (o Oscillate) ToAnimation() Animation {
	must(o.Axis == "x" || o.Axis == "y" || o.Axis == "z",
		"Oscillate.Axis must be x|y|z; got %q", o.Axis)
	return Animation{
		Mode: "oscillate", Axis: o.Axis,
		AmplitudeMM: o.AmplitudeMM, PeriodS: o.PeriodS,
		PhaseOffsetS: o.PhaseOffsetS,
	}
}

type Orbit struct {
	RadiusMM float64
	PeriodS  float64
}

func (o Orbit) ToAnimation() Animation {
	return Animation{Mode: "orbit", RadiusMM: o.RadiusMM, PeriodS: o.PeriodS}
}

type Pulse struct {
	AmplitudeMM float64
	PeriodS     float64
	Axis        string // empty = no axis (for Sphere/Capsule); "x"/"y"/"z" for Box
}

func (p Pulse) ToAnimation() Animation {
	return Animation{
		Mode: "pulse", AmplitudeMM: p.AmplitudeMM,
		PeriodS: p.PeriodS, Axis: p.Axis,
	}
}

type Breathe struct {
	Amplitude float64
	PeriodS   float64
}

func (b Breathe) ToAnimation() Animation {
	return Animation{Mode: "breathe", Amplitude: b.Amplitude, PeriodS: b.PeriodS}
}

type Flicker struct {
	PeriodS           float64
	DutyCycle         float64
	PhaseOffsetS      float64
	RotateUUIDOnReadd *bool // nil = default true; explicit pointer when caller wants false
}

func (f Flicker) ToAnimation() Animation {
	return Animation{
		Mode: "flicker", PeriodS: f.PeriodS, DutyCycle: f.DutyCycle,
		PhaseOffsetS: f.PhaseOffsetS, RotateUUIDOnReadd: f.RotateUUIDOnReadd,
	}
}

type Lifecycle struct {
	AppearS      float64
	AliveS       float64
	DisappearS   float64
	GoneS        float64
	PhaseOffsetS float64
}

func (l Lifecycle) ToAnimation() Animation {
	return Animation{
		Mode: "lifecycle",
		AppearS: l.AppearS, AliveS: l.AliveS,
		DisappearS: l.DisappearS, GoneS: l.GoneS,
		PhaseOffsetS: l.PhaseOffsetS,
	}
}

type ForceVector struct {
	PeriodS           float64
	LengthAmplitudeMM float64
	RadiusAmplitudeMM float64
	TiltDeg           float64
	PrecessionSpeed   float64
	ColorSpeed        float64
}

func (fv ForceVector) ToAnimation() Animation {
	return Animation{
		Mode: "force_vector", PeriodS: fv.PeriodS,
		LengthAmplitudeMM: fv.LengthAmplitudeMM,
		RadiusAmplitudeMM: fv.RadiusAmplitudeMM,
		TiltDeg:           fv.TiltDeg,
		PrecessionSpeed:   fv.PrecessionSpeed,
		ColorSpeed:        fv.ColorSpeed,
	}
}

type Trajectory struct {
	Waypoints []Pose
	DurationS float64
	Loop      bool
}

func (t Trajectory) ToAnimation() Animation {
	must(len(t.Waypoints) >= 2,
		"Trajectory needs at least 2 waypoints; got %d", len(t.Waypoints))
	return Animation{
		Mode: "trajectory", Waypoints: t.Waypoints,
		DurationS: t.DurationS, Loop: t.Loop, HasLoop: true,
	}
}

// ---- helpers -----------------------------------------------------------

func animOf(a AnimationSpec) Animation {
	if a == nil {
		return Animation{Mode: "none"}
	}
	return a.ToAnimation()
}

// fillPose makes sure OZ defaults to 1 (identity orientation) when
// hasOrient wasn't set. Mirrors PoseJSON.toPose behavior.
func fillPose(p Pose) Pose {
	if !p.hasOrient && p.OX == 0 && p.OY == 0 && p.OZ == 0 {
		p.OZ = 1.0
		p.hasOrient = true
	}
	return p
}

func must(cond bool, format string, args ...any) {
	if !cond {
		panic(fmt.Sprintf(format, args...))
	}
}
