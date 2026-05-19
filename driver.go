// playground-driver — Generic component that mutates a Scene and
// pushes the resulting events to a visualizer.
//
// Go mirror of src/driver.py. Owns a visuals.Scene, ticks at
// config'd Hz via a goroutine, pushes events to its visualizer
// dependency via apply_events. Looks up the visualizer through the
// in-process registry; fails fast if not found.
package exampleviz

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/viam-labs/viam-viz-helpers-go"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// DriverModel is the registered model identifier for the driver.
var DriverModel = resource.NewModel(
	"viam", "example-visualizations-go", "playground-driver",
)

const (
	DefaultDriverTickHz = 30.0
	MaxDriverTickHz     = 30.0
	DefaultRecipe       = "marching_boxes"
)

func init() {
	resource.RegisterComponent(generic.API, DriverModel,
		resource.Registration[resource.Resource, *DriverConfig]{
			Constructor: newPlaygroundDriver,
		},
	)
}

// DriverConfig is the driver's attribute schema.
type DriverConfig struct {
	Visualizer string   `json:"visualizer"`
	Recipe     string   `json:"recipe,omitempty"`
	TickHz     *float64 `json:"tick_hz,omitempty"`
	Namespace  string   `json:"namespace,omitempty"`
}

func (c *DriverConfig) Validate(path string) ([]string, []string, error) {
	if c.Visualizer == "" {
		return nil, nil, fmt.Errorf("%s.visualizer is required", path)
	}
	recipe := c.Recipe
	if recipe == "" {
		recipe = DefaultRecipe
	}
	if _, ok := Recipes[recipe]; !ok {
		names := make([]string, 0, len(Recipes))
		for k := range Recipes {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, nil, fmt.Errorf(
			"%s.recipe %q is unknown; valid: %v", path, recipe, names,
		)
	}
	if c.TickHz != nil {
		if *c.TickHz <= 0 || *c.TickHz > MaxDriverTickHz {
			return nil, nil, fmt.Errorf(
				"%s.tick_hz must be in (0, %v]", path, MaxDriverTickHz,
			)
		}
	}
	return nil, nil, nil
}

// playgroundDriver — Generic component driving a visualizer.
//
// Holds a direct reference to its visualizer (typed as
// worldstatestore.Service for portability — when the in-process
// registry returns a *playgroundVisualizer, we just call its
// DoCommand method through the interface).
type playgroundDriver struct {
	resource.Named
	resource.TriviallyCloseable
	logger logging.Logger

	mu         sync.Mutex
	visName    string
	visualizer worldstatestore.Service
	recipeName string
	tickHz     float64
	namespace  string
	scene      *visuals.Scene
	recipe     Recipe
	tickCancel context.CancelFunc
	tickDone   chan struct{}
	t0         time.Time
}

func newPlaygroundDriver(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (resource.Resource, error) {
	d := &playgroundDriver{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	if err := d.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *playgroundDriver) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*DriverConfig](conf)
	if err != nil {
		return err
	}

	d.mu.Lock()
	// Cancel previous tick task if any.
	if d.tickCancel != nil {
		d.tickCancel()
		d.mu.Unlock()
		// Wait for prior loop to exit (releases mu inside the loop).
		if d.tickDone != nil {
			<-d.tickDone
		}
		d.mu.Lock()
	}

	d.visName = cfg.Visualizer
	d.recipeName = cfg.Recipe
	if d.recipeName == "" {
		d.recipeName = DefaultRecipe
	}
	d.tickHz = DefaultDriverTickHz
	if cfg.TickHz != nil {
		d.tickHz = *cfg.TickHz
	}
	d.namespace = cfg.Namespace
	d.recipe = Recipes[d.recipeName]

	// Look up the visualizer in the in-process registry. No gRPC
	// fallback today — both models ship from the same binary so the
	// visualizer must be in this process.
	raw := visuals.Lookup(d.visName)
	if raw == nil {
		d.mu.Unlock()
		registered := visuals.RegisteredNames()
		return fmt.Errorf(
			"visualizer %q not found in the in-process registry; "+
				"ensure it's configured and listed before this driver in "+
				"'depends_on'. Registered: %v. "+
				"(Cross-process driver→visualizer via gRPC isn't supported yet.)",
			d.visName, registered,
		)
	}
	wss, ok := raw.(worldstatestore.Service)
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf(
			"resource registered as %q is not a worldstatestore.Service (got %T)",
			d.visName, raw,
		)
	}
	d.visualizer = wss

	// Capture the prior scene before we overwrite d.scene. We'll
	// push REMOVED events for its labels so the prior recipe's
	// visuals disappear from the renderer before the new recipe's
	// visuals appear. Without this, switching recipes (or even
	// re-running the same recipe with different parameters) leaves
	// the prior recipe's labels visible in the renderer alongside
	// the new ones.
	prevScene := d.scene

	// Build fresh Scene from the recipe.
	d.scene = visuals.NewScene("world")
	initialEvents := d.recipe.Initial(d.scene)
	d.t0 = time.Now()

	// Start tick goroutine.
	tickCtx, cancel := context.WithCancel(context.Background())
	d.tickCancel = cancel
	d.tickDone = make(chan struct{})
	d.mu.Unlock()

	// Push REMOVED events for the prior scene's labels (if any), so
	// switching recipes doesn't leave stale visuals in the renderer.
	if prevScene != nil && prevScene.Len() > 0 {
		if err := d.sendEvents(ctx, prevScene.Clear()); err != nil {
			d.logger.Warnw("failed to clear prior scene", "err", err)
		}
	}

	// Push initial scene synchronously so callers see state right away.
	if err := d.sendEvents(ctx, initialEvents); err != nil {
		d.logger.Warnw("initial scene push failed", "err", err)
	}

	go d.tickLoop(tickCtx)

	d.logger.Infow("playground-driver reconfigure",
		"visualizer", d.visName,
		"recipe", d.recipeName,
		"tick_hz", d.tickHz,
		"namespace", d.namespace,
		"visualizer_type", fmt.Sprintf("%T", d.visualizer),
	)
	return nil
}

func (d *playgroundDriver) Close(ctx context.Context) error {
	d.mu.Lock()
	cancel := d.tickCancel
	done := d.tickDone
	scene := d.scene
	vis := d.visualizer
	d.tickCancel = nil
	d.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}

	// Best-effort: clear our entities from the visualizer.
	if scene != nil && vis != nil {
		events := scene.Clear()
		_ = d.sendEvents(ctx, events)
	}
	return nil
}

func (d *playgroundDriver) tickLoop(ctx context.Context) {
	defer close(d.tickDone)
	period := time.Duration(float64(time.Second) / d.tickHz)
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t := time.Since(d.t0).Seconds()
			events := d.recipe.Tick(d.scene, t)
			if err := d.sendEvents(ctx, events); err != nil {
				d.logger.Warnw("driver tick send failed", "err", err)
			}
		}
	}
}

func (d *playgroundDriver) sendEvents(ctx context.Context, events []visuals.SceneEvent) error {
	if len(events) == 0 {
		return nil
	}
	wire := visuals.EventsToWire(events)
	// Convert []map[string]any to []any for DoCommand's map[string]any
	// payload field.
	wireAny := make([]any, len(wire))
	for i, w := range wire {
		wireAny[i] = w
	}
	cmd := map[string]any{
		"command": "apply_events",
		"events":  wireAny,
	}
	if d.namespace != "" {
		cmd["namespace"] = d.namespace
	}
	_, err := d.visualizer.DoCommand(ctx, cmd)
	return err
}

// DoCommand exposes info / recipes for debugging.
func (d *playgroundDriver) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	cmd, _ := command["command"].(string)
	switch cmd {
	case "info":
		d.mu.Lock()
		defer d.mu.Unlock()
		return map[string]any{
			"visualizer":      d.visName,
			"recipe":          d.recipeName,
			"tick_hz":         d.tickHz,
			"namespace":       d.namespace,
			"scene_size":      d.scene.Len(),
			"visualizer_type": fmt.Sprintf("%T", d.visualizer),
			"tick_running":    d.tickCancel != nil,
		}, nil
	case "recipes":
		names := make([]string, 0, len(Recipes))
		for k := range Recipes {
			names = append(names, k)
		}
		sort.Strings(names)
		return map[string]any{"recipes": names}, nil
	}
	return nil, errors.New("unknown command")
}
