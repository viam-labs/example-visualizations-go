package exampleviz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/viam-labs/viam-viz-helpers-go"
	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

var (
	worldstatestoreAPI = worldstatestore.API
	genericAPI         = generic.API
)

func clearRegistry() {
	for _, n := range visuals.RegisteredNames() {
		visuals.Unregister(n)
	}
}

func newVisualizer(t *testing.T, name string) *playgroundVisualizer {
	t.Helper()
	pv := &playgroundVisualizer{}
	pv.Named = resource.NewName(worldstatestoreAPI, name).AsNamed()
	pv.logger = logging.NewTestLogger(t)
	pv.SceneServiceBase.Hooks = pv
	pv.SceneServiceBase.Logger = pv.logger
	pv.SceneServiceBase.DefaultTickHz = DefaultTickHz
	pv.SceneServiceBase.DefaultUUIDStrategy = DefaultUUIDStrategy
	pv.SceneServiceBase.DefaultParentFrame = DefaultParentFrame
	pv.SceneServiceBase.MaxTickHz = 30.0
	if err := pv.SceneServiceBase.ReconfigureWith(nil, 30, "stable", "world"); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}
	visuals.Register(name, pv)
	return pv
}

func newDriver(t *testing.T, visName, recipe string, tickHz float64, namespace string) *playgroundDriver {
	t.Helper()
	d := &playgroundDriver{
		Named:  resource.NewName(genericAPI, "drv").AsNamed(),
		logger: logging.NewTestLogger(t),
	}
	cfg := &DriverConfig{
		Visualizer: visName,
		Recipe:     recipe,
		Namespace:  namespace,
	}
	cfg.TickHz = &tickHz
	conf := resource.Config{
		Name:                "drv",
		API:                 genericAPI,
		Model:               DriverModel,
		ConvertedAttributes: cfg,
	}
	if err := d.Reconfigure(context.Background(), nil, conf); err != nil {
		t.Fatalf("driver reconfigure: %v", err)
	}
	return d
}

// ---- happy path -------------------------------------------------------

func TestDriverVisualizerPipeline_InitialScene(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 20, "")
	defer d.Close(context.Background())

	// MarchingBoxes installs 5 boxes.
	vis.Mu().Lock()
	count := len(vis.State())
	vis.Mu().Unlock()
	if count != 5 {
		t.Errorf("expected 5 items in visualizer, got %d", count)
	}
}

func TestDriverVisualizerPipeline_TickUpdatesPose(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 30, "")
	defer d.Close(context.Background())

	vis.Mu().Lock()
	initialY := vis.State()["march_0"].BasePose.Y
	vis.Mu().Unlock()

	// Wait a few tick periods.
	time.Sleep(200 * time.Millisecond)

	vis.Mu().Lock()
	afterY := vis.State()["march_0"].BasePose.Y
	vis.Mu().Unlock()

	if afterY == initialY {
		// Allow one more cycle in case we sampled at sin=0 boundary.
		time.Sleep(100 * time.Millisecond)
		vis.Mu().Lock()
		afterY = vis.State()["march_0"].BasePose.Y
		vis.Mu().Unlock()
	}
	if afterY == initialY {
		t.Errorf("expected y to change after ticks (initial=%v, after=%v)", initialY, afterY)
	}
}

func TestDriverVisualizerPipeline_UsesInProcessRef(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 10, "")
	defer d.Close(context.Background())

	// Driver's visualizer field is the concrete *playgroundVisualizer,
	// proving the registry path short-circuited what would have been
	// a gRPC client stub.
	if pv, ok := d.visualizer.(*playgroundVisualizer); !ok || pv != vis {
		t.Errorf("driver.visualizer is not the in-process visualizer (got %T)", d.visualizer)
	}
}

// ---- failure modes ---------------------------------------------------

func TestDriver_FailsWhenVisualizerNotRegistered(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	d := &playgroundDriver{
		Named:  resource.NewName(genericAPI, "drv").AsNamed(),
		logger: logging.NewTestLogger(t),
	}
	cfg := &DriverConfig{Visualizer: "ghost", Recipe: "marching_boxes"}
	conf := resource.Config{
		Name:                "drv",
		API:                 genericAPI,
		Model:               DriverModel,
		ConvertedAttributes: cfg,
	}
	err := d.Reconfigure(context.Background(), nil, conf)
	if err == nil || !strings.Contains(err.Error(), "not found in the in-process registry") {
		t.Errorf("expected registry-lookup error, got %v", err)
	}
}

// ---- namespace -------------------------------------------------------

func TestDriverVisualizerPipeline_NamespacePrefixesLabels(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 20, "ns1")
	defer d.Close(context.Background())

	vis.Mu().Lock()
	defer vis.Mu().Unlock()
	state := vis.State()
	if _, ok := state["ns1/march_0"]; !ok {
		t.Errorf("expected ns1/march_0 in state, got labels: %v", keys(state))
	}
	if _, ok := state["march_0"]; ok {
		t.Errorf("unprefixed label should not exist")
	}
}

func TestDriverVisualizerPipeline_TwoDriversNamespaced(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d1 := newDriver(t, "vis", "marching_boxes", 20, "a")
	defer d1.Close(context.Background())
	d2 := newDriver(t, "vis", "pulsing_spheres", 20, "b")
	defer d2.Close(context.Background())

	vis.Mu().Lock()
	defer vis.Mu().Unlock()
	state := vis.State()
	count := len(state)
	if count != 5+3 {
		t.Errorf("expected 8 items, got %d (labels=%v)", count, keys(state))
	}
}

// ---- reconfigure clears prior scene -----------------------------------

func TestDriverVisualizerPipeline_ReconfigureClearsPriorRecipeLabels(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	vis := newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 20, "")
	defer d.Close(context.Background())

	vis.Mu().Lock()
	hasMarch := false
	for k := range vis.State() {
		if strings.HasPrefix(k, "march_") {
			hasMarch = true
			break
		}
	}
	vis.Mu().Unlock()
	if !hasMarch {
		t.Fatal("expected march_* labels after initial reconfigure")
	}

	// Reconfigure to a different recipe.
	tickHz := 20.0
	cfg := &DriverConfig{
		Visualizer: "vis",
		Recipe:     "pulsing_spheres",
		TickHz:     &tickHz,
	}
	conf := resource.Config{
		Name:                "drv",
		API:                 genericAPI,
		Model:               DriverModel,
		ConvertedAttributes: cfg,
	}
	if err := d.Reconfigure(context.Background(), nil, conf); err != nil {
		t.Fatalf("reconfigure: %v", err)
	}

	vis.Mu().Lock()
	defer vis.Mu().Unlock()
	for k := range vis.State() {
		if strings.HasPrefix(k, "march_") {
			t.Errorf("march_* label still present after recipe switch: %v", k)
		}
	}
	pulseCount := 0
	for k := range vis.State() {
		if strings.HasPrefix(k, "pulse_") {
			pulseCount++
		}
	}
	if pulseCount == 0 {
		t.Errorf("expected pulse_* labels after recipe switch, got labels: %v",
			keys(vis.State()))
	}
}

// ---- DoCommand surface ----------------------------------------------

func TestDriver_InfoCommand(t *testing.T) {
	clearRegistry()
	defer clearRegistry()

	_ = newVisualizer(t, "vis")
	d := newDriver(t, "vis", "marching_boxes", 10, "")
	defer d.Close(context.Background())

	info, err := d.DoCommand(context.Background(), map[string]any{"command": "info"})
	if err != nil {
		t.Fatal(err)
	}
	if info["visualizer"] != "vis" || info["recipe"] != "marching_boxes" {
		t.Errorf("unexpected info: %v", info)
	}
	vt := info["visualizer_type"].(string)
	if !strings.Contains(vt, "playgroundVisualizer") {
		t.Errorf("visualizer_type should be playgroundVisualizer, got %q", vt)
	}
}

// ---- helpers ---------------------------------------------------------

func keys(m map[string]*visuals.ItemState) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
