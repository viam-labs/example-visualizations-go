# ViamVizHelpers — Go library plan

> **Status: extracted.** As of 0.0.44 the library lives in [`viam-labs/viam-viz-helpers-go`](https://github.com/viam-labs/viam-viz-helpers-go) and this module depends on it via `go.mod`. The original-plan name `vizhelpers` and subpackage layout (`/wsstore`) were superseded during extraction; the actual package is named `visuals` and lives at the repo root for import path simplicity. This file is kept as historical context for the design decisions.

A Go library that extracts the reusable scaffolding from this module so a Viam world-state-store author writes the scene they want, not the wire format underneath it. Designed as the upstream-bound complement to `viamrobotics/visualization`; starts at `viam-labs/viam-viz-helpers-go` so iteration doesn't churn upstream packages while we settle the API.

- **Project name:** ViamVizHelpers
- **Repo:** `github.com/viam-labs/viam-viz-helpers-go`
- **Imported package name:** `visuals` (callers type `visuals.Box(...)`)
- **Module path:** `github.com/viam-labs/viam-viz-helpers-go` (package `visuals` at root)

## Why now (and not earlier)

This is the second pass at a library plan. The first pass (now lives in [`example-visualizations-python/LIBRARY_PLAN.md`](https://github.com/viam-labs/example-visualizations-python/blob/main/LIBRARY_PLAN.md)) chose Python because the prototype was Python and most Viam module authors write Python. That's still true.

The case for Go now is concrete in a way it wasn't before:

- **There's a working Go module to extract from.** [`example-visualizations-go`](https://github.com/viam-labs/example-visualizations-go) ships the same scene as the Python original — 3,634 lines of Go that already handle every gotcha in [`LESSONS.md`](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md). Library design becomes "factor this out" instead of "imagine what we'd want."
- **`viamkit/viz` already exists.** It owns `Box`/`Sphere`/`Capsule`/`Point` `*commonpb.Transform` builders. ViamVizHelpers can build on it instead of duplicating the easy 20%.
- **Upstream merge target is Go.** `viamrobotics/visualization` is the canonical viewer-side repo. A Go library can share types with the viewer at the upstream-merge point with no language jump.

The Python library plan stays valid as the sibling deliverable. They can coexist; one might emerge as primary once we see which language new modules actually adopt.

## What gets extracted from `example-visualizations-go`

Concrete LOC budget:

| File in this module | Lines today | What gets extracted | Stays in module |
|---|---:|---|---|
| `geometries.go` | 601 | metadata struct builder, PCD parse+chunk, STL→PLY, PLY vertex-color extractor, asset path helpers, procedural arrow geometry. ~80% of the file. | ~120 lines (a handful of project-specific helpers) |
| `animation.go` | 518 | All 11 animation modes + the `Animation`/`Overrides`/`TickResult` types + the camelCase field-mask path constants. ~95%. | ~25 lines (the path constants list, re-exported via the library) |
| `config.go` | 449 | JSON-shape `Pose`/`Color`/`Dims` parsing, `validateItem`, asset path resolution from `os.Executable()`. ~60%. | ~180 lines (the module-specific `Config`/`ItemConfig` schema) |
| `presets.go` | 643 | `offsetBaseItems`, `colorWheelChildren`, `waypointsWithTangentOrientations`. ~10%. | ~580 lines (the actual preset content — that's the module) |
| `service.go` | 899 | Whole-service scaffolding: state map, subscriber fanout, animation tick goroutine, UUID strategy, DoCommand dispatcher, chunked-delivery `get_entity_chunk` verb. ~90%. | ~90 lines (project-specific verbs, model registration) |

**Migration math:** ~3,100 lines of `example-visualizations-go` move into the library. The module shrinks from 3,634 lines to ~810 lines, almost all of which is the preset content (the scene the module actually paints).

**Future modules** that aren't trying to be renderer-behavior probes import the library and ship in a couple hundred lines.

## Relationship to `viamkit/viz`

`viamkit/viz` (in `viam-labs/viamkit`) already owns:

- `viz.Box{...}.ToTransform()`, `viz.Sphere`, `viz.Capsule`, `viz.Point`
- `viz.PoseToProto` / `viz.PoseFromProto`
- `viz.Removal(uuid)` for stream removals

ViamVizHelpers complements rather than competes. Concretely:

- **For primitive proto builders** (Box, Sphere, Capsule, Point): `vizhelpers` calls `viz.*.ToTransform()` internally where the API is sufficient. Where `viz.Box` doesn't carry metadata (it lacks color / opacity / show_axes_helper), `vizhelpers` adds them on top.
- **For everything else** — Mesh, PointCloud, metadata struct schema, animation modes, asset loading, service base, chunked delivery: `vizhelpers` owns it. `viamkit/viz` was scoped to the wire-format helpers, not the per-tick animation surface.
- **Eventually**: anything in `vizhelpers` that becomes broadly useful can move into `viamkit/viz` via a PR. The split is provisional, not architectural.

## Scope

**In scope.**

1. Ergonomic geometry constructors (`Box`, `Sphere`, `Capsule`, `Point`, `Arrow`, `Mesh`, `PointCloud`) with the right defaults and metadata wiring.
2. Asset loading + format conversion (PLY pass-through, STL→PLY, PCD writer with `pointcloud.ToPCD` byte parity, `os.Executable()`-anchored path resolution).
3. All 11 animation modes (`none`, `orbit`, `oscillate`, `spin`, `swing`, `pulse`, `trajectory`, `force_vector`, `breathe`, `flicker`, `lifecycle`).
4. `Scene` type: state map, subscriber fanout with backpressure, animation tick goroutine, stable + versioned UUID strategies, rotate-on-readd default for the renderer cache bug.
5. `wsstore.Base` — embeddable service base that gives `ListUUIDs` / `GetTransform` / `StreamTransformChanges` / `DoCommand` for free; user overrides `BuildScene()`.
6. Field-mask path constants (camelCase, single source of truth).
7. Chunked point-cloud delivery — gated behind `WithChunked(...)` and marked `Experimental` in godoc until the viz team confirms the schema.

**Out of scope (initially).**

- The `drawv1.Shape` channel (lines, NURBS, points-with-size). World-state-store services can't emit those today.
- GLTF/GLB/OBJ. Pass-through to PLY only.
- A scene-description DSL. The Go API is the description language.

## Package layout

Two packages. Most users only need the top-level `vizhelpers` import.

```
viam-viz-helpers/
├── go.mod                                # github.com/viam-labs/viam-viz-helpers
├── README.md
├── LICENSE                               # Apache-2.0
│
├── vizhelpers.go                         # Scene type + functional options (WithTickHz, WithUUIDStrategy, WithParentFrame)
├── geom.go                               # Box, Sphere, Capsule, Point, Arrow, Mesh, PointCloud — typed constructors + functional options (WithPose, WithColor, WithOpacity, WithParent, WithAnimation, WithShowAxesHelper, WithChunked)
├── pose.go                               # Pose{} dataclass; Identity(); FromMm(x, y, z); lerp helpers; tangent-from-positions
├── color.go                              # Color type + RGB/Hex/Named; LifecycleAppearing/Alive/Disappearing constants
├── anim.go                               # Animation interface; Spin{}, Oscillate{}, Pulse{}, Swing{}, Orbit{}, Trajectory{}, ForceVector{}, Breathe{}, Flicker{}, Lifecycle{} structs
├── fieldmask.go                          # Public camelCase path constants
├── files.go                              # LoadPLY/LoadSTL/LoadPCD convenience for callers not inside a module
│
├── internal/                             # No external import; wire-format details
│   ├── pcd/pcd.go                        # PCD writer matching pointcloud.ToPCD byte-for-byte; ParsePCD splits header/body/stride
│   ├── ply/ply.go                        # PLY reader; extract per-vertex colors
│   ├── stl/stl.go                        # Binary STL reader; STLToPLY converter
│   └── metadata/metadata.go              # Build(...) emitting all five required keys + optional chunks
│
├── wsstore/                              # Optional: WSStore service base (pulls in go.viam.com/rdk)
│   └── wsstore.go                        # Base struct — embed; SetScene(s *vizhelpers.Scene); ListUUIDs/GetTransform/StreamTransformChanges/DoCommand
│
├── *_test.go                             # Byte-parity tests + animation pinned-t tests + service streaming tests
└── examples/
    ├── one_box/main.go                   # smallest "I want to see something"
    ├── animation_gallery/main.go         # one example per mode
    └── module/main.go                    # full RDK module using wsstore.Base
```

Two reasons the `wsstore` package is split:

1. **Dependency weight.** `go.viam.com/rdk` is heavy. Users who just want to build a `Scene` for testing (no RDK module) shouldn't pay for it.
2. **Upstream landing.** When the module-side service code eventually moves into `viamrobotics/visualization` or the RDK, the `wsstore` subpackage migrates in isolation while `vizhelpers` stays at the standalone import path.

## Public API sketch

The shape an author writes. Concrete, not contract-final.

```go
import (
    "context"
    "fmt"

    "github.com/viam-labs/viam-viz-helpers"
    "github.com/viam-labs/viam-viz-helpers/wsstore"
)

// A module that uses the library — total LOC for the module is ~50.
type MyModule struct {
    wsstore.Base
}

func (m *MyModule) BuildScene(ctx context.Context) (*vizhelpers.Scene, error) {
    s := vizhelpers.New(
        vizhelpers.WithTickHz(5),
        vizhelpers.WithUUIDStrategy(vizhelpers.UUIDStable),
    )

    // Static items — units in mm, color as RGB triple / named / hex.
    s.MustAdd(vizhelpers.Box("base",
        vizhelpers.WithPose(vizhelpers.Identity()),
        vizhelpers.WithDimsMm(100, 100, 100),
        vizhelpers.WithColor(vizhelpers.RGB(230, 25, 75)),
        vizhelpers.WithOpacity(0.8),
    ))

    // Animated sphere — animations are structs, not magic strings.
    s.MustAdd(vizhelpers.Sphere("bobber",
        vizhelpers.WithPose(vizhelpers.FromMm(300, 0, 0)),
        vizhelpers.WithRadiusMm(90),
        vizhelpers.WithColor(vizhelpers.Named("green")),
        vizhelpers.WithAnimation(vizhelpers.Oscillate{
            Axis: vizhelpers.AxisY, AmplitudeMM: 100, PeriodS: 3,
        }),
    ))

    // STL auto-converts to PLY; PLY vertex colors transcode to metadata.colors.
    mesh, err := vizhelpers.MeshFromFile("bunny", "assets/bunny.stl",
        vizhelpers.WithPose(vizhelpers.FromMm(600, 0, 0)),
        vizhelpers.WithColor(vizhelpers.Hex("#FF8000")),
    )
    if err != nil { return nil, err }
    s.MustAdd(mesh)

    // Frame composition — child inherits parent's animated pose.
    s.MustAdd(vizhelpers.Sphere("anchor",
        vizhelpers.WithPose(vizhelpers.FromMm(0, 0, 500)),
        vizhelpers.WithShowAxesHelper(true),
        vizhelpers.WithAnimation(vizhelpers.Spin{PeriodS: 6}),
    ))
    s.MustAdd(vizhelpers.Capsule("attached",
        vizhelpers.WithParent("anchor"),
        vizhelpers.WithPose(vizhelpers.FromMm(200, 0, 0)),
        vizhelpers.WithRadiusMm(20), vizhelpers.WithLengthMm(150),
        vizhelpers.WithAnimation(vizhelpers.Spin{PeriodS: 2}),
    ))

    // Chunked delivery (experimental, gated).
    s.MustAdd(vizhelpers.PointCloudFromFile("helix", "assets/helix.pcd",
        vizhelpers.WithPose(vizhelpers.FromMm(1000, 0, 0)),
        vizhelpers.WithChunked(2000),
    ))

    // Lifecycle convention — staggered phases.
    for i := 0; i < 5; i++ {
        s.MustAdd(vizhelpers.Box(fmt.Sprintf("lifecycle_%02d", i),
            vizhelpers.WithPose(vizhelpers.FromMm(float64(i-2)*250, 0, 0)),
            vizhelpers.WithDimsMm(120, 120, 120),
            vizhelpers.WithAnimation(vizhelpers.Lifecycle{
                AppearS: 1, AliveS: 2, DisappearS: 1, GoneS: 2,
                PhaseOffsetS: float64(i) * 6.0 / 5,
            }),
        ))
    }
    return s, nil
}
```

For "just draw a scene to test something" with no RDK module:

```go
s := vizhelpers.New()
s.MustAdd(vizhelpers.Box("hello", vizhelpers.WithDimsMm(100, 100, 100)))
// s.ListUUIDs(), s.GetTransform(uuid), s.Stream(ctx) are directly callable.
```

What the author **does not** write:

- The metadata struct (all five required keys, base64-packed colors/opacities, the visualization library schema).
- PCD headers matching `pointcloud.ToPCD` byte-for-byte.
- STL → PLY conversion.
- The `os.Executable()`-anchored asset path resolution (0.0.1 of this module learned this the hard way — `VIAM_MODULE_DATA` is the wrong place to look for bundled assets).
- Field-mask path strings (camelCase, single source of truth in `fieldmask.go`).
- Monotonic-counter UUID rotation on REMOVED→ADDED transitions.
- Per-subscriber channels with backpressure + initial-burst + clean-up-on-context-cancel.
- DoCommand dispatch for `list` / `remove` / `clear` / `preset` / `snapshot` / `set_uuid_strategy` / `get_entity_chunk`.

## Delivery order

Not strict phases — just the order to build in, since each piece is usable as soon as it exists. Tag `v0.x` when a chunk is stable.

1. **Geometry constructors + asset loaders.** Equivalent to today's `geometries.go` + `internal/pcd|ply|stl|metadata`. Build a single Transform that the viewer renders correctly. Already useful for ad-hoc one-shots and for the byte-parity test rigs.

2. **`Scene` + animation modes.** Today's `animation.go` + the core of `service.go` (state map, subscriber fanout, tick goroutine, UUID strategies). At this point a user can drive a fully-animated scene entirely in-process — useful for headless tests and for code that drives the viewer over gRPC without an RDK module wrapper.

3. **`wsstore.Base`.** The embeddable service. After this lands a new RDK module is ~50 lines of Go. This is the milestone where `example-visualizations-go` migrates: rewrite the service to embed `wsstore.Base`, override `BuildScene()`, delete the boilerplate. Migration is the validation test.

4. **Chunked delivery.** Gated behind `WithChunked(...)`, marked experimental in godoc until the viz team confirms the schema. Implementation isolated to one file.

5. **Drawing-API support (post-viz-team-clarification).** Lines, NURBS, points-with-size. Only after the viewer's drawing-API channel is exposed to module-side services.

Each step has a milestone: `example-visualizations-go` adopts the matching piece, tests stay green throughout, and the module gets shorter.

## Testing strategy

- **Byte-parity against `pointcloud.ToPCD`.** Write a small PCD through the library and a small PCD through a Go shell-out to `pointcloud.ToPCD`; assert equality. Catches header drift the same way `test_assets_units.py` does in the Python module.
- **Field-mask regression tests.** Enumerate every animation mode, assert each emits camelCase paths. Would have caught the 0.0.32 Python regression before it shipped.
- **Animation correctness at pinned t.** At `t=0`, `t=T/4`, `t=T/2`, `t=3T/4` the math is exact.
- **Stream behavior.** Subscribe to a scene, drive ticks, assert the event sequence: initial burst → UPDATED (stable) or REMOVED+ADDED (versioned) → REMOVED on remove.
- **Visual verification.** `examples/` programs deployed to a Viam machine for manual confirmation. Mandatory release gate.

## Upstream landing

Three plausible homes; decided at 1.0:

- **Stay at `viam-labs/viam-viz-helpers` indefinitely.** Lowest friction; opt-in for users; viam-labs is the natural home for experimental / community-maintained packages.
- **Move into `viamkit/viz`** as an expanded namespace (`viamkit/viz/anim`, `viamkit/viz/scene`, `viamkit/viz/wsstore`). Best for discoverability inside the viam-labs ecosystem; requires viamkit maintainer review.
- **Merge into `viamrobotics/visualization`** as the canonical module-side helper. Strongest position — types shared with the viewer side, no language jump — but requires viz-team review velocity.

Most likely outcome: starts at viam-labs, lands in `viamkit/viz` once the wire format stabilizes, ratchets toward `viamrobotics/visualization` if the viz team wants it.

## Risks

- **Viewer wire format is partially undocumented.** [`LESSONS.md::chunked-delivery-schema`](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md) is the worst offender. Mitigation: gate experimental features behind explicit knobs; mark them in godoc. Don't let unverified contracts ship in the default path.
- **Field-mask paths could flip from camelCase to snake_case in a future viewer.** Mitigation: paths centralized in `fieldmask.go`. A renderer-side change is a one-commit library update.
- **API churn pre-1.0.** Module authors adopting 0.x take the upgrade tax. Mitigation: each release ships migration notes for any breaking change. Semver from 1.0.
- **`viamkit/viz` evolves under us.** If the viamkit maintainer expands `viz.*` to overlap with `vizhelpers.*`, we have to reconcile. Mitigation: keep a one-line dependency on `viamkit/viz` for the primitive builders only, so any divergence shows up at compile time.

## What this unlocks

Before the library: a Go author writes ~3,000 lines to learn the same lessons the Python author already learned. Each gotcha is a separate debugging cycle that ends in `LESSONS.md`.

After the library: a Go author writes the scene they want in ~50 lines. The library knows every gotcha. The findings in `LESSONS.md` survive as code, not as folklore — and the upstream merge path through `viamkit/viz` or `viamrobotics/visualization` is clear.

## Open questions for the viz team

Five questions block 1.0 design decisions. Same as the Python plan — they apply identically.

1. **Is camelCase or snake_case the field-mask path convention?** The spec says snake_case; the renderer empirically only honors camelCase.
2. **What is the `metadata.chunks` schema?** Field names + types for the sub-struct. The library has placeholders (`chunk_size` / `total` / `total_points` / `stride`) but needs the canonical names.
3. **How does the viewer issue `get_entity_chunk` DoCommands?** Auto-fetch on seeing `metadata.chunks`, or only on user action?
4. **What relationship types does the viewer recognize?** `HoverLink` is the only one we've seen mentioned. The library should expose constants for each.
5. **Is there a plan to expose the `drawv1.Shape` channel** (lines, NURBS, points-with-size) to world-state-store-style services? Determines whether ViamVizHelpers needs a drawing-API surface or just absorbs the WSStore-side concerns.
