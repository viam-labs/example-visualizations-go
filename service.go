// standalone-playground — thin sceneSprites over visuals.SceneServiceBase.
//
// The library owns the WSS plumbing (state map, subscriber fanout,
// animation tick goroutine, UUID strategy, standard DoCommand
// verbs including apply_events). This file plugs in the standalone
// model's MODEL identifier and the module-specific hooks (geometry
// building, asset reading, animation tick math, preset lookup, the
// get_entity_chunk custom verb).
//
// The companion files visualizer.go and driver.go register the
// other two models the module ships (playground-visualizer +
// playground-driver). See CLAUDE.md for the three-model architecture.
package exampleviz

import (
	"context"
	"errors"
	"fmt"
	"os"

	"exampleviz/visuals"
	commonpb "go.viam.com/api/common/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// Model is the registered model identifier.
var Model = resource.NewModel("viam", "example-visualizations-go", "standalone-playground")

func init() {
	resource.RegisterService(worldstatestore.API, Model,
		resource.Registration[worldstatestore.Service, *Config]{
			Constructor: newSceneSprites,
		},
	)
}

type sceneSprites struct {
	resource.Named
	resource.TriviallyCloseable
	visuals.SceneServiceBase

	logger logging.Logger
}

func newSceneSprites(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	s := &sceneSprites{
		Named:  conf.ResourceName().AsNamed(),
		logger: logger,
	}
	s.SceneServiceBase.Hooks = s
	s.SceneServiceBase.Logger = logger
	s.SceneServiceBase.DefaultTickHz = DefaultTickHz
	s.SceneServiceBase.DefaultUUIDStrategy = DefaultUUIDStrategy
	s.SceneServiceBase.DefaultParentFrame = DefaultParentFrame
	s.SceneServiceBase.DefaultPreset = DefaultPreset
	s.SceneServiceBase.MaxTickHz = 30.0
	if err := s.Reconfigure(ctx, deps, conf); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *sceneSprites) Reconfigure(
	ctx context.Context,
	_ resource.Dependencies,
	conf resource.Config,
) error {
	cfg, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return err
	}

	tickHz := DefaultTickHz
	if cfg.TickHz != nil {
		tickHz = *cfg.TickHz
	}
	uuidStrategy := cfg.UUIDStrategy
	if uuidStrategy == "" {
		uuidStrategy = DefaultUUIDStrategy
	}
	parentFrame := cfg.ParentFrame
	if parentFrame == "" {
		parentFrame = DefaultParentFrame
	}

	var items []Item
	if len(cfg.Items) > 0 {
		for _, ic := range cfg.Items {
			items = append(items, ic.toItem())
		}
	} else {
		name := cfg.Preset
		if name == "" {
			name = DefaultPreset
		}
		preset, ok := Presets[name]
		if !ok {
			return fmt.Errorf("unknown preset %q", name)
		}
		items = preset()
	}

	if err := s.SceneServiceBase.ReconfigureWith(items, tickHz, uuidStrategy, parentFrame); err != nil {
		return err
	}
	s.logger.Infow("reconfigure",
		"tick_hz", tickHz,
		"uuid_strategy", uuidStrategy,
		"parent_frame", parentFrame,
		"items", len(items),
	)
	return nil
}

func (s *sceneSprites) Close(ctx context.Context) error {
	return s.SceneServiceBase.Close(ctx)
}

// DoCommand disambiguates between SceneServiceBase.DoCommand and
// resource.Named.DoCommand (both promoted via embedding). Forwards
// to the library implementation, which dispatches the standard
// verbs and falls through to HandleCustomCommand for everything
// else.
func (s *sceneSprites) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	return s.SceneServiceBase.DoCommand(ctx, command)
}

// ---- SceneHooks implementation ---------------------------------------

func (s *sceneSprites) BuildGeometry(item Item, override visuals.BaseGeom) (*commonpb.Geometry, error) {
	switch item.Type {
	case "box":
		dims := item.DimsMM
		if override.HasDims {
			dims = override.Dims
		}
		return buildBox([3]float64{dims.X, dims.Y, dims.Z}, item.Label), nil
	case "sphere":
		r := item.RadiusMM
		if override.RadiusMM > 0 {
			r = override.RadiusMM
		}
		return buildSphere(r, item.Label), nil
	case "capsule":
		r, l := item.RadiusMM, item.LengthMM
		if override.RadiusMM > 0 {
			r = override.RadiusMM
		}
		if override.LengthMM > 0 {
			l = override.LengthMM
		}
		return buildCapsule(r, l, item.Label), nil
	case "point":
		return buildPoint(item.Label), nil
	case "arrow":
		l, r := item.LengthMM, item.RadiusMM
		if override.LengthMM > 0 {
			l = override.LengthMM
		}
		if override.RadiusMM > 0 {
			r = override.RadiusMM
		}
		return buildArrow(l, r, item.Label), nil
	case "mesh":
		raw, err := s.ReadAsset(item.MeshPath)
		if err != nil {
			return nil, err
		}
		// raw_stl: bug-demo for the viz team — content_type="stl"
		// is silently dropped by the viewer despite proto/RDK claims
		// to support it. See LESSONS.md::mesh-formats.
		if item.RawSTL {
			return buildMesh(raw, "stl", item.Label, true)
		}
		ply, err := visuals.LoadMeshBytesAsPLY(raw, item.MeshPath)
		if err != nil {
			return nil, err
		}
		return buildMesh(ply, "ply", item.Label, false)
	case "pointcloud":
		if override.PCDBytesOverride != nil {
			return buildPointcloud(override.PCDBytesOverride, item.Label), nil
		}
		raw, err := s.ReadAsset(item.PointcloudPath)
		if err != nil {
			return nil, err
		}
		return buildPointcloud(raw, item.Label), nil
	}
	return nil, fmt.Errorf("unknown item type %q", item.Type)
}

func (s *sceneSprites) ReadAsset(path string) ([]byte, error) {
	p := resolveAssetPath(path)
	return os.ReadFile(p)
}

func (s *sceneSprites) ComputeTick(item Item, basePose Pose, baseGeom visuals.BaseGeom, t float64) visuals.TickResult {
	return ComputeTick(item.Type, item.Animation, basePose, baseGeom, t)
}

func (s *sceneSprites) IsAnimated(item Item) bool {
	return visuals.IsAnimated(item.Animation)
}

func (s *sceneSprites) LoadPreset(name string) ([]Item, error) {
	fn, ok := Presets[name]
	if !ok {
		return nil, fmt.Errorf("unknown preset %q", name)
	}
	return fn(), nil
}

func (s *sceneSprites) BaseGeomForItem(item Item) visuals.BaseGeom {
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

// HandleCustomCommand handles the playground-specific
// get_entity_chunk verb. Returns (response, true, nil) when the
// verb is handled; (nil, false, nil) for any other verb so the
// base falls through to its debug-snapshot reply.
func (s *sceneSprites) HandleCustomCommand(
	ctx context.Context, command map[string]any,
) (map[string]any, bool, error) {
	cmd, _ := command["command"].(string)
	if cmd != "get_entity_chunk" {
		return nil, false, nil
	}
	label, _ := command["label"].(string)
	uuidStr, _ := command["uuid"].(string)
	chunkIdxF, _ := command["chunk_index"].(float64)
	chunkIdx := int(chunkIdxF)

	s.Mu().Lock()
	defer s.Mu().Unlock()
	state := s.State()
	var target *visuals.ItemState
	if label != "" {
		target = state[label]
	} else if uuidStr != "" {
		for _, st := range state {
			if string(st.UUID) == uuidStr {
				target = st
				break
			}
		}
	}
	if target == nil {
		return nil, true, errors.New("get_entity_chunk requires a valid 'label' or 'uuid'")
	}
	if target.ChunkedState == nil {
		return nil, true, fmt.Errorf("entity %q is not chunked", target.Item.Label)
	}
	cs := target.ChunkedState
	chunkPCD, err := visuals.BuildPCDChunk(cs.HeaderBytes, cs.BodyBytes, cs.Stride, chunkIdx, cs.ChunkSizePoints)
	if err != nil {
		return nil, true, err
	}
	return map[string]any{
		"label":        target.Item.Label,
		"chunk_index":  chunkIdx,
		"n_chunks":     cs.NChunks,
		"total_points": cs.TotalPoints,
		"pcd_b64":      visuals.EncodeBase64(chunkPCD),
	}, true, nil
}
