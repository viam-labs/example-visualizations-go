// simple-scene-example — the smallest possible world-state-store
// service. Publishes three static geometries to the Viam 3D scene
// viewer.
//
// READ THIS FIRST if you're learning the visuals library. This file
// is the canonical "I want to add geometries to the 3D scene viewer
// and that's it" example. Total length: ~100 lines including this
// comment block.
//
// What it demonstrates
// --------------------
//
//  1. Wiring a service that embeds visuals.SceneServiceBase to get
//     the world-state-store gRPC implementation, state map,
//     subscriber fan-out, animation tick loop, UUID strategy, and
//     the standard DoCommand verbs (list / clear / snapshot /
//     apply_events / etc.) for free.
//
//  2. Wiring visuals.BasicSceneHooks to get defaults for the
//     SceneHooks methods that only matter to services with
//     animations, presets, or custom DoCommand verbs. With it
//     embedded, this service only implements two hook methods.
//
//  3. Building a scene from typed visuals.Box / visuals.Sphere /
//     visuals.Capsule values. visuals.ToItems(...) is the bridge to
//     the wire-format Item list the service consumes.
//
// What it does NOT demonstrate
// ----------------------------
//
//  - Animations (see standalone-playground for the 11 modes)
//  - Configurable items / presets (see standalone-playground)
//  - Meshes and point clouds (see standalone-playground)
//  - Custom DoCommand verbs (see standalone-playground's
//    get_entity_chunk)
//  - The driver pattern (see playground-driver + playground-visualizer)
//
// Configure it as a `rdk:service:world_state_store` service with
// model `viam:example-visualizations-go:simple-scene-example`. No
// attributes are required — the scene is hardcoded.
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

// simpleScene embeds the library bases. SceneServiceBase carries
// the WSS plumbing; BasicSceneHooks carries the default
// implementations of the optional SceneHooks methods.
type simpleScene struct {
	resource.Named
	resource.TriviallyCloseable
	visuals.SceneServiceBase
	visuals.BasicSceneHooks
}

func newSimpleScene(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	s := &simpleScene{Named: conf.ResourceName().AsNamed()}

	// Tell the embedded SceneServiceBase to call back into us for
	// the hooks (BuildGeometry, ReadAsset, etc.). Method lookup
	// finds the overrides on simpleScene first, the BasicSceneHooks
	// defaults second.
	s.SceneServiceBase.Hooks = s
	s.SceneServiceBase.Logger = logger

	// The framework doesn't call Reconfigure automatically on initial
	// construction — do it explicitly.
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

// Reconfigure builds the (hardcoded) scene and hands it to
// SceneServiceBase. Real services would parse `conf.Attributes` here
// to pick up tick_hz / uuid_strategy / parent_frame; we just take
// the library defaults.
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
	// Zero / empty arguments → SceneServiceBase uses its defaults
	// (30 Hz, stable UUIDs, world parent frame).
	return s.SceneServiceBase.ReconfigureWith(items, 0, "", "")
}

func (s *simpleScene) Close(ctx context.Context) error {
	return s.SceneServiceBase.Close(ctx)
}

// DoCommand disambiguates SceneServiceBase.DoCommand from
// resource.Named.DoCommand — both promoted via embedding — and
// forwards to the library implementation.
func (s *simpleScene) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	return s.SceneServiceBase.DoCommand(ctx, command)
}

// ---- SceneHooks (only the two that this service needs) ----------------

// BuildGeometry: dispatch to the library helper that handles the
// standard non-asset primitive types.
func (s *simpleScene) BuildGeometry(item visuals.Item, _ visuals.BaseGeom) (*commonpb.Geometry, error) {
	return visuals.BuildBasicGeometry(item)
}

// ReadAsset: this service uses no meshes or point clouds. If the
// library ever asks for an asset (it won't, since we don't ship any
// mesh/pointcloud items), return a clear error so the bug is loud.
func (s *simpleScene) ReadAsset(path string) ([]byte, error) {
	return nil, fmt.Errorf("simple-scene-example doesn't load assets (path=%q)", path)
}
