// Pure builders that turn a normalized Item into the proto pieces
// the WorldStateStore service emits — a *commonpb.Geometry, an
// optional metadata *structpb.Struct, and a *commonpb.Pose.
//
// Each Item carries a primitive type, label, pose, optional color
// /opacity, and shape-specific fields. The builders do no validation
// — that lives in validateItem. They do not read files for the
// mesh/pointcloud builders either; the caller passes bytes in, so
// tests can drive the builders without touching disk.
package exampleviz

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	structpb "google.golang.org/protobuf/types/known/structpb"
)

// SupportedTypes — closed set of item.type values.
var SupportedTypes = []string{"box", "sphere", "capsule", "point", "arrow", "mesh", "pointcloud"}

// SupportedMeshContentTypes — the renderer is strict here; uppercase
// variants are rejected. See LESSONS.md::mesh-formats.
var SupportedMeshContentTypes = []string{"ply", "stl"}

// RendererMeshContentType — the viewer only renders PLY meshes. STL
// is converted to PLY on the way in via stlToPLY.
const RendererMeshContentType = "ply"

// PointMarkerRadiusMM — radius used for the "point" primitive. The
// Geometry oneof has no Point variant; a radius=0 sphere is invisible
// in the viewer. A small visible radius gives the user something to
// see while reading as a "marker" rather than a sphere.
const PointMarkerRadiusMM = 8.0

// ---- color / opacity / metadata --------------------------------------

// Color is a 0..255 RGB triple.
type Color struct {
	R, G, B int
}

// clampU8 clamps any integer into 0..255.
func clampU8(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// extractPLYVertexColors parses an ASCII PLY and returns per-vertex
// (R, G, B) tuples if the file carries property uchar red/green/blue.
// Returns nil if the PLY doesn't have vertex colors or can't be parsed.
//
// Why this exists: the Viam 3D scene viewer reads
// Transform.metadata.colors for per-vertex coloring, NOT PLY's own
// embedded vertex colors. Transcoding PLY → metadata.colors is the
// only way a vertex-colored PLY renders correctly.
//
// Binary PLY is not currently supported — only ASCII.
func extractPLYVertexColors(ply []byte) [][3]int {
	text := string(ply)
	if !strings.HasPrefix(text, "ply\n") {
		return nil
	}
	headerEnd := strings.Index(text, "end_header")
	if headerEnd < 0 {
		return nil
	}
	header := text[:headerEnd]
	if !strings.Contains(header, "format ascii") {
		return nil
	}

	lines := strings.Split(text, "\n")
	vertexCount := 0
	var vertexProps []string
	parsingVertex := false
	headerEndLine := -1
	for i, line := range lines {
		if line == "end_header" {
			headerEndLine = i + 1
			break
		}
		if strings.HasPrefix(line, "element vertex ") {
			n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "element vertex ")))
			if err != nil {
				return nil
			}
			vertexCount = n
			parsingVertex = true
		} else if strings.HasPrefix(line, "element ") {
			parsingVertex = false
		} else if parsingVertex && strings.HasPrefix(line, "property ") {
			fields := strings.Fields(line)
			vertexProps = append(vertexProps, fields[len(fields)-1])
		}
	}
	if headerEndLine < 0 || vertexCount == 0 {
		return nil
	}
	rIdx, gIdx, bIdx := -1, -1, -1
	for i, p := range vertexProps {
		switch p {
		case "red":
			rIdx = i
		case "green":
			gIdx = i
		case "blue":
			bIdx = i
		}
	}
	if rIdx < 0 || gIdx < 0 || bIdx < 0 {
		return nil
	}
	colors := make([][3]int, 0, vertexCount)
	for i := 0; i < vertexCount; i++ {
		li := headerEndLine + i
		if li >= len(lines) {
			return nil
		}
		parts := strings.Fields(lines[li])
		if len(parts) < len(vertexProps) {
			return nil
		}
		rv, e1 := strconv.Atoi(parts[rIdx])
		gv, e2 := strconv.Atoi(parts[gIdx])
		bv, e3 := strconv.Atoi(parts[bIdx])
		if e1 != nil || e2 != nil || e3 != nil {
			return nil
		}
		colors = append(colors, [3]int{rv, gv, bv})
	}
	if len(colors) == 0 {
		return nil
	}
	return colors
}

// MetadataOpts controls what build_metadata emits.
type MetadataOpts struct {
	Color          *Color
	Opacity        *float64
	ShowAxesHelper bool
	Invisible      bool
	VertexColors   [][3]int
	Chunks         map[string]any
}

// buildMetadata encodes the metadata struct the 3D viewer reads.
//
// Schema comes from viamrobotics/visualization (draw/transform.go).
// MUST include all five required keys (colors, color_format,
// opacities, show_axes_helper, invisible) — omitting any of them
// produces an invisible entity per LESSONS.md::metadata-keys-must-
// all-be-present. Optional `chunks` sub-struct declares chunked
// delivery of a large entity (experimental).
func buildMetadata(opts MetadataOpts) *structpb.Struct {
	fields := map[string]any{}

	if len(opts.VertexColors) > 0 {
		packed := make([]byte, 0, len(opts.VertexColors)*3)
		for _, c := range opts.VertexColors {
			packed = append(packed,
				byte(clampU8(c[0])), byte(clampU8(c[1])), byte(clampU8(c[2])))
		}
		fields["colors"] = base64.StdEncoding.EncodeToString(packed)
	} else if opts.Color != nil {
		rgb := []byte{byte(clampU8(opts.Color.R)), byte(clampU8(opts.Color.G)), byte(clampU8(opts.Color.B))}
		fields["colors"] = base64.StdEncoding.EncodeToString(rgb)
	} else {
		fields["colors"] = ""
	}
	fields["color_format"] = 1.0 // COLOR_FORMAT_RGB

	alpha := 255
	if opts.Opacity != nil {
		alpha = clampU8(int(math.Round(*opts.Opacity * 255)))
	}
	fields["opacities"] = base64.StdEncoding.EncodeToString([]byte{byte(alpha)})
	fields["show_axes_helper"] = opts.ShowAxesHelper
	fields["invisible"] = opts.Invisible

	if opts.Chunks != nil {
		fields["chunks"] = opts.Chunks
	}

	s, err := structpb.NewStruct(fields)
	if err != nil {
		// Shouldn't happen; we control the input.
		return nil
	}
	return s
}

// ---- pose ------------------------------------------------------------

// Pose is the JSON-shape pose dict (mm + orientation vector + theta).
type Pose struct {
	X, Y, Z       float64
	OX, OY, OZ    float64
	Theta         float64
	hasOrient     bool // tracks whether OX/OY/OZ were explicitly set
}

// IdentityPose returns a zero-pose with OZ=1 (identity orientation
// vector in Viam's convention).
func IdentityPose() Pose {
	return Pose{OZ: 1.0}
}

// PoseXYZ is a convenience constructor.
func PoseXYZ(x, y, z float64) Pose {
	return Pose{X: x, Y: y, Z: z, OZ: 1.0}
}

// buildPose converts the JSON-shape Pose to commonpb.Pose. If the
// caller didn't set any of OX/OY/OZ, we default OZ=1 (identity).
func buildPose(p Pose) *commonpb.Pose {
	oz := p.OZ
	if p.OX == 0 && p.OY == 0 && p.OZ == 0 {
		oz = 1.0
	}
	return &commonpb.Pose{
		X:     p.X,
		Y:     p.Y,
		Z:     p.Z,
		OX:    p.OX,
		OY:    p.OY,
		OZ:    oz,
		Theta: p.Theta,
	}
}

// ---- primitive Geometry builders -------------------------------------

func buildBox(dims [3]float64, label string) *commonpb.Geometry {
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Box{
			Box: &commonpb.RectangularPrism{
				DimsMm: &commonpb.Vector3{X: dims[0], Y: dims[1], Z: dims[2]},
			},
		},
	}
}

func buildSphere(radiusMM float64, label string) *commonpb.Geometry {
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Sphere{
			Sphere: &commonpb.Sphere{RadiusMm: radiusMM},
		},
	}
}

func buildCapsule(radiusMM, lengthMM float64, label string) *commonpb.Geometry {
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Capsule{
			Capsule: &commonpb.Capsule{RadiusMm: radiusMM, LengthMm: lengthMM},
		},
	}
}

func buildPoint(label string) *commonpb.Geometry {
	return buildSphere(PointMarkerRadiusMM, label)
}

func buildMesh(plyBytes []byte, contentType, label string) (*commonpb.Geometry, error) {
	if contentType != RendererMeshContentType {
		return nil, fmt.Errorf("build_mesh requires content_type %q; got %q (STL must be converted via stlToPLY first)",
			RendererMeshContentType, contentType)
	}
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{ContentType: contentType, Mesh: plyBytes},
		},
	}, nil
}

func buildPointcloud(pcdBytes []byte, label string) *commonpb.Geometry {
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Pointcloud{
			Pointcloud: &commonpb.PointCloud{PointCloud: pcdBytes},
		},
	}
}

// ---- arrow (procedural PLY) ------------------------------------------

// arrowPLYBytes generates a procedural arrow mesh along local +Z.
// Coordinates are written in METERS (RDK PLY reader multiplies by
// 1000). tipRadiusMM defaults to 2× shaft radius; tipLengthMM
// defaults to 28% of total length.
func arrowPLYBytes(lengthMM, shaftRadiusMM float64) []byte {
	tipRadiusMM := 2.0 * shaftRadiusMM
	tipLengthMM := math.Max(0.05*lengthMM, 0.28*lengthMM)
	shaftLengthMM := math.Max(0, lengthMM-tipLengthMM)
	sides := 12

	verts := [][3]float64{{0, 0, 0}} // v0: shaft bottom center
	// Shaft bottom ring (z=0, shaftRadius).
	for i := 0; i < sides; i++ {
		t := 2 * math.Pi * float64(i) / float64(sides)
		verts = append(verts, [3]float64{shaftRadiusMM * math.Cos(t), shaftRadiusMM * math.Sin(t), 0})
	}
	// Shaft top ring (z=shaftLength, shaftRadius).
	for i := 0; i < sides; i++ {
		t := 2 * math.Pi * float64(i) / float64(sides)
		verts = append(verts, [3]float64{shaftRadiusMM * math.Cos(t), shaftRadiusMM * math.Sin(t), shaftLengthMM})
	}
	// Cone base ring (z=shaftLength, tipRadius).
	for i := 0; i < sides; i++ {
		t := 2 * math.Pi * float64(i) / float64(sides)
		verts = append(verts, [3]float64{tipRadiusMM * math.Cos(t), tipRadiusMM * math.Sin(t), shaftLengthMM})
	}
	apexIdx := 1 + 3*sides
	verts = append(verts, [3]float64{0, 0, shaftLengthMM + tipLengthMM})

	botRing := 1
	topRing := 1 + sides
	coneRing := 1 + 2*sides

	var faces [][]int
	// Bottom cap fan.
	for i := 0; i < sides; i++ {
		curr := botRing + i
		next := botRing + (i+1)%sides
		faces = append(faces, []int{0, next, curr})
	}
	// Shaft side quads → triangles.
	for i := 0; i < sides; i++ {
		b := botRing + i
		bn := botRing + (i+1)%sides
		tt := topRing + i
		tn := topRing + (i+1)%sides
		faces = append(faces, []int{b, bn, tt})
		faces = append(faces, []int{bn, tn, tt})
	}
	// Washer between shaft top and cone base.
	for i := 0; i < sides; i++ {
		inner := topRing + i
		innerN := topRing + (i+1)%sides
		outer := coneRing + i
		outerN := coneRing + (i+1)%sides
		faces = append(faces, []int{inner, outer, innerN})
		faces = append(faces, []int{innerN, outer, outerN})
	}
	// Cone side triangles.
	for i := 0; i < sides; i++ {
		b := coneRing + i
		bn := coneRing + (i+1)%sides
		faces = append(faces, []int{b, bn, apexIdx})
	}

	return plyASCIIBytes(verts, faces, nil)
}

func plyASCIIBytes(verts [][3]float64, faces [][]int, vertexColors [][3]int) []byte {
	hasColors := vertexColors != nil
	var b strings.Builder
	b.WriteString("ply\n")
	b.WriteString("format ascii 1.0\n")
	b.WriteString(fmt.Sprintf("element vertex %d\n", len(verts)))
	b.WriteString("property float x\n")
	b.WriteString("property float y\n")
	b.WriteString("property float z\n")
	if hasColors {
		b.WriteString("property uchar red\n")
		b.WriteString("property uchar green\n")
		b.WriteString("property uchar blue\n")
	}
	b.WriteString(fmt.Sprintf("element face %d\n", len(faces)))
	b.WriteString("property list uchar int vertex_indices\n")
	b.WriteString("end_header\n")
	for i, v := range verts {
		if hasColors {
			c := vertexColors[i]
			b.WriteString(fmt.Sprintf("%.6f %.6f %.6f %d %d %d\n",
				v[0]/1000.0, v[1]/1000.0, v[2]/1000.0,
				clampU8(c[0]), clampU8(c[1]), clampU8(c[2])))
		} else {
			b.WriteString(fmt.Sprintf("%.6f %.6f %.6f\n", v[0]/1000.0, v[1]/1000.0, v[2]/1000.0))
		}
	}
	for _, f := range faces {
		b.WriteString(fmt.Sprintf("%d", len(f)))
		for _, idx := range f {
			b.WriteString(fmt.Sprintf(" %d", idx))
		}
		b.WriteString("\n")
	}
	return []byte(b.String())
}

func buildArrow(lengthMM, radiusMM float64, label string) *commonpb.Geometry {
	ply := arrowPLYBytes(lengthMM, radiusMM)
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{ContentType: RendererMeshContentType, Mesh: ply},
		},
	}
}

// ---- STL → PLY conversion --------------------------------------------

// stlToPLY converts binary STL bytes to ASCII PLY bytes. The viewer
// only renders PLY on the wire even though the RDK parses both.
// Per-triangle vertices (no dedup) — fine for the assets shipped
// in this module.
func stlToPLY(stl []byte) ([]byte, error) {
	if len(stl) < 84 {
		return nil, fmt.Errorf("STL data too small (need >=84 bytes for header)")
	}
	nTris := int(binary.LittleEndian.Uint32(stl[80:84]))
	expected := 84 + nTris*50
	if len(stl) < expected {
		return nil, fmt.Errorf("STL truncated: expected %d bytes for %d triangles, got %d",
			expected, nTris, len(stl))
	}
	verts := make([][3]float64, 0, nTris*3)
	faces := make([][]int, 0, nTris)
	offset := 84
	for t := 0; t < nTris; t++ {
		offset += 12 // skip normal
		face := make([]int, 3)
		for v := 0; v < 3; v++ {
			x := math.Float32frombits(binary.LittleEndian.Uint32(stl[offset : offset+4]))
			y := math.Float32frombits(binary.LittleEndian.Uint32(stl[offset+4 : offset+8]))
			z := math.Float32frombits(binary.LittleEndian.Uint32(stl[offset+8 : offset+12]))
			offset += 12
			face[v] = len(verts)
			verts = append(verts, [3]float64{float64(x), float64(y), float64(z)})
		}
		offset += 2 // skip attribute byte count
		faces = append(faces, face)
	}
	// STL stores meters directly — don't divide by 1000.
	var b strings.Builder
	b.WriteString("ply\n")
	b.WriteString("format ascii 1.0\n")
	b.WriteString(fmt.Sprintf("element vertex %d\n", len(verts)))
	b.WriteString("property float x\n")
	b.WriteString("property float y\n")
	b.WriteString("property float z\n")
	b.WriteString(fmt.Sprintf("element face %d\n", len(faces)))
	b.WriteString("property list uchar int vertex_indices\n")
	b.WriteString("end_header\n")
	for _, v := range verts {
		b.WriteString(fmt.Sprintf("%.6f %.6f %.6f\n", v[0], v[1], v[2]))
	}
	for _, f := range faces {
		b.WriteString(fmt.Sprintf("3 %d %d %d\n", f[0], f[1], f[2]))
	}
	return []byte(b.String()), nil
}

func loadMeshBytesAsPLY(asset []byte, sourcePath string) ([]byte, error) {
	fmt2, err := inferMeshContentType(sourcePath)
	if err != nil {
		return nil, err
	}
	if fmt2 == "stl" {
		return stlToPLY(asset)
	}
	return asset, nil
}

func inferMeshContentType(p string) (string, error) {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
	for _, ct := range SupportedMeshContentTypes {
		if ext == ct {
			return ext, nil
		}
	}
	return "", fmt.Errorf("mesh content type %q is not supported; only %v are accepted by the viewer",
		ext, SupportedMeshContentTypes)
}

// ---- PCD chunked-delivery helpers ------------------------------------

// parsePCDBinary splits a PCDBinary blob into (header, body, stride,
// totalPoints). Used by chunked-delivery: callers split body on
// stride boundaries to emit individual chunks.
func parsePCDBinary(pcd []byte) ([]byte, []byte, int, int, error) {
	marker := []byte("DATA binary\n")
	idx := indexBytes(pcd, marker)
	if idx < 0 {
		return nil, nil, 0, 0, fmt.Errorf("PCD: missing 'DATA binary' marker")
	}
	headerEnd := idx + len(marker)
	header := pcd[:headerEnd]
	body := pcd[headerEnd:]

	var sizeLine, countLine string
	for _, line := range strings.Split(string(header), "\n") {
		if strings.HasPrefix(line, "SIZE ") {
			sizeLine = strings.TrimPrefix(line, "SIZE ")
		}
		if strings.HasPrefix(line, "COUNT ") {
			countLine = strings.TrimPrefix(line, "COUNT ")
		}
	}
	if sizeLine == "" || countLine == "" {
		return nil, nil, 0, 0, fmt.Errorf("PCD: missing SIZE or COUNT")
	}
	sizes, err := parseIntFields(sizeLine)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("PCD: SIZE parse: %w", err)
	}
	counts, err := parseIntFields(countLine)
	if err != nil {
		return nil, nil, 0, 0, fmt.Errorf("PCD: COUNT parse: %w", err)
	}
	if len(sizes) != len(counts) {
		return nil, nil, 0, 0, fmt.Errorf("PCD: SIZE/COUNT length mismatch")
	}
	stride := 0
	for i := range sizes {
		stride += sizes[i] * counts[i]
	}
	if stride <= 0 {
		return nil, nil, 0, 0, fmt.Errorf("PCD: invalid stride %d", stride)
	}
	totalPoints := len(body) / stride
	return header, body, stride, totalPoints, nil
}

// buildPCDChunk builds a self-contained PCDBinary blob containing only
// the chunk at chunkIndex. Rewrites WIDTH and POINTS so the result is
// a valid standalone PCD.
func buildPCDChunk(header, body []byte, stride, chunkIndex, chunkSizePoints int) ([]byte, error) {
	totalPoints := len(body) / stride
	start := chunkIndex * chunkSizePoints
	if start >= totalPoints {
		return nil, fmt.Errorf("chunk_index %d out of range; total_points=%d chunk_size=%d",
			chunkIndex, totalPoints, chunkSizePoints)
	}
	end := start + chunkSizePoints
	if end > totalPoints {
		end = totalPoints
	}
	n := end - start
	bodySlice := body[start*stride : end*stride]

	var newLines []string
	for _, line := range strings.Split(string(header), "\n") {
		switch {
		case strings.HasPrefix(line, "WIDTH "):
			newLines = append(newLines, fmt.Sprintf("WIDTH %d", n))
		case strings.HasPrefix(line, "POINTS "):
			newLines = append(newLines, fmt.Sprintf("POINTS %d", n))
		default:
			newLines = append(newLines, line)
		}
	}
	out := []byte(strings.Join(newLines, "\n"))
	out = append(out, bodySlice...)
	return out, nil
}

func parseIntFields(s string) ([]int, error) {
	out := []int{}
	for _, f := range strings.Fields(s) {
		v, err := strconv.Atoi(f)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func indexBytes(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
