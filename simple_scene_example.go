// simple-scene-example — the smallest possible world-state-store
// service. Publishes three static geometries plus one animated box
// to the Viam 3D scene viewer.
//
// READ THIS FIRST if you're learning the visuals library. This is
// the canonical "I want to add geometries to the Viam 3D scene
// viewer" reference. Every method that a Viam Go module author has
// to write to ship a working WSS service is in this file — no
// helper-method embeds from elsewhere in the library, no hidden
// shortcuts.
//
// What the library gives you for free
// -----------------------------------
//
// By embedding visuals.SceneServiceBase, the gRPC WorldStateStore
// implementation (ListUUIDs / GetTransform / StreamTransformChanges),
// the state map, the subscriber broadcast, the animation tick loop,
// the UUID strategy, and the standard DoCommand verbs (list / clear
// / snapshot / apply_events / etc.) all just work.
//
// The library also hides the renderer's quirks. A subscriber sees a
// clean stream of ADDED / UPDATED / REMOVED events that the viewer
// honors — including for color and opacity changes, which the
// library translates into a transparent REMOVE + re-ADD with a fresh
// UUID under the hood (the viewer's UPDATED handler drops
// metadata.* paths; see LESSONS.md for the full story).
//
// What this file shows
// --------------------
//
//  1. Registering the model in init().
//  2. The constructor (newSimpleScene), Reconfigure, Close, and
//     DoCommand entry points the framework calls.
//  3. Building a scene from typed visuals.Box / .Sphere / .Capsule
//     and handing it to the library via SetScene.
//  4. The SceneTick hook (optional visuals.SceneTicker interface):
//     mutate typed Visual objects, return scene.Update(...) events.
//     The library diffs against the committed snapshot, emits the
//     right field-mask paths, and broadcasts to subscribers.
//  5. The minimum SceneHooks surface — BuildGeometry +
//     BaseGeomForItem. Everything else (asset loading, presets,
//     custom DoCommand verbs, the legacy per-item animation path)
//     is an opt-in interface the module can omit.
//
// What it does NOT show
// ---------------------
//
//   - Configurable items / presets, mesh / pointcloud assets, custom
//     DoCommand verbs — see the standalone-playground model in
//     service.go.
//   - The driver-visualizer split — see playground-driver and
//     playground-visualizer.
//
// Configure as a `rdk:service:world_state_store` service with model
// viam:example-visualizations-go:simple-scene-example. No attributes
// are required — the scene is hardcoded.
package exampleviz

import (
	"context"
	"math"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"

	"github.com/viam-labs/viam-viz-helpers-go"
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

// simpleScene embeds visuals.SceneServiceBase — the library does the
// WSS plumbing — and implements the SceneHooks methods plus the
// optional SceneTicker interface for the new per-frame animation
// API.
type simpleScene struct {
	resource.Named
	visuals.SceneServiceBase

	// The moving box: a typed Box object whose fields we mutate on
	// every tick. SetScene installs it in s.Scene; SceneTick mutates
	// it; s.Scene.Update(s.movingBox) emits the diff.
	movingBox *visuals.Box

	// Hierarchical layer: a pivot Frame with two children parented
	// to it. Rotating the pivot transports the children — they
	// don't need their own updates.
	pivot       *visuals.Frame
	childSphere *visuals.Sphere
	childBox    *visuals.Box
}

func newSimpleScene(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	s := &simpleScene{Named: conf.ResourceName().AsNamed()}

	// Tell the embedded SceneServiceBase which instance to call back
	// into for the hooks (BuildGeometry, SceneTick, etc.).
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

// Reconfigure builds the scene from typed shape values and hands it
// to the library via SetScene.
func (s *simpleScene) Reconfigure(
	_ context.Context, _ resource.Dependencies, _ resource.Config,
) error {
	red := visuals.Color{R: 230, G: 25, B: 75}
	green := visuals.Color{R: 60, G: 180, B: 75}
	blue := visuals.Color{R: 0, G: 130, B: 200}
	orange := visuals.Color{R: 255, G: 100, B: 0}
	opacity := 1.0

	// The moving box — kept as a field so SceneTick can mutate it.
	s.movingBox = &visuals.Box{
		Label:   "moving_box",
		Pose:    visuals.PoseAt(400, 0, 200, 0, 0, 1, 0),
		DimsMM:  visuals.BoxDims{X: 120, Y: 120, Z: 120},
		Color:   &orange,
		Opacity: &opacity,
	}

	// Hierarchical layer: pivot Frame + two children. Only the
	// pivot's pose updates each tick; the children are parented to
	// it and inherit the transform via the renderer's frame chain.
	yellow := visuals.Color{R: 255, G: 255, B: 0}
	magenta := visuals.Color{R: 255, G: 0, B: 255}
	s.pivot = &visuals.Frame{
		Label: "pivot",
		Pose:  visuals.PoseAt(-700, 0, 300, 0, 0, 1, 0),
	}
	s.childSphere = &visuals.Sphere{
		Label:       "pivot_child_sphere",
		Pose:        visuals.PoseAt(80, 0, 0, 0, 0, 1, 0), // 80mm along pivot's +X
		ParentFrame: "pivot",
		RadiusMM:    30,
		Color:       &yellow,
	}
	s.childBox = &visuals.Box{
		Label:       "pivot_child_box",
		Pose:        visuals.PoseAt(-80, 0, 0, 0, 0, 1, 0), // 80mm along pivot's -X
		ParentFrame: "pivot",
		DimsMM:      visuals.BoxDims{X: 40, Y: 40, Z: 40},
		Color:       &magenta,
	}

	return s.SetScene(visuals.SetSceneOpts{},
		// Three static primitives in a row.
		&visuals.Box{
			Label:  "demo_box",
			Pose:   visuals.PoseAt(-400, 0, 100, 0, 0, 1, 0),
			DimsMM: visuals.BoxDims{X: 150, Y: 150, Z: 150},
			Color:  &red,
		},
		&visuals.Sphere{
			Label:    "demo_sphere",
			Pose:     visuals.PoseAt(-150, 0, 100, 0, 0, 1, 0),
			RadiusMM: 90,
			Color:    &green,
		},
		&visuals.Capsule{
			Label:    "demo_capsule",
			Pose:     visuals.PoseAt(100, 0, 100, 0, 0, 1, 0),
			RadiusMM: 50,
			LengthMM: 200,
			Color:    &blue,
		},
		// The animated box (mutated in SceneTick).
		s.movingBox,
		// Hierarchical group: pivot + two children. Only the pivot's
		// pose updates each tick; the children follow.
		s.pivot,
		s.childSphere,
		s.childBox,
	)
}

// Close is provided by the embedded visuals.SceneServiceBase (it
// cancels the tick goroutine and closes subscriber channels). No
// disambiguation needed since we're not embedding a competing
// TriviallyCloseable.

// DoCommand disambiguates SceneServiceBase.DoCommand from
// resource.Named.DoCommand (both promoted via embedding).
func (s *simpleScene) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	return s.SceneServiceBase.DoCommand(ctx, command)
}

// ---- SceneTicker (the new per-frame animation API) -------------------

// SceneTick is called by the library's tick loop at tick_hz (default
// 30 Hz). Mutate typed Visual objects in place; return the diff
// events from scene.Update(...). The library broadcasts the events
// to subscribers and translates metadata-only updates (color,
// opacity) into renderer-visible REMOVE + re-ADD with a fresh UUID.
//
// The moving box does four animations simultaneously:
//
//   - Position: orbit around its anchor at radius 150 mm, period 4 s.
//   - Scale: pulse symmetrically between 80 mm and 160 mm, period 2 s.
//   - Color: hue cycles through the rainbow at period 6 s. Library
//     respawns the entity on each step so the viewer actually paints
//     the change.
//   - Opacity: sinusoidal between 0.3 and 1.0, period 3 s. Same
//     library-side translation as color.
func (s *simpleScene) SceneTick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
	// --- Moving box: four animations on one Visual ---------------
	// Position: orbit around (400, 0, 200) at radius 150 mm,
	// period 4 s — using the visuals.OrbitPose helper.
	s.movingBox.Pose = visuals.OrbitPose(
		visuals.PoseAt(400, 0, 200, 0, 0, 1, 0),
		4.0, 150.0, t, "z",
	)
	// Scale: pulse all three dimensions symmetrically between
	// 80 and 160 mm — using the visuals.PulseRange helper.
	scale := visuals.PulseRange(80, 160, 2.0, t)
	s.movingBox.DimsMM = visuals.BoxDims{X: scale, Y: scale, Z: scale}
	// Color / opacity trigger renderer respawns (the viewer drops
	// metadata.* paths on UPDATED, so the library emits REMOVE +
	// re-ADD with a fresh UUID). Snap to bounded step counts so the
	// respawn rate doesn't pin to tick_hz; see visuals.SnapStep.
	hue := visuals.SnapStep(math.Mod(t/6.0, 1.0), 24, 0, 1) // 24 hues / 6 s
	c := visuals.HSVToRGB(hue, 1, 1)
	s.movingBox.Color = &c
	opRaw := 0.3 + 0.7*(1+math.Sin(2*math.Pi*t/3.0))/2.0
	op := visuals.SnapStep(opRaw, 12, 0.3, 1.0)
	s.movingBox.Opacity = &op

	// --- Hierarchical group: only the pivot updates --------------
	// Rotate the pivot around its own +Z. The two children are
	// parented to "pivot" via ParentFrame; the renderer composes
	// the parent transform automatically.
	s.pivot.Pose = visuals.PoseAt(
		-700, 0, 300,
		0, 0, 1, math.Mod(t*60, 360), // 60° per second
	)

	var events []visuals.SceneEvent
	if e, _ := scene.Update(s.movingBox); len(e) > 0 {
		events = append(events, e...)
	}
	if e, _ := scene.Update(s.pivot); len(e) > 0 {
		events = append(events, e...)
	}
	return events
}

// No SceneHooks methods are needed: this service publishes only
// standard primitives, so the library's built-in defaults
// (visuals.BuildBasicGeometry + visuals.DefaultBaseGeomForItem)
// handle every item. The only hook this service implements is
// SceneTick above, via the optional SceneTicker interface.
//
// Optional interfaces this service deliberately omits:
//   - GeometryBuilder.BuildGeometry  (only needed for custom primitive types)
//   - BaseGeomProvider.BaseGeomForItem (same)
//   - AssetReader.ReadAsset          (no mesh / pointcloud assets)
//   - PresetLoader.LoadPreset        (no presets)
//   - CustomCommandHandler.HandleCustomCommand (no custom DoCommand verbs)
//   - LegacyAnimator.ComputeTick / IsAnimated (using SceneTick instead)
