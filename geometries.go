// Service-side geometry builders.
//
// Turns the item structs the service consumes into commonpb.Geometry
// protos. This is the bridge between the library's typed scene
// objects (produced by exampleviz/visuals) and the WorldStateStore
// wire format.
//
// The pure asset I/O — PLY/STL/PCD parsers, the metadata Struct
// builder, the procedural arrow generator — lives in
// exampleviz/visuals. This file keeps the small set of
// viam-proto-emitting builders that need to be co-located with the
// service.
package exampleviz

import (
	"fmt"
	"os"
	"path/filepath"

	"exampleviz/visuals"
	commonpb "go.viam.com/api/common/v1"
)

// SupportedTypes — closed set of item.type values (config-level).
var SupportedTypes = []string{"box", "sphere", "capsule", "point", "arrow", "mesh", "pointcloud"}

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
	return buildSphere(visuals.PointMarkerRadiusMM, label)
}

// buildMesh embeds mesh bytes into a Geometry. contentType MUST be
// "ply" unless allowNonPLY is set — the viewer only renders PLY.
// The opt-out exists for the playground's raw-STL bug-demo.
func buildMesh(plyBytes []byte, contentType, label string, allowNonPLY bool) (*commonpb.Geometry, error) {
	if !allowNonPLY && contentType != visuals.RendererMeshContentType {
		return nil, fmt.Errorf("build_mesh requires content_type %q; got %q (STL must be converted via STLToPLY first)",
			visuals.RendererMeshContentType, contentType)
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

func buildArrow(lengthMM, radiusMM float64, label string) *commonpb.Geometry {
	ply := visuals.ArrowPLYBytes(lengthMM, radiusMM)
	return &commonpb.Geometry{
		Label: label,
		GeometryType: &commonpb.Geometry_Mesh{
			Mesh: &commonpb.Mesh{ContentType: visuals.RendererMeshContentType, Mesh: ply},
		},
	}
}

// readAsset resolves an asset path relative to the module dir and
// reads it. Absolute paths are honored as-is.
func readAsset(assetPath string) ([]byte, error) {
	p := assetPath
	if !filepath.IsAbs(p) && ModuleDir != "" {
		p = filepath.Join(ModuleDir, p)
	}
	return os.ReadFile(p)
}
