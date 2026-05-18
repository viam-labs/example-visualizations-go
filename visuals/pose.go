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

import "math"

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

// LerpPose interpolates between two poses.
//
// Position (X, Y, Z) is linearly interpolated. The orientation is
// interpolated by true quaternion SLERP — the orientation vector +
// theta is converted to a quaternion, SLERPed in SO(3), then
// converted back. This avoids the discontinuities of the naive
// lerp-and-normalize on (OX, OY, OZ) when the path passes through
// the OV singularity at |OZ| = 1 (where the renderer's roll
// reference is unstable).
//
// t should be in [0, 1]; values outside that range extrapolate (no
// clamping).
//
// Useful for motion-plan playback: feed in two adjacent waypoint
// poses from a planner output, interpolate the runner's pose at
// each tick, and the orientation visibly rotates between waypoints
// with no axis flips.
func LerpPose(a, b Pose, t float64) Pose {
	a = fillPose(a)
	b = fillPose(b)
	qa := ovToQuat(a.OX, a.OY, a.OZ, a.Theta)
	qb := ovToQuat(b.OX, b.OY, b.OZ, b.Theta)
	qi := slerpQuat(qa, qb, t)
	ox, oy, oz, theta := quatToOV(qi[0], qi[1], qi[2], qi[3])
	return Pose{
		X:         a.X + (b.X-a.X)*t,
		Y:         a.Y + (b.Y-a.Y)*t,
		Z:         a.Z + (b.Z-a.Z)*t,
		OX:        ox,
		OY:        oy,
		OZ:        oz,
		Theta:     theta,
		hasOrient: true,
	}
}

// ovToQuat converts a Viam orientation vector + theta (degrees) to a
// unit quaternion (w, x, y, z).
//
// Uses the ZYZ Euler decomposition: R = R_z(phi) R_y(delta) R_z(theta_rad)
// where phi = atan2(OY, OX) and delta = acos(OZ). At the singularity
// |OZ| ≈ 1 (local Z aligned with world ±Z), phi is undefined; we
// collapse it into theta.
func ovToQuat(ox, oy, oz, thetaDeg float64) [4]float64 {
	theta := thetaDeg * math.Pi / 180.0
	sinDeltaSq := ox*ox + oy*oy
	halfT := theta / 2
	if sinDeltaSq < 1e-12 {
		if oz >= 0 {
			return [4]float64{math.Cos(halfT), 0, 0, math.Sin(halfT)}
		}
		// oz=-1: 180° rotation around world Y composed with the roll.
		return [4]float64{0, -math.Sin(halfT), math.Cos(halfT), 0}
	}
	phi := math.Atan2(oy, ox)
	clampedOZ := oz
	if clampedOZ > 1 {
		clampedOZ = 1
	} else if clampedOZ < -1 {
		clampedOZ = -1
	}
	delta := math.Acos(clampedOZ)
	cp, sp := math.Cos(phi/2), math.Sin(phi/2)
	cd, sd := math.Cos(delta/2), math.Sin(delta/2)
	ct, st := math.Cos(halfT), math.Sin(halfT)
	a, b, c, d := cd*ct, sd*st, sd*ct, cd*st
	return [4]float64{
		cp*a - sp*d,
		cp*b - sp*c,
		cp*c + sp*b,
		cp*d + sp*a,
	}
}

// quatToOV is the inverse of ovToQuat. Returns (OX, OY, OZ, thetaDeg).
func quatToOV(w, x, y, z float64) (float64, float64, float64, float64) {
	ox := 2 * (x*z + w*y)
	oy := 2 * (y*z - w*x)
	oz := 1 - 2*(x*x+y*y)
	if n := math.Sqrt(ox*ox + oy*oy + oz*oz); n > 1e-9 {
		ox, oy, oz = ox/n, oy/n, oz/n
	}
	var theta float64
	sinDeltaSq := ox*ox + oy*oy
	if sinDeltaSq < 1e-12 {
		// At the singularity, phi and theta are degenerate — extract
		// the total rotation around world Z. The oz=-1 branch matches
		// the forward convention in ovToQuat.
		if oz >= 0 {
			theta = math.Atan2(2*(w*z+x*y), 1-2*(y*y+z*z))
		} else {
			theta = math.Atan2(2*(w*z-x*y), 2*(y*y+z*z)-1)
		}
	} else {
		r20 := 2 * (x*z - w*y)
		r21 := 2 * (y*z + w*x)
		theta = math.Atan2(r21, -r20)
	}
	return ox, oy, oz, theta * 180.0 / math.Pi
}

// slerpQuat does spherical linear interpolation between two unit
// quaternions, taking the shorter arc.
func slerpQuat(qa, qb [4]float64, t float64) [4]float64 {
	w1, x1, y1, z1 := qa[0], qa[1], qa[2], qa[3]
	w2, x2, y2, z2 := qb[0], qb[1], qb[2], qb[3]
	dot := w1*w2 + x1*x2 + y1*y2 + z1*z2
	if dot < 0 {
		w2, x2, y2, z2 = -w2, -x2, -y2, -z2
		dot = -dot
	}
	var w, x, y, z float64
	if dot > 0.9995 {
		w = w1 + (w2-w1)*t
		x = x1 + (x2-x1)*t
		y = y1 + (y2-y1)*t
		z = z1 + (z2-z1)*t
	} else {
		if dot > 1 {
			dot = 1
		} else if dot < -1 {
			dot = -1
		}
		theta0 := math.Acos(dot)
		sinTheta0 := math.Sin(theta0)
		theta := theta0 * t
		sinTheta := math.Sin(theta)
		s1 := math.Cos(theta) - dot*sinTheta/sinTheta0
		s2 := sinTheta / sinTheta0
		w = s1*w1 + s2*w2
		x = s1*x1 + s2*x2
		y = s1*y1 + s2*y2
		z = s1*z1 + s2*z2
	}
	n := math.Sqrt(w*w + x*x + y*y + z*z)
	return [4]float64{w / n, x / n, y / n, z / n}
}
