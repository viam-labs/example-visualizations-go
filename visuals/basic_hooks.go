// BasicSceneHooks + BuildBasicGeometry — the easy-mode entry points
// for writing a world-state-store service with this library.
//
// The SceneHooks interface has 7 methods. Five of them (ComputeTick,
// IsAnimated, LoadPreset, BaseGeomForItem, HandleCustomCommand) only
// matter to services that animate, expose presets, or add custom
// DoCommand verbs. For a service that does none of those, this file
// provides:
//
//   - BasicSceneHooks — a zero-state struct you embed in your service
//     to get defaults for those 5 methods. With it, you only need to
//     implement BuildGeometry and ReadAsset yourself.
//   - BuildBasicGeometry(item) — a one-call dispatcher that produces
//     the commonpb.Geometry for box / sphere / capsule / point /
//     arrow primitives, so your BuildGeometry hook becomes one line.
//
// See simple_scene_example.go (in the example-visualizations-go
// repo) for the minimal service using both.
package visuals

import (
	"context"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
)

// BasicSceneHooks provides default implementations of the optional
// SceneHooks methods. Embed it in your service struct alongside
// SceneServiceBase. With both embedded, your subclass only has to
// implement BuildGeometry and ReadAsset.
//
//	type myService struct {
//	    resource.Named
//	    resource.TriviallyCloseable
//	    visuals.SceneServiceBase
//	    visuals.BasicSceneHooks   // <-- defaults for the other 5 hooks
//	}
//
// Override any of the embedded methods in your subclass if you do
// need animation, presets, or custom DoCommand verbs.
type BasicSceneHooks struct{}

// ComputeTick: no animation — the runner pose equals the base pose.
func (BasicSceneHooks) ComputeTick(_ Item, basePose Pose, _ BaseGeom, _ float64) TickResult {
	return TickResult{Pose: basePose}
}

// IsAnimated: this service has no animated items.
func (BasicSceneHooks) IsAnimated(_ Item) bool { return false }

// LoadPreset: this service has no presets.
func (BasicSceneHooks) LoadPreset(name string) ([]Item, error) {
	return nil, fmt.Errorf("this service has no presets (LoadPreset called for %q)", name)
}

// BaseGeomForItem: extract the shape-specific base fields for the
// standard primitive types. Subclasses that add new sugar types must
// override and forward to the default for the types they don't add.
func (BasicSceneHooks) BaseGeomForItem(item Item) BaseGeom {
	bg := BaseGeom{}
	switch item.Type {
	case "box":
		if item.HasDims {
			bg.Dims = item.DimsMM
			bg.HasDims = true
		}
	case "sphere":
		bg.RadiusMM = item.RadiusMM
	case "capsule", "arrow":
		bg.RadiusMM = item.RadiusMM
		bg.LengthMM = item.LengthMM
	}
	return bg
}

// HandleCustomCommand: no custom verbs — falls through to the base
// class's debug-snapshot reply.
func (BasicSceneHooks) HandleCustomCommand(_ context.Context, _ map[string]any) (map[string]any, bool, error) {
	return nil, false, nil
}

// BuildBasicGeometry builds the commonpb.Geometry proto for the
// standard non-asset primitive types: box, sphere, capsule, point,
// arrow. Mesh and pointcloud are excluded because they require I/O
// (a ReadAsset call) and a content-type / PLY-conversion decision —
// services that use those need their own BuildGeometry hook.
//
// Typical use in your BuildGeometry hook:
//
//	func (s *myService) BuildGeometry(item visuals.Item, _ visuals.BaseGeom) (*commonpb.Geometry, error) {
//	    return visuals.BuildBasicGeometry(item)
//	}
//
// For animation-driven size overrides, see the standalone-playground
// model — it consults the BaseGeom argument before building.
func BuildBasicGeometry(item Item) (*commonpb.Geometry, error) {
	switch item.Type {
	case "box":
		d := item.DimsMM
		return &commonpb.Geometry{
			Label: item.Label,
			GeometryType: &commonpb.Geometry_Box{
				Box: &commonpb.RectangularPrism{
					DimsMm: &commonpb.Vector3{X: d.X, Y: d.Y, Z: d.Z},
				},
			},
		}, nil
	case "sphere":
		return &commonpb.Geometry{
			Label: item.Label,
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{RadiusMm: item.RadiusMM},
			},
		}, nil
	case "capsule":
		return &commonpb.Geometry{
			Label: item.Label,
			GeometryType: &commonpb.Geometry_Capsule{
				Capsule: &commonpb.Capsule{
					RadiusMm: item.RadiusMM,
					LengthMm: item.LengthMM,
				},
			},
		}, nil
	case "point":
		return &commonpb.Geometry{
			Label: item.Label,
			GeometryType: &commonpb.Geometry_Sphere{
				Sphere: &commonpb.Sphere{RadiusMm: PointMarkerRadiusMM},
			},
		}, nil
	case "arrow":
		// Arrow is procedurally generated as a PLY mesh — a tiny
		// helper bakes the cylinder+cone into bytes.
		ply := ArrowPLYBytes(item.LengthMM, item.RadiusMM)
		return &commonpb.Geometry{
			Label: item.Label,
			GeometryType: &commonpb.Geometry_Mesh{
				Mesh: &commonpb.Mesh{ContentType: "ply", Mesh: ply},
			},
		}, nil
	}
	return nil, fmt.Errorf("BuildBasicGeometry doesn't handle item type %q (use a custom BuildGeometry hook for mesh / pointcloud)", item.Type)
}
