// Animation modes for the example-visualizations-go playground.
//
// An Item's Animation block selects a mode and per-mode params. At
// each tick, ComputeTick returns the per-item pose + geometry
// overrides for time t (seconds since the animation started) plus
// the field-mask paths the viewer needs in the UPDATED event.
//
// IMPORTANT: Paths are camelCase, not snake_case. The official
// worldstatestore guide says snake_case, but the renderer
// empirically only honors the camelCase form the RDK fake emits
// (rdk/services/worldstatestore/fake/moving_geos_world.go). Filed
// upstream — see LESSONS.md::snake-case-field-mask-paths-do-not-work.
package exampleviz

import (
	"math"
)

// The path constants, SupportedModes, lifecycle convention colors,
// Animation, IsAnimated, Overrides, and BoxDims types have moved to
// the `visuals` subpackage. This file now just hosts the tick code
// that consumes them. See aliases.go for the unqualified re-exports
// that keep the rest of the package's references working.

// BaseGeom holds the shape-specific base dim/radius/length fields.
// Only one set of fields is meaningful per shape type. The
// pcdBytesOverride field is a service-layer escape hatch for chunked
// delivery: when non-nil, the geometry builder for pointcloud items
// emits these bytes instead of reading the file fresh.
type BaseGeom struct {
	RadiusMM         float64
	LengthMM         float64
	Dims             BoxDims
	HasDims          bool
	pcdBytesOverride []byte
}

// TickResult is what ComputeTick returns.
type TickResult struct {
	Pose      Pose
	Geom      BaseGeom
	Paths     []string
	Overrides *Overrides
}

// ComputeTick is the pure per-tick animation function. Given an item
// (whose Type and Animation are read), base pose, base geometry,
// and time t in seconds since the animation started, returns:
//
//   - new pose: full pose with animation deltas composed onto base
//   - new geom: geometry overrides; fields not touched pass through
//   - paths: ordered list of field-mask paths for the UPDATED event
//   - overrides: optional metadata override (color/opacity/in_scene)
func ComputeTick(itemType string, anim Animation, base Pose, baseGeom BaseGeom, t float64) TickResult {
	newPose := base
	if newPose.OX == 0 && newPose.OY == 0 && newPose.OZ == 0 {
		newPose.OZ = 1.0
	}
	newGeom := baseGeom

	mode := anim.Mode
	if mode == "" {
		mode = "none"
	}

	switch mode {
	case "none":
		return TickResult{Pose: newPose, Geom: newGeom}

	case "orbit":
		radius := anim.RadiusMM
		if radius == 0 {
			radius = 100
		}
		period := anim.PeriodS
		if period <= 0 {
			period = 5
		}
		angle := 2 * math.Pi * t / period
		newPose.X = base.X + radius*math.Cos(angle)
		newPose.Y = base.Y + radius*math.Sin(angle)
		return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathX, PathY}}

	case "oscillate":
		axis := anim.Axis
		if !contains(SupportedAxes, axis) {
			axis = "y"
		}
		amp := anim.AmplitudeMM
		if amp == 0 {
			amp = 100
		}
		period := anim.PeriodS
		if period <= 0 {
			period = 4
		}
		delta := amp * math.Sin(2*math.Pi*t/period)
		switch axis {
		case "x":
			newPose.X = base.X + delta
			return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathX}}
		case "z":
			newPose.Z = base.Z + delta
			return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathZ}}
		default:
			newPose.Y = base.Y + delta
			return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathY}}
		}

	case "spin":
		period := anim.PeriodS
		if period <= 0 {
			period = 4
		}
		theta := math.Mod(360.0*t/period, 360.0)
		newPose.Theta = theta
		return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathTheta}}

	case "swing":
		ampDeg := anim.AmplitudeDeg
		if ampDeg == 0 {
			ampDeg = 45
		}
		period := anim.PeriodS
		if period <= 0 {
			period = 4
		}
		baseTheta := base.Theta
		newPose.Theta = baseTheta + ampDeg*math.Sin(2*math.Pi*t/period)
		return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathTheta}}

	case "pulse":
		amp := anim.AmplitudeMM
		if amp == 0 {
			amp = 25
		}
		period := anim.PeriodS
		if period <= 0 {
			period = 3
		}
		delta := amp * math.Sin(2*math.Pi*t/period)
		switch itemType {
		case "sphere":
			newGeom.RadiusMM = math.Max(0.1, baseGeom.RadiusMM+delta)
			return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathSphereRadius}}
		case "capsule":
			newGeom.RadiusMM = math.Max(0.1, baseGeom.RadiusMM+delta)
			newGeom.LengthMM = math.Max(0.1, baseGeom.LengthMM+delta)
			return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathCapsuleRadius, PathCapsuleLength}}
		case "box":
			boxAxis := anim.Axis
			d := baseGeom.Dims
			switch boxAxis {
			case "x":
				newGeom.Dims = BoxDims{X: math.Max(0.1, d.X+delta), Y: d.Y, Z: d.Z}
				newGeom.HasDims = true
				return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathBoxDimsX}}
			case "y":
				newGeom.Dims = BoxDims{X: d.X, Y: math.Max(0.1, d.Y+delta), Z: d.Z}
				newGeom.HasDims = true
				return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathBoxDimsY}}
			case "z":
				newGeom.Dims = BoxDims{X: d.X, Y: d.Y, Z: math.Max(0.1, d.Z+delta)}
				newGeom.HasDims = true
				return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathBoxDimsZ}}
			default:
				newGeom.Dims = BoxDims{
					X: math.Max(0.1, d.X+delta),
					Y: math.Max(0.1, d.Y+delta),
					Z: math.Max(0.1, d.Z+delta),
				}
				newGeom.HasDims = true
				return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{PathBoxDimsX, PathBoxDimsY, PathBoxDimsZ}}
			}
		default:
			return TickResult{Pose: newPose, Geom: newGeom}
		}

	case "trajectory":
		if len(anim.Waypoints) < 2 {
			return TickResult{Pose: newPose, Geom: newGeom}
		}
		duration := anim.DurationS
		if duration <= 0 {
			duration = 8
		}
		loop := true
		if anim.HasLoop {
			loop = anim.Loop
		}
		var progress float64
		if loop {
			progress = math.Mod(t, duration) / duration
		} else {
			progress = math.Max(0, math.Min(1, t/duration))
		}
		nSeg := len(anim.Waypoints) - 1
		segProgress := progress * float64(nSeg)
		segIdx := int(segProgress)
		var alpha float64
		if segIdx >= nSeg {
			segIdx = nSeg - 1
			alpha = 1.0
		} else {
			alpha = segProgress - float64(segIdx)
		}
		a := anim.Waypoints[segIdx]
		b := anim.Waypoints[segIdx+1]
		newPose.X = lerp(a.X, b.X, alpha)
		newPose.Y = lerp(a.Y, b.Y, alpha)
		newPose.Z = lerp(a.Z, b.Z, alpha)
		ox := lerp(a.OX, b.OX, alpha)
		oy := lerp(a.OY, b.OY, alpha)
		oz := lerp(a.OZ, b.OZ, alpha)
		if a.OZ == 0 && a.OX == 0 && a.OY == 0 {
			oz = lerp(1.0, b.OZ, alpha)
		}
		if b.OZ == 0 && b.OX == 0 && b.OY == 0 {
			oz = lerp(a.OZ, 1.0, alpha)
		}
		norm := math.Sqrt(ox*ox + oy*oy + oz*oz)
		if norm > 1e-9 {
			newPose.OX, newPose.OY, newPose.OZ = ox/norm, oy/norm, oz/norm
		} else {
			newPose.OX, newPose.OY, newPose.OZ = 0, 0, 1
		}
		newPose.Theta = lerp(a.Theta, b.Theta, alpha)
		return TickResult{Pose: newPose, Geom: newGeom, Paths: []string{
			PathX, PathY, PathZ, PathOX, PathOY, PathOZ, PathTheta,
		}}

	case "force_vector":
		period := anim.PeriodS
		if period <= 0 {
			period = 4
		}
		phase := 2 * math.Pi * t / period
		lenAmp := anim.LengthAmplitudeMM
		if lenAmp == 0 {
			lenAmp = 60
		}
		baseLen := baseGeom.LengthMM
		if baseLen == 0 {
			baseLen = 200
		}
		newGeom.LengthMM = math.Max(0.1, baseLen+lenAmp*math.Sin(phase))
		radAmp := anim.RadiusAmplitudeMM
		if radAmp == 0 {
			radAmp = 4
		}
		baseRad := baseGeom.RadiusMM
		if baseRad == 0 {
			baseRad = 10
		}
		newGeom.RadiusMM = math.Max(0.1, baseRad+radAmp*math.Sin(phase+math.Pi/3))
		tilt := anim.TiltDeg
		if tilt == 0 {
			tilt = 45
		}
		tiltRad := tilt * math.Pi / 180.0
		precSpeed := anim.PrecessionSpeed
		if precSpeed == 0 {
			precSpeed = 1
		}
		precAngle := phase * precSpeed
		newPose.OX = math.Sin(tiltRad) * math.Cos(precAngle)
		newPose.OY = math.Sin(tiltRad) * math.Sin(precAngle)
		newPose.OZ = math.Cos(tiltRad)
		newPose.Theta = 0
		colorSpeed := anim.ColorSpeed
		if colorSpeed == 0 {
			colorSpeed = 1
		}
		hue := math.Mod(t*colorSpeed/period, 1.0)
		r, g, b := hsvToRGBu8(hue, 1, 1)
		col := Color{R: r, G: g, B: b}
		return TickResult{Pose: newPose, Geom: newGeom,
			Paths: []string{
				PathCapsuleLength, PathSphereRadius,
				PathOX, PathOY, PathOZ, PathTheta,
				PathMetadataColor,
			},
			Overrides: &Overrides{Color: &col},
		}

	case "breathe":
		period := anim.PeriodS
		if period <= 0 {
			period = 4
		}
		amp := anim.Amplitude
		if amp == 0 {
			amp = 0.4
		}
		// baseOpacity isn't carried in BaseGeom; defaults to 1.0 if
		// the item didn't set it. Callers can plumb item.Opacity if
		// they want this to compose onto a non-1.0 base.
		baseOp := 1.0
		op := baseOp + amp*math.Sin(2*math.Pi*t/period)
		if op < 0 {
			op = 0
		}
		if op > 1 {
			op = 1
		}
		return TickResult{Pose: newPose, Geom: newGeom,
			Paths:     []string{PathMetadataOpac},
			Overrides: &Overrides{Opacity: &op},
		}

	case "flicker":
		period := anim.PeriodS
		if period <= 0 {
			period = 3
		}
		duty := anim.DutyCycle
		if duty == 0 {
			duty = 0.5
		}
		if duty < 0 {
			duty = 0
		}
		if duty > 1 {
			duty = 1
		}
		phaseOff := anim.PhaseOffsetS
		phase := math.Mod(t+phaseOff, period) / period
		inScene := phase < duty
		return TickResult{Pose: newPose, Geom: newGeom,
			Overrides: &Overrides{InScene: &inScene},
		}

	case "lifecycle":
		appear := anim.AppearS
		if appear == 0 {
			appear = 1
		}
		alive := anim.AliveS
		if alive == 0 {
			alive = 2
		}
		disappear := anim.DisappearS
		if disappear == 0 {
			disappear = 1
		}
		gone := anim.GoneS
		if gone == 0 {
			gone = 2
		}
		phaseOff := anim.PhaseOffsetS
		loop := true
		if anim.HasLoop {
			loop = anim.Loop
		}
		periodTotal := appear + alive + disappear + gone
		if periodTotal <= 0 {
			return TickResult{Pose: newPose, Geom: newGeom}
		}
		var phaseT float64
		if loop {
			phaseT = math.Mod(t+phaseOff, periodTotal)
		} else {
			phaseT = math.Min(t+phaseOff, periodTotal)
		}
		var color Color
		var opacity float64
		inScene := true
		switch {
		case phaseT < appear:
			color = LifecycleColorAppearing
			opacity = LifecycleOpacityAppearing
		case phaseT < appear+alive:
			color = LifecycleColorAlive
			opacity = LifecycleOpacityAlive
		case phaseT < appear+alive+disappear:
			color = LifecycleColorDisappearing
			opacity = LifecycleOpacityDispearing
		default:
			color = LifecycleColorDisappearing
			opacity = 0
			inScene = false
		}
		return TickResult{Pose: newPose, Geom: newGeom,
			Paths: []string{PathMetadataColor, PathMetadataOpac},
			Overrides: &Overrides{
				Color:   &color,
				Opacity: &opacity,
				InScene: &inScene,
			},
		}
	}

	// Unknown mode falls through as static.
	return TickResult{Pose: newPose, Geom: newGeom}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func lerp(a, b, alpha float64) float64 {
	return a + alpha*(b-a)
}

func hsvToRGBu8(h, s, v float64) (int, int, int) {
	i := int(h*6) % 6
	f := h*6 - float64(int(h*6))
	p := v * (1 - s)
	q := v * (1 - s*f)
	tt := v * (1 - s*(1-f))
	var r, g, b float64
	switch i {
	case 0:
		r, g, b = v, tt, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, tt
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = tt, p, v
	default:
		r, g, b = v, p, q
	}
	return int(r*255) & 0xFF, int(g*255) & 0xFF, int(b*255) & 0xFF
}
