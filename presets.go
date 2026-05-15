// Named scene bundles. Each preset is a function returning a list
// of Item dicts in the same shape the config schema produces.
// Returning Items (not protos) keeps presets serializable for the
// snapshot DoCommand round-trip.
package exampleviz

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// PrimitiveRowSpacingMM — X-axis spacing between primitives in the
// row-style preset.
const PrimitiveRowSpacingMM = 400.0

// ---- Label constants (must match Python sibling repo's presets.py and
// scripts/generate_assets.py). The text PLY assets are baked in the
// Python repo and copied verbatim into ours, so the filename
// convention and sizing have to agree.

const (
	ItemLabelHeightMM   = 25.0
	RowLabelHeightMM    = 70.0
	ItemLabelZOffsetMM  = -180.0 // labels sit BELOW the item
	RowLabelZOffsetMM   = 0.0    // row labels inline with row items
)

// Item-label color palette — dark, varied. Cycled by a deterministic
// hash of the label text so the same label always gets the same
// color across reconfigure cycles.
var itemLabelColors = []Color{
	{R: 50, G: 50, B: 50},    // near-black
	{R: 90, G: 30, B: 90},    // dark plum
	{R: 30, G: 70, B: 50},    // dark teal-green
	{R: 110, G: 60, B: 30},   // dark sienna
	{R: 30, G: 50, B: 100},   // dark navy
}

// rowLabelColor — single distinct color for row labels.
var rowLabelColor = Color{R: 30, G: 30, B: 80}

// labelAssetFilenameSafeRe — characters that aren't [A-Za-z0-9_-] get
// replaced with '_' so filenames stay portable. Must match Python's
// re.sub(r"[^A-Za-z0-9_-]", "_", text).
var labelAssetFilenameSafeRe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func labelAssetFilename(text string, heightMM float64) string {
	safe := labelAssetFilenameSafeRe.ReplaceAllString(text, "_")
	return fmt.Sprintf("text__%dmm__%s.ply", int(heightMM+0.5), safe)
}

// shouldLabel — only label items in the world frame, skip repeating
// groups (flicker grids, color-wheel children). Mirrors
// presets.py::_should_label.
func shouldLabel(it Item) bool {
	if it.Label == "" {
		return false
	}
	if strings.HasPrefix(it.Label, "label_") {
		return false
	}
	if strings.Contains(it.Label, "morph_grid") || strings.Contains(it.Label, "_wheel_") {
		return false
	}
	if it.ParentFrame != "" && it.ParentFrame != "world" {
		return false
	}
	return true
}

// labelColorFor — deterministic palette pick from sum-of-ords.
func labelColorFor(target string) Color {
	sum := 0
	for _, r := range target {
		sum += int(r)
	}
	return itemLabelColors[sum%len(itemLabelColors)]
}

// labelFor — build a label-mesh item floating below `target`.
func labelFor(target Item) Item {
	c := labelColorFor(target.Label)
	return Item{
		Type:  "mesh",
		Label: "label_" + target.Label,
		Pose: Pose{
			X:  target.Pose.X,
			Y:  target.Pose.Y,
			Z:  target.Pose.Z + ItemLabelZOffsetMM,
			OZ: 1.0,
		},
		MeshPath: "assets/" + labelAssetFilename(target.Label, ItemLabelHeightMM),
		Color:    &c,
		Opacity:  ptr(1.0),
	}
}

// withItemLabels — append a label-mesh sibling to every world-parented
// item that passes shouldLabel.
func withItemLabels(items []Item) []Item {
	out := make([]Item, 0, len(items)*2)
	out = append(out, items...)
	for _, it := range items {
		if shouldLabel(it) {
			out = append(out, labelFor(it))
		}
	}
	return out
}

// rowLabel — large text-label-mesh at the left edge of a row,
// inline with the row's items (Z = 0).
func rowLabel(rowName string, y, x float64) Item {
	return Item{
		Type:     "mesh",
		Label:    "label_row_" + rowName,
		Pose:     Pose{X: x, Y: y, Z: RowLabelZOffsetMM, OZ: 1.0},
		MeshPath: "assets/" + labelAssetFilename(rowName, RowLabelHeightMM),
		Color:    &rowLabelColor,
		Opacity:  ptr(1.0),
	}
}

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
	return ToItems(
		Box{Label: "demo_box", Pose: poseAt(-4*sp, 0, 0),
			DimsMM: BoxDims{X: 150, Y: 150, Z: 150},
			Color: &Color{R: 230, G: 25, B: 75}, Opacity: ptr(1.0)},
		Sphere{Label: "demo_sphere", Pose: poseAt(-3*sp, 0, 0),
			RadiusMM: 90, Color: &Color{R: 60, G: 180, B: 75}, Opacity: ptr(1.0)},
		Capsule{Label: "demo_capsule", Pose: poseAt(-2*sp, 0, 0),
			RadiusMM: 50, LengthMM: 200,
			Color: &Color{R: 0, G: 130, B: 200}, Opacity: ptr(1.0)},
		Point{Label: "demo_point", Pose: poseAt(-1*sp, 0, 0),
			Color: &Color{R: 255, G: 225, B: 25}, Opacity: ptr(1.0)},
		Arrow{Label: "demo_arrow", Pose: poseAt(0, 0, 0),
			LengthMM: 220, RadiusMM: 12,
			Color: &Color{R: 145, G: 30, B: 180}, Opacity: ptr(1.0)},
		Mesh{Label: "demo_icosahedron", Pose: poseAt(1*sp, 0, 0),
			MeshPath: "assets/icosahedron.ply",
			Color: &Color{R: 240, G: 50, B: 230}, Opacity: ptr(1.0)},
		// STL→PLY converted at load time. Works.
		Mesh{Label: "demo_bunny", Pose: poseAt(2*sp, 0, 0),
			MeshPath: "assets/bunny.stl",
			Color: &Color{R: 245, G: 130, B: 49}, Opacity: ptr(1.0)},
		// Same STL, shipped raw with content_type="stl" (bypassing
		// stl_to_ply). Bug-demo: proto/RDK contract accepts STL on
		// the wire (NewMeshFromProto at rdk/spatialmath/mesh.go:234-
		// 243) but the viewer drops it silently. Should render as
		// empty space right of the working twin.
		Mesh{Label: "demo_bunny_raw_stl", Pose: poseAt(3*sp, 0, 0),
			MeshPath: "assets/bunny.stl", RawSTL: true,
			Color: &Color{R: 245, G: 130, B: 49}, Opacity: ptr(1.0)},
		Mesh{Label: "demo_torus", Pose: poseAt(4*sp, 0, 0),
			MeshPath: "assets/torus.ply",
			Color: &Color{R: 70, G: 240, B: 240}, Opacity: ptr(1.0)},
		Mesh{Label: "demo_teapot", Pose: poseAt(5*sp, 0, 0),
			MeshPath: "assets/teapot.ply",
			Color: &Color{R: 60, G: 180, B: 75}, Opacity: ptr(1.0)},
		// PLY w/ per-vertex rainbow colors. The service transcodes
		// embedded vertex colors to metadata.colors, but the viewer
		// collapses N colors to one uniform tint (the FIRST color).
		// No `Color` here so vertex colors are the only color source.
		Mesh{Label: "demo_colorful_sphere_mesh", Pose: poseAt(6*sp, 0, 0),
			MeshPath: "assets/colorful_sphere.ply", Opacity: ptr(1.0)},
		// PLY w/ per-FACE rainbow colors instead of per-vertex.
		// Untested.
		Mesh{Label: "demo_colorful_sphere_faces_mesh", Pose: poseAt(7*sp, 0, 0),
			MeshPath: "assets/colorful_sphere_faces.ply", Opacity: ptr(1.0)},
		// PLY w/ UV coords + comment TextureFile. commonpb.Mesh has
		// no slot for texture bytes. Untested.
		Mesh{Label: "demo_uv_sphere_mesh", Pose: poseAt(8*sp, 0, 0),
			MeshPath: "assets/uv_sphere.ply", Opacity: ptr(1.0)},
		// PCD version — per-point RGB embedded; works because viewer
		// reads PCD per-point colors when metadata.colors is absent.
		PointCloud{Label: "demo_colorful_sphere", Pose: poseAt(9*sp, 0, 0),
			PointcloudPath: "assets/colorful_sphere.pcd", Opacity: ptr(1.0)},
		PointCloud{Label: "demo_pointcloud", Pose: poseAt(10*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0)},
		PointCloud{Label: "demo_pointcloud_chunked", Pose: poseAt(11*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0),
			Chunked: true, ChunkSize: 2000},
	)
}

// ---- orientation vectors ---------------------------------------------

func orientationVectorsPreset() []Item {
	sp := PrimitiveRowSpacingMM
	const hostR = 18.0
	frame := func(label string, x, ox, oy, oz, theta float64) Sphere {
		return Sphere{
			Label: label,
			Pose:  PoseAt(x, 0, 0, ox, oy, oz, theta),
			RadiusMM:       hostR,
			Color:          &Color{R: 220, G: 220, B: 220},
			Opacity:        ptr(0.35),
			ShowAxesHelper: true,
		}
	}
	s := 1.0 / math.Sqrt(2)
	return ToItems(
		frame("frame_+Z", -2*sp, 0, 0, 1, 0),
		frame("frame_+X", -sp, 1, 0, 0, 0),
		frame("frame_+Y", 0, 0, 1, 0, 0),
		frame("frame_+XY", sp, s, s, 0, 0),
		frame("frame_+Z_theta45", 2*sp, 0, 0, 1, 45),
	)
}

// ---- reference frame demo (component of frame_composition) -----------

func referenceFrameDemo() []Item {
	const axisLength = 200.0
	const axisRadius = 12.0
	half := axisLength / 2.0
	visuals := []Visual{
		// Anchor — spins around Z; children inherit.
		Sphere{Label: "spinning_frame", Pose: identityPose(),
			RadiusMM: 12, Color: &Color{R: 255, G: 255, B: 255},
			Opacity: ptr(0.6), ShowAxesHelper: true,
			Animation: Spin{PeriodS: 6}},
		// RGB axis triad — three capsules parented to anchor.
		Capsule{Label: "spinning_frame_axis_x", ParentFrame: "spinning_frame",
			Pose: PoseAt(half, 0, 0, 1, 0, 0, 0),
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color: &Color{R: 230, G: 25, B: 75}, Opacity: ptr(1.0)},
		Capsule{Label: "spinning_frame_axis_y", ParentFrame: "spinning_frame",
			Pose: PoseAt(0, half, 0, 0, 1, 0, 0),
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color: &Color{R: 60, G: 180, B: 75}, Opacity: ptr(1.0)},
		Capsule{Label: "spinning_frame_axis_z", ParentFrame: "spinning_frame",
			Pose: PoseAt(0, 0, half, 0, 0, 1, 0),
			RadiusMM: axisRadius, LengthMM: axisLength,
			Color: &Color{R: 0, G: 130, B: 200}, Opacity: ptr(1.0)},
		// Mesh — orbits with anchor AND spins on its own axis.
		Mesh{Label: "spinning_frame_attached_mesh", ParentFrame: "spinning_frame",
			Pose: Pose{X: 700, OZ: 1, hasOrient: true},
			MeshPath: "assets/icosahedron.ply",
			Color: &Color{R: 240, G: 200, B: 50}, Opacity: ptr(1.0),
			Animation: Spin{PeriodS: 2}},
		// Invisible wheel hub — adds its own spin on top of mesh +
		// anchor. Tiny radius + opacity 0 keeps it invisible; we
		// avoid Invisible=true because the viewer's behavior for
		// invisible parents in the composition tree is unverified.
		Sphere{Label: "spinning_frame_wheel_hub",
			ParentFrame: "spinning_frame_attached_mesh",
			Pose:        identityPose(),
			RadiusMM:    4,
			Color:       &Color{R: 255, G: 255, B: 255},
			Opacity:     ptr(0.0),
			Animation:   Spin{PeriodS: 10}},
	}
	for _, sv := range colorWheelChildrenVisuals("spinning_frame_wheel_hub", 10, 220.0, 24.0) {
		visuals = append(visuals, sv)
	}
	return ToItems(visuals...)
}

func colorWheelChildrenVisuals(parent string, count int, ringR, sphereR float64) []Sphere {
	out := make([]Sphere, 0, count)
	for i := 0; i < count; i++ {
		hue := float64(i) / float64(count)
		r, g, b := hsvToRGBu8(hue, 1, 1)
		angle := 2 * math.Pi * float64(i) / float64(count)
		out = append(out, Sphere{
			Label:       fmt.Sprintf("%s_wheel_%02d", parent, i),
			ParentFrame: parent,
			Pose: PoseAt(ringR*math.Cos(angle), ringR*math.Sin(angle), 0,
				0, 0, 1, 0),
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
	jointColor := &Color{R: 230, G: 25, B: 75}
	clawColor := &Color{R: 220, G: 220, B: 70}

	return ToItems(
		// Base — stout capsule, swings on Z.
		Capsule{Label: "arm_base", Pose: poseAt(0, 0, baseH/2),
			RadiusMM: linkR * 1.6, LengthMM: baseH,
			Color: &Color{R: 70, G: 70, B: 75}, Opacity: ptr(1.0),
			Animation: Swing{AmplitudeDeg: 75.0, PeriodS: 8}},
		// Shoulder — OY=1 tips local +Z into world +Y.
		Sphere{Label: "arm_shoulder", ParentFrame: "arm_base",
			Pose:     PoseAt(0, 0, baseH/2+linkR, 0, 1, 0, 0),
			RadiusMM: jointR, Color: jointColor, Opacity: ptr(1.0)},
		// Upper arm.
		Capsule{Label: "arm_upper", ParentFrame: "arm_shoulder",
			Pose: poseAt(0, 0, upperL/2),
			RadiusMM: linkR, LengthMM: upperL,
			Color: &Color{R: 100, G: 130, B: 200}, Opacity: ptr(1.0)},
		// Elbow — bounded RoM.
		Sphere{Label: "arm_elbow", ParentFrame: "arm_upper",
			Pose:     PoseAt(0, 0, upperL/2+linkR, 0, 1, 0, -60),
			RadiusMM: jointR * 0.8,
			Color: jointColor, Opacity: ptr(1.0),
			Animation: Swing{AmplitudeDeg: 50.0, PeriodS: 5}},
		// Forearm.
		Capsule{Label: "arm_forearm", ParentFrame: "arm_elbow",
			Pose: poseAt(0, 0, forearmL/2),
			RadiusMM: linkR * 0.85, LengthMM: forearmL,
			Color: &Color{R: 100, G: 180, B: 110}, Opacity: ptr(1.0)},
		// Wrist — rolls about the forearm; the 2-finger claw makes
		// the rotation visible.
		Sphere{Label: "arm_wrist", ParentFrame: "arm_forearm",
			Pose:     poseAt(0, 0, forearmL/2+linkR*0.6),
			RadiusMM: jointR * 0.65,
			Color: jointColor, Opacity: ptr(1.0),
			Animation: Swing{AmplitudeDeg: 90.0, PeriodS: 6}},
		// Claw palm.
		Box{Label: "claw_palm", ParentFrame: "arm_wrist",
			Pose:   poseAt(0, 0, jointR*0.6+palmThick/2),
			DimsMM: BoxDims{X: 70, Y: 28, Z: palmThick},
			Color: clawColor, Opacity: ptr(1.0)},
		// Left finger.
		Box{Label: "claw_left_finger", ParentFrame: "claw_palm",
			Pose:   poseAt(-22, 0, palmThick/2+fingerL/2),
			DimsMM: BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color: clawColor, Opacity: ptr(1.0),
			Animation: Oscillate{Axis: "x", AmplitudeMM: -10.0, PeriodS: 3}},
		// Right finger.
		Box{Label: "claw_right_finger", ParentFrame: "claw_palm",
			Pose:   poseAt(22, 0, palmThick/2+fingerL/2),
			DimsMM: BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color: clawColor, Opacity: ptr(1.0),
			Animation: Oscillate{Axis: "x", AmplitudeMM: 10.0, PeriodS: 3}},
	)
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
	visuals := []Visual{}
	for i, wp := range waypoints {
		visuals = append(visuals, Sphere{
			Label: fmt.Sprintf("traj_wp_%02d", i), Pose: wp,
			RadiusMM: 18, Color: &Color{R: 200, G: 200, B: 220},
			Opacity: ptr(0.45), ShowAxesHelper: true,
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
		visuals = append(visuals, Capsule{
			Label: fmt.Sprintf("traj_seg_%02d", i),
			Pose: PoseAt((a.X+b.X)/2, (a.Y+b.Y)/2, (a.Z+b.Z)/2,
				dx/segLen, dy/segLen, dz/segLen, 0),
			RadiusMM: 5, LengthMM: segLen,
			Color: &Color{R: 100, G: 130, B: 240}, Opacity: ptr(0.95),
		})
	}
	visuals = append(visuals, Sphere{
		Label: "traj_runner", Pose: waypoints[0],
		RadiusMM: 28, Color: &Color{R: 230, G: 40, B: 80},
		Opacity: ptr(0.9), ShowAxesHelper: true,
		Animation: Trajectory{Waypoints: waypoints, DurationS: 12.0, Loop: true},
	})
	return ToItems(visuals...)
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
	return ToItems(Arrow{
		Label: "force_vector", Pose: identityPose(),
		LengthMM: 220, RadiusMM: 10,
		Color: &Color{R: 230, G: 60, B: 100}, Opacity: ptr(1.0),
		Animation: ForceVector{
			PeriodS: 5.0, LengthAmplitudeMM: 80, RadiusAmplitudeMM: 5,
			TiltDeg: 45, PrecessionSpeed: 1.0, ColorSpeed: 0.7,
		},
	})
}

// ---- geometry morph --------------------------------------------------

func geometryMorphPreset() []Item {
	visuals := []Visual{}
	slotX := 0.0

	// Pulsing sphere.
	visuals = append(visuals, Sphere{
		Label: "morph_pulse_sphere", Pose: poseAt(slotX, 0, 0),
		RadiusMM: 70, Color: &Color{R: 230, G: 60, B: 100}, Opacity: ptr(1.0),
		Animation: Pulse{AmplitudeMM: 35, PeriodS: 3},
	})
	slotX += 350

	// Box stretching along Z.
	visuals = append(visuals, Box{
		Label: "morph_stretch_box", Pose: poseAt(slotX, 0, 0),
		DimsMM: BoxDims{X: 100, Y: 100, Z: 150},
		Color: &Color{R: 100, G: 180, B: 230}, Opacity: ptr(1.0),
		Animation: Pulse{Axis: "z", AmplitudeMM: 100, PeriodS: 4},
	})
	slotX += 350

	// Capsule breathing in opacity.
	visuals = append(visuals, Capsule{
		Label: "morph_breathe_capsule", Pose: poseAt(slotX, 0, 0),
		RadiusMM: 45, LengthMM: 240,
		Color: &Color{R: 220, G: 200, B: 60}, Opacity: ptr(0.7),
		Animation: Breathe{Amplitude: 0.55, PeriodS: 1.5},
	})
	slotX += 380

	// Two 5×5 flicker grids — green works (UUID rotates), red is
	// the bug-demo (UUID stable across re-adds).
	gridN := 5
	gridSpacing := 80.0
	periodS := 4.0
	addGrid := func(prefix string, originX float64, col *Color, rotateUUID bool) {
		for row := 0; row < gridN; row++ {
			for col2 := 0; col2 < gridN; col2++ {
				phaseOff := float64(row+col2) / float64(2*gridN-1) * periodS
				ru := rotateUUID
				visuals = append(visuals, Sphere{
					Label: fmt.Sprintf("%s_%d%d", prefix, row, col2),
					Pose: poseAt(originX+float64(col2)*gridSpacing,
						float64(row)*gridSpacing, 0),
					RadiusMM: 22, Color: col, Opacity: ptr(1.0),
					Animation: Flicker{
						PeriodS: periodS, DutyCycle: 0.55,
						PhaseOffsetS: phaseOff, RotateUUIDOnReadd: &ru,
					},
				})
			}
		}
	}
	workingOriginX := slotX + 60
	addGrid("morph_grid", workingOriginX, &Color{R: 80, G: 200, B: 140}, true)
	brokenOriginX := workingOriginX + float64(gridN)*gridSpacing + 80
	addGrid("morph_grid_broken", brokenOriginX, &Color{R: 230, G: 60, B: 60}, false)
	return ToItems(visuals...)
}

// ---- lifecycle demo --------------------------------------------------

func lifecycleDemoPreset() []Item {
	count := 5
	sp := 250.0
	appearS, aliveS, disappearS, goneS := 1.0, 2.0, 1.0, 2.0
	periodS := appearS + aliveS + disappearS + goneS
	visuals := make([]Visual, 0, count)
	for i := 0; i < count; i++ {
		off := float64(i) / float64(count) * periodS
		visuals = append(visuals, Box{
			Label: fmt.Sprintf("lifecycle_%02d", i),
			Pose:  poseAt((float64(i)-float64(count-1)/2.0)*sp, 0, 0),
			DimsMM: BoxDims{X: 120, Y: 120, Z: 120},
			// Color/Opacity overridden every tick by the lifecycle
			// animation; static values are placeholders.
			Color: &Color{R: 128, G: 128, B: 128}, Opacity: ptr(1.0),
			Animation: Lifecycle{
				AppearS: appearS, AliveS: aliveS,
				DisappearS: disappearS, GoneS: goneS,
				PhaseOffsetS: off,
			},
		})
	}
	return ToItems(visuals...)
}

// ---- chunked PCD demo (standalone) -----------------------------------

func chunkedPCDDemoPreset() []Item {
	return ToItems(PointCloud{
		Label: "chunked_helix", Pose: identityPose(),
		PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0),
		Chunked: true, ChunkSize: 2000,
	})
}

// ---- all (Y-stacked) -------------------------------------------------

func allPreset() []Item {
	row := 500.0
	armGap := 1500.0
	out := []Item{}
	// Wrap each sub-preset with item-labels before offsetting into
	// its row so labels follow their target items.
	out = append(out, offsetBaseItems(withItemLabels(trajectoryPreviewPreset()), "y", -2*row)...)
	out = append(out, offsetBaseItemsY(withItemLabels(orientationVectorsPreset()), -row)...)
	out = append(out, offsetBaseItemsY(withItemLabels(primitivesPreset()), 0.0)...)
	out = append(out, offsetBaseItemsY(withItemLabels(lifecycleDemoPreset()), row)...)
	out = append(out, offsetBaseItemsY(withItemLabels(geometryMorphPreset()), 2*row)...)
	fv := offsetBaseItems(withItemLabels(forceVectorDemoPreset()), "x", -500.0)
	fv = offsetBaseItems(fv, "y", 2*row)
	out = append(out, fv...)
	out = append(out, offsetBaseItemsY(withItemLabels(frameCompositionPreset()), 2*row+armGap)...)

	// Row labels: inline with each row's Z plane, just to the left
	// of the row's leftmost item.
	rowLabelX := -2200.0
	out = append(out, rowLabel("trajectory_preview", -2*row, rowLabelX))
	out = append(out, rowLabel("orientation_vectors", -row, rowLabelX))
	out = append(out, rowLabel("primitives", 0.0, rowLabelX))
	out = append(out, rowLabel("lifecycle_demo", row, rowLabelX))
	// force_vector_demo + geometry_morph share row Y; split labels by
	// ±70 mm in Y so the names don't overlap.
	out = append(out, rowLabel("force_vector_demo", 2*row-70.0, rowLabelX))
	out = append(out, rowLabel("geometry_morph", 2*row+70.0, rowLabelX))
	out = append(out, rowLabel("frame_composition", 2*row+armGap, rowLabelX))
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
