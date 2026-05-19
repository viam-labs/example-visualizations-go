// simple-scene-example — the smallest possible world-state-store
// service. Publishes three static geometries to the Viam 3D scene
// viewer.
//
// READ THIS FIRST if you're learning the visuals library. This is
// the canonical "I just want to add a few geometries to the 3D
// scene viewer" reference. Everything a Viam Go module author has
// to write to ship a working WSS service is in this file — no
// helper-method embeds from elsewhere in the library, no hidden
// shortcuts. Each method below is one a new user would write by
// hand.
//
// What the library gives you for free
// -----------------------------------
//
// By embedding visuals.SceneServiceBase, the gRPC WorldStateStore
// implementation (ListUUIDs / GetTransform / StreamTransformChanges),
// the state map, the subscriber broadcast, the animation tick
// loop, the UUID strategy, and the standard DoCommand verbs (list
// / clear / snapshot / apply_events / etc.) all just work.
//
// What this file shows
// --------------------
//
//   - Registering the model in init().
//   - The constructor (newSimpleScene), Reconfigure, Close, and
//     DoCommand entry points the framework calls.
//   - Building a scene from typed visuals.Box / .Sphere / .Capsule
//     and handing it to the library.
//   - All 7 SceneHooks methods inline: BuildGeometry, ReadAsset,
//     ComputeTick, IsAnimated, LoadPreset, BaseGeomForItem,
//     HandleCustomCommand. For this static, asset-free, no-preset
//     scene most are one-line stubs — but they ARE the surface a
//     new user has to write.
//
// What it does NOT show
// ---------------------
//
//   - Animations, presets, mesh / pointcloud assets, custom
//     DoCommand verbs. The standalone-playground model in
//     service.go demonstrates each of those.
//
// Configure as a `rdk:service:world_state_store` service with model
// viam:example-visualizations-go:simple-scene-example. No attributes
// are required — the scene is hardcoded.
package exampleviz

import (
	"context"
	"fmt"

	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"

	"exampleviz/visuals"
)

// SimpleModel is the registered model identifier.
var SimpleModel = resource.NewModel(
	"viam", "example-visualizations-go", "simple-scene-example",
)

func init() {
	resource.RegisterService(worldstatestore.API, SimpleModel,
		resource.Registration[worldstatestore.Service, resource.NoNativeConfig]{
			Constructor: newSimpleScene,
		},
	)
}

// simpleScene embeds visuals.SceneServiceBase — the library does
// the WSS plumbing — and implements the 7 SceneHooks methods
// directly. resource.Named and resource.TriviallyCloseable provide
// the boilerplate the SDK framework requires from every resource.
type simpleScene struct {
	resource.Named
	resource.TriviallyCloseable
	visuals.SceneServiceBase
}

func newSimpleScene(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	s := &simpleScene{Named: conf.ResourceName().AsNamed()}

	// Tell the embedded SceneServiceBase which instance to call
	// back into for the hooks (BuildGeometry, ComputeTick, etc).
	s.SceneServiceBase.Hooks = s
	s.SceneServiceBase.Logger = logger

	// The framework does NOT call Reconfigure automatically on
	// initial construction — call it explicitly here, otherwise
	// the service starts with no items.
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

// Reconfigure builds the (hardcoded) scene and hands it to the
// library. Real services would parse conf.Attributes here to pick
// up tick_hz / uuid_strategy / parent_frame; we just take the
// library defaults.
func (s *simpleScene) Reconfigure(
	_ context.Context, _ resource.Dependencies, _ resource.Config,
) error {
	red := visuals.Color{R: 230, G: 25, B: 75}
	green := visuals.Color{R: 60, G: 180, B: 75}
	blue := visuals.Color{R: 0, G: 130, B: 200}

	items := visuals.ToItems(
		visuals.Box{
			Label:  "demo_box",
			Pose:   visuals.PoseAt(-300, 0, 100, 0, 0, 1, 0),
			DimsMM: visuals.BoxDims{X: 150, Y: 150, Z: 150},
			Color:  &red,
		},
		visuals.Sphere{
			Label:    "demo_sphere",
			Pose:     visuals.PoseAt(0, 0, 100, 0, 0, 1, 0),
			RadiusMM: 90,
			Color:    &green,
		},
		visuals.Capsule{
			Label:    "demo_capsule",
			Pose:     visuals.PoseAt(300, 0, 100, 0, 0, 1, 0),
			RadiusMM: 50,
			LengthMM: 200,
			Color:    &blue,
		},
	)
	// Zero / empty arguments fall back to the library defaults
	// (30 Hz tick budget, stable UUIDs, "world" parent frame). The
	// library installs items, broadcasts ADDED to any subscribers,
	// and starts the tick task if any items animate (none here).
	return s.SceneServiceBase.ReconfigureWith(items, 0, "", "")
}

func (s *simpleScene) Close(ctx context.Context) error {
	return s.SceneServiceBase.Close(ctx)
}

// DoCommand disambiguates SceneServiceBase.DoCommand from
// resource.Named.DoCommand (both promoted via embedding) and
// forwards to the library implementation.
func (s *simpleScene) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	return s.SceneServiceBase.DoCommand(ctx, command)
}

// ---- SceneHooks (all 7 — written out so a reader sees the full
//      surface a service has to implement) ------------------------------

// BuildGeometry: dispatch via the library helper for the standard
// non-asset primitive types. A service using meshes or point clouds
// would dispatch on item.Type and call ReadAsset for those.
func (s *simpleScene) BuildGeometry(item visuals.Item, _ visuals.BaseGeom) (*commonpb.Geometry, error) {
	return visuals.BuildBasicGeometry(item)
}

// ReadAsset: this service uses no meshes or point clouds, so the
// library will never call this. Return a clear error if it does.
func (s *simpleScene) ReadAsset(path string) ([]byte, error) {
	return nil, fmt.Errorf("simple-scene-example doesn't load assets (path=%q)", path)
}

// ComputeTick: no animation. The runner pose is the base pose; no
// geometry overrides, no field-mask paths, no metadata overrides.
func (s *simpleScene) ComputeTick(_ visuals.Item, basePose visuals.Pose, _ visuals.BaseGeom, _ float64) visuals.TickResult {
	return visuals.TickResult{Pose: basePose}
}

// IsAnimated: no items animate.
func (s *simpleScene) IsAnimated(_ visuals.Item) bool { return false }

// LoadPreset: no presets.
func (s *simpleScene) LoadPreset(name string) ([]visuals.Item, error) {
	return nil, fmt.Errorf("simple-scene-example has no presets (got %q)", name)
}

// BaseGeomForItem: extract the shape-specific fields the library
// needs to apply geom overrides during animation. We don't animate,
// but the library still calls this on every item install.
func (s *simpleScene) BaseGeomForItem(item visuals.Item) visuals.BaseGeom {
	bg := visuals.BaseGeom{}
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

// HandleCustomCommand: no custom DoCommand verbs. Return
// (nil, false, nil) so the library falls through to its
// debug-snapshot default.
func (s *simpleScene) HandleCustomCommand(_ context.Context, _ map[string]any) (map[string]any, bool, error) {
	return nil, false, nil
}
