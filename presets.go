// Named scene bundles. Each preset is a function returning a list
// of visuals.Item dicts in the same shape the config schema produces.
// Returning Items (not protos) keeps presets serializable for the
// snapshot DoCommand round-trip.
package exampleviz

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/viam-labs/viam-viz-helpers-go"
)

// PrimitiveRowSpacingMM — X-axis spacing between primitives in the
// row-style preset.
const PrimitiveRowSpacingMM = 400.0

// ---- Label constants (must match Python sibling repo's presets.py and
// scripts/generate_assets.py). The text PLY assets are baked in the
// Python repo and copied verbatim into ours, so the filename
// convention and sizing have to agree.

const (
	ItemLabelHeightMM  = 25.0
	RowLabelHeightMM   = 70.0
	ItemLabelZOffsetMM = -180.0 // labels sit BELOW the item
	RowLabelZOffsetMM  = 0.0    // row labels inline with row items
)

// visuals.Item-label color palette — dark, varied. Cycled by a deterministic
// hash of the label text so the same label always gets the same
// color across reconfigure cycles.
var itemLabelColors = []visuals.Color{
	{R: 50, G: 50, B: 50},  // near-black
	{R: 90, G: 30, B: 90},  // dark plum
	{R: 30, G: 70, B: 50},  // dark teal-green
	{R: 110, G: 60, B: 30}, // dark sienna
	{R: 30, G: 50, B: 100}, // dark navy
}

// rowLabelColor — single distinct color for row labels.
var rowLabelColor = visuals.Color{R: 30, G: 30, B: 80}

// labelAssetFilenameSafeRe — characters that aren't [A-Za-z0-9_-] get
// replaced with '_' so filenames stay portable. Must match Python's
// re.sub(r"[^A-Za-z0-9_-]", "_", text).
var labelAssetFilenameSafeRe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func labelAssetFilename(text string, heightMM float64) string {
	safe := labelAssetFilenameSafeRe.ReplaceAllString(text, "_")
	return fmt.Sprintf("text__%dmm__%s.ply", int(heightMM+0.5), safe)
}

// shouldLabel — only label items in the world frame, skip repeating
// groups (flicker grids, color-wheel children). Reads label /
// parent_frame from the Visual via ToItem(); returns false for any
// non-Visual passed in.
func shouldLabel(v interface{}) bool {
	vis, ok := v.(visuals.Visual)
	if !ok {
		return false
	}
	it := vis.ToItem()
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
func labelColorFor(target string) visuals.Color {
	sum := 0
	for _, r := range target {
		sum += int(r)
	}
	return itemLabelColors[sum%len(itemLabelColors)]
}

// labelFor — build a Mesh label floating below `target`. Returns
// nil if target isn't a Visual.
func labelFor(target interface{}) *visuals.Mesh {
	vis, ok := target.(visuals.Visual)
	if !ok {
		return nil
	}
	it := vis.ToItem()
	c := labelColorFor(it.Label)
	return &visuals.Mesh{
		Label: "label_" + it.Label,
		Pose: visuals.Pose{
			X:  it.Pose.X,
			Y:  it.Pose.Y,
			Z:  it.Pose.Z + ItemLabelZOffsetMM,
			OZ: 1.0,
		},
		MeshPath: "assets/" + labelAssetFilename(it.Label, ItemLabelHeightMM),
		Color:    &c,
		Opacity:  ptr(1.0),
	}
}

// withItemLabels — append a Mesh label sibling to every world-
// parented item that passes shouldLabel.
func withItemLabels(items []interface{}) []interface{} {
	out := make([]interface{}, 0, len(items)*2)
	out = append(out, items...)
	for _, it := range items {
		if shouldLabel(it) {
			if lbl := labelFor(it); lbl != nil {
				out = append(out, lbl)
			}
		}
	}
	return out
}

// rowLabel — large Mesh label at the left edge of a row, inline
// with the row's items (Z = 0).
func rowLabel(rowName string, y, x float64) *visuals.Mesh {
	c := rowLabelColor
	return &visuals.Mesh{
		Label:    "label_row_" + rowName,
		Pose:     visuals.Pose{X: x, Y: y, Z: RowLabelZOffsetMM, OZ: 1.0},
		MeshPath: "assets/" + labelAssetFilename(rowName, RowLabelHeightMM),
		Color:    &c,
		Opacity:  ptr(1.0),
	}
}

// Presets — registry of named bundle functions.
var Presets = map[string]func() []interface{}{
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

// visualsToInterfaces converts []visuals.Visual to []interface{} so
// the result can be combined with other Visuals/Composites in a
// preset's return slice. Go won't auto-convert between the two
// slice types even though every Visual is an interface{}.
func visualsToInterfaces(vs []visuals.Visual) []interface{} {
	out := make([]interface{}, 0, len(vs))
	for _, v := range vs {
		out = append(out, v)
	}
	return out
}

func identityPose() visuals.Pose          { return visuals.Pose{OZ: 1.0} }
func poseXY(x, y float64) visuals.Pose    { return visuals.Pose{X: x, Y: y, OZ: 1.0} }
func poseAt(x, y, z float64) visuals.Pose { return visuals.Pose{X: x, Y: y, Z: z, OZ: 1.0} }

// ---- primitives ------------------------------------------------------

func primitivesPreset() []interface{} {
	sp := PrimitiveRowSpacingMM
	return []interface{}{
		&visuals.Box{Label: "demo_box", Pose: poseAt(-4*sp, 0, 0),
			DimsMM: visuals.BoxDims{X: 150, Y: 150, Z: 150},
			Color:  &visuals.Color{R: 230, G: 25, B: 75}, Opacity: ptr(1.0)},
		&visuals.Sphere{Label: "demo_sphere", Pose: poseAt(-3*sp, 0, 0),
			RadiusMM: 90, Color: &visuals.Color{R: 60, G: 180, B: 75}, Opacity: ptr(1.0)},
		&visuals.Capsule{Label: "demo_capsule", Pose: poseAt(-2*sp, 0, 0),
			RadiusMM: 50, LengthMM: 200,
			Color: &visuals.Color{R: 0, G: 130, B: 200}, Opacity: ptr(1.0)},
		&visuals.Point{Label: "demo_point", Pose: poseAt(-1*sp, 0, 0),
			Color: &visuals.Color{R: 255, G: 225, B: 25}, Opacity: ptr(1.0)},
		&visuals.Arrow{Label: "demo_arrow", Pose: poseAt(0, 0, 0),
			LengthMM: 220, RadiusMM: 12,
			Color: &visuals.Color{R: 145, G: 30, B: 180}, Opacity: ptr(1.0)},
		&visuals.Mesh{Label: "demo_icosahedron", Pose: poseAt(1*sp, 0, 0),
			MeshPath: "assets/icosahedron.ply",
			Color:    &visuals.Color{R: 240, G: 50, B: 230}, Opacity: ptr(1.0)},
		// STL→PLY converted at load time. Works.
		&visuals.Mesh{Label: "demo_bunny", Pose: poseAt(2*sp, 0, 0),
			MeshPath: "assets/bunny.stl",
			Color:    &visuals.Color{R: 245, G: 130, B: 49}, Opacity: ptr(1.0)},
		// Same STL, shipped raw with content_type="stl" (bypassing
		// stl_to_ply). Bug-demo: proto/RDK contract accepts STL on
		// the wire (NewMeshFromProto at rdk/spatialmath/mesh.go:234-
		// 243) but the viewer drops it silently. Should render as
		// empty space right of the working twin.
		&visuals.Mesh{Label: "demo_bunny_raw_stl", Pose: poseAt(3*sp, 0, 0),
			MeshPath: "assets/bunny.stl", RawSTL: true,
			Color: &visuals.Color{R: 245, G: 130, B: 49}, Opacity: ptr(1.0)},
		&visuals.Mesh{Label: "demo_torus", Pose: poseAt(4*sp, 0, 0),
			MeshPath: "assets/torus.ply",
			Color:    &visuals.Color{R: 70, G: 240, B: 240}, Opacity: ptr(1.0)},
		&visuals.Mesh{Label: "demo_teapot", Pose: poseAt(5*sp, 0, 0),
			MeshPath: "assets/teapot.ply",
			Color:    &visuals.Color{R: 60, G: 180, B: 75}, Opacity: ptr(1.0)},
		// PLY w/ per-vertex rainbow colors. The service transcodes
		// embedded vertex colors to metadata.colors, but the viewer
		// collapses N colors to one uniform tint (the FIRST color).
		// No `visuals.Color` here so vertex colors are the only color source.
		&visuals.Mesh{Label: "demo_colorful_sphere_mesh", Pose: poseAt(6*sp, 0, 0),
			MeshPath: "assets/colorful_sphere.ply", Opacity: ptr(1.0)},
		// PLY w/ per-FACE rainbow colors instead of per-vertex.
		// Untested.
		&visuals.Mesh{Label: "demo_colorful_sphere_faces_mesh", Pose: poseAt(7*sp, 0, 0),
			MeshPath: "assets/colorful_sphere_faces.ply", Opacity: ptr(1.0)},
		// PLY w/ UV coords + comment TextureFile. commonpb.Mesh has
		// no slot for texture bytes. Untested.
		&visuals.Mesh{Label: "demo_uv_sphere_mesh", Pose: poseAt(8*sp, 0, 0),
			MeshPath: "assets/uv_sphere.ply", Opacity: ptr(1.0)},
		// PCD version — per-point RGB embedded; works because viewer
		// reads PCD per-point colors when metadata.colors is absent.
		&visuals.PointCloud{Label: "demo_colorful_sphere", Pose: poseAt(9*sp, 0, 0),
			PointcloudPath: "assets/colorful_sphere.pcd", Opacity: ptr(1.0)},
		&visuals.PointCloud{Label: "demo_pointcloud", Pose: poseAt(10*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0)},
		&visuals.PointCloud{Label: "demo_pointcloud_chunked", Pose: poseAt(11*sp, 0, 0),
			PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0),
			Chunked: true, ChunkSize: 2000},
	}
}

// ---- orientation vectors ---------------------------------------------

func orientationVectorsPreset() []interface{} {
	sp := PrimitiveRowSpacingMM
	const hostR = 18.0
	frame := func(label string, x, ox, oy, oz, theta float64) *visuals.Sphere {
		return &visuals.Sphere{
			Label:          label,
			Pose:           visuals.PoseAt(x, 0, 0, ox, oy, oz, theta),
			RadiusMM:       hostR,
			Color:          &visuals.Color{R: 220, G: 220, B: 220},
			Opacity:        ptr(0.35),
			ShowAxesHelper: true,
		}
	}
	s := 1.0 / math.Sqrt(2)
	return []interface{}{
		frame("frame_+Z", -2*sp, 0, 0, 1, 0),
		frame("frame_+X", -sp, 1, 0, 0, 0),
		frame("frame_+Y", 0, 0, 1, 0, 0),
		frame("frame_+XY", sp, s, s, 0, 0),
		frame("frame_+Z_theta45", 2*sp, 0, 0, 1, 45),
	}
}

// ---- reference frame demo (component of frame_composition) -----------

func referenceFrameDemo() []interface{} {
	const axisLength = 200.0
	const axisRadius = 12.0
	// Anchor + RGB axes triad — single composite. The anchor spins;
	// the axes inherit motion through the parent-frame chain.
	frame := visuals.CoordinateFrame{
		Label: "spinning_frame", Pose: identityPose(),
		SizeMM: axisLength, AxisRadiusMM: axisRadius, AnchorRadiusMM: 12,
		// ShowAxesHelper omitted — defaults to true on visuals.CoordinateFrame.
		Animation: visuals.Spin{PeriodS: 6},
	}
	items := visualsToInterfaces(frame.ToVisuals())
	items = append(items, []interface{}{
		// visuals.Mesh — orbits with anchor AND spins on its own axis.
		&visuals.Mesh{Label: "spinning_frame_attached_mesh", ParentFrame: "spinning_frame",
			Pose:     visuals.PoseAt(700, 0, 0, 0, 0, 1, 0),
			MeshPath: "assets/icosahedron.ply",
			Color:    &visuals.Color{R: 240, G: 200, B: 50}, Opacity: ptr(1.0),
			Animation: visuals.Spin{PeriodS: 2}},
		// Invisible wheel hub — adds its own spin on top of mesh +
		// anchor. Tiny radius + opacity 0 keeps it invisible; we
		// avoid Invisible=true because the viewer's behavior for
		// invisible parents in the composition tree is unverified.
		&visuals.Sphere{Label: "spinning_frame_wheel_hub",
			ParentFrame: "spinning_frame_attached_mesh",
			Pose:        identityPose(),
			RadiusMM:    4,
			Color:       &visuals.Color{R: 255, G: 255, B: 255},
			Opacity:     ptr(0.0),
			Animation:   visuals.Spin{PeriodS: 10}},
	}...)
	for _, sv := range colorWheelChildrenVisuals("spinning_frame_wheel_hub", 10, 220.0, 24.0) {
		items = append(items, sv)
	}
	return items
}

func colorWheelChildrenVisuals(parent string, count int, ringR, sphereR float64) []*visuals.Sphere {
	out := make([]*visuals.Sphere, 0, count)
	for i := 0; i < count; i++ {
		hue := float64(i) / float64(count)
		r, g, b := hsvToRGBu8(hue, 1, 1)
		angle := 2 * math.Pi * float64(i) / float64(count)
		out = append(out, &visuals.Sphere{
			Label:       fmt.Sprintf("%s_wheel_%02d", parent, i),
			ParentFrame: parent,
			Pose: visuals.PoseAt(ringR*math.Cos(angle), ringR*math.Sin(angle), 0,
				0, 0, 1, 0),
			RadiusMM: sphereR,
			Color:    &visuals.Color{R: r, G: g, B: b},
			Opacity:  ptr(1.0),
		})
	}
	return out
}

// ---- robot arm (component of frame_composition) ----------------------

func robotArm() []interface{} {
	const linkR = 25.0
	const baseH = 80.0
	const upperL = 220.0
	const forearmL = 180.0
	const jointR = 35.0
	const palmThick = 10.0
	const fingerL = 70.0
	const fingerThick = 8.0
	jointColor := &visuals.Color{R: 230, G: 25, B: 75}
	clawColor := &visuals.Color{R: 220, G: 220, B: 70}

	return []interface{}{
		// Base — stout capsule, swings on Z.
		&visuals.Capsule{Label: "arm_base", Pose: poseAt(0, 0, baseH/2),
			RadiusMM: linkR * 1.6, LengthMM: baseH,
			Color: &visuals.Color{R: 70, G: 70, B: 75}, Opacity: ptr(1.0),
			Animation: visuals.Swing{AmplitudeDeg: 75.0, PeriodS: 8}},
		// Shoulder — OY=1 tips local +Z into world +Y.
		&visuals.Sphere{Label: "arm_shoulder", ParentFrame: "arm_base",
			Pose:     visuals.PoseAt(0, 0, baseH/2+linkR, 0, 1, 0, 0),
			RadiusMM: jointR, Color: jointColor, Opacity: ptr(1.0)},
		// Upper arm.
		&visuals.Capsule{Label: "arm_upper", ParentFrame: "arm_shoulder",
			Pose:     poseAt(0, 0, upperL/2),
			RadiusMM: linkR, LengthMM: upperL,
			Color: &visuals.Color{R: 100, G: 130, B: 200}, Opacity: ptr(1.0)},
		// Elbow — bounded RoM.
		&visuals.Sphere{Label: "arm_elbow", ParentFrame: "arm_upper",
			Pose:     visuals.PoseAt(0, 0, upperL/2+linkR, 0, 1, 0, -60),
			RadiusMM: jointR * 0.8,
			Color:    jointColor, Opacity: ptr(1.0),
			Animation: visuals.Swing{AmplitudeDeg: 50.0, PeriodS: 5}},
		// Forearm.
		&visuals.Capsule{Label: "arm_forearm", ParentFrame: "arm_elbow",
			Pose:     poseAt(0, 0, forearmL/2),
			RadiusMM: linkR * 0.85, LengthMM: forearmL,
			Color: &visuals.Color{R: 100, G: 180, B: 110}, Opacity: ptr(1.0)},
		// Wrist — rolls about the forearm; the 2-finger claw makes
		// the rotation visible.
		&visuals.Sphere{Label: "arm_wrist", ParentFrame: "arm_forearm",
			Pose:     poseAt(0, 0, forearmL/2+linkR*0.6),
			RadiusMM: jointR * 0.65,
			Color:    jointColor, Opacity: ptr(1.0),
			Animation: visuals.Swing{AmplitudeDeg: 90.0, PeriodS: 6}},
		// Claw palm.
		&visuals.Box{Label: "claw_palm", ParentFrame: "arm_wrist",
			Pose:   poseAt(0, 0, jointR*0.6+palmThick/2),
			DimsMM: visuals.BoxDims{X: 70, Y: 28, Z: palmThick},
			Color:  clawColor, Opacity: ptr(1.0)},
		// Left finger.
		&visuals.Box{Label: "claw_left_finger", ParentFrame: "claw_palm",
			Pose:   poseAt(-22, 0, palmThick/2+fingerL/2),
			DimsMM: visuals.BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color:  clawColor, Opacity: ptr(1.0),
			Animation: visuals.Oscillate{Axis: "x", AmplitudeMM: -10.0, PeriodS: 3}},
		// Right finger.
		&visuals.Box{Label: "claw_right_finger", ParentFrame: "claw_palm",
			Pose:   poseAt(22, 0, palmThick/2+fingerL/2),
			DimsMM: visuals.BoxDims{X: fingerThick, Y: fingerThick, Z: fingerL},
			Color:  clawColor, Opacity: ptr(1.0),
			Animation: visuals.Oscillate{Axis: "x", AmplitudeMM: 10.0, PeriodS: 3}},
	}
}

// ---- frame composition -----------------------------------------------

func frameCompositionPreset() []interface{} {
	out := []interface{}{}
	out = append(out, offsetBaseItems(referenceFrameDemo(), "x", -1000.0)...)
	out = append(out, offsetBaseItems(robotArm(), "x", 1000.0)...)
	return out
}

// ---- trajectory preview ----------------------------------------------

func trajectoryPreviewPreset() []interface{} {
	positions := []visuals.Pose{
		{X: 0, Y: 0, Z: 0, Theta: 0},
		{X: 300, Y: 150, Z: 200, Theta: 120},
		{X: 500, Y: 300, Z: 300, Theta: 240},
		{X: 700, Y: 150, Z: 200, Theta: 120},
		{X: 1000, Y: 0, Z: 0, Theta: 0},
	}
	waypoints := waypointsWithTangentOrientations(positions)
	items := []interface{}{}
	for i, wp := range waypoints {
		items = append(items, &visuals.Sphere{
			Label: fmt.Sprintf("traj_wp_%02d", i), Pose: wp,
			RadiusMM: 18, Color: &visuals.Color{R: 200, G: 200, B: 220},
			Opacity: ptr(0.45), ShowAxesHelper: true,
		})
	}
	// Polyline as a capsule chain via the visuals.Line composite.
	items = append(items, visualsToInterfaces(visuals.Line{
		LabelPrefix: "traj", Points: waypoints, WidthMM: 10,
		Color: &visuals.Color{R: 100, G: 130, B: 240}, Opacity: ptr(0.95),
	}.ToVisuals())...)
	items = append(items, &visuals.Sphere{
		Label: "traj_runner", Pose: waypoints[0],
		RadiusMM: 28, Color: &visuals.Color{R: 230, G: 40, B: 80},
		Opacity: ptr(0.9), ShowAxesHelper: true,
		Animation: visuals.Trajectory{Waypoints: waypoints, DurationS: 12.0, Loop: true},
	})
	return items
}

func waypointsWithTangentOrientations(positions []visuals.Pose) []visuals.Pose {
	n := len(positions)
	out := make([]visuals.Pose, 0, n)
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
		out = append(out, visuals.Pose{
			X: p.X, Y: p.Y, Z: p.Z,
			OX: tx / norm, OY: ty / norm, OZ: tz / norm,
			Theta: p.Theta,
		})
	}
	return out
}

// ---- force vector demo -----------------------------------------------

func forceVectorDemoPreset() []interface{} {
	return []interface{}{&visuals.Arrow{
		Label: "force_vector", Pose: identityPose(),
		LengthMM: 220, RadiusMM: 10,
		Color: &visuals.Color{R: 230, G: 60, B: 100}, Opacity: ptr(1.0),
		Animation: visuals.ForceVector{
			PeriodS: 5.0, LengthAmplitudeMM: 80, RadiusAmplitudeMM: 5,
			TiltDeg: 45, PrecessionSpeed: 1.0, ColorSpeed: 0.7,
		},
	}}
}

// ---- geometry morph --------------------------------------------------

func geometryMorphPreset() []interface{} {
	items := []interface{}{}
	slotX := 0.0

	// Pulsing sphere.
	items = append(items, &visuals.Sphere{
		Label: "morph_pulse_sphere", Pose: poseAt(slotX, 0, 0),
		RadiusMM: 70, Color: &visuals.Color{R: 230, G: 60, B: 100}, Opacity: ptr(1.0),
		Animation: visuals.Pulse{AmplitudeMM: 35, PeriodS: 3},
	})
	slotX += 350

	// visuals.Box stretching along Z.
	items = append(items, &visuals.Box{
		Label: "morph_stretch_box", Pose: poseAt(slotX, 0, 0),
		DimsMM: visuals.BoxDims{X: 100, Y: 100, Z: 150},
		Color:  &visuals.Color{R: 100, G: 180, B: 230}, Opacity: ptr(1.0),
		Animation: visuals.Pulse{Axis: "z", AmplitudeMM: 100, PeriodS: 4},
	})
	slotX += 350

	// visuals.Capsule breathing in opacity.
	items = append(items, &visuals.Capsule{
		Label: "morph_breathe_capsule", Pose: poseAt(slotX, 0, 0),
		RadiusMM: 45, LengthMM: 240,
		Color: &visuals.Color{R: 220, G: 200, B: 60}, Opacity: ptr(0.7),
		Animation: visuals.Breathe{Amplitude: 0.55, PeriodS: 1.5},
	})
	slotX += 380

	// Two 5×5 flicker grids — green works (UUID rotates), red is
	// the bug-demo (UUID stable across re-adds).
	gridN := 5
	gridSpacing := 80.0
	periodS := 4.0
	addGrid := func(prefix string, originX float64, col *visuals.Color, rotateUUID bool) {
		for row := 0; row < gridN; row++ {
			for col2 := 0; col2 < gridN; col2++ {
				phaseOff := float64(row+col2) / float64(2*gridN-1) * periodS
				ru := rotateUUID
				items = append(items, &visuals.Sphere{
					Label: fmt.Sprintf("%s_%d%d", prefix, row, col2),
					Pose: poseAt(originX+float64(col2)*gridSpacing,
						float64(row)*gridSpacing, 0),
					RadiusMM: 22, Color: col, Opacity: ptr(1.0),
					Animation: visuals.Flicker{
						PeriodS: periodS, DutyCycle: 0.55,
						PhaseOffsetS: phaseOff, RotateUUIDOnReadd: &ru,
					},
				})
			}
		}
	}
	workingOriginX := slotX + 60
	addGrid("morph_grid", workingOriginX, &visuals.Color{R: 80, G: 200, B: 140}, true)
	brokenOriginX := workingOriginX + float64(gridN)*gridSpacing + 80
	addGrid("morph_grid_broken", brokenOriginX, &visuals.Color{R: 230, G: 60, B: 60}, false)
	return items
}

// ---- lifecycle demo --------------------------------------------------

func lifecycleDemoPreset() []interface{} {
	count := 5
	sp := 250.0
	appearS, aliveS, disappearS, goneS := 1.0, 2.0, 1.0, 2.0
	periodS := appearS + aliveS + disappearS + goneS
	items := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		off := float64(i) / float64(count) * periodS
		items = append(items, &visuals.Box{
			Label:  fmt.Sprintf("lifecycle_%02d", i),
			Pose:   poseAt((float64(i)-float64(count-1)/2.0)*sp, 0, 0),
			DimsMM: visuals.BoxDims{X: 120, Y: 120, Z: 120},
			// visuals.Color/Opacity overridden every tick by the lifecycle
			// animation; static values are placeholders.
			Color: &visuals.Color{R: 128, G: 128, B: 128}, Opacity: ptr(1.0),
			Animation: visuals.Lifecycle{
				AppearS: appearS, AliveS: aliveS,
				DisappearS: disappearS, GoneS: goneS,
				PhaseOffsetS: off,
			},
		})
	}
	return items
}

// ---- chunked PCD demo (standalone) -----------------------------------

func chunkedPCDDemoPreset() []interface{} {
	return []interface{}{&visuals.PointCloud{
		Label: "chunked_helix", Pose: identityPose(),
		PointcloudPath: "assets/helix.pcd", Opacity: ptr(1.0),
		Chunked: true, ChunkSize: 2000,
	}}
}

// ---- all (Y-stacked) -------------------------------------------------

func allPreset() []interface{} {
	row := 500.0
	armGap := 1500.0
	out := []interface{}{}
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
// composition. For Trajectory animations, the waypoint coordinates
// are shifted too — otherwise the runner would walk an un-shifted
// path while its anchor was translated.
//
// Mutates the underlying shape pointers in place; the same slice
// is returned for caller convenience. Non-Visual entries (e.g.,
// Composites) are passed through untouched — composites are
// expanded inside Scene.Add, so offsetting the composite's
// constituent Visuals (which happen separately) is what matters.
func offsetBaseItems(items []interface{}, axis string, delta float64) []interface{} {
	shiftPose := func(p visuals.Pose) visuals.Pose {
		switch axis {
		case "x":
			p.X += delta
		case "y":
			p.Y += delta
		case "z":
			p.Z += delta
		}
		return p
	}
	shiftTrajectory := func(spec visuals.AnimationSpec) visuals.AnimationSpec {
		tr, ok := spec.(visuals.Trajectory)
		if !ok {
			return spec
		}
		shifted := make([]visuals.Pose, len(tr.Waypoints))
		for i, wp := range tr.Waypoints {
			shifted[i] = shiftPose(wp)
		}
		return visuals.Trajectory{
			Waypoints: shifted, DurationS: tr.DurationS, Loop: tr.Loop,
		}
	}
	for _, it := range items {
		switch v := it.(type) {
		case *visuals.Box:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.Sphere:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.Capsule:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.Point:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.Arrow:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.Mesh:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
				v.Animation = shiftTrajectory(v.Animation)
			}
		case *visuals.PointCloud:
			if v.ParentFrame == "" || v.ParentFrame == "world" {
				v.Pose = shiftPose(v.Pose)
			}
		}
	}
	return items
}

func offsetBaseItemsY(items []interface{}, dy float64) []interface{} {
	return offsetBaseItems(items, "y", dy)
}
