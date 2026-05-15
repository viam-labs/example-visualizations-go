package visuals

// SceneServiceBase — inheritable WorldStateStore service base.
//
// A module author who wants to publish a world-state-store scene
// embeds SceneServiceBase in their service struct and provides a
// SceneHooks implementation that fills in the module-specific
// bits — geometry building, asset reading, animation tick math,
// preset lookup, custom DoCommand verbs.
//
// SceneServiceBase owns the generic WSS plumbing: state map,
// subscriber fanout, the animation tick goroutine, UUID strategy,
// and the standard nine DoCommand verbs (list / remove / clear /
// preset / snapshot / set_uuid_strategy).
//
// Usage:
//
//   type myService struct {
//       resource.Named
//       resource.TriviallyCloseable
//       visuals.SceneServiceBase
//   }
//
//   // Implement SceneHooks on *myService.
//   func (s *myService) BuildGeometry(...) (*commonpb.Geometry, error) { ... }
//   func (s *myService) ReadAsset(path string) ([]byte, error) { ... }
//   // ... etc
//
//   // Wire up in your constructor:
//   func newMyService(...) (worldstatestore.Service, error) {
//       s := &myService{Named: ...}
//       s.SceneServiceBase.Hooks = s
//       s.SceneServiceBase.Logger = logger
//       if err := s.Reconfigure(ctx, deps, conf); err != nil {
//           return nil, err
//       }
//       return s, nil
//   }

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	commonpb "go.viam.com/api/common/v1"
	wsspb "go.viam.com/api/service/worldstatestore/v1"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/services/worldstatestore"
)

// SceneHooks is the contract a module fills in. The base struct
// holds an instance and calls into it for everything that depends
// on module-specific concerns (asset paths, primitive types,
// animation tick math, preset registry, custom DoCommand verbs).
type SceneHooks interface {
	// BuildGeometry constructs the commonpb.Geometry proto for an
	// item. Module-specific because the primitive-type set is
	// module-specific. Honors per-tick geom overrides (mostly for
	// pulse and chunked-pointcloud paths).
	BuildGeometry(item Item, override BaseGeom) (*commonpb.Geometry, error)

	// ReadAsset reads an asset's bytes. Modules typically resolve
	// relative paths against their installed module directory.
	ReadAsset(path string) ([]byte, error)

	// ComputeTick is the per-tick animation evaluator. Returns the
	// new pose + geom overrides, the field-mask paths the viewer
	// needs in the UPDATED event, and optional metadata overrides
	// (color/opacity/in_scene).
	ComputeTick(item Item, basePose Pose, baseGeom BaseGeom, t float64) TickResult

	// IsAnimated returns true iff this item's animation should tick.
	IsAnimated(item Item) bool

	// LoadPreset fetches a named preset's item list. Modules with
	// no presets should return an error.
	LoadPreset(name string) ([]Item, error)

	// BaseGeomForItem extracts the shape-specific base fields.
	BaseGeomForItem(item Item) BaseGeom

	// HandleCustomCommand handles DoCommand verbs the base class
	// doesn't know. Return (nil, false, nil) to indicate the verb
	// is not handled — the base falls through to its default
	// debug-snapshot reply.
	HandleCustomCommand(ctx context.Context, command map[string]any) (response map[string]any, handled bool, err error)
}

// ItemState is the per-item runtime state.
type ItemState struct {
	Item            Item
	BasePose        Pose
	BaseGeom        BaseGeom
	UUID            []byte
	Transform       *commonpb.Transform
	VisibleToViewer bool
	ChunkedState    *ChunkedState
}

// ChunkedState carries the parsed PCD body for chunked-delivery
// pointcloud items. Reused for later get_entity_chunk fetches.
type ChunkedState struct {
	HeaderBytes     []byte
	BodyBytes       []byte
	Stride          int
	TotalPoints     int
	ChunkSizePoints int
	NChunks         int
}

// SceneServiceBase is the inheritable WSS service base. See package
// docstring for the contract.
type SceneServiceBase struct {
	Hooks  SceneHooks
	Logger logging.Logger

	// Defaults — set these in the module's constructor before
	// calling Reconfigure (zero values mean "use a sensible fallback").
	DefaultTickHz          float64
	DefaultUUIDStrategy    string
	DefaultParentFrame     string
	DefaultPreset          string
	DefaultChunkSizePoints int
	MaxTickHz              float64

	mu          sync.Mutex
	state       map[string]*ItemState
	subscribers []chan worldstatestore.TransformChange
	tickStop    chan struct{}
	tickDone    chan struct{}
	animT0      time.Time

	tickHz       float64
	uuidStrategy string
	parentFrame  string
}

// ReconfigureWith does the SceneServiceBase reconfigure. Takes the
// items list the module produced (from config or preset) and the
// effective tick/uuid/parent-frame attributes.
//
// Modules typically call this from their own Reconfigure after
// parsing the config struct.
func (s *SceneServiceBase) ReconfigureWith(
	items []Item,
	tickHz float64,
	uuidStrategy string,
	parentFrame string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tickHz = tickHz
	if s.tickHz == 0 {
		s.tickHz = s.defaultTickHzOr()
	}
	s.uuidStrategy = uuidStrategy
	if s.uuidStrategy == "" {
		s.uuidStrategy = s.defaultUUIDStrategyOr()
	}
	s.parentFrame = parentFrame
	if s.parentFrame == "" {
		s.parentFrame = s.defaultParentFrameOr()
	}

	// Stop any prior tick.
	if s.tickStop != nil {
		close(s.tickStop)
		s.tickStop = nil
	}

	prior := make([]*commonpb.Transform, 0, len(s.state))
	for _, st := range s.state {
		prior = append(prior, st.Transform)
	}
	s.state = map[string]*ItemState{}

	for _, it := range items {
		if err := s.installItemLocked(it); err != nil {
			if s.Logger != nil {
				s.Logger.Warnw("install_item failed", "label", it.Label, "err", err)
			}
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
			Transform:  st.Transform,
		})
	}

	animated := false
	for _, st := range s.state {
		if s.Hooks.IsAnimated(st.Item) {
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
	return nil
}

// Close shuts down the tick goroutine and closes all subscriber
// channels.
func (s *SceneServiceBase) Close(_ context.Context) error {
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

// installItemLocked: caller holds s.mu. Builds the initial Transform
// and stores into s.state. Handles chunked-delivery setup for
// pointcloud items.
func (s *SceneServiceBase) installItemLocked(item Item) error {
	if _, exists := s.state[item.Label]; exists {
		return fmt.Errorf("duplicate item label %q", item.Label)
	}
	basePose := item.Pose
	if basePose.OX == 0 && basePose.OY == 0 && basePose.OZ == 0 {
		basePose.OZ = 1.0
	}
	baseGeom := s.Hooks.BaseGeomForItem(item)
	uuid := InitialUUID(item.Label, s.uuidStrategy)

	var chunks map[string]any
	var cstate *ChunkedState

	if item.Type == "pointcloud" && item.Chunked {
		full, err := s.Hooks.ReadAsset(item.PointcloudPath)
		if err != nil {
			return fmt.Errorf("read pointcloud %s: %w", item.PointcloudPath, err)
		}
		header, body, stride, total, err := ParsePCDBinary(full)
		if err != nil {
			return fmt.Errorf("parse PCD %s: %w", item.PointcloudPath, err)
		}
		chunkSize := item.ChunkSize
		if chunkSize <= 0 {
			chunkSize = s.defaultChunkSizeOr()
		}
		nChunks := (total + chunkSize - 1) / chunkSize
		firstChunk, err := BuildPCDChunk(header, body, stride, 0, chunkSize)
		if err != nil {
			return err
		}
		baseGeom.PCDBytesOverride = firstChunk
		chunks = map[string]any{
			"chunk_size":   float64(chunkSize),
			"total":        float64(nChunks),
			"total_points": float64(total),
			"stride":       float64(stride),
		}
		cstate = &ChunkedState{
			HeaderBytes:     header,
			BodyBytes:       body,
			Stride:          stride,
			TotalPoints:     total,
			ChunkSizePoints: chunkSize,
			NChunks:         nChunks,
		}
	}

	geom, err := s.Hooks.BuildGeometry(item, baseGeom)
	if err != nil {
		return err
	}
	tf, err := s.buildTransform(item, basePose, geom, uuid, chunks)
	if err != nil {
		return err
	}
	s.state[item.Label] = &ItemState{
		Item:            item,
		BasePose:        basePose,
		BaseGeom:        baseGeom,
		UUID:            uuid,
		Transform:       tf,
		VisibleToViewer: true,
		ChunkedState:    cstate,
	}
	return nil
}

// buildTransform assembles the *commonpb.Transform from an item +
// pose + geom. Handles vertex-color transcoding to metadata.colors.
func (s *SceneServiceBase) buildTransform(
	item Item, pose Pose, geom *commonpb.Geometry, uuid []byte,
	chunks map[string]any,
) (*commonpb.Transform, error) {
	parent := item.ParentFrame
	if parent == "" {
		parent = s.parentFrame
	}
	var vertexColors [][3]int
	if item.Color == nil && geom != nil && geom.GetMesh() != nil {
		vertexColors = ExtractPLYVertexColors(geom.GetMesh().Mesh)
	}
	md := BuildMetadata(MetadataOpts{
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
			Pose:           poseToProto(pose),
		},
		PhysicalObject: geom,
		Metadata:       md,
	}, nil
}

// poseToProto: simple conversion (kept private; module doesn't need
// to provide pose-to-proto because the library can do it).
func poseToProto(p Pose) *commonpb.Pose {
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

// ---- WorldStateStore API ---------------------------------------------

func (s *SceneServiceBase) ListUUIDs(_ context.Context, _ map[string]any) ([][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][]byte, 0, len(s.state))
	for _, st := range s.state {
		if !st.VisibleToViewer {
			continue
		}
		out = append(out, append([]byte(nil), st.UUID...))
	}
	return out, nil
}

func (s *SceneServiceBase) GetTransform(_ context.Context, uuid []byte, _ map[string]any) (*commonpb.Transform, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.state {
		if string(st.UUID) == string(uuid) {
			if !st.VisibleToViewer {
				return nil, fmt.Errorf("uuid %q is currently not in the scene (flicker/lifecycle animation has it temporarily removed)",
					string(uuid))
			}
			return st.Transform, nil
		}
	}
	return nil, fmt.Errorf("unknown uuid %q", string(uuid))
}

func (s *SceneServiceBase) StreamTransformChanges(ctx context.Context, _ map[string]any) (*worldstatestore.TransformChangeStream, error) {
	ch := make(chan worldstatestore.TransformChange, 256)
	s.mu.Lock()
	s.subscribers = append(s.subscribers, ch)
	for _, st := range s.state {
		if !st.VisibleToViewer {
			continue
		}
		select {
		case ch <- worldstatestore.TransformChange{
			ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  st.Transform,
		}:
		default:
			if s.Logger != nil {
				s.Logger.Warn("subscriber queue full at initial burst; some ADDED events dropped")
			}
		}
	}
	s.mu.Unlock()

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

func (s *SceneServiceBase) broadcastLocked(c worldstatestore.TransformChange) {
	for _, ch := range s.subscribers {
		select {
		case ch <- c:
		default:
			if s.Logger != nil {
				s.Logger.Warn("subscriber queue full; dropping event")
			}
		}
	}
}

// ---- animation tick --------------------------------------------------

func (s *SceneServiceBase) tickLoop(stop <-chan struct{}, done chan<- struct{}) {
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
				if s.Logger != nil {
					s.Logger.Warnw("tick failed", "err", err)
				}
			}
		}
	}
}

func (s *SceneServiceBase) tickOnce() error {
	t := time.Since(s.animT0).Seconds()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.state {
		if !s.Hooks.IsAnimated(st.Item) {
			continue
		}
		res := s.Hooks.ComputeTick(st.Item, st.BasePose, st.BaseGeom, t)
		if res.Overrides != nil && res.Overrides.InScene != nil {
			wantIn := *res.Overrides.InScene
			was := st.VisibleToViewer
			if wantIn && !was {
				rotate := true
				if st.Item.Animation.RotateUUIDOnReadd != nil {
					rotate = *st.Item.Animation.RotateUUIDOnReadd
				}
				if rotate {
					st.UUID = VersionedUUID(st.Item.Label)
				}
				itemForAdd := st.Item
				if res.Overrides.Color != nil {
					c := *res.Overrides.Color
					itemForAdd.Color = &c
				}
				if res.Overrides.Opacity != nil {
					op := *res.Overrides.Opacity
					itemForAdd.Opacity = &op
				}
				geom, err := s.Hooks.BuildGeometry(st.Item, res.Geom)
				if err != nil {
					return err
				}
				newTF, err := s.buildTransform(itemForAdd, res.Pose, geom, st.UUID, nil)
				if err != nil {
					return err
				}
				st.Transform = newTF
				st.VisibleToViewer = true
				s.broadcastLocked(worldstatestore.TransformChange{
					ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
					Transform:  newTF,
				})
				continue
			}
			if !wantIn && was {
				st.VisibleToViewer = false
				s.broadcastLocked(worldstatestore.TransformChange{
					ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
					Transform:  st.Transform,
				})
				continue
			}
			if !wantIn && !was {
				continue
			}
			// Both on — fall through to UPDATED.
		}
		if len(res.Paths) == 0 {
			continue
		}
		geom, err := s.Hooks.BuildGeometry(st.Item, res.Geom)
		if err != nil {
			return err
		}
		itemForTF := st.Item
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
			newTF, err := s.buildTransform(itemForTF, res.Pose, geom, st.UUID, nil)
			if err != nil {
				return err
			}
			st.Transform = newTF
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType:    wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
				Transform:     newTF,
				UpdatedFields: append([]string(nil), res.Paths...),
			})
		} else {
			oldTF := st.Transform
			st.UUID = VersionedUUID(st.Item.Label)
			newTF, err := s.buildTransform(itemForTF, res.Pose, geom, st.UUID, nil)
			if err != nil {
				return err
			}
			st.Transform = newTF
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

// DoCommand handles the standard set of WSS verbs. Unknown verbs fall
// through to the hooks' HandleCustomCommand; if that returns
// handled=false, returns a debug snapshot.
func (s *SceneServiceBase) DoCommand(ctx context.Context, command map[string]any) (map[string]any, error) {
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
				"type":           st.Item.Type,
				"uuid":           string(st.UUID),
				"animation_mode": st.Item.Animation.Mode,
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
			Transform:  st.Transform,
		})
		return map[string]any{"removed": true}, nil

	case "clear":
		s.mu.Lock()
		defer s.mu.Unlock()
		count := len(s.state)
		for _, st := range s.state {
			s.broadcastLocked(worldstatestore.TransformChange{
				ChangeType: wsspb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
				Transform:  st.Transform,
			})
		}
		s.state = map[string]*ItemState{}
		return map[string]any{"removed_count": count}, nil

	case "preset":
		name, _ := command["name"].(string)
		if name == "" {
			return nil, errors.New("preset requires a 'name'")
		}
		items, err := s.Hooks.LoadPreset(name)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		prior := make([]*commonpb.Transform, 0, len(s.state))
		for _, st := range s.state {
			prior = append(prior, st.Transform)
		}
		s.state = map[string]*ItemState{}
		for _, it := range items {
			if err := s.installItemLocked(it); err != nil {
				if s.Logger != nil {
					s.Logger.Warnw("preset install failed", "label", it.Label, "err", err)
				}
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
				Transform:  st.Transform,
			})
		}
		return map[string]any{"loaded": name, "count": len(s.state)}, nil

	case "set_uuid_strategy":
		strategy, _ := command["strategy"].(string)
		ok := false
		for _, v := range ValidStrategies {
			if v == strategy {
				ok = true
				break
			}
		}
		if !ok {
			return nil, fmt.Errorf("strategy must be one of %v, got %q", ValidStrategies, strategy)
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.uuidStrategy = strategy
		return map[string]any{"strategy": strategy}, nil

	case "snapshot":
		s.mu.Lock()
		defer s.mu.Unlock()
		items := []map[string]any{}
		labels := make([]string, 0, len(s.state))
		for k := range s.state {
			labels = append(labels, k)
		}
		sort.Strings(labels)
		for _, l := range labels {
			items = append(items, itemAsJSON(s.state[l].Item))
		}
		return map[string]any{"config": map[string]any{
			"tick_hz":       s.tickHz,
			"uuid_strategy": s.uuidStrategy,
			"parent_frame":  s.parentFrame,
			"items":         items,
		}}, nil
	}

	// Custom verbs through the hooks.
	resp, handled, err := s.Hooks.HandleCustomCommand(ctx, command)
	if err != nil {
		return nil, err
	}
	if handled {
		return resp, nil
	}

	// Default: debug snapshot.
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

// ---- helpers --------------------------------------------------------

// State returns the read-locked state map for module access (e.g.,
// custom DoCommand verbs that need to look up an item by label).
// Caller must hold s.Mu() while iterating; returned map is the
// same underlying map.
func (s *SceneServiceBase) State() map[string]*ItemState {
	return s.state
}

// Mu returns the base's mutex so module-side custom verbs can hold
// it across multi-step operations on the state map.
func (s *SceneServiceBase) Mu() *sync.Mutex {
	return &s.mu
}

// TickStopChan returns the current tick goroutine's stop channel,
// or nil if no tick task is running. Module code shouldn't need
// this directly — exposed mostly for debug introspection.
func (s *SceneServiceBase) TickStopChan() chan struct{} { return s.tickStop }

func (s *SceneServiceBase) defaultTickHzOr() float64 {
	if s.DefaultTickHz > 0 {
		return s.DefaultTickHz
	}
	return 30.0
}

func (s *SceneServiceBase) defaultUUIDStrategyOr() string {
	if s.DefaultUUIDStrategy != "" {
		return s.DefaultUUIDStrategy
	}
	return "stable"
}

func (s *SceneServiceBase) defaultParentFrameOr() string {
	if s.DefaultParentFrame != "" {
		return s.DefaultParentFrame
	}
	return "world"
}

func (s *SceneServiceBase) defaultChunkSizeOr() int {
	if s.DefaultChunkSizePoints > 0 {
		return s.DefaultChunkSizePoints
	}
	return 1000
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

// EncodeBase64 is exported because module-side custom DoCommand
// verbs (e.g., get_entity_chunk) commonly need to base64-encode
// bytes for the response, and importing encoding/base64 directly is
// boilerplate.
func EncodeBase64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
