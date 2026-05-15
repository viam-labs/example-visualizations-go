# CLAUDE.md — example-visualizations-go

Operational context for future agents working on this repo. This is the **Go port** of `viam-labs/example-visualizations-python` (Python). The two modules emit the same wire format, accept the same config schema, and ship the same three-model architecture; the differences are language-level only.

## What this is

A Viam module that ships **three models** demonstrating different patterns for driving the Viam 3D scene viewer:

- **`viam:example-visualizations-go:standalone-playground`** (`rdk:service:world_state_store`) — the monolith. Owns the WSS contract, scene state, animation tick, and the full DoCommand surface. Configurable presets exercise every supported primitive and animation. **This is the renderer-behavior probe.**
- **`viam:example-visualizations-go:playground-visualizer`** (`rdk:service:world_state_store`) — passive WSS. Holds state and serves the renderer, rejects items/preset config. Receives mutations at runtime via the `apply_events` DoCommand.
- **`viam:example-visualizations-go:playground-driver`** (`rdk:component:generic`) — Generic component. Owns a `visuals.Scene`, ticks via goroutine, pushes mutations to its paired visualizer. Domain logic lives in *recipes* in `recipes.go`.

- **GitHub:** `viam-labs/example-visualizations-go`
- **Registry:** `viam:example-visualizations-go`
- **Sibling:** [`viam-labs/example-visualizations-python`](https://github.com/viam-labs/example-visualizations-python) — the Python original, primary playground for finding renderer-side gotchas. Read its `LESSONS.md` before debugging anything wire-format-shaped.

## File layout

```
service.go              # standalone-playground: sceneSprites — thin wrapper over visuals.SceneServiceBase. Plugs in the module's MODEL, hooks delegating to geometries.go/animation.go/presets.go, and the playground-specific get_entity_chunk verb.
visualizer.go           # playground-visualizer: playgroundVisualizer (embeds sceneSprites). Rejects items/preset config; registers itself in visuals.Registry on Reconfigure.
driver.go               # playground-driver: playgroundDriver — Generic component. Looks up visualizer via visuals.Lookup, owns visuals.Scene, ticks via goroutine, pushes apply_events.
recipes.go              # Recipe interface + MarchingBoxes + PulsingSpheres + Recipes registry map.
geometries.go           # Proto builders: build_box/sphere/capsule/point/arrow/mesh/pointcloud, build_metadata, build_pose, stlToPLY, PLY vertex-color extraction, PCD parse + chunking.
animation.go            # 11 modes: none, orbit, oscillate, spin, swing, pulse, trajectory, force_vector, breathe, flicker, lifecycle. ComputeTick returns TickResult{Pose, Geom, Paths, Overrides}. camelCase field-mask path constants.
config.go               # Config + ItemConfig (JSON-parsed) + Item (runtime). Validate() runs schema checks.
presets.go              # 9 presets used by standalone-playground.
aliases.go              # Unqualified re-exports of visuals.* types so the long-lived module files (presets.go, animation.go, etc.) compile without visuals. prefixes. Transitional — slated for removal in a dedicated pass.
cmd/module/main.go      # module.ModularMain entrypoint — registers all three models.

visuals/                # The typed visualization library (planned ViamVizHelpers Go).
visuals/pose.go         # Pose struct + Pose helpers + fillPose.
visuals/color.go        # Color + BoxDims types.
visuals/shapes.go       # Visual interface + Box/Sphere/Capsule/Point/Arrow/Mesh/PointCloud structs + ToItem method.
visuals/animations.go   # AnimationSpec interface + concrete specs + Animation runtime struct + Path* field-mask constants.
visuals/composites.go   # Composite interface + CoordinateFrame/Line/BoundingBox + ArrowFromTo.
visuals/scene.go        # Scene + SceneEvent + diff logic.
visuals/wire.go         # ItemFromMap (wire-format dict → Item) + EventsToWire (SceneEvent → wire). Mirrors viam_visuals.events_to_wire.
visuals/service.go      # SceneServiceBase — inheritable WSS service. Owns state, subscribers, tick loop, DoCommand dispatch including applyEvents.
visuals/registry.go     # In-process resource registry (Register/Lookup/Unregister/RegisteredNames).
visuals/uuid_strategy.go      # InitialUUID / VersionedUUID / ValidStrategies.
visuals/item.go               # The denormalized Item value type (the wire-format struct).
visuals/mesh_io.go            # STL→PLY conversion + PLY vertex-color extraction.
visuals/pcd_io.go             # PCD parser + chunked-delivery splitter.
visuals/metadata.go           # Metadata struct builder (the visualization library schema).
visuals/internal/             # Pure-data constants used by mesh / pcd helpers.

assets/                 # Shipped reference geometry — copied from the Python repo verbatim.
meta.json               # Module metadata. Lists all three models.
VERSION                 # Single-line semver. Bump before `make upload` — registry rejects duplicates.
Makefile                # `make test`, `make`, `make module.tar.gz`, `make upload`. The binary target depends on Makefile, go.mod, *.go, cmd/module/*.go, visuals/*.go — if you add a new package directory, audit this list.
*_test.go               # Go tests, run via `go test ./...` or `make test`.
```

## Architecture

### Three-model split

The split exists to demonstrate the `visuals` library's architecture:

- **standalone-playground** is the "everything in one place" example. The WSS service owns presets, animation tick, item lifecycle. Use this when scene content is static or driven entirely by config.
- **playground-visualizer** + **playground-driver** is the "domain logic owns the scene" example. The visualizer serves the renderer; the driver runs domain code (a recipe) at tick rate and pushes mutations. Use this pattern in real modules where scene content comes from running code.

Both patterns embed `visuals.SceneServiceBase`. The library does all the WSS plumbing — state, subscribers, broadcast, the standard DoCommand verbs, the animation tick loop. Module authors implement the `SceneHooks` interface (geometry building, asset reading, animation tick math, preset lookup).

### Lifecycle

**Standalone / visualizer (services):**

1. `viam-server` exec's `bin/example-visualizations-go` → `module.ModularMain`.
2. On initial resource creation the framework calls the model's constructor (e.g., `newSceneSprites`). The constructor builds the struct and calls `Reconfigure` explicitly. Same quirk as the Python module — neither the Python nor Go `EasyResource` / `module.ModularMain` auto-reconfigure on construction.
3. On every subsequent reconfigure the framework calls `Validate` then `Reconfigure`.
4. `Reconfigure` cancels any prior tick task, rebuilds the state map (from `Items` / `Preset` for standalone, empty for visualizer), broadcasts REMOVED for the prior world and ADDED for the new world to existing subscribers, then restarts the tick task if any items animate.
5. `Close` cancels the tick task on shutdown. The visualizer's Close also unregisters from `visuals.Registry`.

**Driver (Generic component):**

1. Framework calls `newPlaygroundDriver(...)`. The constructor builds the struct and calls `Reconfigure` explicitly (`module.ModularMain` doesn't auto-call it).
2. `Reconfigure` parses config, looks up the visualizer via `visuals.Lookup(cfg.Visualizer)`, builds a fresh `visuals.Scene` from the recipe's `Initial(scene)`, then starts a goroutine that pushes the initial events and enters the tick loop. The lookup short-circuits the framework's gRPC stub — the driver holds a direct Go reference to the visualizer instance.
3. The tick loop calls `recipe.Tick(scene, t)` every `1/tick_hz` seconds and pushes the returned events as a batched `apply_events` DoCommand to the visualizer.
4. `Close` cancels the tick goroutine, waits for it to exit, and best-effort clears the driver's labels from the visualizer (via REMOVED events).

### State

`SceneServiceBase.state` is `map[label]*ItemState`. Each `ItemState` holds the original `Item`, `BasePose`, `BaseGeom`, `UUID` (matches label under stable strategy, has a timestamp+counter suffix under versioned), the cached `*commonpb.Transform`, a `VisibleToViewer` flag (flips during flicker/lifecycle), and an optional `ChunkedState` for chunked pointclouds.

`visuals.Scene` (in `visuals/scene.go`) holds a separate `map[label]*sceneEntry`. `sceneEntry` carries the live `Visual` (interface, typically a pointer like `*Box`) plus the wire-format `Item` snapshot from the last `Add`/`Update`. `scene.Update(visual)` diffs `visual.ToItem()` against the snapshot to compute field-mask paths.

### Animation tick (standalone-playground)

`tickLoop` runs at `tickHz` Hz (default 30, max 30). Each tick calls `tickOnce`, which iterates animated items and dispatches based on `uuidStrategy`:

- **`stable`** — recompute pose/geom, update the cached transform in place, push UPDATED with `UpdatedFields=paths`. Paths MUST be camelCase (see gotchas).
- **`versioned`** — allocate a new UUID, build a fresh transform, push REMOVED for the old + ADDED for the new.

Animations that mutate scene-graph membership (`flicker`, `lifecycle`) return `Overrides.InScene`. The tick emits REMOVED/ADDED on transition edges (with UUID rotation on the rising edge to dodge the renderer cache) and falls through to UPDATED for non-membership overrides (color/opacity) while the entity stays visible.

If no items animate, the tick task isn't started.

### Driver tick

The driver's tick lives client-side in `driver.go::tickLoop` — a goroutine fired from `Reconfigure`. The driver doesn't go through `SceneServiceBase.tickLoop`; it owns its own ticker:

```go
ticker := time.NewTicker(period)
for {
    select {
    case <-ctx.Done(): return
    case <-ticker.C:
        t := time.Since(d.t0).Seconds()
        events := d.recipe.Tick(d.scene, t)
        d.sendEvents(ctx, events)
    }
}
```

`sendEvents` calls `visualizer.DoCommand(ctx, {"command":"apply_events", "events": eventsAny, "namespace": d.namespace})`. The visualizer's `applyEvents` handler updates its state map and broadcasts UPDATEDs (or ADDED/REMOVEDs) to subscribers.

### Subscriber fanout

Each subscriber owns a `chan worldstatestore.TransformChange` with `cap=256`. Initial-burst on join is a `select { case ch <- ADDED }` per current visible item (drops with warn if the queue is full). `broadcastLocked` does non-blocking sends; full queues drop the event with a warn.

### DoCommand surface

`SceneServiceBase.DoCommand` dispatches the standard verbs inherited by both standalone and visualizer:

| verb | semantics |
| --- | --- |
| `list` | one summary per item |
| `remove` | drop one label |
| `clear` | drop all items |
| `preset` | hard reset to a named preset (visualizer's LoadPreset returns an error) |
| `snapshot` | dump current state as pasteable config |
| `set_uuid_strategy` | flip stable / versioned at runtime |
| `apply_events` | batched ADDED/UPDATED/REMOVED matching SceneEvent wire shape; the driver→visualizer transport |
| `get_entity_chunk` | playground-specific custom verb (chunked PCD delivery) |

Unrecognized / missing `command` falls through to `HandleCustomCommand` (a hook on the SceneHooks interface) and then to a debug snapshot. The debug snapshot includes diagnostic counters (`broadcasts_total`, `updates_total`, `last_broadcast`) — useful for confirming an apply_events round-trip without grepping logs.

> The Python module also implements `add` and `update` DoCommand verbs. The Go port doesn't (yet) — the driver pattern is the recommended way to push runtime mutations. If you need ad-hoc add/update verbs in Go, port the bodies from `viam_visuals/service.py::do_command`.

### In-process registry

`visuals.Register(name, instance)` stashes a resource in a module-local map keyed by name. The visualizer calls it in `Reconfigure`; the driver calls `visuals.Lookup(cfg.Visualizer)` at construction.

When both models live in the same module process (always, since both ship from one binary), the driver holds a direct Go reference (`worldstatestore.Service` interface, concrete type `*playgroundVisualizer`) and calls its `DoCommand` as a normal method. No serialization, no socket round-trip. Confirmed at runtime via the driver's `info` DoCommand, which reports `visualizer_type: "*exampleviz.playgroundVisualizer"` on success.

## Conventions and gotchas

Every load-bearing finding from the Python module's `LESSONS.md` applies. The Go-specific ones (also in the Python LESSONS.md but worth highlighting here):

- **Mesh / PCD file coordinates are in METERS, not millimeters.** RDK readers multiply by 1000 to convert.
- **The viewer renders only PLY meshes.** STL is converted on the wire via `stlToPLY`.
- **PCD header must match `pointcloud.ToPCD` byte-for-byte.** Leading `#` comments or `VERSION 0.7` (vs `VERSION .7`) silently fail.
- **Transform.metadata uses the `viamrobotics/visualization` schema.** All five required keys (`colors`, `color_format`, `opacities`, `show_axes_helper`, `invisible`) must be present.
- **Field-mask paths MUST be camelCase, not snake_case.** The renderer ignores snake_case paths silently. See `visuals/animations.go::Path*` constants.
- **Chunked delivery for point clouds is experimental, schema unverified.** Implemented for parity with Python; viewer behavior on `metadata.chunks` and `get_entity_chunk` DoCommand isn't confirmed.
- **Renderer caches REMOVED UUIDs.** Lifecycle / flicker / respawn-style animations rotate UUIDs on every re-add.
- **`module.ModularMain` doesn't auto-reconfigure on initial construction.** Constructors must call `Reconfigure` explicitly. Same trap as Python's `EasyResource.new`.
- **Go in-process DoCommand preserves concrete slice types.** When the driver calls `visualizer.DoCommand(...)` in-process, `[]string` stays `[]string` and `[]map[string]any` stays itself. Over gRPC, structpb erases both to `[]any`. The visualizer's `applyEvents` handler must accept both shapes; that's what `coerceStringSlice` / `coerceEventsSlice` in `visuals/service.go` are for. A type assertion `evt["paths"].([]any)` would silently fail in-process, drop the field-mask, and produce UPDATEDs with no `UpdatedFields` — which the renderer treats as a no-op.
- **The Makefile binary target must list every package directory.** Originally `Makefile::$(MODULE_BINARY)` had deps `Makefile go.mod *.go cmd/module/*.go` — it missed `visuals/*.go`. Changes inside the library package didn't trigger rebuilds, so `make module.tar.gz` repeatedly shipped stale binaries under new version numbers. If you add a new package directory, audit this line.

## Tests

`make test` runs `go test ./...`. Coverage focuses on the format/math layer and the new architecture pieces:

- Animation modes return the correct field-mask paths at pinned t values (`animation_test.go`).
- Metadata struct emits all five required keys + base64-correct packing (`geometries_test.go`).
- Geometry builders produce the expected proto shape.
- PCD parsing + chunking is byte-aware.
- `visuals.Scene` round-trips (`visuals/scene_test.go`): add/update/remove/clear/add_or_update + composite expansion + namespace handling.
- `visuals.Register / Lookup` registry semantics (`visuals/registry_test.go`).
- `applyEvents` ADDED/UPDATED/REMOVED handling + namespace prefix + per-event error capture + the regression test for the typed `[]string` paths shape (`visuals/apply_events_test.go`).
- End-to-end driver+visualizer pipeline including the in-process registry resolution and tick-driven updates (`driver_visualizer_test.go`).

What's NOT covered yet (room to grow):
- Asset units (the Python module's `test_assets_units.py` is the canonical reference).
- Every preset's content verification.
- Standalone-playground full streaming + tick coverage at the service level.

## Releasing

1. Bump `VERSION` (registry rejects duplicates).
2. Commit.
3. `make upload` (runs `make module.tar.gz` + `viam module upload --version=$(cat VERSION)`).
4. Push.

**Verify the binary actually rebuilt.** Run `md5sum bin/example-visualizations-go ~/.viam/packages/data/module/*example-visualizations-go*$(cat VERSION)*/bin/example-visualizations-go` after a deploy — if the hashes match the prior version's, the Makefile dep list missed a directory and you've shipped a stale binary under a new tag.

## Don't

- **Don't change `visuals/animations.go::Path*` to snake_case** until the viz team confirms the renderer accepts snake_case. The 0.0.32 Python version broke every animation by trying this; we reverted in 0.0.33.
- **Don't shadow `viamkit/viz`'s primitives.** The two have different APIs and slightly different defaults; rolling our own keeps this module independent.
- **Don't add `viamkit` as a dependency.** Self-contained build matters here — the registry consumer should be a single binary with no surprise deps.
- **Don't deploy without auditing the Makefile dep list** if you've added new package directories.
- **Don't bypass the in-process registry** by passing a stub through `Dependencies`. The framework would give you a gRPC client even when both models live in the same process; the registry trick is what makes the driver→visualizer hot path cheap.

## Releasing notes

Current pre-release version sequence (latest first):

- 0.0.14 — Makefile dep list now includes `visuals/*.go`, fixing the stale-binary bug. Diagnostic counters land too (broadcasts_total, updates_total, last_broadcast).
- 0.0.13 — diagnostic counters added to the debug snapshot (would have shipped sooner, see Makefile note).
- 0.0.12 — apply_events paths coercion fix (handles []string in-process vs []any gRPC).
- 0.0.11 — three-model architecture (playground-visualizer + playground-driver + recipes + apply_events + visuals/wire.go).
- 0.0.10 — standalone-playground rename + Scene library + in-process registry.
- 0.0.9 — SceneServiceBase migration: module's service.go shrunk 898 → 281 LOC; library grew visuals/service.go at 839 LOC.
- 0.0.8 — composites + visual verification on dell-2.
- 0.0.1..0.0.7 — Go port bootstrap: every geometry primitive, every animation mode, every preset matching the Python module.
