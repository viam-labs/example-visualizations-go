// SceneSprites — WorldStateStore service that publishes a
// configurable set of primitives to the Viam 3D scene viewer.
//
// Owns the item list, per-item base pose + base geometry, the
// cached *commonpb.Transform for each item (what subscribers see),
// the subscriber list with per-subscriber channels, and the
// per-cycle animation task.
//
// Two UUID strategies (selectable at runtime):
//
//   - "stable": every item keeps its UUID for life. Animation pushes
//     UPDATED events with field-mask paths matching the camelCase
//     conventions in moving_geos_world.go. See LESSONS.md::
//     snake-case-field-mask-paths-do-not-work.
//   - "versioned": every animation tick re-emits the item with a
//     fresh timestamp-suffixed UUID; pushes REMOVED for the prior
//     version then ADDED for the new version. Use this if UPDATED
//     events stop being honored by the viewer.
package exampleviz

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"exampleviz/visuals"
	commonpb "go.viam.com/api/common/v1"
	wsspb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// Model is the registered model identifier.
var Model = resource.NewModel("viam", "example-visualizations-go", "playground")

func init() {
	resource.RegisterService(worldstatestore.API, Model,
		resource.Registration[worldstatestore.Service, *Config]{
			Constructor: newSceneSprites,
		},
	)
}

// state holds one item's runtime state.
type itemState struct {
	item            Item
	basePose        Pose
	baseGeom        BaseGeom
	uuid            []byte
	transform       *commonpb.Transform
	visibleToViewer bool
	// Chunked-delivery state for pointcloud items. nil if not chunked.
	chunkedState *chunkedState
}

type chunkedState struct {
	headerBytes     []byte
	bodyBytes       []byte
	stride          int
	totalPoints     int
	chunkSizePoints int
	nChunks         int
}

type sceneSprites struct {
	resource.Named
	resource.TriviallyCloseable // close handled below via overridden Close

	logger logging.Logger
	cfg    *Config

	mu sync.Mutex
	// label -> state
	state map[string]*itemState
	// Active subscribers — each holds a per-subscriber channel.
	subscribers []chan worldstatestore.TransformChange
	// Animation tick task control.
	tickStop  chan struct{}
	tickDone  chan struct{}
	animT0    time.Time
	// Cached config values.
	tickHz       float64
	uuidStrategy string
	parentFrame  string
}

// Versioned-UUID counter; monotonic across the process so collisions
// within the same millisecond can't happen.
var versionedCounter int64

func versionedUUID(label string) []byte {
	c := atomic.AddInt64(&versionedCounter, 1)
	return []byte(fmt.Sprintf("%s_%d_%d", label, time.Now().UnixMilli(), c))
}

func initialUUID(label, strategy string) []byte {
	if strategy == "versioned" {
		return versionedUUID(label)
	}
	return []byte(label)
}

func newSceneSprites(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (worldstatestore.Service, error) {
	s := &sceneSprites{
		Named:        conf.ResourceName().AsNamed(),
		logger:       logger,
		state:        map[string]*itemState{},
		tickHz:       DefaultTickHz,
		uuidStrategy: DefaultUUIDStrategy,
		parentFrame:  DefaultParentFrame,
	}
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
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg = cfg
	if cfg.TickHz != nil {
		s.tickHz = *cfg.TickHz
	} else {
		s.tickHz = DefaultTickHz
	}
	s.uuidStrategy = cfg.UUIDStrategy
	if s.uuidStrategy == "" {
		s.uuidStrategy = DefaultUUIDStrategy
	}
	s.parentFrame = cfg.ParentFrame
	if s.parentFrame == "" {
		s.parentFrame = DefaultParentFrame
	}

	// Build the items list.
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

	// Stop any prior tick.
	if s.tickStop != nil {
		close(s.tickStop)
		s.tickStop = nil
		// Don't wait — the goroutine will exit and we shouldn't block reconfigure.
	}

	// Snapshot prior transforms for REMOVED broadcast.
	prior := make([]*commonpb.Transform, 0, len(s.state))
	for _, st := range s.state {
		prior = append(prior, st.transform)
	}
	s.state = map[string]*itemState{}

	for _, it := range items {
		if err := s.installItemLocked(it); err != nil {
			s.logger.Warnw("install_item failed", "label", it.Label, "err", err)
		}
	}

	for _, tf := range prior {
		s.broadcastLocked(worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
			Transform:  tf,
		})
	}
	for _, st := range s.state {
		s.broadcastLocked(worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  st.transform,
		})
	}

	// Restart the tick loop if any items are animated.
	animated := false
	for _, st := range s.state {
		if IsAnimated(st.item.Animation) {
			animated = true
			break
		}
	}
	if animated {
		s.tickStop = make(chan struct{})
		s.tickDone = make(chan struct{})
		s.animT0 = time.Now()
		go s.tickLoop(s.tickStop, s.tickDone)
	}

	s.logger.Infow("reconfigure",
		"tick_hz", s.tickHz,
		"uuid_strategy", s.uuidStrategy,
		"parent_frame", s.parentFrame,
		"items", len(s.state),
	)
	return nil
}

func (s *sceneSprites) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.tickStop != nil {
		close(s.tickStop)
		s.tickStop = nil
	}
	for _, ch := range s.subscribers {
		close(ch)
	}
	s.subscribers = nil
	s.mu.Unlock()
	return nil
}

// installItemLocked: caller holds s.mu. Builds initial Transform and
// stores into s.state. Handles chunked-delivery setup for pointclouds.
func (s *sceneSprites) installItemLocked(item Item) error {
	if _, exists := s.state[item.Label]; exists {
		return fmt.Errorf("duplicate item label %q", item.Label)
	}
	basePose := item.Pose
	if basePose.OX == 0 && basePose.OY == 0 && basePose.OZ == 0 {
		basePose.OZ = 1.0
	}
	baseGeom := baseGeomForItem(item)
	uuid := initialUUID(item.Label, s.uuidStrategy)

	var chunks map[string]any
	var cstate *chunkedState

	if item.Type == "pointcloud" && item.Chunked {
		path := resolveAssetPath(item.PointcloudPath)
		full, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read pointcloud %s: %w", path, err)
		}
		header, body, stride, total, err := visuals.ParsePCDBinary(full)
		if err != nil {
			return fmt.Errorf("parse PCD %s: %w", path, err)
		}
		chunkSize := item.ChunkSize
		if chunkSize <= 0 {
			chunkSize = 1000
		}
		nChunks := (total + chunkSize - 1) / chunkSize
		firstChunk, err := visuals.BuildPCDChunk(header, body, stride, 0, chunkSize)
		if err != nil {
			return err
		}
		// Stash first-chunk bytes for the initial Transform; the
		// full body lives in chunkedState for later DoCommand fetches.
		baseGeom.pcdBytesOverride = firstChunk
		chunks = map[string]any{
			"chunk_size":   float64(chunkSize),
			"total":        float64(nChunks),
			"total_points": float64(total),
			"stride":       float64(stride),
		}
		cstate = &chunkedState{
			headerBytes:     header,
			bodyBytes:       body,
			stride:          stride,
			totalPoints:     total,
			chunkSizePoints: chunkSize,
			nChunks:         nChunks,
		}
	}

	geom, err := buildGeometryForItem(item, baseGeom)
	if err != nil {
		return err
	}
	tf, err := buildTransform(item, basePose, geom, uuid, s.parentFrame, chunks)
	if err != nil {
		return err
	}
	s.state[item.Label] = &itemState{
		item:            item,
		basePose:        basePose,
		baseGeom:        baseGeom,
		uuid:            uuid,
		transform:       tf,
		visibleToViewer: true,
		chunkedState:    cstate,
	}
	return nil
}

func baseGeomForItem(item Item) BaseGeom {
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

// buildGeometryForItem dispatches to the right geometry builder.
// Honors per-tick geom overrides when present (pulse on dims/radius).
func buildGeometryForItem(item Item, geom BaseGeom) (*commonpb.Geometry, error) {
	switch item.Type {
	case "box":
		dims := item.DimsMM
		if geom.HasDims {
			dims = geom.Dims
		}
		return buildBox([3]float64{dims.X, dims.Y, dims.Z}, item.Label), nil
	case "sphere":
		r := item.RadiusMM
		if geom.RadiusMM > 0 {
			r = geom.RadiusMM
		}
		return buildSphere(r, item.Label), nil
	case "capsule":
		r, l := item.RadiusMM, item.LengthMM
		if geom.RadiusMM > 0 {
			r = geom.RadiusMM
		}
		if geom.LengthMM > 0 {
			l = geom.LengthMM
		}
		return buildCapsule(r, l, item.Label), nil
	case "point":
		return buildPoint(item.Label), nil
	case "arrow":
		l, r := item.LengthMM, item.RadiusMM
		if geom.LengthMM > 0 {
			l = geom.LengthMM
		}
		if geom.RadiusMM > 0 {
			r = geom.RadiusMM
		}
		return buildArrow(l, r, item.Label), nil
	case "mesh":
		path := resolveAssetPath(item.MeshPath)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read mesh %s: %w", path, err)
		}
		// raw_stl: bug-demo for the viz team — ship the STL bytes
		// straight through with content_type="stl" so the viewer's
		// silent-drop is observable. See LESSONS.md::mesh-formats
		// in the Python sibling repo.
		if item.RawSTL {
			return buildMesh(raw, "stl", item.Label, true)
		}
		ply, err := visuals.LoadMeshBytesAsPLY(raw, item.MeshPath)
		if err != nil {
			return nil, err
		}
		return buildMesh(ply, "ply", item.Label, false)
	case "pointcloud":
		if geom.pcdBytesOverride != nil {
			return buildPointcloud(geom.pcdBytesOverride, item.Label), nil
		}
		path := resolveAssetPath(item.PointcloudPath)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read pointcloud %s: %w", path, err)
		}
		return buildPointcloud(raw, item.Label), nil
	}
	return nil, fmt.Errorf("unknown item type %q", item.Type)
}

// buildTransform assembles the full *commonpb.Transform for an item.
func buildTransform(
	item Item, pose Pose, geom *commonpb.Geometry, uuid []byte, parentFrame string,
	chunks map[string]any,
) (*commonpb.Transform, error) {
	parent := item.ParentFrame
	if parent == "" {
		parent = parentFrame
	}
	// If the user didn't set a uniform color AND we have a PLY mesh
	// with embedded per-vertex colors, transcode them into metadata.colors.
	var vertexColors [][3]int
	if item.Color == nil && geom != nil && geom.GetMesh() != nil {
		vertexColors = visuals.ExtractPLYVertexColors(geom.GetMesh().Mesh)
	}
	md := visuals.BuildMetadata(visuals.MetadataOpts{
		Color:          item.Color,
		Opacity:        item.Opacity,
		ShowAxesHelper: item.ShowAxesHelper,
		Invisible:      item.Invisible,
		VertexColors:   vertexColors,
		Chunks:         chunks,
	})
	return &commonpb.Transform{
		Uuid:           uuid,
		ReferenceFrame: item.Label,
		PoseInObserverFrame: &commonpb.PoseInFrame{
			ReferenceFrame: parent,
			Pose:           buildPose(pose),
		},
		PhysicalObject: geom,
		Metadata:       md,
	}, nil
}

// ---- WorldStateStore API ---------------------------------------------

func (s *sceneSprites) ListUUIDs(_ context.Context, _ map[string]any) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, 0, len(s.state))
	for _, st := range s.state {
		if !st.visibleToViewer {
			continue
		}
		out = append(out, append([]byte(nil), st.uuid...))
	}
	return out, nil
}

func (s *sceneSprites) GetTransform(_ context.Context, uuid []byte, _ map[string]any) (*commonpb.Transform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.state {
		if string(st.uuid) == string(uuid) {
			if !st.visibleToViewer {
				return nil, fmt.Errorf("uuid %q is currently not in the scene (flicker/lifecycle animation has it temporarily removed)",
					string(uuid))
			}
			return st.transform, nil
		}
	}
	return nil, fmt.Errorf("unknown uuid %q", string(uuid))
}

func (s *sceneSprites) StreamTransformChanges(ctx context.Context, _ map[string]any) (*worldstatestore.TransformChangeStream, error) {
	ch := make(chan worldstatestore.TransformChange, 256)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	// Initial burst: ADDED for everything currently visible.
	for _, st := range s.state {
		if !st.visibleToViewer {
			continue
		}
		select {
		case ch <- worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  st.transform,
		}:
		default:
			s.logger.Warn("subscriber queue full at initial burst; some ADDED events dropped")
		}
	}
	s.mu.Unlock()

	// Wrap channel into a stream and clean up the subscriber on context cancel.
	go func() {
		<-ctx.Done()
		s.mu.Lock()
		for i, c := range s.subscribers {
			if c == ch {
				s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
				break
			}
		}
		s.mu.Unlock()
	}()
	return worldstatestore.NewTransformChangeStreamFromChannel(ctx, ch), nil
}

// broadcastLocked: caller holds s.mu.
func (s *sceneSprites) broadcastLocked(c worldstatestore.TransformChange) {
	for _, ch := range s.subscribers {
		select {
		case ch <- c:
		default:
			s.logger.Warn("subscriber queue full; dropping event")
		}
	}
}

// ---- animation tick --------------------------------------------------

func (s *sceneSprites) tickLoop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	period := time.Duration(float64(time.Second) / math.Max(0.01, s.tickHz))
	ticker := time.NewTicker(period)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if err := s.tickOnce(); err != nil {
				s.logger.Warnw("tick failed", "err", err)
			}
		}
	}
}

func (s *sceneSprites) tickOnce() error {
	t := time.Since(s.animT0).Seconds()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.state {
		if !IsAnimated(st.item.Animation) {
			continue
		}
		res := ComputeTick(st.item.Type, st.item.Animation, st.basePose, st.baseGeom, t)
		// Scene-graph mutation path (flicker, lifecycle).
		if res.Overrides != nil && res.Overrides.InScene != nil {
			wantIn := *res.Overrides.InScene
			was := st.visibleToViewer
			if wantIn && !was {
				// Rising edge: ADD.
				rotate := true
				if st.item.Animation.RotateUUIDOnReadd != nil {
					rotate = *st.item.Animation.RotateUUIDOnReadd
				}
				if rotate {
					st.uuid = versionedUUID(st.item.Label)
				}
				itemForAdd := st.item
				if res.Overrides.Color != nil {
					c := *res.Overrides.Color
					itemForAdd.Color = &c
				}
				if res.Overrides.Opacity != nil {
					op := *res.Overrides.Opacity
					itemForAdd.Opacity = &op
				}
				geom, err := buildGeometryForItem(st.item, res.Geom)
				if err != nil {
					return err
				}
				newTF, err := buildTransform(itemForAdd, res.Pose, geom, st.uuid, s.parentFrame, nil)
				if err != nil {
					return err
				}
				st.transform = newTF
				st.visibleToViewer = true
				s.broadcastLocked(worldstatestore.TransformChange{
					ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
					Transform:  newTF,
				})
				continue
			}
			if !wantIn && was {
				st.visibleToViewer = false
				s.broadcastLocked(worldstatestore.TransformChange{
					ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
					Transform:  st.transform,
				})
				continue
			}
			if !wantIn && !was {
				continue
			}
			// Both on — fall through to UPDATED with color/opacity.
		}
		if len(res.Paths) == 0 {
			continue
		}
		geom, err := buildGeometryForItem(st.item, res.Geom)
		if err != nil {
			return err
		}
		itemForTF := st.item
		if res.Overrides != nil {
			if res.Overrides.Color != nil {
				c := *res.Overrides.Color
				itemForTF.Color = &c
			}
			if res.Overrides.Opacity != nil {
				op := *res.Overrides.Opacity
				itemForTF.Opacity = &op
			}
		}
		if s.uuidStrategy == "stable" {
			newTF, err := buildTransform(itemForTF, res.Pose, geom, st.uuid, s.parentFrame, nil)
			if err != nil {
				return err
			}
			st.transform = newTF
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType:    wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
				Transform:     newTF,
				UpdatedFields: append([]string(nil), res.Paths...),
			})
		} else {
			oldTF := st.transform
			st.uuid = versionedUUID(st.item.Label)
			newTF, err := buildTransform(itemForTF, res.Pose, geom, st.uuid, s.parentFrame, nil)
			if err != nil {
				return err
			}
			st.transform = newTF
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
				Transform:  oldTF,
			})
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
				Transform:  newTF,
			})
		}
	}
	return nil
}

// ---- DoCommand -------------------------------------------------------

func (s *sceneSprites) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
	cmd, _ := command["command"].(string)

	switch cmd {
	case "list":
		s.mu.Lock()
		defer s.mu.Unlock()
		items := make([]map[string]any, 0, len(s.state))
		labels := make([]string, 0, len(s.state))
		for k := range s.state {
			labels = append(labels, k)
		}
		sort.Strings(labels)
		for _, label := range labels {
			st := s.state[label]
			items = append(items, map[string]any{
				"label":          label,
				"type":           st.item.Type,
				"uuid":           string(st.uuid),
				"animation_mode": st.item.Animation.Mode,
			})
		}
		return map[string]any{"items": items}, nil

	case "remove":
		label, _ := command["label"].(string)
		if label == "" {
			return nil, errors.New("remove requires a 'label'")
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		st, ok := s.state[label]
		if !ok {
			return map[string]any{"removed": false}, nil
		}
		delete(s.state, label)
		s.broadcastLocked(worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
			Transform:  st.transform,
		})
		return map[string]any{"removed": true}, nil

	case "clear":
		s.mu.Lock()
		defer s.mu.Unlock()
		count := len(s.state)
		for _, st := range s.state {
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
				Transform:  st.transform,
			})
		}
		s.state = map[string]*itemState{}
		return map[string]any{"removed_count": count}, nil

	case "preset":
		name, _ := command["name"].(string)
		if name == "" {
			return nil, errors.New("preset requires a 'name'")
		}
		fn, ok := Presets[name]
		if !ok {
			return nil, fmt.Errorf("unknown preset %q", name)
		}
		items := fn()
		s.mu.Lock()
		defer s.mu.Unlock()
		prior := make([]*commonpb.Transform, 0, len(s.state))
		for _, st := range s.state {
			prior = append(prior, st.transform)
		}
		s.state = map[string]*itemState{}
		for _, it := range items {
			if err := s.installItemLocked(it); err != nil {
				s.logger.Warnw("preset install failed", "label", it.Label, "err", err)
			}
		}
		for _, tf := range prior {
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
				Transform:  tf,
			})
		}
		for _, st := range s.state {
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
				Transform:  st.transform,
			})
		}
		return map[string]any{"loaded": name, "count": len(s.state)}, nil

	case "set_uuid_strategy":
		strategy, _ := command["strategy"].(string)
		if !contains(ValidUUIDStrategies, strategy) {
			return nil, fmt.Errorf("strategy must be one of %v, got %q", ValidUUIDStrategies, strategy)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.uuidStrategy = strategy
		return map[string]any{"strategy": strategy}, nil

	case "get_entity_chunk":
		return s.doGetEntityChunk(command)

	case "snapshot":
		s.mu.Lock()
		defer s.mu.Unlock()
		// Best-effort round-trip — emit items as JSON-friendly maps.
		items := []map[string]any{}
		labels := make([]string, 0, len(s.state))
		for k := range s.state {
			labels = append(labels, k)
		}
		sort.Strings(labels)
		for _, l := range labels {
			items = append(items, itemAsJSON(s.state[l].item))
		}
		return map[string]any{"config": map[string]any{
			"tick_hz":       s.tickHz,
			"uuid_strategy": s.uuidStrategy,
			"parent_frame":  s.parentFrame,
			"items":         items,
		}}, nil
	}

	// Default / unrecognized: debug snapshot.
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"tick_hz":          s.tickHz,
		"uuid_strategy":    s.uuidStrategy,
		"parent_frame":     s.parentFrame,
		"item_count":       len(s.state),
		"subscriber_count": len(s.subscribers),
		"tick_running":     s.tickStop != nil,
	}, nil
}

func (s *sceneSprites) doGetEntityChunk(command map[string]any) (map[string]any, error) {
	label, _ := command["label"].(string)
	uuidStr, _ := command["uuid"].(string)
	chunkIdxF, _ := command["chunk_index"].(float64)
	chunkIdx := int(chunkIdxF)
	s.mu.Lock()
	defer s.mu.Unlock()
	var target *itemState
	if label != "" {
		target = s.state[label]
	} else if uuidStr != "" {
		for _, st := range s.state {
			if string(st.uuid) == uuidStr {
				target = st
				break
			}
		}
	}
	if target == nil {
		return nil, errors.New("get_entity_chunk requires a valid 'label' or 'uuid'")
	}
	if target.chunkedState == nil {
		return nil, fmt.Errorf("entity %q is not chunked", target.item.Label)
	}
	cs := target.chunkedState
	chunkPCD, err := visuals.BuildPCDChunk(cs.headerBytes, cs.bodyBytes, cs.stride, chunkIdx, cs.chunkSizePoints)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"label":        target.item.Label,
		"chunk_index":  chunkIdx,
		"n_chunks":     cs.nChunks,
		"total_points": cs.totalPoints,
		"pcd_b64":      base64.StdEncoding.EncodeToString(chunkPCD),
	}, nil
}

// itemAsJSON: best-effort round-trip back to a JSON-ish item dict
// for the snapshot DoCommand verb.
func itemAsJSON(it Item) map[string]any {
	m := map[string]any{
		"type":  it.Type,
		"label": it.Label,
	}
	if it.ParentFrame != "" {
		m["parent_frame"] = it.ParentFrame
	}
	m["pose"] = map[string]any{
		"x": it.Pose.X, "y": it.Pose.Y, "z": it.Pose.Z,
		"ox": it.Pose.OX, "oy": it.Pose.OY, "oz": it.Pose.OZ,
		"theta": it.Pose.Theta,
	}
	if it.HasDims {
		m["dims_mm"] = map[string]any{"x": it.DimsMM.X, "y": it.DimsMM.Y, "z": it.DimsMM.Z}
	}
	if it.RadiusMM != 0 {
		m["radius_mm"] = it.RadiusMM
	}
	if it.LengthMM != 0 {
		m["length_mm"] = it.LengthMM
	}
	if it.MeshPath != "" {
		m["mesh_path"] = it.MeshPath
	}
	if it.PointcloudPath != "" {
		m["pointcloud_path"] = it.PointcloudPath
	}
	if it.Color != nil {
		m["color"] = map[string]any{"r": it.Color.R, "g": it.Color.G, "b": it.Color.B}
	}
	if it.Opacity != nil {
		m["opacity"] = *it.Opacity
	}
	if it.ShowAxesHelper {
		m["show_axes_helper"] = true
	}
	if it.Invisible {
		m["invisible"] = true
	}
	if it.Chunked {
		m["chunked"] = true
		if it.ChunkSize > 0 {
			m["chunk_size"] = it.ChunkSize
		}
	}
	if IsAnimated(it.Animation) {
		// Just emit mode + a few common fields; full round-trip is best-effort.
		am := map[string]any{"mode": it.Animation.Mode}
		if it.Animation.PeriodS != 0 {
			am["period_s"] = it.Animation.PeriodS
		}
		if it.Animation.AmplitudeMM != 0 {
			am["amplitude_mm"] = it.Animation.AmplitudeMM
		}
		if it.Animation.Axis != "" {
			am["axis"] = it.Animation.Axis
		}
		m["animation"] = am
	}
	return m
}

// pcdBytesOverride lives on BaseGeom (extended in geometries.go's
// BaseGeom struct via the pcdBytesOverride field). Declared inline
// to keep geometries.go pure of service concerns.
// -- This is implemented as an extra field via type assertions. To
// avoid adding service-specific state to the pure geometry layer,
// we instead carry it in the chunkedState struct above. The geometry
// builder reads from baseGeom only when it has been set to a non-zero
// pcd. We achieve this by passing baseGeom.pcdBytesOverride through.
// (The field is declared on BaseGeom in geometries.go to keep the
// type accessible everywhere.)
//
// Silence "declared and not used" for json import below if codepaths
// later rely on it.
var _ = json.Marshal
var _ = io.EOF
var _ = filepath.Join
