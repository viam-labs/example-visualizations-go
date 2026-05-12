# CLAUDE.md — example-visualizations-go

Operational context for future agents working on this repo. This is the **Go port** of `viam-labs/example-visualizations` (Python). The two modules emit the same wire format and accept the same config schema; the differences are language-level only.

## What this is

- **GitHub:** `viam-labs/example-visualizations-go`
- **Registry:** `viam:example-visualizations-go`
- **Model:** `viam:example-visualizations-go:scene-primitives`
- **API:** `rdk:service:world_state_store`
- **Sibling:** [`viam-labs/example-visualizations`](https://github.com/viam-labs/example-visualizations) — the Python original, primary playground for finding renderer-side gotchas. Read its `LESSONS.md` before debugging anything wire-format-shaped.

## File layout

```
cmd/module/main.go      # module.ModularMain entrypoint
geometries.go           # Proto builders: build_box/sphere/capsule/point/arrow/mesh/pointcloud, build_metadata, build_pose, stlToPLY, PLY vertex-color extraction, PCD parse + chunking.
animation.go            # 11 modes: none, orbit, oscillate, spin, swing, pulse, trajectory, force_vector, breathe, flicker, lifecycle. ComputeTick returns (Pose, BaseGeom, []paths, *Overrides). camelCase field-mask path constants.
config.go               # Config + ItemConfig (JSON-parsed) + Item (runtime). Validate() runs schema checks.
presets.go              # 9 presets: primitives, orientation_vectors, frame_composition (with referenceFrameDemo + robotArm helpers), trajectory_preview, force_vector_demo, geometry_morph, lifecycle_demo, chunked_pcd_demo, all.
service.go              # sceneSprites — implements worldstatestore.Service. State map, per-subscriber channels, animation tick goroutine, UUID strategies, DoCommand dispatcher.
assets/                 # Shipped reference geometry — copied from the Python repo verbatim.
meta.json               # Module metadata. module_id + model must match service.go::Model.
VERSION                 # Single-line semver. Bump before `make upload` — registry rejects duplicates.
Makefile                # `make test`, `make`, `make module.tar.gz`, `make upload`.
*_test.go               # Go tests, run via `go test ./...` or `make test`.
```

## Architecture

### Lifecycle

1. `viam-server` exec's `bin/example-visualizations-go` → `module.ModularMain`.
2. On initial resource creation the framework calls `newSceneSprites(...)`, which builds the struct and calls `Reconfigure` explicitly. Same quirk as the Python module — Go's `module.ModularMain` doesn't auto-reconfigure, so we do it.
3. On every subsequent reconfigure the framework calls `Validate` then `Reconfigure`.
4. `Reconfigure` cancels any prior tick task, rebuilds the state map from `Items` (or the named `Preset`), broadcasts REMOVED for the prior world and ADDED for the new world to existing subscribers, then restarts the tick task if any items animate.
5. `Close` cancels the tick task on shutdown.

### State

`s.state` is `map[label]*itemState`. Each `itemState` holds the original `Item`, `basePose`, `baseGeom`, `uuid` (matches label under stable strategy, has a timestamp+counter suffix under versioned), the cached `*commonpb.Transform`, a `visibleToViewer` flag (flips during flicker/lifecycle), and an optional `chunkedState` for chunked pointclouds.

### Animation tick

`tickLoop` runs at `tickHz` Hz (default 30, max 30). Each tick calls `tickOnce`, which iterates animated items and dispatches based on `uuidStrategy`:

- **`stable`** — recompute pose/geom, update the cached transform in place, push UPDATED with `UpdatedFields=paths`. Paths MUST be camelCase (see gotchas).
- **`versioned`** — allocate a new UUID, build a fresh transform, push REMOVED for the old + ADDED for the new.

Animations that mutate scene-graph membership (`flicker`, `lifecycle`) return `Overrides.InScene`. The tick emits REMOVED/ADDED on transition edges (with UUID rotation on the rising edge to dodge the renderer cache) and falls through to UPDATED for non-membership overrides (color/opacity) while the entity stays visible.

If no items animate, the tick task isn't started.

### Subscriber fanout

Each subscriber owns a `chan worldstatestore.TransformChange` with `cap=256`. Initial-burst on join is a `select { case ch <- ADDED }` per current visible item (drops with warn if the queue is full). `broadcastLocked` does non-blocking sends; full queues drop the event with a warn.

### DoCommand surface

Seven verbs implemented: `list`, `remove`, `clear`, `preset`, `snapshot`, `set_uuid_strategy`, `get_entity_chunk`. (Python has two more — `add`, `update` — not yet ported.) Unrecognized / missing `command` returns a debug snapshot.

## Conventions and gotchas

Every load-bearing finding from the Python module's `LESSONS.md` applies:

- **Mesh / PCD file coordinates are in METERS, not millimeters.** RDK readers multiply by 1000 to convert.
- **The viewer renders only PLY meshes.** STL is converted on the wire via `stlToPLY`.
- **PCD header must match `pointcloud.ToPCD` byte-for-byte.** Leading `#` comments or `VERSION 0.7` (vs `VERSION .7`) silently fail.
- **Transform.metadata uses the `viamrobotics/visualization` schema.** All five required keys must be present.
- **Field-mask paths MUST be camelCase, not snake_case.** The renderer ignores snake_case paths silently. See `animation.go::Path*` constants.
- **Chunked delivery for point clouds is experimental, schema unverified.** Implemented for parity with Python; viewer behavior on `metadata.chunks` and `get_entity_chunk` DoCommand isn't confirmed.
- **Renderer caches REMOVED UUIDs.** Lifecycle / flicker / respawn-style animations rotate UUIDs on every re-add.

## Tests

`make test` runs `go test ./...`. Coverage today is focused on the format / math layer:

- Animation modes return the correct field-mask paths at pinned t values (`animation_test.go`).
- Metadata struct emits all five required keys + base64-correct packing (`geometries_test.go`).
- Geometry builders produce the expected proto shape.
- PCD parsing + chunking is byte-aware.

What's NOT covered yet (room to grow):
- Asset units (the Python module's `test_assets_units.py` is the canonical reference).
- Config validation edge cases.
- Service-level streaming + tick.
- Every preset's content.

## Releasing

1. Bump `VERSION` (registry rejects duplicates).
2. Commit.
3. `make upload` (runs `make module.tar.gz` + `viam module upload --version=$(cat VERSION)`).
4. Push.

## Don't

- **Don't change `animation.go::Path*` to snake_case** until the viz team confirms the renderer accepts snake_case. The 0.0.32 Python version broke every animation by trying this; we reverted in 0.0.33.
- **Don't shadow `viamkit/viz`'s primitives.** The two have different APIs and slightly different defaults; rolling our own keeps this module independent.
- **Don't add `viamkit` as a dependency.** Self-contained build matters here — the registry consumer should be a single binary with no surprise deps.
