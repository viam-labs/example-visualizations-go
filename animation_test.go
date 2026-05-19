package exampleviz

import (
	"math"
	"testing"

	"github.com/viam-labs/viam-viz-helpers-go"
)

const eps = 1e-6

func nearly(a, b float64) bool { return math.Abs(a-b) < eps }

// ---- field-mask path constants — these are load-bearing -----------

func TestFieldMaskPathsAreCamelCase(t *testing.T) {
	cases := []struct {
		name, want string
		got        string
	}{
		{"Theta", "poseInObserverFrame.pose.theta", visuals.PathTheta},
		{"X", "poseInObserverFrame.pose.x", visuals.PathX},
		{"Y", "poseInObserverFrame.pose.y", visuals.PathY},
		{"Z", "poseInObserverFrame.pose.z", visuals.PathZ},
		{"OX", "poseInObserverFrame.pose.oX", visuals.PathOX},
		{"OY", "poseInObserverFrame.pose.oY", visuals.PathOY},
		{"OZ", "poseInObserverFrame.pose.oZ", visuals.PathOZ},
		{"SphereRadius", "physicalObject.geometryType.value.radiusMm", visuals.PathSphereRadius},
		{"BoxDimsX", "physicalObject.geometryType.value.dimsMm.x", visuals.PathBoxDimsX},
		{"MetaColor", "metadata.color", visuals.PathMetadataColor},
		{"MetaOpacity", "metadata.opacity", visuals.PathMetadataOpac},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// ---- orbit ---------------------------------------------------------

func TestOrbitAtTZeroIsOnXAxis(t *testing.T) {
	anim := visuals.Animation{Mode: "orbit", RadiusMM: 200, PeriodS: 4}
	res := ComputeTick("sphere", anim, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 50}, 0)
	if !nearly(res.Pose.X, 200) {
		t.Errorf("X = %v, want 200", res.Pose.X)
	}
	if !nearly(res.Pose.Y, 0) {
		t.Errorf("Y = %v, want 0", res.Pose.Y)
	}
	if len(res.Paths) != 2 || res.Paths[0] != visuals.PathX || res.Paths[1] != visuals.PathY {
		t.Errorf("paths = %v, want [%s %s]", res.Paths, visuals.PathX, visuals.PathY)
	}
}

func TestOrbitAtQuarterPeriodIsOnYAxis(t *testing.T) {
	anim := visuals.Animation{Mode: "orbit", RadiusMM: 200, PeriodS: 4}
	res := ComputeTick("sphere", anim, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 50}, 1.0)
	if math.Abs(res.Pose.X) > 1e-9 {
		t.Errorf("X = %v, want ~0", res.Pose.X)
	}
	if !nearly(res.Pose.Y, 200) {
		t.Errorf("Y = %v, want 200", res.Pose.Y)
	}
}

// ---- oscillate -----------------------------------------------------

func TestOscillateDefaultAxisIsY(t *testing.T) {
	anim := visuals.Animation{Mode: "oscillate", AmplitudeMM: 100, PeriodS: 4}
	res := ComputeTick("box", anim, visuals.IdentityPose(), visuals.BaseGeom{}, 1.0)
	if !nearly(res.Pose.Y, 100) {
		t.Errorf("Y = %v, want 100", res.Pose.Y)
	}
	if len(res.Paths) != 1 || res.Paths[0] != visuals.PathY {
		t.Errorf("paths = %v, want [%s]", res.Paths, visuals.PathY)
	}
}

func TestOscillateAxisX(t *testing.T) {
	anim := visuals.Animation{Mode: "oscillate", Axis: "x", AmplitudeMM: 250, PeriodS: 4}
	res := ComputeTick("box", anim, visuals.IdentityPose(), visuals.BaseGeom{}, 1.0)
	if !nearly(res.Pose.X, 250) {
		t.Errorf("X = %v, want 250", res.Pose.X)
	}
	if len(res.Paths) != 1 || res.Paths[0] != visuals.PathX {
		t.Errorf("paths = %v, want [%s]", res.Paths, visuals.PathX)
	}
}

// ---- spin ----------------------------------------------------------

func TestSpinAtQuarterPeriodIs90Deg(t *testing.T) {
	anim := visuals.Animation{Mode: "spin", PeriodS: 4}
	res := ComputeTick("capsule", anim, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 30, LengthMM: 200}, 1.0)
	if !nearly(res.Pose.Theta, 90) {
		t.Errorf("theta = %v, want 90", res.Pose.Theta)
	}
}

func TestSpinWrapsModulo360(t *testing.T) {
	anim := visuals.Animation{Mode: "spin", PeriodS: 4}
	res := ComputeTick("box", anim, visuals.IdentityPose(), visuals.BaseGeom{}, 4.5)
	if !nearly(res.Pose.Theta, 45) {
		t.Errorf("theta = %v, want 45", res.Pose.Theta)
	}
}

// ---- pulse ---------------------------------------------------------

func TestPulseSphereModulatesRadius(t *testing.T) {
	anim := visuals.Animation{Mode: "pulse", AmplitudeMM: 50, PeriodS: 4}
	res := ComputeTick("sphere", anim, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 100}, 1.0)
	if !nearly(res.Geom.RadiusMM, 150) {
		t.Errorf("radius = %v, want 150", res.Geom.RadiusMM)
	}
	if len(res.Paths) != 1 || res.Paths[0] != visuals.PathSphereRadius {
		t.Errorf("paths = %v", res.Paths)
	}
}

func TestPulseBoxAxisZOnlyModulatesZ(t *testing.T) {
	anim := visuals.Animation{Mode: "pulse", Axis: "z", AmplitudeMM: 100, PeriodS: 4}
	base := visuals.BaseGeom{Dims: visuals.BoxDims{X: 100, Y: 100, Z: 100}, HasDims: true}
	res := ComputeTick("box", anim, visuals.IdentityPose(), base, 1.0)
	if !nearly(res.Geom.Dims.Z, 200) {
		t.Errorf("z = %v, want 200", res.Geom.Dims.Z)
	}
	if !nearly(res.Geom.Dims.X, 100) || !nearly(res.Geom.Dims.Y, 100) {
		t.Errorf("non-z dims changed: %+v", res.Geom.Dims)
	}
	if len(res.Paths) != 1 || res.Paths[0] != visuals.PathBoxDimsZ {
		t.Errorf("paths = %v", res.Paths)
	}
}

// ---- breathe -------------------------------------------------------

func TestBreatheOscillatesOpacity(t *testing.T) {
	anim := visuals.Animation{Mode: "breathe", Amplitude: 0.3, PeriodS: 4}
	res := ComputeTick("capsule", anim, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 50, LengthMM: 200}, 1.0)
	if res.Overrides == nil || res.Overrides.Opacity == nil {
		t.Fatalf("expected visuals.Overrides.Opacity to be set")
	}
	if !nearly(*res.Overrides.Opacity, 1.0) {
		t.Errorf("opacity = %v, want 1.0", *res.Overrides.Opacity)
	}
	if len(res.Paths) != 1 || res.Paths[0] != visuals.PathMetadataOpac {
		t.Errorf("paths = %v", res.Paths)
	}
}

// ---- flicker -------------------------------------------------------

func TestFlickerInScene(t *testing.T) {
	a := visuals.Animation{Mode: "flicker", PeriodS: 4.0}
	b := visuals.Animation{Mode: "flicker", PeriodS: 4.0, PhaseOffsetS: 2.0}
	rA := ComputeTick("sphere", a, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 30}, 0)
	rB := ComputeTick("sphere", b, visuals.IdentityPose(), visuals.BaseGeom{RadiusMM: 30}, 0)
	if rA.Overrides == nil || rA.Overrides.InScene == nil || !*rA.Overrides.InScene {
		t.Errorf("A should be in_scene at t=0")
	}
	if rB.Overrides == nil || rB.Overrides.InScene == nil || *rB.Overrides.InScene {
		t.Errorf("B (offset 2.0) should be NOT in_scene at t=0")
	}
}

// ---- lifecycle -----------------------------------------------------

func TestLifecyclePhases(t *testing.T) {
	anim := visuals.Animation{Mode: "lifecycle",
		AppearS: 1, AliveS: 2, DisappearS: 1, GoneS: 2}
	base := visuals.BaseGeom{Dims: visuals.BoxDims{X: 100, Y: 100, Z: 100}, HasDims: true}
	// Appearing.
	r := ComputeTick("box", anim, visuals.IdentityPose(), base, 0.5)
	if r.Overrides == nil || r.Overrides.InScene == nil || !*r.Overrides.InScene {
		t.Fatal("expected in_scene at appearing phase")
	}
	if *r.Overrides.Color != visuals.LifecycleColorAppearing {
		t.Errorf("color = %v, want appearing", *r.Overrides.Color)
	}
	// Alive.
	r = ComputeTick("box", anim, visuals.IdentityPose(), base, 2.0)
	if *r.Overrides.Color != visuals.LifecycleColorAlive {
		t.Errorf("color = %v, want alive", *r.Overrides.Color)
	}
	// Disappearing.
	r = ComputeTick("box", anim, visuals.IdentityPose(), base, 3.5)
	if *r.Overrides.Color != visuals.LifecycleColorDisappearing {
		t.Errorf("color = %v, want disappearing", *r.Overrides.Color)
	}
	// Gone.
	r = ComputeTick("box", anim, visuals.IdentityPose(), base, 5.0)
	if r.Overrides.InScene == nil || *r.Overrides.InScene {
		t.Errorf("expected in_scene=false at gone phase")
	}
}

func TestLifecyclePhaseOffsetShiftsPhase(t *testing.T) {
	common := visuals.Animation{AppearS: 1, AliveS: 2, DisappearS: 1, GoneS: 2}
	a := common
	a.Mode = "lifecycle"
	b := common
	b.Mode = "lifecycle"
	b.PhaseOffsetS = 2.0
	base := visuals.BaseGeom{Dims: visuals.BoxDims{X: 100, Y: 100, Z: 100}, HasDims: true}
	rA := ComputeTick("box", a, visuals.IdentityPose(), base, 0.5)
	rB := ComputeTick("box", b, visuals.IdentityPose(), base, 0.5)
	if *rA.Overrides.Color != visuals.LifecycleColorAppearing {
		t.Errorf("A = %v, want appearing", *rA.Overrides.Color)
	}
	if *rB.Overrides.Color != visuals.LifecycleColorAlive {
		t.Errorf("B = %v, want alive", *rB.Overrides.Color)
	}
}

// ---- visuals.SupportedModes constant ---------------------------------------

func TestSupportedModesCovers11(t *testing.T) {
	want := map[string]bool{
		"none": true, "orbit": true, "oscillate": true, "spin": true,
		"swing": true, "pulse": true, "trajectory": true,
		"force_vector": true, "breathe": true, "flicker": true,
		"lifecycle": true,
	}
	if len(visuals.SupportedModes) != len(want) {
		t.Fatalf("got %d modes, want %d", len(visuals.SupportedModes), len(want))
	}
	for _, m := range visuals.SupportedModes {
		if !want[m] {
			t.Errorf("unexpected mode %q", m)
		}
	}
}

// ---- visuals.IsAnimated ----------------------------------------------------

func TestIsAnimatedNoneFalse(t *testing.T) {
	if visuals.IsAnimated(visuals.Animation{Mode: "none"}) {
		t.Error("none should be non-animated")
	}
}

func TestIsAnimatedTrueForAnyNonNone(t *testing.T) {
	for _, m := range []string{"orbit", "spin", "pulse", "lifecycle"} {
		if !visuals.IsAnimated(visuals.Animation{Mode: m}) {
			t.Errorf("%q should be animated", m)
		}
	}
}
