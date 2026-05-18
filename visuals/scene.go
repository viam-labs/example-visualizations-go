// Scene — typed state container with object-based mutation API.
//
// Mirrors viam_visuals.Scene from the Python sibling. Holds Visuals
// (or expanded constituents of Composites) by label, tracks the wire-
// format Item committed for each one, and emits SceneEvent records
// describing what changed when callers mutate and re-submit.
//
// Typical loop:
//
//	scene := visuals.NewScene("world")
//
//	bbox := &visuals.BoundingBox{Label: "obj_a",
//	    DimsMM: visuals.BoxDims{X: 100, Y: 200, Z: 50}}
//	scene.Add(bbox)
//
//	// ...detection moves...
//	bbox.Pose = visuals.PoseAt(500, -200, 100, 0)
//	bbox.Color = &visuals.Color{R: 0, G: 255, B: 0}
//	events := scene.Update(bbox)
//	// events == [{Kind: "updated", Label: "obj_a",
//	//             Paths: ["poseInObserverFrame.pose.x", ...,
//	//                     "metadata.colors"]}]
//
// Callers must pass POINTERS to Visual structs so mutations between
// Add and Update are visible through the interface. Value-typed
// Visuals work for Add but produce no diffs on Update — the scene
// re-snapshots a copy and compares against the original copy, which
// won't have changed.
//
// The diff is state-based, not patch-based: Scene snapshots the
// item at Add time and re-snapshots after each Update. Field-mask
// paths come from comparing snapshots, so callers can mutate any
// subset of the fields without specifying which.
//
// This struct deliberately doesn't broadcast anywhere; it produces
// events the caller (or a wrapping service) consumes. A future
// revision of SceneServiceBase can hold a Scene internally and
// forward events to its subscribers, but the type works standalone
// for tests and for callers writing their own service plumbing.
package visuals

import (
	"fmt"
	"sort"
)

// Event kinds.
const (
	EventAdded   = "added"
	EventUpdated = "updated"
	EventRemoved = "removed"
)

// SceneEvent is one state-change record produced by Scene mutation
// methods. Item carries the current wire-format item for ADDED /
// UPDATED (zero-value for REMOVED). Paths is the field-mask path
// list for UPDATED (always camelCase; the renderer ignores
// snake_case).
type SceneEvent struct {
	Kind  string
	Label string
	Item  Item
	Paths []string
}

// sceneEntry is one row of scene state — the live object reference
// plus the last-committed wire-format Item used for diffing.
type sceneEntry struct {
	visual    Visual
	committed Item
}

// Scene is a typed state container with object-based add / update /
// remove. A scene is a mapping from label to Visual (or a
// constituent of an expanded Composite). Call sites get back
// SceneEvent slices that downstream service plumbing can forward to
// WSS subscribers.
type Scene struct {
	parentFrame string
	state       map[string]*sceneEntry
}

// NewScene returns a Scene parented to parentFrame (use "world" for
// the default root).
func NewScene(parentFrame string) *Scene {
	if parentFrame == "" {
		parentFrame = "world"
	}
	return &Scene{
		parentFrame: parentFrame,
		state:       make(map[string]*sceneEntry),
	}
}

// ParentFrame returns the scene's root frame.
func (s *Scene) ParentFrame() string { return s.parentFrame }

// Len returns the number of Visuals currently in the scene.
func (s *Scene) Len() int { return len(s.state) }

// Labels returns the current labels, sorted lexicographically.
func (s *Scene) Labels() []string {
	out := make([]string, 0, len(s.state))
	for k := range s.state {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Get returns the live Visual for label, or nil.
func (s *Scene) Get(label string) Visual {
	e, ok := s.state[label]
	if !ok {
		return nil
	}
	return e.visual
}

// Contains reports whether label is currently in the scene.
func (s *Scene) Contains(label string) bool {
	_, ok := s.state[label]
	return ok
}

// Add inserts Visuals (or expanded Composites). Each constituent
// gets its own ADDED event in input order. Returns an error if any
// label collides with an existing entry — partial adds are rolled
// back so callers can retry without inspecting state.
func (s *Scene) Add(visuals ...interface{}) ([]SceneEvent, error) {
	flat, err := flattenVisuals(visuals)
	if err != nil {
		return nil, err
	}
	// Pre-check for duplicates so we don't half-add.
	for _, v := range flat {
		item := v.ToItem()
		if _, ok := s.state[item.Label]; ok {
			return nil, fmt.Errorf("duplicate label %q", item.Label)
		}
	}
	out := make([]SceneEvent, 0, len(flat))
	for _, v := range flat {
		item := v.ToItem()
		s.state[item.Label] = &sceneEntry{visual: v, committed: item}
		out = append(out, SceneEvent{
			Kind: EventAdded, Label: item.Label, Item: item,
		})
	}
	return out, nil
}

// Update diffs each Visual against its committed snapshot and
// returns UPDATED events for the changed ones. Visuals that haven't
// changed produce no event. Returns an error if any label isn't in
// the scene; partial updates are not applied in that case.
func (s *Scene) Update(visuals ...interface{}) ([]SceneEvent, error) {
	flat, err := flattenVisuals(visuals)
	if err != nil {
		return nil, err
	}
	// Pre-check membership.
	var missing []string
	for _, v := range flat {
		item := v.ToItem()
		if _, ok := s.state[item.Label]; !ok {
			missing = append(missing, item.Label)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("unknown label(s): %v", missing)
	}
	out := make([]SceneEvent, 0, len(flat))
	for _, v := range flat {
		newItem := v.ToItem()
		entry := s.state[newItem.Label]
		paths := diffPaths(entry.committed, newItem)
		if len(paths) == 0 {
			continue
		}
		entry.committed = newItem
		entry.visual = v
		out = append(out, SceneEvent{
			Kind: EventUpdated, Label: newItem.Label,
			Item: newItem, Paths: paths,
		})
	}
	return out, nil
}

// AddOrUpdate upserts — ADDs any Visuals not currently in the scene,
// UPDATEs any that exist (emitting an event only if something
// changed). Useful for tick loops that produce a fresh visual list
// each frame without tracking the lifecycle themselves.
func (s *Scene) AddOrUpdate(visuals ...interface{}) ([]SceneEvent, error) {
	flat, err := flattenVisuals(visuals)
	if err != nil {
		return nil, err
	}
	out := make([]SceneEvent, 0, len(flat))
	for _, v := range flat {
		item := v.ToItem()
		if _, exists := s.state[item.Label]; exists {
			events, err := s.Update(v)
			if err != nil {
				return nil, err
			}
			out = append(out, events...)
		} else {
			events, err := s.Add(v)
			if err != nil {
				return nil, err
			}
			out = append(out, events...)
		}
	}
	return out, nil
}

// Remove drops Visuals by label or by object reference. Composite
// objects expand and remove each constituent. Visuals not in the
// scene are skipped silently — the call is idempotent. Returns
// REMOVED events for the entries actually removed.
func (s *Scene) Remove(targets ...interface{}) []SceneEvent {
	labels := flattenLabels(targets)
	out := make([]SceneEvent, 0, len(labels))
	for _, lab := range labels {
		if _, ok := s.state[lab]; ok {
			delete(s.state, lab)
			out = append(out, SceneEvent{Kind: EventRemoved, Label: lab})
		}
	}
	return out
}

// Clear removes every Visual from the scene. Returns REMOVED events
// in label order for everything that was present.
func (s *Scene) Clear() []SceneEvent {
	labels := s.Labels()
	out := make([]SceneEvent, 0, len(labels))
	for _, lab := range labels {
		delete(s.state, lab)
		out = append(out, SceneEvent{Kind: EventRemoved, Label: lab})
	}
	return out
}

// ---- helpers -----------------------------------------------------------

// flattenVisuals expands any Composite into its constituent Visuals
// and asserts that every item is either Visual or Composite.
func flattenVisuals(in []interface{}) ([]Visual, error) {
	out := make([]Visual, 0, len(in))
	for _, x := range in {
		switch v := x.(type) {
		case Composite:
			out = append(out, v.ToVisuals()...)
		case Visual:
			out = append(out, v)
		default:
			return nil, fmt.Errorf(
				"expected Visual or Composite, got %T", x,
			)
		}
	}
	return out, nil
}

// flattenLabels resolves a mix of string / Visual / Composite into
// the underlying labels.
func flattenLabels(in []interface{}) []string {
	out := make([]string, 0, len(in))
	for _, x := range in {
		switch v := x.(type) {
		case string:
			out = append(out, v)
		case Composite:
			for _, vis := range v.ToVisuals() {
				out = append(out, vis.ToItem().Label)
			}
		case Visual:
			out = append(out, v.ToItem().Label)
		}
	}
	return out
}

// diffPaths returns the field-mask path list describing what
// changed between two wire-format items.
//
// Only emits paths the renderer honors on UPDATED events. The
// motion-tools renderer at
// useWorldState.svelte.ts::updateEntity matches just two prefixes:
//
//   - "poseInObserverFrame.pose*" — re-reads the pose, updates the
//     entity's Pose trait.
//   - "physicalObject*" — re-reads geometryType.value and dispatches
//     to traits.Box / Capsule / Sphere / mesh-BufferGeometry. There
//     is no pointcloud case; pcd updates would no-op.
//
// All "metadata.*" paths (color, colors, opacity, opacities,
// show_axes_helper, invisible) are dropped silently. Metadata
// changes only propagate at spawn time; to refresh metadata on the
// renderer, REMOVE + ADD the entity with a fresh UUID (lifecycle-
// style label rotation, or the versioned UUID strategy on the
// visualizer).
//
// See LESSONS.md::renderer-honors-only-pose-and-physicalobject-on-updated.
func diffPaths(old, new Item) []string {
	var paths []string

	// Pose: per-subfield diff. The renderer's check is
	// path.startsWith("poseInObserverFrame.pose") and re-reads the
	// full pose; emitting per-axis paths is informational.
	if old.Pose.X != new.Pose.X {
		paths = append(paths, "poseInObserverFrame.pose.x")
	}
	if old.Pose.Y != new.Pose.Y {
		paths = append(paths, "poseInObserverFrame.pose.y")
	}
	if old.Pose.Z != new.Pose.Z {
		paths = append(paths, "poseInObserverFrame.pose.z")
	}
	if old.Pose.Theta != new.Pose.Theta {
		paths = append(paths, "poseInObserverFrame.pose.theta")
	}

	// Geometry scalars the renderer rebuilds via physicalObject.*.
	if old.RadiusMM != new.RadiusMM {
		paths = append(paths, "physicalObject.geometryType.value.radiusMm")
	}
	if old.LengthMM != new.LengthMM {
		paths = append(paths, "physicalObject.geometryType.value.lengthMm")
	}

	// Box dims_mm: per-axis diff. The renderer reads the full Box
	// geometry on any physicalObject* path.
	if old.HasDims || new.HasDims {
		if old.DimsMM.X != new.DimsMM.X {
			paths = append(paths, "physicalObject.geometryType.value.dimsMm.x")
		}
		if old.DimsMM.Y != new.DimsMM.Y {
			paths = append(paths, "physicalObject.geometryType.value.dimsMm.y")
		}
		if old.DimsMM.Z != new.DimsMM.Z {
			paths = append(paths, "physicalObject.geometryType.value.dimsMm.z")
		}
	}

	// Mesh path swap: renderer re-parses the PLY and sets
	// traits.BufferGeometry.
	if old.MeshPath != new.MeshPath {
		paths = append(paths, "physicalObject.mesh")
	}

	// NOTE: pointcloud_path changes do not get an UPDATED path —
	// the renderer's updateEntity has no "pointcloud" case. Re-
	// spawn the entity (REMOVE + ADD with a fresh label) to update
	// a pcd. Color/opacity/show_axes_helper/invisible changes
	// likewise omitted — see file header.

	return paths
}

