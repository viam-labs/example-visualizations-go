// Imperative pose-composing helpers for animation.
//
// These functions take a "base" pose (the rest state of the entity)
// plus a time t (elapsed seconds), and return a new Pose with the
// appropriate animation math applied. They're pure — the input
// base is not mutated.
//
// Pair with SceneServiceBase.SceneTick:
//
//	type myService struct {
//	    resource.Named
//	    resource.TriviallyCloseable
//	    visuals.SceneServiceBase
//	    box      *visuals.Box
//	    basePose visuals.Pose
//	}
//
//	func (s *myService) SceneTick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
//	    s.box.Pose = visuals.SpinPose(s.basePose, 3.0, t)
//	    events, _ := scene.Update(s.box)
//	    return events
//	}
//
// The helpers below cover the simple "absolute t → pose" animation
// modes. For animations that mutate non-pose fields (radius pulse,
// color cycle, etc.), mutate the field directly in your SceneTick;
// no helper is needed.
package visuals

import (
	"fmt"
	"math"
)

// SpinPose returns base with Theta set to a continuous rotation of
// 360°/periodS per second.
//
// Theta = (360° × t / periodS) mod 360° — absolute (base.Theta is
// ignored), so callers don't need to track accumulated angle.
func SpinPose(base Pose, periodS, t float64) Pose {
	b := fillPose(base)
	b.Theta = math.Mod(360.0*t/periodS, 360.0)
	return b
}

// OrbitPose returns base with position orbiting in a circle of
// radiusMM around base's position, in the plane perpendicular to
// axis. Default axis is "z" (orbit in world XY). Use "y" for XZ
// (vertical loop) or "x" for YZ.
func OrbitPose(base Pose, periodS, radiusMM, t float64, axis string) Pose {
	b := fillPose(base)
	phase := 2 * math.Pi * t / periodS
	c, s := math.Cos(phase), math.Sin(phase)
	switch axis {
	case "z", "":
		b.X += radiusMM * c
		b.Y += radiusMM * s
	case "y":
		b.X += radiusMM * c
		b.Z += radiusMM * s
	case "x":
		b.Y += radiusMM * c
		b.Z += radiusMM * s
	default:
		panic(fmt.Sprintf("axis must be 'x', 'y', or 'z'; got %q", axis))
	}
	return b
}

// OscillatePose returns base with one position axis offset by
// amplitudeMM × sin(2π t / periodS). Pass axis="x"/"y"/"z" (default
// "y").
func OscillatePose(base Pose, periodS, amplitudeMM, t float64, axis string) Pose {
	b := fillPose(base)
	delta := amplitudeMM * math.Sin(2*math.Pi*t/periodS)
	switch axis {
	case "x":
		b.X += delta
	case "y", "":
		b.Y += delta
	case "z":
		b.Z += delta
	default:
		panic(fmt.Sprintf("axis must be 'x', 'y', or 'z'; got %q", axis))
	}
	return b
}

// SwingPose returns base with theta swinging sinusoidally around
// base.Theta:
//
//	theta = base.Theta + amplitudeDeg × sin(2π t / periodS)
//
// Unlike SpinPose, swing is relative to base.Theta — useful for
// pendulum-like motion where the "rest" theta matters.
func SwingPose(base Pose, periodS, amplitudeDeg, t float64) Pose {
	b := fillPose(base)
	b.Theta = b.Theta + amplitudeDeg*math.Sin(2*math.Pi*t/periodS)
	return b
}
