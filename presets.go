// Named scene bundles. Each preset is a function returning a list
// of Item dicts in the same shape the config schema produces.
// Returning Items (not protos) keeps presets serializable for the
// snapshot DoCommand round-trip.
package exampleviz

import (
	"fmt"
	"math"
)

// PrimitiveRowSpacingMM — X-axis spacing between primitives in the
// row-style preset.
const PrimitiveRowSpacingMM = 400.0

// Presets — registry of named bundle functions.
var Presets = map[string]func() []Item{
	"all":                 allPreset,
	"primitives":          primitivesPreset,
	"orientation_vectors": orientationVectorsPreset,
	"frame_composition":   frameCompositionPreset,
	"trajectory_preview":  trajectoryPreviewPreset,
	"force_vector_demo":   forceVectorDemoPreset,
	"geometry_morph":      geometryMorphPreset,
	"lifecycle_demo":      lifecycleDemoPreset,
	"chunked_pcd_demo":    chunkedPCDDemoPreset,
}

func ptr[T any](v T) *T { return &v }

func identityPose() Pose            { return Pose{OZ: 1.0} }
func poseXY(x, y float64) Pose      { return Pose{X: x, Y: y, OZ: 1.0} }
func poseAt(x, y, z float64) Pose   { return Pose{X: x, Y: y, Z: z, OZ: 1.0} }

// ---- primitives ------------------------------------------------------

func primitivesPreset() []Item {
	sp := PrimitiveRowSpacingMM
	return []Item{
		{
			Type: "box", Label: "demo_box",
			Pose: poseAt(-4*sp, 0, 0), HasDims: true,
			DimsMM:  BoxDims{X: 150, Y: 150, Z: 150},
			Color:   &Color{R: 230, G: 25, B: 75},
			Opacity: ptr(1.0),
		},
		{
			Type: "sphere", Label: "demo_sphere",
			Pose:     poseAt(-3*sp, 0, 0),
			RadiusMM: 90,
			Color:    &Color{R: 60, G: 180, B: 75},
			Opacity:  ptr(1.0),
		},
		{
			Type: "capsule", Label: "demo_capsule",
			Pose:     poseAt(-2*sp, 0, 0),
			RadiusMM: 50, LengthMM: 200,
			Color:   &Color{R: 0, G: 130, B: 200},
			Opacity: ptr(1.0),
		},
		{
			Type: "point", Label: "demo_point",
			Pose:    poseAt(-1*sp, 0, 0),
			Color:   &Color{R: 255, G: 225, B: 25},
			Opacity: ptr(1.0),
		},
		{
			Type: "arrow", Label: "demo_arrow",
			Pose:     poseAt(0, 0, 0),
			LengthMM: 220, RadiusMM: 12,
			Color:   &Color{R: 145, G: 30, B: 180},
			Opacity: ptr(1.0),
		},
		{
			Type: "mesh", Label: "demo_icosahedron",
			Pose:     poseAt(1*sp, 0, 0),
			MeshPath: "assets/icosahedron.ply",
			Color:    &Color{R: 240, G: 50, B: 230},
			Opacity:  ptr(1.0),
		},
		{
			Type: "mesh", Label: "demo_bunny",
			Pose:     poseAt(2*sp, 0, 0),
			MeshPath: "assets/bunny.stl",
			Color:    &Color{R: 245, G: 130, B: 49},
			Opacity:  ptr(1.0),
		},
		{
			Type: "mesh", Label: "demo_torus",
			Pose:     poseAt(3*sp, 0, 0),
			MeshPath: "assets/torus.ply",
			Color:    &Color{R: 70, G: 240, B: 240},
			Opacity:  ptr(1.0),
		},
		{
			Type: "mesh", Label: "demo_teapot",
			Pose:     poseAt(4*sp, 0, 0),
			MeshPath: "assets/teapot.ply",
			Color:    &Color{R: 60, G: 180, B: 75},
			Opacity:  ptr(1.0),
		},
		{
			Type: "pointcloud", Label: "demo_colorful_sphere",
			Pose:           poseAt(5*sp, 0, 0),
			PointcloudPath: "assets/colorful_sphere.pcd",
			Opacity:        ptr(1.0),
		},
		{
			Type: "pointcloud", Label: "demo_pointcloud",
			Pose:           poseAt(6*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd",
			Opacity:        ptr(1.0),
		},
		{
			Type: "pointcloud", Label: "demo_pointcloud_chunked",
			Pose:           poseAt(7*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd",
			Opacity:        ptr(1.0),
			Chunked:        true,
			ChunkSize:      2000,
		},
	}
}

// ---- orientation vectors ---------------------------------------------

func orientationVectorsPreset() []Item {
	sp := PrimitiveRowSpacingMM
	const hostR = 18.0
	mk := func(label string, pose Pose) Item {
		return Item{
			Type: "sphere", Label: label, Pose: pose,
			RadiusMM:       hostR,
			Color:          &Color{R: 220, G: 220, B: 220},
			Opacity:        ptr(0.35),
			ShowAxesHelper: true,
		}
	}
	out := []Item{}
	out = append(out, mk("frame_+Z", Pose{X: -2 * sp, OZ: 1}))
	out = append(out, mk("frame_+X", Pose{X: -sp, OX: 1}))
	out = append(out, mk("frame_+Y", Pose{X: 0, OY: 1}))
	s := 1.0 / math.Sqrt(2)
	out = append(out, mk("frame_+XY", Pose{X: sp, OX: s, OY: s}))
	out = append(out, mk("frame_+Z_theta45", Pose{X: 2 * sp, OZ: 1, Theta: 45}))
	return out
}

// ---- reference frame demo (component of frame_composition) -----------

func referenceFrameDemo() []Item {
	const axisLength = 200.0
	const axisRadius = 12.0
	half := axisLength / 2.0
	out := []Item{
		// Anchor — spins around Z; children inherit.
		{
			Type: "sphere", Label: "spinning_frame",
			Pose:           identityPose(),
			RadiusMM:       12,
			Color:          &Color{R: 255, G: 255, B: 255},
			Opacity:        ptr(0.6),
			ShowAxesHelper: true,
			Animation:      Animation{Mode: "spin", PeriodS: 6},
		},
		// +X axis (red).
		{
			Type: "capsule", Label: "spinning_frame_axis_x", ParentFrame: "spinning_frame",
			Pose:     Pose{X: half, OX: 1},
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color:   &Color{R: 230, G: 25, B: 75},
			Opacity: ptr(1.0),
		},
		// +Y axis (green).
		{
			Type: "capsule", Label: "spinning_frame_axis_y", ParentFrame: "spinning_frame",
			Pose:     Pose{Y: half, OY: 1},
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color:   &Color{R: 60, G: 180, B: 75},
			Opacity: ptr(1.0),
		},
		// +Z axis (blue).
		{
			Type: "capsule", Label: "spinning_frame_axis_z", ParentFrame: "spinning_frame",
			Pose:     Pose{Z: half, OZ: 1},
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color:   &Color{R: 0, G: 130, B: 200},
			Opacity: ptr(1.0),
		},
		// Attached mesh — spins on its own axis at a different rate.
		{
			Type: "mesh", Label: "spinning_frame_attached_mesh", ParentFrame: "spinning_frame",
			Pose:      Pose{X: 700, OZ: 1},
			MeshPath:  "assets/icosahedron.ply",
			Color:     &Color{R: 240, G: 200, B: 50},
			Opacity:   ptr(1.0),
			Animation: Animation{Mode: "spin", PeriodS: 2},
		},
		// Invisible wheel hub — spins on its own axis.
		{
			Type: "sphere", Label: "spinning_frame_wheel_hub",
			ParentFrame: "spinning_frame_attached_mesh",
			Pose:        identityPose(),
			RadiusMM:    4,
			Color:       &Color{R: 255, G: 255, B: 255},
			Opacity:     ptr(0.0),
			Animation:   Animation{Mode: "spin", PeriodS: 10},
		},
	}
	out = append(out, colorWheelChildren("spinning_frame_wheel_hub", 10, 220.0, 24.0)...)
	return out
}

func colorWheelChildren(parent string, count int, ringR, sphereR float64) []Item {
	out := []Item{}
	for i := 0; i < count; i++ {
		hue := float64(i) / float64(count)
		r, g, b := hsvToRGBu8(hue, 1, 1)
		angle := 2 * math.Pi * float64(i) / float64(count)
		out = append(out, Item{
			Type:        "sphere",
			Label:       fmt.Sprintf("%s_wheel_%02d", parent, i),
			ParentFrame: parent,
			Pose: Pose{
				X:  ringR * math.Cos(angle),
				Y:  ringR * math.Sin(angle),
				OZ: 1,
			},
			RadiusMM: sphereR,
			Color:    &Color{R: r, G: g, B: b},
			Opacity:  ptr(1.0),
		})
	}
	return out
}

// ---- robot arm (component of frame_composition) ----------------------

func robotArm() []Item {
	const linkR = 25.0
	const baseH = 80.0
	const upperL = 220.0
	const forearmL = 180.0
	const jointR = 35.0
	const palmThick = 10.0
	const fingerL = 70.0
	const fingerThick = 8.0

	out := []Item{
		{
			Type: "capsule", Label: "arm_base",
			Pose:     Pose{Z: baseH / 2, OZ: 1},
			RadiusMM: linkR * 1.6, LengthMM: baseH,
			Color:     &Color{R: 70, G: 70, B: 75},
			Opacity:   ptr(1.0),
			Animation: Animation{Mode: "swing", AmplitudeDeg: 75.0, PeriodS: 8},
		},
		{
			Type: "sphere", Label: "arm_shoulder", ParentFrame: "arm_base",
			Pose: Pose{
				Z: baseH/2 + linkR, OY: 1,
			},
			RadiusMM: jointR,
			Color:    &Color{R: 230, G: 25, B: 75},
			Opacity:  ptr(1.0),
		},
		{
			Type: "capsule", Label: "arm_upper", ParentFrame: "arm_shoulder",
			Pose:     Pose{Z: upperL / 2, OZ: 1},
			RadiusMM: linkR, LengthMM: upperL,
			Color:   &Color{R: 100, G: 130, B: 200},
			Opacity: ptr(1.0),
		},
		{
			Type: "sphere", Label: "arm_elbow", ParentFrame: "arm_upper",
			Pose: Pose{Z: upperL/2 + linkR, OY: 1, Theta: -60},
			RadiusMM:  jointR * 0.8,
			Color:     &Color{R: 230, G: 25, B: 75},
			Opacity:   ptr(1.0),
			Animation: Animation{Mode: "swing", AmplitudeDeg: 50.0, PeriodS: 5},
		},
		{
			Type: "capsule", Label: "arm_forearm", ParentFrame: "arm_elbow",
			Pose:     Pose{Z: forearmL / 2, OZ: 1},
			RadiusMM: linkR * 0.85, LengthMM: forearmL,
			Color:   &Color{R: 100, G: 180, B: 110},
			Opacity: ptr(1.0),
		},
		{
			Type: "sphere", Label: "arm_wrist", ParentFrame: "arm_forearm",
			Pose:      Pose{Z: forearmL/2 + linkR*0.6, OZ: 1},
			RadiusMM:  jointR * 0.65,
			Color:     &Color{R: 230, G: 25, B: 75},
			Opacity:   ptr(1.0),
			Animation: Animation{Mode: "swing", AmplitudeDeg: 90.0, PeriodS: 6},
		},
		{
			Type: "box", Label: "claw_palm", ParentFrame: "arm_wrist",
			Pose:    Pose{Z: jointR*0.6 + palmThick/2, OZ: 1},
			HasDims: true,
			DimsMM:  BoxDims{X: 70, Y: 28, Z: palmThick},
			Color:   &Color{R: 220, G: 220, B: 70},
			Opacity: ptr(1.0),
		},
		{
			Type: "box", Label: "claw_left_finger", ParentFrame: "claw_palm",
			Pose:    Pose{X: -22, Z: palmThick/2 + fingerL/2, OZ: 1},
			HasDims: true,
			DimsMM:  BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color:   &Color{R: 220, G: 220, B: 70},
			Opacity: ptr(1.0),
			Animation: Animation{
				Mode: "oscillate", Axis: "x", AmplitudeMM: -10.0, PeriodS: 3,
			},
		},
		{
			Type: "box", Label: "claw_right_finger", ParentFrame: "claw_palm",
			Pose:    Pose{X: 22, Z: palmThick/2 + fingerL/2, OZ: 1},
			HasDims: true,
			DimsMM:  BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color:   &Color{R: 220, G: 220, B: 70},
			Opacity: ptr(1.0),
			Animation: Animation{
				Mode: "oscillate", Axis: "x", AmplitudeMM: 10.0, PeriodS: 3,
			},
		},
	}
	return out
}

// ---- frame composition -----------------------------------------------

func frameCompositionPreset() []Item {
	out := []Item{}
	out = append(out, offsetBaseItems(referenceFrameDemo(), "x", -1000.0)...)
	out = append(out, offsetBaseItems(robotArm(), "x", 1000.0)...)
	return out
}

// ---- trajectory preview ----------------------------------------------

func trajectoryPreviewPreset() []Item {
	positions := []Pose{
		{X: 0, Y: 0, Z: 0, Theta: 0},
		{X: 300, Y: 150, Z: 200, Theta: 120},
		{X: 500, Y: 300, Z: 300, Theta: 240},
		{X: 700, Y: 150, Z: 200, Theta: 120},
		{X: 1000, Y: 0, Z: 0, Theta: 0},
	}
	waypoints := waypointsWithTangentOrientations(positions)
	out := []Item{}
	for i, wp := range waypoints {
		out = append(out, Item{
			Type:           "sphere",
			Label:          fmt.Sprintf("traj_wp_%02d", i),
			Pose:           wp,
			RadiusMM:       18,
			Color:          &Color{R: 200, G: 200, B: 220},
			Opacity:        ptr(0.45),
			ShowAxesHelper: true,
		})
	}
	for i := 0; i < len(waypoints)-1; i++ {
		a := waypoints[i]
		b := waypoints[i+1]
		dx, dy, dz := b.X-a.X, b.Y-a.Y, b.Z-a.Z
		segLen := math.Sqrt(dx*dx + dy*dy + dz*dz)
		if segLen < 1e-6 {
			continue
		}
		out = append(out, Item{
			Type:  "capsule",
			Label: fmt.Sprintf("traj_seg_%02d", i),
			Pose: Pose{
				X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2, Z: (a.Z + b.Z) / 2,
				OX: dx / segLen, OY: dy / segLen, OZ: dz / segLen,
			},
			RadiusMM: 5, LengthMM: segLen,
			Color:   &Color{R: 100, G: 130, B: 240},
			Opacity: ptr(0.95),
		})
	}
	out = append(out, Item{
		Type:           "sphere",
		Label:          "traj_runner",
		Pose:           waypoints[0],
		RadiusMM:       28,
		Color:          &Color{R: 230, G: 40, B: 80},
		Opacity:        ptr(0.9),
		ShowAxesHelper: true,
		Animation: Animation{
			Mode:      "trajectory",
			Waypoints: waypoints,
			DurationS: 12.0,
			Loop:      true,
			HasLoop:   true,
		},
	})
	return out
}

func waypointsWithTangentOrientations(positions []Pose) []Pose {
	n := len(positions)
	out := make([]Pose, 0, n)
	for i := range positions {
		p := positions[i]
		var tx, ty, tz float64
		switch {
		case i == 0:
			tx = positions[1].X - p.X
			ty = positions[1].Y - p.Y
			tz = positions[1].Z - p.Z
		case i == n-1:
			tx = p.X - positions[i-1].X
			ty = p.Y - positions[i-1].Y
			tz = p.Z - positions[i-1].Z
		default:
			tx = positions[i+1].X - positions[i-1].X
			ty = positions[i+1].Y - positions[i-1].Y
			tz = positions[i+1].Z - positions[i-1].Z
		}
		norm := math.Sqrt(tx*tx + ty*ty + tz*tz)
		if norm == 0 {
			norm = 1
		}
		out = append(out, Pose{
			X: p.X, Y: p.Y, Z: p.Z,
			OX: tx / norm, OY: ty / norm, OZ: tz / norm,
			Theta: p.Theta,
		})
	}
	return out
}

// ---- force vector demo -----------------------------------------------

func forceVectorDemoPreset() []Item {
	return []Item{{
		Type: "arrow", Label: "force_vector",
		Pose:     identityPose(),
		LengthMM: 220, RadiusMM: 10,
		Color:   &Color{R: 230, G: 60, B: 100},
		Opacity: ptr(1.0),
		Animation: Animation{
			Mode:              "force_vector",
			PeriodS:           5.0,
			LengthAmplitudeMM: 80,
			RadiusAmplitudeMM: 5,
			TiltDeg:           45,
			PrecessionSpeed:   1.0,
			ColorSpeed:        0.7,
		},
	}}
}

// ---- geometry morph --------------------------------------------------

func geometryMorphPreset() []Item {
	out := []Item{}
	slotX := 0.0

	out = append(out, Item{
		Type: "sphere", Label: "morph_pulse_sphere",
		Pose:     poseAt(slotX, 0, 0),
		RadiusMM: 70,
		Color:    &Color{R: 230, G: 60, B: 100},
		Opacity:  ptr(1.0),
		Animation: Animation{
			Mode: "pulse", AmplitudeMM: 35, PeriodS: 3,
		},
	})
	slotX += 350

	out = append(out, Item{
		Type: "box", Label: "morph_stretch_box",
		Pose:    poseAt(slotX, 0, 0),
		HasDims: true,
		DimsMM:  BoxDims{X: 100, Y: 100, Z: 150},
		Color:   &Color{R: 100, G: 180, B: 230},
		Opacity: ptr(1.0),
		Animation: Animation{
			Mode: "pulse", Axis: "z", AmplitudeMM: 100, PeriodS: 4,
		},
	})
	slotX += 350

	out = append(out, Item{
		Type: "capsule", Label: "morph_breathe_capsule",
		Pose:     poseAt(slotX, 0, 0),
		RadiusMM: 45, LengthMM: 240,
		Color:   &Color{R: 220, G: 200, B: 60},
		Opacity: ptr(0.7),
		Animation: Animation{
			Mode: "breathe", Amplitude: 0.55, PeriodS: 1.5,
		},
	})
	slotX += 380

	gridN := 5
	gridSpacing := 80.0
	periodS := 4.0

	addGrid := func(prefix string, originX float64, col Color, rotateUUID bool) {
		for row := 0; row < gridN; row++ {
			for col2 := 0; col2 < gridN; col2++ {
				phaseOff := float64(row+col2) / float64(2*gridN-1) * periodS
				ru := rotateUUID
				out = append(out, Item{
					Type:  "sphere",
					Label: fmt.Sprintf("%s_%d%d", prefix, row, col2),
					Pose: poseAt(
						originX+float64(col2)*gridSpacing,
						float64(row)*gridSpacing,
						0,
					),
					RadiusMM: 22,
					Color:    &col,
					Opacity:  ptr(1.0),
					Animation: Animation{
						Mode:              "flicker",
						PeriodS:           periodS,
						DutyCycle:         0.55,
						PhaseOffsetS:      phaseOff,
						RotateUUIDOnReadd: &ru,
					},
				})
			}
		}
	}
	workingOriginX := slotX + 60
	addGrid("morph_grid", workingOriginX, Color{R: 80, G: 200, B: 140}, true)
	brokenOriginX := workingOriginX + float64(gridN)*gridSpacing + 80
	addGrid("morph_grid_broken", brokenOriginX, Color{R: 230, G: 60, B: 60}, false)
	return out
}

// ---- lifecycle demo --------------------------------------------------

func lifecycleDemoPreset() []Item {
	count := 5
	sp := 250.0
	appearS, aliveS, disappearS, goneS := 1.0, 2.0, 1.0, 2.0
	periodS := appearS + aliveS + disappearS + goneS
	out := []Item{}
	for i := 0; i < count; i++ {
		off := float64(i) / float64(count) * periodS
		out = append(out, Item{
			Type: "box",
			Label: fmt.Sprintf("lifecycle_%02d", i),
			Pose:    poseAt((float64(i)-float64(count-1)/2.0)*sp, 0, 0),
			HasDims: true,
			DimsMM:  BoxDims{X: 120, Y: 120, Z: 120},
			Color:   &Color{R: 128, G: 128, B: 128},
			Opacity: ptr(1.0),
			Animation: Animation{
				Mode:         "lifecycle",
				AppearS:      appearS,
				AliveS:       aliveS,
				DisappearS:   disappearS,
				GoneS:        goneS,
				PhaseOffsetS: off,
			},
		})
	}
	return out
}

// ---- chunked PCD demo (standalone) -----------------------------------

func chunkedPCDDemoPreset() []Item {
	return []Item{{
		Type: "pointcloud", Label: "chunked_helix",
		Pose:           identityPose(),
		PointcloudPath: "assets/helix.pcd",
		Opacity:        ptr(1.0),
		Chunked:        true,
		ChunkSize:      2000,
	}}
}

// ---- all (Y-stacked) -------------------------------------------------

func allPreset() []Item {
	row := 500.0
	armGap := 1500.0
	out := []Item{}
	out = append(out, offsetBaseItems(trajectoryPreviewPreset(), "y", -2*row)...)
	out = append(out, offsetBaseItemsY(orientationVectorsPreset(), -row)...)
	out = append(out, offsetBaseItemsY(primitivesPreset(), 0.0)...)
	out = append(out, offsetBaseItemsY(lifecycleDemoPreset(), row)...)
	out = append(out, offsetBaseItemsY(geometryMorphPreset(), 2*row)...)
	fv := offsetBaseItems(forceVectorDemoPreset(), "x", -500.0)
	fv = offsetBaseItems(fv, "y", 2*row)
	out = append(out, fv...)
	out = append(out, offsetBaseItemsY(frameCompositionPreset(), 2*row+armGap)...)
	return out
}

// ---- offset helpers --------------------------------------------------

// offsetBaseItems: translate items whose parent_frame is empty or
// "world" by delta along axis. Items parented to another emitted
// Transform are left alone — they inherit the offset through frame
// composition. For trajectory animations, the waypoint coordinates
// inside animation.waypoints are shifted too.
func offsetBaseItems(items []Item, axis string, delta float64) []Item {
	out := make([]Item, 0, len(items))
	for _, it := range items {
		pf := it.ParentFrame
		newIt := it
		if pf == "" || pf == "world" {
			switch axis {
			case "x":
				newIt.Pose.X += delta
			case "y":
				newIt.Pose.Y += delta
			case "z":
				newIt.Pose.Z += delta
			}
			if newIt.Animation.Mode == "trajectory" && len(newIt.Animation.Waypoints) > 0 {
				shifted := make([]Pose, len(newIt.Animation.Waypoints))
				for i, wp := range newIt.Animation.Waypoints {
					shifted[i] = wp
					switch axis {
					case "x":
						shifted[i].X += delta
					case "y":
						shifted[i].Y += delta
					case "z":
						shifted[i].Z += delta
					}
				}
				newIt.Animation.Waypoints = shifted
			}
		}
		out = append(out, newIt)
	}
	return out
}

func offsetBaseItemsY(items []Item, dy float64) []Item {
	return offsetBaseItems(items, "y", dy)
}
