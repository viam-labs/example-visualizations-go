# example-visualizations-go

Go port of [`viam:example-visualizations`](https://github.com/viam-labs/example-visualizations-python) — a Viam module that adds every supported geometry primitive (box, sphere, capsule, point, mesh PLY/STL, point cloud PCD) to the Viam 3D scene viewer so you can poke each one and see what its config knobs do.

The module is a single `rdk:service:world_state_store` implementation called **`viam:example-visualizations-go:scene-primitives`**. Same behaviour, same wire format, same gotchas as the Python original. Default config emits one of every primitive in a row along X. Runtime `DoCommand` verbs let you add, remove, update, animate, snapshot, and toggle the renderer UUID strategy without reconfiguring.

> See the Python version's [LESSONS.md](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md) for the full set of findings about the viewer's wire format. Everything documented there applies here verbatim — Go and Python differ only in implementation language, not in what the renderer accepts.

## Why two versions?

The Python module is the playground: fast to iterate, easy to read, hits every renderer-side gotcha and writes them down. The Go module is the reference implementation: same scene, same animations, same DoCommand surface, but built on `go.viam.com/rdk` so it can ship into Go-shop pipelines, share types with `viamrobotics/visualization`, and serve as the seed for the ViamVizHelpers Go library when the Python side stabilizes.

If you're choosing one to start from:
- **Most authors should pick Python** unless their build pipeline is Go-only. The Python version evolves faster.
- **Go-shop workcells** that want zero-Python deployment can use this version. Identical scene content.

## Quickstart

Add the service to a machine, no config attributes needed:

```jsonc
{
  "modules": [
    {
      "type": "registry",
      "name": "viam_example-visualizations-go",
      "module_id": "viam:example-visualizations-go",
      "version": "0.0.1"
    }
  ],
  "services": [
    {
      "name": "scene",
      "namespace": "rdk",
      "type": "world_state_store",
      "model": "viam:example-visualizations-go:scene-primitives",
      "attributes": {}
    }
  ]
}
```

Open the machine's **3D scene** tab. With no `preset` attribute set, the default loads `all` — every preset stacked along Y so you see the full tour in one viewport. To see just the 12-item primitives row, set `"preset": "primitives"` in the service attributes.

`preset` can be set to one of: `all` (default), `primitives`, `orientation_vectors`, `frame_composition`, `trajectory_preview`, `force_vector_demo`, `geometry_morph`, `lifecycle_demo`, `chunked_pcd_demo`.

## Config / DoCommand reference

The config schema and DoCommand surface match the Python version 1:1. See [`viam-labs/example-visualizations-python/README.md`](https://github.com/viam-labs/example-visualizations-python/blob/main/README.md#config-reference) for the full reference — the JSON wire format is identical.

Animation modes available: `none`, `orbit`, `oscillate`, `spin`, `swing`, `pulse`, `trajectory`, `force_vector`, `breathe`, `flicker`, `lifecycle`.

DoCommand verbs available: `list`, `remove`, `clear`, `preset`, `snapshot`, `set_uuid_strategy`, `get_entity_chunk`. (The Python version also has `add` and `update`; those aren't ported yet — see "Differences from the Python version" below.)

## Development

```sh
go mod tidy            # one-time dep resolve
make test              # go test ./...
make                   # build the binary into bin/example-visualizations-go
make module.tar.gz     # build the registry tarball
make upload            # build + upload at $(cat VERSION)
```

The binary lives at `bin/example-visualizations-go`. `meta.json::entrypoint` points there so the tarball ships a self-contained binary.

## Architecture

Single Go package at the repo root. Each file owns one concern:

- `geometries.go` — proto builders (Box, Sphere, Capsule, Point, Arrow, Mesh, PointCloud), metadata struct, PCD parsing + chunking, STL→PLY conversion, PLY vertex color extraction.
- `animation.go` — pose/geom math for all 11 modes (`none`, `orbit`, `oscillate`, `spin`, `swing`, `pulse`, `trajectory`, `force_vector`, `breathe`, `flicker`, `lifecycle`). Field-mask path constants (camelCase — see gotchas).
- `config.go` — `Config` and `ItemConfig` JSON-parsed structs + `Validate()`. The `Item` runtime type used internally.
- `presets.go` — all nine presets (`primitives`, `orientation_vectors`, `frame_composition`, `trajectory_preview`, `force_vector_demo`, `geometry_morph`, `lifecycle_demo`, `chunked_pcd_demo`, `all`).
- `service.go` — the `SceneSprites` service: state map, per-subscriber channels with backpressure, animation tick goroutine, stable vs. versioned UUID strategies, DoCommand dispatcher.
- `cmd/module/main.go` — `module.ModularMain` entrypoint.

## Conventions and gotchas

Everything in [LESSONS.md from the Python version](https://github.com/viam-labs/example-visualizations-python/blob/main/LESSONS.md) applies. The load-bearing ones for Go authors:

- **Field-mask paths are camelCase.** The official worldstatestore guide says snake_case; the renderer empirically only honors the camelCase form. See `animation.go::Path*` for the canonical constants.
- **PCD format must match `pointcloud.ToPCD` byte-for-byte.** Leading `#` comments and `VERSION 0.7` (vs `VERSION .7`) both break the viewer's strict-order parser. We ship the same generated assets as the Python module.
- **Mesh content type is lowercase `ply` or `stl`.** STL is converted to PLY on the wire via `stlToPLY` in `geometries.go`.
- **Metadata struct uses the visualization library's schema, NOT the RDK fake's.** All five required keys (`colors`, `color_format`, `opacities`, `show_axes_helper`, `invisible`) must be present.
- **The renderer caches REMOVED UUIDs and drops subsequent ADDED for the same UUID.** Flicker / lifecycle / respawn-style animations must rotate the UUID on every re-add or the entity stays gone until the page is refreshed. Default behavior here matches the Python version (rotate on re-add unless `rotate_uuid_on_readd: false`).

## Differences from the Python version

This port aims at feature parity for the **rendering** surface (what the viewer sees). A few playground/maintenance niceties from the Python version aren't yet implemented:

- **`add` and `update` DoCommand verbs** — the Python version lets users add/update items at runtime without reconfiguring. The Go version only has `remove`, `clear`, `preset`, `snapshot`, `set_uuid_strategy`, `get_entity_chunk`, and `list`. `add` / `update` are straightforward additions (port the body of `do_command` from the Python `service.py`); not done yet because the visual presets exercise the rendering paths fine without them.
- **Test coverage** — Go has ~25 tests covering the core math + format helpers. Python has 253 tests covering the full surface. Plenty of room to grow.

These are not differences in what the viewer renders; just gaps in the playground knobs and test rigor.

## License

Apache-2.0.
