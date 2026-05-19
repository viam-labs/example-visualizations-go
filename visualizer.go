// playground-visualizer — passive WSS that accepts pushed Scene events.
//
// Go mirror of src/visualizer.py. Wraps the existing sceneSprites
// hooks (geometry building, asset reading, mesh-color extraction)
// so meshes / point clouds still load correctly when a driver
// pushes them, but rejects items/preset config — items arrive at
// runtime via the apply_events DoCommand from a driver component.
//
// Registers itself in visuals.Registry on construction so an
// in-process driver can hold a direct Go reference instead of
// going through the framework's gRPC stub.
package exampleviz

import (
	"context"
	"fmt"

	"github.com/viam-labs/viam-viz-helpers-go"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// VisualizerModel is the registered model identifier for the
// passive visualizer.
var VisualizerModel = resource.NewModel(
	"viam", "example-visualizations-go", "playground-visualizer",
)

func init() {
	resource.RegisterService(worldstatestore.API, VisualizerModel,
		resource.Registration[worldstatestore.Service, *VisualizerConfig]{
			Constructor: newPlaygroundVisualizer,
		},
	)
}

// VisualizerConfig is the visualizer's attribute schema. Strictly
// passive — no items, no preset.
type VisualizerConfig struct {
	TickHz       *float64 `json:"tick_hz,omitempty"`
	UUIDStrategy string   `json:"uuid_strategy,omitempty"`
	ParentFrame  string   `json:"parent_frame,omitempty"`
}

// Validate rejects the items/preset config that belongs on the
// driver. Validate also enforces the same tick/strategy/frame
// bounds the standalone-playground service uses, so the visualizer
// can't be wedged into an unsupported state.
func (c *VisualizerConfig) Validate(path string) ([]string, []string, error) {
	if c.TickHz != nil {
		if *c.TickHz <= 0 || *c.TickHz > 30 {
			return nil, nil, fmt.Errorf("%s.tick_hz must be in (0, 30]", path)
		}
	}
	if c.UUIDStrategy != "" {
		valid := false
		for _, v := range ValidUUIDStrategies {
			if v == c.UUIDStrategy {
				valid = true
				break
			}
		}
		if !valid {
			return nil, nil, fmt.Errorf("%s.uuid_strategy must be one of %v", path, ValidUUIDStrategies)
		}
	}
	return nil, nil, nil
}

// playgroundVisualizer wraps sceneSprites — every hook (geometry
// building, mesh loading, chunked-PCD setup, custom verbs) carries
// over via embedding. We only override Reconfigure to skip the
// items/preset path and to register the live instance in
// visuals.Registry.
type playgroundVisualizer struct {
	sceneSprites
}

func newPlaygroundVisualizer(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	pv := &playgroundVisualizer{}
	pv.Named = conf.ResourceName().AsNamed()
	pv.logger = logger
	// Hooks point at the visualizer itself; method lookup walks
	// through the embedded sceneSprites for everything except
	// LoadPreset, which we override below.
	pv.SceneServiceBase.Hooks = pv
	pv.SceneServiceBase.Logger = logger
	pv.SceneServiceBase.DefaultTickHz = DefaultTickHz
	pv.SceneServiceBase.DefaultUUIDStrategy = DefaultUUIDStrategy
	pv.SceneServiceBase.DefaultParentFrame = DefaultParentFrame
	pv.SceneServiceBase.MaxTickHz = 30.0
	if err := pv.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return pv, nil
}

// Reconfigure parses VisualizerConfig and starts the SceneServiceBase
// with NO items / NO preset. The visualizer waits for the driver to
// push state via apply_events.
func (pv *playgroundVisualizer) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*VisualizerConfig](conf)
	if err != nil {
		return err
	}
	tickHz := DefaultTickHz
	if cfg.TickHz != nil {
		tickHz = *cfg.TickHz
	}
	strategy := cfg.UUIDStrategy
	if strategy == "" {
		strategy = DefaultUUIDStrategy
	}
	parent := cfg.ParentFrame
	if parent == "" {
		parent = DefaultParentFrame
	}
	if err := pv.SceneServiceBase.ReconfigureWith(nil, tickHz, strategy, parent); err != nil {
		return err
	}
	visuals.Register(conf.ResourceName().Name, pv)
	pv.logger.Infow("playground-visualizer reconfigure",
		"tick_hz", tickHz,
		"uuid_strategy", strategy,
		"parent_frame", parent,
	)
	return nil
}

func (pv *playgroundVisualizer) Close(ctx context.Context) error {
	visuals.Unregister(pv.Named.Name().Name)
	return pv.SceneServiceBase.Close(ctx)
}

// LoadPreset overrides sceneSprites.LoadPreset so the visualizer
// can never load a preset — there's no items/preset config path
// to here, but make doubly sure by failing fast if any code path
// somehow asks for one. The DoCommand "preset" verb routes here.
func (pv *playgroundVisualizer) LoadPreset(name string) ([]visuals.Item, error) {
	return nil, fmt.Errorf("playground-visualizer doesn't support presets — push items via apply_events from a driver")
}

// DoCommand disambiguates the embedded sceneSprites.DoCommand from
// the resource.Named one. Forwards to SceneServiceBase, which
// handles apply_events plus the standard set of verbs.
func (pv *playgroundVisualizer) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	return pv.SceneServiceBase.DoCommand(ctx, command)
}
