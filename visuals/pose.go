// Package visuals — typed visual scene constructors for Viam.
//
// A small library for building Viam world-state-store scenes from
// typed Go values instead of hand-built Item literals. Each shape
// (Box, Sphere, Capsule, …) and animation (Spin, Pulse, Lifecycle, …)
// is a struct that validates its parameters at construction (via
// ToItem / ToAnimation) and produces the wire-format types the
// service consumes.
//
// Typical use:
//
//	import "exampleviz/visuals"
//
//	box := visuals.Box{Label: "demo_box",
//	    DimsMM: visuals.BoxDims{X: 100, Y: 200, Z: 50},
//	    Color: &visuals.Color{R: 230, G: 25, B: 75},
//	}
//	items := visuals.ToItems(box)
//
// This is the in-repo bootstrap version of the library. The public
// API here is stable; the eventual extraction to a standalone module
// github.com/viam-labs/viam-visuals will not change the surface.
package visuals

// Pose is position (mm) + orientation vector + theta.
//
// The Viam world-state-store wire format encodes each entity's pose
// as (x, y, z) in millimeters plus an orientation specified by an
// orientation vector (ox, oy, oz) and a rotation theta (in degrees)
// around that vector. Identity is OZ=1, everything else zero — the
// entity's local +Z aligns with world +Z.
type Pose struct {
	X, Y, Z    float64
	OX, OY, OZ float64
	Theta      float64
	// hasOrient tracks whether the orientation vector was explicitly
	// set, so zero-valued Pose literals default to identity (OZ=1)
	// rather than to a degenerate zero-orientation vector.
	hasOrient bool
}

// IdentityPose returns the identity pose: origin, OZ=1, theta=0.
func IdentityPose() Pose { return Pose{OZ: 1.0, hasOrient: true} }

// PoseXYZ builds a Pose with the given position and identity
// orientation. Convenience for the common case of "where" without
// "which way."
func PoseXYZ(x, y, z float64) Pose {
	return Pose{X: x, Y: y, Z: z, OZ: 1.0, hasOrient: true}
}

// PoseAt is the long-form Pose constructor — fields named so the
// call site reads top-to-bottom.
func PoseAt(x, y, z, ox, oy, oz, theta float64) Pose {
	return Pose{
		X: x, Y: y, Z: z,
		OX: ox, OY: oy, OZ: oz, Theta: theta,
		hasOrient: true,
	}
}

// fillPose ensures OZ defaults to 1 when orientation wasn't
// explicitly set. Used by the Visual constructors so a zero-value
// Pose{} reads as identity rather than a degenerate orientation
// vector.
func fillPose(p Pose) Pose {
	if !p.hasOrient && p.OX == 0 && p.OY == 0 && p.OZ == 0 {
		p.OZ = 1.0
		p.hasOrient = true
	}
	return p
}
