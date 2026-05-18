# example-visualizations-go

Go port of [`viam:example-visualizations-python`](https://github.com/viam-labs/example-visualizations-python) — a Viam module that adds every supported geometry primitive (box, sphere, capsule, point, mesh PLY/STL, point cloud PCD) to the Viam 3D scene viewer so you can poke each one and see what its config knobs do.

The module ships **three models**, each demonstrating a different way to build a scene against the world-state-store service:

| Model | API | What it does |
| --- | --- | --- |
| `viam:example-visualizations-go:standalone-playground` | `rdk:service:world_state_store` | The monolith. Owns the WSS contract, the scene, the animation tick, and the runtime DoCommand surface. Drop-in service backed by configurable presets. |
| `viam:example-visualizations-go:playground-visualizer` | `rdk:service:world_state_store` | Passive WSS. Holds state and serves the renderer, but doesn't decide what to draw. Items arrive at runtime via the `apply_events` DoCommand from a paired driver. |
| `viam:example-visualizations-go:playground-driver` | `rdk:component:generic` | Generic component that owns a `visuals.Scene`, ticks at config'd Hz, and pushes scene mutations to its visualizer. Domain logic lives in *recipes* — small Go types that seed and animate the scene. |

The Go port mirrors the Python module 1:1 — same models, same wire format, same gotchas. Use whichever language matches your stack.

> See the Python version's [LESSONS.md](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md) for the full set of findings about the viewer's wire format. Everything documented there applies here verbatim — Go and Python differ only in implementation language, not in what the renderer accepts.

## Why two language ports?

The Python module is the playground: fast to iterate, easy to read, hits every renderer-side gotcha and writes them down. The Go module is the reference implementation: same scene, same animations, same DoCommand surface, but built on `go.viam.com/rdk` so it can ship into Go-shop pipelines, share types with `viamrobotics/visualization`, and serve as the seed for the eventual Go library when the Python side stabilizes.

If you're choosing one to start from:
- **Most authors should pick Python** unless their build pipeline is Go-only. The Python version evolves faster.
- **Go-shop workcells** that want zero-Python deployment can use this version. Identical scene content.

## Which model do I want?

- **`standalone-playground`** — you want every supported primitive and animation in one configurable service, with runtime DoCommand surface. The renderer-behavior probe.
- **`playground-driver` + `playground-visualizer`** — you want to write Go *code* that drives the scene (a detector publishing bounding boxes, a planner publishing trajectories, anything that ticks). The driver mutates a `visuals.Scene` of typed `Visual` pointers; the visualizer republishes those to the renderer. Both ship from one module binary and share a process, so events exchange via direct Go method calls — no gRPC overhead on the hot path.

You can run all three side-by-side; they don't interfere.

## Standalone-playground quickstart

Add the service to a machine, no config attributes needed:

```jsonc
{
  "modules": [
    {
      "type": "registry",
      "name": "viam_example-visualizations-go",
      "module_id": "viam:example-visualizations-go",
      "version": "0.0.14"
    }
  ],
  "services": [
    {
      "name": "scene",
      "namespace": "rdk",
      "type": "world_state_store",
      "model": "viam:example-visualizations-go:standalone-playground",
      "attributes": {}
    }
  ]
}
```

Open the machine's **3D scene** tab. With no `preset` attribute set, the default loads `all` — every preset stacked along Y so you see the full tour in one viewport. To see just the 12-item primitives row, set `"preset": "primitives"` in the service attributes.

`preset` can be set to one of: `all` (default), `primitives`, `orientation_vectors`, `frame_composition`, `trajectory_preview`, `force_vector_demo`, `geometry_morph`, `lifecycle_demo`, `chunked_pcd_demo`.

See the [Python README](https://github.com/viam-labs/example-visualizations-python/blob/main/README.md#standalone-playground-quickstart) for what each preset contains — the Go version produces the same scene content.

## Driver + visualizer quickstart

The driver/visualizer pair is the architecture for modules whose scene content comes from *running code*, not from a static config.

```jsonc
{
  "services": [
    {
      "name": "scene_visualizer",
      "namespace": "rdk",
      "type": "world_state_store",
      "model": "viam:example-visualizations-go:playground-visualizer",
      "attributes": {}
    }
  ],
  "components": [
    {
      "name": "scene_driver",
      "namespace": "rdk",
      "type": "generic",
      "model": "viam:example-visualizations-go:playground-driver",
      "attributes": {
        "visualizer": "scene_visualizer",
        "recipe": "marching_boxes",
        "tick_hz": 5
      },
      "depends_on": ["scene_visualizer"]
    }
  ]
}
```

The driver looks up its visualizer at construction time via the in-process registry, so mutations travel as direct Go method calls — no gRPC hop between them even though they're separate Viam resources.

Available recipes (in `recipes.go`):

- `marching_boxes` — five boxes in a row along X, each bobbing in Y on a phase-offset sine wave. Simplest end-to-end recipe; useful for confirming the pipeline works.
- `pulsing_spheres` — three spheres pulsing their radius on phase-offset sine waves. Exercises the `physicalObject.geometryType.value.radiusMm` field-mask path, complementary to the pose-only `marching_boxes`.
- `all_primitives` — one of every supported shape (box, sphere, capsule, point, arrow, mesh, pointcloud) in a row, static. Driver-side equivalent of the standalone-playground `primitives` preset. The mesh and pointcloud items reference assets in the module's installed directory — the visualizer resolves them at install time.
- `detections_overlay` — four translucent bounding boxes drifting on circular paths. The canonical driver-shaped use case: a perception module producing detections per tick. Demonstrates `scene.AddOrUpdate(composite)` — the first tick fires ADDED, subsequent ticks fire UPDATED with pose paths.
- `coordinate_frames_arm` — three spinning coordinate-frame triads + an articulated 5-link arm. Demonstrates composite expansion (`CoordinateFrame` → anchor sphere + 3 axis capsules) and chained `ParentFrame` propagation (each arm link parents to the prior link's label). Joint angles are driver-computed.
- `trajectory_runner` — a "runner" sphere walking through 5 waypoints with linear interpolation, plus the static path drawn as a `Line` composite (capsule chain). Mirrors the standalone-playground's `trajectory_preview` preset.
- `lifecycle_garden` — 5 plots cycling through appear → alive → disappear → gone phases at staggered offsets. Demonstrates scene-graph mutation from the driver: each cycle calls `scene.Add` with a fresh version label (so the renderer's REMOVED-UUID cache doesn't drop the re-add), `scene.Update` for color/opacity transitions, and `scene.Remove` during the gone phase.
- `force_vector` — animated force-vector arrow. Length and radius oscillate on phase-offset sine waves; orientation precesses around world +Z at a fixed 45° tilt. Mirrors the standalone-playground's `force_vector_demo`. (Go struct is `ForceVectorRecipe` — the unqualified `ForceVector` is taken by the visuals AnimationSpec alias.)
- `breathing_shapes` — N spheres whose opacity smoothly cycles via the **label-rotation pattern** — the only working pattern for live opacity changes given the renderer's UPDATED handler ignores `metadata.*` paths. Each opacity step is a fresh label; the previous version is REMOVED, the new one is ADDED.
- `all` — every recipe above, run simultaneously, stacked along Y. Each recipe struct has a `YOrigin` field; the `all` recipe instantiates each one at a distinct Y offset. Driver-side equivalent of the standalone-playground's `all` preset.

Driver attributes:

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `visualizer` | string | **required** | Resource name of the paired visualizer service. The driver looks this up in the in-process registry. |
| `recipe` | string | `"marching_boxes"` | Recipe name. See `recipes.go::Recipes` for the registry. |
| `tick_hz` | number (0, 30] | `5` | Driver tick rate. Each tick calls `recipe.Tick(scene, t)`. |
| `namespace` | string | `""` | Optional label prefix. Two drivers can push to one visualizer if they use different namespaces. |

Driver DoCommand verbs:

| `command` | Payload | Returns |
| --- | --- | --- |
| `info` | `{}` | `{visualizer, recipe, tick_hz, namespace, scene_size, visualizer_type, tick_running}` — `visualizer_type` is the concrete Go type, useful for confirming the in-process registry path resolved (`*exampleviz.playgroundVisualizer` on success). |
| `recipes` | `{}` | `{recipes: [...]}` — names available in `recipes.go::Recipes`. |

## Writing your own recipe

A recipe implements the `Recipe` interface:

```go
type Recipe interface {
    Name() string
    Initial(scene *visuals.Scene) []visuals.SceneEvent
    Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent
}
```

Example: a single box that bobs in Y.

```go
type BobbingBox struct{}

func (BobbingBox) Name() string { return "bobbing_box" }

func (BobbingBox) Initial(scene *visuals.Scene) []visuals.SceneEvent {
    box := &visuals.Box{
        Label: "bob",
        Pose:  visuals.Pose{Z: 100},
        DimsMM: visuals.BoxDims{X: 100, Y: 100, Z: 100},
    }
    events, _ := scene.Add(box)
    return events
}

func (BobbingBox) Tick(scene *visuals.Scene, t float64) []visuals.SceneEvent {
    v := scene.Get("bob")
    box, _ := v.(*visuals.Box)
    box.Pose = visuals.Pose{Y: 100 * math.Sin(2*math.Pi*t/3), Z: 100}
    events, _ := scene.Update(box)
    return events
}
```

Register it in `recipes.go::Recipes` and the driver picks it up by name. `Scene` snapshots each visual's wire-format `Item` at `Add` time and diffs against the post-mutation `Item` on `Update`, so the returned `SceneEvent`s carry exactly the field-mask paths that changed — no manual path bookkeeping.

## visuals library

The recipe pattern is built on a small typed library co-located in this repo at `visuals/`. It's the Go side of the planned **ViamVizHelpers** library; the Python sibling lives at [example-visualizations-python/viam_visuals](https://github.com/viam-labs/example-visualizations-python/tree/main/viam_visuals). The public surface today:

- **Shapes** — `Box`, `Sphere`, `Capsule`, `Point`, `Arrow`, `Mesh`, `PointCloud`. Construction-time validation via `ToItem()` (called by `Scene` and `ToItems`). The `Pose` struct carries position + orientation vector + theta.
- **Animations** — typed specs `Spin`, `Swing`, `Oscillate`, `Orbit`, `Pulse`, `Breathe`, `Flicker`, `Lifecycle`, `ForceVector`, `Trajectory`. Assigned to a `Visual`'s `Animation` field.
- **Composites** — `CoordinateFrame`, `Line`, `BoundingBox`, plus `ArrowFromTo(start, end, ...)`. Each implements `Composite` and expands into a list of typed `Visual` instances via `ToVisuals()`; `scene.Add(composite)` flattens automatically.
- **`Scene`** — typed state container with object-based mutation. `scene.Add(visual)`, `box.Pose = newPose; scene.Update(box)`, `scene.AddOrUpdate(...)`, `scene.Remove(visual_or_label)`. Returns `SceneEvent` records that the driver serializes via `visuals.EventsToWire(events)` for the `apply_events` wire format. Callers pass pointer `Visual`s (e.g. `&Box{...}`) so mutations are visible through the interface.
- **`SceneServiceBase`** — the inheritable WSS service. Owns state, subscribers, broadcast, the standard DoCommand verbs (`list`, `remove`, `clear`, `preset`, `snapshot`, `set_uuid_strategy`, `get_entity_chunk`, `apply_events`), and the animation tick goroutine. Subclasses implement the `SceneHooks` interface (geometry building, asset reading, animation tick, preset lookup). Both `standalone-playground` and `playground-visualizer` embed it.
- **`Register` / `Lookup` / `Unregister`** — in-process resource registry. Lets a downstream resource hold a direct Go reference to an upstream resource that lives in the same module process, skipping the framework's gRPC stub. The visualizer registers itself in `Reconfigure`; the driver looks it up at construction.
- **Wire helpers** — `ItemFromMap(map[string]any) (Item, error)` parses an `apply_events` payload item dict; `EventsToWire([]SceneEvent) []map[string]any` serializes for the wire.

`visuals` is the stable surface. The eventual extraction to a standalone module will not change the public API.

## Config / DoCommand reference

The config schema and DoCommand surface match the Python version 1:1. See [`example-visualizations-python/README.md`](https://github.com/viam-labs/example-visualizations-python/blob/main/README.md#config-reference-standalone-playground) for the full reference — the JSON wire format is identical.

Animation modes available: `none`, `orbit`, `oscillate`, `spin`, `swing`, `pulse`, `trajectory`, `force_vector`, `breathe`, `flicker`, `lifecycle`.

DoCommand verbs implemented on the Go `SceneServiceBase`: `list`, `remove`, `clear`, `preset`, `snapshot`, `set_uuid_strategy`, `get_entity_chunk`, `apply_events`.

> The Python version also has `add` and `update` DoCommand verbs. Those aren't ported to Go (yet) because the recommended pattern for runtime mutations is the driver→visualizer pair: write a recipe and let it push events. `add` / `update` are straightforward additions if needed — port the bodies of those branches from the Python `viam_visuals/service.py::do_command`.

## In-process registry — what's actually going on

The driver and visualizer both ship from the same module binary. When viam-server creates instances of each, both live in the same OS process. By default Viam would still hand the driver a gRPC client stub for the visualizer (since the framework can't assume same-process locality), meaning every `apply_events` call would round-trip through structpb serialization + a local socket.

`visuals.Register(name, instance)` stashes the visualizer in a module-local map keyed by resource name. The driver's `Reconfigure` calls `visuals.Lookup(cfg.Visualizer)` and casts the result to `worldstatestore.Service`. If found, the driver holds a direct Go reference and calls `visualizer.DoCommand(...)` as a normal method — no serialization, no gRPC. Confirmed at runtime via the driver's `info` DoCommand, which reports `visualizer_type` as `*exampleviz.playgroundVisualizer` on success.

This is also the source of a Go-specific gotcha: in-process method calls preserve concrete Go slice element types, while gRPC erases them to `[]any` via structpb. The visualizer's `apply_events` handler must accept both shapes — `[]string` for the paths list and `[]map[string]any` for the events list when called in-process, `[]any` of either when called over gRPC. See `visuals/service.go::coerceStringSlice` / `coerceEventsSlice`.

## Development

```sh
go mod tidy            # one-time dep resolve
make test              # go test ./...
make                   # build the binary into bin/example-visualizations-go
make module.tar.gz     # build the registry tarball
make upload            # build + upload at $(cat VERSION)
```

The binary lives at `bin/example-visualizations-go`. `meta.json::entrypoint` points there so the tarball ships a self-contained binary.

> **Watch the Makefile dep list.** The binary target's prerequisite list must include every `.go` directory in the repo. Missing one means edits in that package don't trigger a rebuild and `make module.tar.gz` ships the stale prior binary under a new version number. If you add a new package, audit `Makefile::$(MODULE_BINARY)`. See [LESSONS.md::go-makefile-package-deps](../example-visualizations-python/LESSONS.md) (Python repo).

## File layout

```
service.go              # standalone-playground: sceneSprites — thin wrapper over visuals.SceneServiceBase. Plugs in the module's MODEL, hooks, and the get_entity_chunk verb.
visualizer.go           # playground-visualizer: passive WSS. Wraps sceneSprites; rejects items/preset config; registers itself in visuals.Registry.
driver.go               # playground-driver: Generic component. Looks up visualizer via visuals.Lookup, owns Scene, ticks via goroutine, pushes apply_events.
recipes.go              # Recipe interface + MarchingBoxes + PulsingSpheres + Recipes registry map.
geometries.go           # Proto builders (Box, Sphere, Capsule, Point, Arrow, Mesh, PointCloud), metadata struct, PCD parsing + chunking, STL→PLY conversion, PLY vertex color extraction.
animation.go            # Pose/geom math for all 11 modes. Field-mask path constants (camelCase — see gotchas).
config.go               # Config and ItemConfig JSON-parsed structs + Validate(). The Item runtime type.
presets.go              # All nine presets used by standalone-playground.
aliases.go              # Unqualified re-exports of visuals.* types so legacy package code reads naturally. See "Note on aliases.go" below.
cmd/module/main.go      # module.ModularMain entrypoint — registers all three models.
visuals/                # The typed visualization library (the Python sibling is viam_visuals/).
```

### Note on aliases.go

`aliases.go` re-exports types like `Pose`, `Color`, `Item`, `Animation`, `BaseGeom` etc. from the `visuals` package under their unqualified names. This lets the existing module code (the long-lived `presets.go`, `animation.go`, etc.) reference these types without `visuals.` prefixes, scoping the migration diff.

New code should reference `visuals.*` directly. The aliases will be removed once a dedicated pass migrates the legacy files. Tracked in [LIBRARY_PLAN.md](../example-visualizations-python/LIBRARY_PLAN.md) (Python repo).

## Conventions and gotchas

Everything in [LESSONS.md from the Python version](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md) applies. The load-bearing ones for Go authors:

- **Field-mask paths are camelCase.** The official worldstatestore guide says snake_case; the renderer empirically only honors the camelCase form. See `visuals/animations.go::Path*` for the canonical constants. The same constants are duplicated in the `visuals` package and re-exported via `aliases.go`.
- **PCD format must match `pointcloud.ToPCD` byte-for-byte.** Leading `#` comments and `VERSION 0.7` (vs `VERSION .7`) both break the viewer's strict-order parser. We ship the same generated assets as the Python module.
- **Mesh content type is lowercase `ply` or `stl`.** STL is converted to PLY on the wire via `stlToPLY` in `geometries.go`.
- **Metadata struct uses the visualization library's schema, NOT the RDK fake's.** All five required keys (`colors`, `color_format`, `opacities`, `show_axes_helper`, `invisible`) must be present.
- **The renderer caches REMOVED UUIDs and drops subsequent ADDED for the same UUID.** Flicker / lifecycle / respawn-style animations must rotate the UUID on every re-add or the entity stays gone until the page is refreshed.
- **`EasyResource` (Python) and the equivalent Go base don't auto-call `reconfigure` on construction.** Both services and components require an explicit `reconfigure` invocation in `new`. The Go driver does this in `newPlaygroundDriver`; the Go visualizer inherits the same pattern through `sceneSprites`.
- **In-process DoCommand calls preserve concrete Go slice types.** `[]string` stays `[]string`, `[]map[string]any` stays itself — gRPC would erase both to `[]any`. The `apply_events` handler must coerce both shapes; that's what `coerceStringSlice` / `coerceEventsSlice` are for.

## License

Apache-2.0.
