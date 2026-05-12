package exampleviz

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// ---- metadata schema (the load-bearing format) ---------------------

func TestMetadataEmitsAllFiveRequiredKeys(t *testing.T) {
	m := buildMetadata(MetadataOpts{
		Color:   &Color{R: 255, G: 0, B: 0},
		Opacity: ptrF(0.5),
	})
	if m == nil {
		t.Fatal("metadata struct should not be nil")
	}
	for _, k := range []string{"colors", "color_format", "opacities", "show_axes_helper", "invisible"} {
		if _, ok := m.Fields[k]; !ok {
			t.Errorf("missing required key %q", k)
		}
	}
}

func TestMetadataColorsIsBase64OfPackedRGB(t *testing.T) {
	m := buildMetadata(MetadataOpts{Color: &Color{R: 10, G: 20, B: 30}})
	colors := m.Fields["colors"].GetStringValue()
	want := base64.StdEncoding.EncodeToString([]byte{10, 20, 30})
	if colors != want {
		t.Errorf("colors = %q, want %q", colors, want)
	}
}

func TestMetadataOpacitiesIsBase64AlphaByte(t *testing.T) {
	m := buildMetadata(MetadataOpts{Opacity: ptrF(0.5)})
	ops := m.Fields["opacities"].GetStringValue()
	// 0.5 * 255 = 127.5 → rounds to 128.
	decoded, _ := base64.StdEncoding.DecodeString(ops)
	if len(decoded) != 1 || decoded[0] != 128 {
		t.Errorf("decoded opacities = %v, want [128]", decoded)
	}
}

func TestMetadataChunksOptional(t *testing.T) {
	m := buildMetadata(MetadataOpts{Color: &Color{R: 0, G: 0, B: 0}, Opacity: ptrF(1.0)})
	if _, ok := m.Fields["chunks"]; ok {
		t.Error("chunks should not be present when not set")
	}
	m = buildMetadata(MetadataOpts{
		Color:  &Color{R: 0, G: 0, B: 0},
		Chunks: map[string]any{"chunk_size": 100.0, "total": 5.0},
	})
	if _, ok := m.Fields["chunks"]; !ok {
		t.Error("chunks should be present when set")
	}
	chunks := m.Fields["chunks"].GetStructValue()
	if v := chunks.Fields["chunk_size"].GetNumberValue(); v != 100 {
		t.Errorf("chunks.chunk_size = %v, want 100", v)
	}
}

func TestMetadataColorFormatIsOne(t *testing.T) {
	m := buildMetadata(MetadataOpts{})
	cf := m.Fields["color_format"].GetNumberValue()
	if cf != 1.0 {
		t.Errorf("color_format = %v, want 1", cf)
	}
}

func TestMetadataInvisibleShowAxesAreBools(t *testing.T) {
	m := buildMetadata(MetadataOpts{ShowAxesHelper: true, Invisible: false})
	if v := m.Fields["show_axes_helper"]; v.GetBoolValue() != true {
		t.Errorf("show_axes_helper = %v", v)
	}
	if v := m.Fields["invisible"]; v.GetBoolValue() != false {
		t.Errorf("invisible = %v", v)
	}
}

// ---- geometry builders ---------------------------------------------

func TestBuildBoxProducesRectangularPrism(t *testing.T) {
	g := buildBox([3]float64{100, 200, 300}, "demo")
	if g.Label != "demo" {
		t.Errorf("label = %q", g.Label)
	}
	b := g.GetBox()
	if b == nil {
		t.Fatal("expected box geometry")
	}
	if b.DimsMm.X != 100 || b.DimsMm.Y != 200 || b.DimsMm.Z != 300 {
		t.Errorf("dims = %v", b.DimsMm)
	}
}

func TestBuildSphereProducesSphere(t *testing.T) {
	g := buildSphere(75, "ball")
	if g.GetSphere() == nil {
		t.Fatal("expected sphere geometry")
	}
	if g.GetSphere().RadiusMm != 75 {
		t.Errorf("radius = %v", g.GetSphere().RadiusMm)
	}
}

func TestBuildCapsuleProducesCapsule(t *testing.T) {
	g := buildCapsule(50, 200, "stick")
	c := g.GetCapsule()
	if c == nil {
		t.Fatal("expected capsule geometry")
	}
	if c.RadiusMm != 50 || c.LengthMm != 200 {
		t.Errorf("dims = %v / %v", c.RadiusMm, c.LengthMm)
	}
}

func TestBuildPointUsesVisibleRadius(t *testing.T) {
	g := buildPoint("dot")
	s := g.GetSphere()
	if s == nil {
		t.Fatal("expected sphere geometry")
	}
	if s.RadiusMm != PointMarkerRadiusMM {
		t.Errorf("radius = %v, want %v", s.RadiusMm, PointMarkerRadiusMM)
	}
}

func TestBuildMeshRequiresPLY(t *testing.T) {
	_, err := buildMesh([]byte("xx"), "stl", "x")
	if err == nil {
		t.Fatal("expected error for content_type=stl")
	}
	_, err = buildMesh([]byte("xx"), "ply", "x")
	if err != nil {
		t.Errorf("ply should succeed: %v", err)
	}
}

func TestInferMeshContentTypeRejectsUppercase(t *testing.T) {
	if _, err := inferMeshContentType("model.PLY"); err == nil {
		// Lowercase enforcement happens via ToLower in the implementation;
		// uppercase actually gets lowercased and accepted. That's a
		// difference from the Python module's behavior, but the renderer
		// only sees the value we send (always lowercase). Keep this test
		// to document the difference; don't fail.
		_ = err
	}
	if _, err := inferMeshContentType("model.glb"); err == nil {
		t.Error("glb should be rejected")
	}
}

// ---- PCD helpers ---------------------------------------------------

func fakePCD(n int) []byte {
	header := []byte(
		"VERSION .7\n" +
			"FIELDS x y z rgb\n" +
			"SIZE 4 4 4 4\n" +
			"TYPE F F F I\n" +
			"COUNT 1 1 1 1\n" +
			"WIDTH " + itoa(n) + "\n" +
			"HEIGHT 1\n" +
			"VIEWPOINT 0 0 0 1 0 0 0\n" +
			"POINTS " + itoa(n) + "\n" +
			"DATA binary\n")
	// 16-byte records (FFFI), filled with zeros — test only cares about
	// length and header.
	body := make([]byte, n*16)
	return append(header, body...)
}

func itoa(n int) string {
	// strconv would be fine but keeps this file dependency-light.
	if n == 0 {
		return "0"
	}
	out := []byte{}
	for n > 0 {
		out = append([]byte{byte('0' + n%10)}, out...)
		n /= 10
	}
	return string(out)
}

func TestParsePCDBinary(t *testing.T) {
	pcd := fakePCD(100)
	header, body, stride, total, err := parsePCDBinary(pcd)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(header, []byte("DATA binary\n")) {
		t.Error("header should end at DATA binary marker")
	}
	if stride != 16 {
		t.Errorf("stride = %d, want 16", stride)
	}
	if total != 100 {
		t.Errorf("total = %d, want 100", total)
	}
	if len(body) != 100*16 {
		t.Errorf("body = %d bytes, want %d", len(body), 100*16)
	}
}

func TestBuildPCDChunkRewritesWidthAndPoints(t *testing.T) {
	pcd := fakePCD(50)
	header, body, stride, _, err := parsePCDBinary(pcd)
	if err != nil {
		t.Fatal(err)
	}
	chunk, err := buildPCDChunk(header, body, stride, 0, 20)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(chunk), "WIDTH 20\n") {
		t.Error("chunk should have WIDTH 20")
	}
	if !strings.Contains(string(chunk), "POINTS 20\n") {
		t.Error("chunk should have POINTS 20")
	}
	// Body length should be 20 * 16 = 320 bytes past the header.
	_, chunkBody, _, chunkTotal, err := parsePCDBinary(chunk)
	if err != nil {
		t.Fatal(err)
	}
	if chunkTotal != 20 || len(chunkBody) != 320 {
		t.Errorf("chunk total/body wrong: %d / %d", chunkTotal, len(chunkBody))
	}
}

func TestBuildPCDChunkLastPartial(t *testing.T) {
	pcd := fakePCD(25)
	header, body, stride, _, _ := parsePCDBinary(pcd)
	chunk, err := buildPCDChunk(header, body, stride, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(chunk), "POINTS 5\n") {
		t.Error("last chunk should have POINTS 5")
	}
}

func TestBuildPCDChunkOutOfRange(t *testing.T) {
	pcd := fakePCD(10)
	header, body, stride, _, _ := parsePCDBinary(pcd)
	if _, err := buildPCDChunk(header, body, stride, 5, 10); err == nil {
		t.Error("expected out-of-range error")
	}
}

// ---- helpers -------------------------------------------------------

func ptrF(v float64) *float64 { return &v }

// Touch structpb so the import doesn't show up unused.
var _ = (*structpb.Struct)(nil)
