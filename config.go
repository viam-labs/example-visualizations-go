// Config is the persisted attribute shape and the visuals.Item type plus
// validators. Separated from service.go so the schema is easy to
// scan.
package exampleviz

import (
	"fmt"
	"os"
	"path/filepath"

	"exampleviz/visuals"
)

// ---- attribute names / defaults --------------------------------------

const (
	DefaultTickHz       = 30.0
	DefaultUUIDStrategy = "stable"
	DefaultParentFrame  = "world"
	DefaultPreset       = "all"
)

var ValidUUIDStrategies = []string{"stable", "versioned"}

// Config is the JSON-parsed attributes block. Pointer fields allow
// us to distinguish "unset" from "zero value" where it matters.
type Config struct {
	TickHz       *float64     `json:"tick_hz,omitempty"`
	UUIDStrategy string       `json:"uuid_strategy,omitempty"`
	ParentFrame  string       `json:"parent_frame,omitempty"`
	Preset       string       `json:"preset,omitempty"`
	Items        []ItemConfig `json:"items,omitempty"`
}

// ItemConfig is the JSON-parsed shape of one item.
type ItemConfig struct {
	Type           string         `json:"type"`
	Label          string         `json:"label"`
	ParentFrame    string         `json:"parent_frame,omitempty"`
	Pose           *PoseJSON      `json:"pose,omitempty"`
	DimsMM         *DimsJSON      `json:"dims_mm,omitempty"`
	RadiusMM       *float64       `json:"radius_mm,omitempty"`
	LengthMM       *float64       `json:"length_mm,omitempty"`
	MeshPath       string         `json:"mesh_path,omitempty"`
	RawSTL         bool           `json:"raw_stl,omitempty"`
	PointcloudPath string         `json:"pointcloud_path,omitempty"`
	Color          *ColorJSON     `json:"color,omitempty"`
	Opacity        *float64       `json:"opacity,omitempty"`
	ShowAxesHelper bool           `json:"show_axes_helper,omitempty"`
	Invisible      bool           `json:"invisible,omitempty"`
	Chunked        bool           `json:"chunked,omitempty"`
	ChunkSize      int            `json:"chunk_size,omitempty"`
	Animation      *AnimationJSON `json:"animation,omitempty"`
}

// PoseJSON: any subset of fields, missing default to 0 (with OZ=1).
type PoseJSON struct {
	X     *float64 `json:"x,omitempty"`
	Y     *float64 `json:"y,omitempty"`
	Z     *float64 `json:"z,omitempty"`
	OX    *float64 `json:"ox,omitempty"`
	OY    *float64 `json:"oy,omitempty"`
	OZ    *float64 `json:"oz,omitempty"`
	Theta *float64 `json:"theta,omitempty"`
}

func (p *PoseJSON) toPose() visuals.Pose {
	out := visuals.Pose{OZ: 1.0}
	if p == nil {
		return out
	}
	if p.X != nil {
		out.X = *p.X
	}
	if p.Y != nil {
		out.Y = *p.Y
	}
	if p.Z != nil {
		out.Z = *p.Z
	}
	hasOrient := p.OX != nil || p.OY != nil || p.OZ != nil
	if hasOrient {
		out.OZ = 0
		if p.OX != nil {
			out.OX = *p.OX
		}
		if p.OY != nil {
			out.OY = *p.OY
		}
		if p.OZ != nil {
			out.OZ = *p.OZ
		}
	}
	if p.Theta != nil {
		out.Theta = *p.Theta
	}
	return out
}

// DimsJSON for box dims.
type DimsJSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// ColorJSON for {r, g, b} 0..255.
type ColorJSON struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
}

func (c *ColorJSON) toColor() *visuals.Color {
	if c == nil {
		return nil
	}
	return &visuals.Color{R: c.R, G: c.G, B: c.B}
}

// AnimationJSON parses the per-item animation block. Different
// modes use different params, so all fields are optional.
type AnimationJSON struct {
	Mode              string     `json:"mode,omitempty"`
	RadiusMM          *float64   `json:"radius_mm,omitempty"`
	AmplitudeMM       *float64   `json:"amplitude_mm,omitempty"`
	PeriodS           *float64   `json:"period_s,omitempty"`
	Axis              string     `json:"axis,omitempty"`
	AmplitudeDeg      *float64   `json:"amplitude_deg,omitempty"`
	Waypoints         []PoseJSON `json:"waypoints,omitempty"`
	DurationS         *float64   `json:"duration_s,omitempty"`
	Loop              *bool      `json:"loop,omitempty"`
	LengthAmplitudeMM *float64   `json:"length_amplitude_mm,omitempty"`
	RadiusAmplitudeMM *float64   `json:"radius_amplitude_mm,omitempty"`
	TiltDeg           *float64   `json:"tilt_deg,omitempty"`
	PrecessionSpeed   *float64   `json:"precession_speed,omitempty"`
	ColorSpeed        *float64   `json:"color_speed,omitempty"`
	Amplitude         *float64   `json:"amplitude,omitempty"`
	DutyCycle         *float64   `json:"duty_cycle,omitempty"`
	PhaseOffsetS      *float64   `json:"phase_offset_s,omitempty"`
	RotateUUIDOnReadd *bool      `json:"rotate_uuid_on_readd,omitempty"`
	AppearS           *float64   `json:"appear_s,omitempty"`
	AliveS            *float64   `json:"alive_s,omitempty"`
	DisappearS        *float64   `json:"disappear_s,omitempty"`
	GoneS             *float64   `json:"gone_s,omitempty"`
}

func (a *AnimationJSON) toAnimation() visuals.Animation {
	out := visuals.Animation{Mode: "none"}
	if a == nil {
		return out
	}
	out.Mode = a.Mode
	if out.Mode == "" {
		out.Mode = "none"
	}
	if a.RadiusMM != nil {
		out.RadiusMM = *a.RadiusMM
	}
	if a.AmplitudeMM != nil {
		out.AmplitudeMM = *a.AmplitudeMM
	}
	if a.PeriodS != nil {
		out.PeriodS = *a.PeriodS
	}
	out.Axis = a.Axis
	if a.AmplitudeDeg != nil {
		out.AmplitudeDeg = *a.AmplitudeDeg
	}
	for _, w := range a.Waypoints {
		out.Waypoints = append(out.Waypoints, w.toPose())
	}
	if a.DurationS != nil {
		out.DurationS = *a.DurationS
	}
	if a.Loop != nil {
		out.Loop = *a.Loop
		out.HasLoop = true
	}
	if a.LengthAmplitudeMM != nil {
		out.LengthAmplitudeMM = *a.LengthAmplitudeMM
	}
	if a.RadiusAmplitudeMM != nil {
		out.RadiusAmplitudeMM = *a.RadiusAmplitudeMM
	}
	if a.TiltDeg != nil {
		out.TiltDeg = *a.TiltDeg
	}
	if a.PrecessionSpeed != nil {
		out.PrecessionSpeed = *a.PrecessionSpeed
	}
	if a.ColorSpeed != nil {
		out.ColorSpeed = *a.ColorSpeed
	}
	if a.Amplitude != nil {
		out.Amplitude = *a.Amplitude
	}
	if a.DutyCycle != nil {
		out.DutyCycle = *a.DutyCycle
	}
	if a.PhaseOffsetS != nil {
		out.PhaseOffsetS = *a.PhaseOffsetS
	}
	if a.RotateUUIDOnReadd != nil {
		out.RotateUUIDOnReadd = a.RotateUUIDOnReadd
	}
	if a.AppearS != nil {
		out.AppearS = *a.AppearS
	}
	if a.AliveS != nil {
		out.AliveS = *a.AliveS
	}
	if a.DisappearS != nil {
		out.DisappearS = *a.DisappearS
	}
	if a.GoneS != nil {
		out.GoneS = *a.GoneS
	}
	return out
}

// visuals.Item has moved to the visuals subpackage; the alias in aliases.go
// keeps unqualified references working. ItemConfig.toItem() produces
// the same struct, now living in the library namespace.

func (ic ItemConfig) toItem() visuals.Item {
	out := visuals.Item{
		Type:           ic.Type,
		Label:          ic.Label,
		ParentFrame:    ic.ParentFrame,
		Pose:           ic.Pose.toPose(),
		Color:          ic.Color.toColor(),
		Opacity:        ic.Opacity,
		ShowAxesHelper: ic.ShowAxesHelper,
		Invisible:      ic.Invisible,
		Chunked:        ic.Chunked,
		ChunkSize:      ic.ChunkSize,
		MeshPath:       ic.MeshPath,
		RawSTL:         ic.RawSTL,
		PointcloudPath: ic.PointcloudPath,
		Animation:      ic.Animation.toAnimation(),
	}
	if ic.DimsMM != nil {
		out.DimsMM = visuals.BoxDims{X: ic.DimsMM.X, Y: ic.DimsMM.Y, Z: ic.DimsMM.Z}
		out.HasDims = true
	}
	if ic.RadiusMM != nil {
		out.RadiusMM = *ic.RadiusMM
	}
	if ic.LengthMM != nil {
		out.LengthMM = *ic.LengthMM
	}
	return out
}

// Validate is called by the SDK before reconfigure.
func (c *Config) Validate(path string) ([]string, []string, error) {
	if c.TickHz != nil {
		hz := *c.TickHz
		if hz <= 0 || hz > 30 {
			return nil, nil, fmt.Errorf("%s: tick_hz must be in (0, 30]", path)
		}
	}
	if c.UUIDStrategy != "" && !contains(ValidUUIDStrategies, c.UUIDStrategy) {
		return nil, nil, fmt.Errorf("%s: uuid_strategy must be one of %v, got %q",
			path, ValidUUIDStrategies, c.UUIDStrategy)
	}
	if c.Preset != "" {
		if _, ok := Presets[c.Preset]; !ok {
			names := make([]string, 0, len(Presets))
			for k := range Presets {
				names = append(names, k)
			}
			return nil, nil, fmt.Errorf("%s: preset must be one of %v, got %q",
				path, names, c.Preset)
		}
	}
	seen := map[string]bool{}
	for i, it := range c.Items {
		if err := validateItem(it, path, i); err != nil {
			return nil, nil, err
		}
		if seen[it.Label] {
			return nil, nil, fmt.Errorf("%s: items[%d] duplicate label %q", path, i, it.Label)
		}
		seen[it.Label] = true
	}
	return nil, nil, nil
}

func validateItem(it ItemConfig, path string, idx int) error {
	where := fmt.Sprintf("%s: items[%d]", path, idx)
	if it.Type == "" {
		return fmt.Errorf("%s missing 'type'", where)
	}
	if !contains(SupportedTypes, it.Type) {
		return fmt.Errorf("%s unknown type %q; expected one of %v", where, it.Type, SupportedTypes)
	}
	if it.Label == "" {
		return fmt.Errorf("%s missing or empty 'label'", where)
	}
	mode := "none"
	if it.Animation != nil && it.Animation.Mode != "" {
		mode = it.Animation.Mode
	}
	if !contains(visuals.SupportedModes, mode) {
		return fmt.Errorf("%s unknown animation.mode %q; expected one of %v",
			where, mode, visuals.SupportedModes)
	}
	if mode == "trajectory" {
		if it.Animation == nil || len(it.Animation.Waypoints) < 2 {
			return fmt.Errorf("%s animation.mode 'trajectory' requires animation.waypoints to be a list of 2+ pose dicts",
				where)
		}
	}
	if it.Color != nil {
		for _, ch := range []int{it.Color.R, it.Color.G, it.Color.B} {
			if ch < 0 || ch > 255 {
				return fmt.Errorf("%s color channel must be in [0, 255]", where)
			}
		}
	}
	if it.Opacity != nil {
		op := *it.Opacity
		if op < 0 || op > 1 {
			return fmt.Errorf("%s opacity must be in [0, 1]", where)
		}
	}
	switch it.Type {
	case "box":
		if it.DimsMM == nil {
			return fmt.Errorf("%s box requires 'dims_mm'", where)
		}
		for _, v := range []float64{it.DimsMM.X, it.DimsMM.Y, it.DimsMM.Z} {
			if v <= 0 {
				return fmt.Errorf("%s box dims_mm must all be > 0", where)
			}
		}
	case "sphere":
		if it.RadiusMM == nil {
			return fmt.Errorf("%s sphere requires 'radius_mm'", where)
		}
		if *it.RadiusMM <= 0 {
			return fmt.Errorf("%s sphere radius_mm must be > 0", where)
		}
	case "capsule":
		if it.RadiusMM == nil || it.LengthMM == nil {
			return fmt.Errorf("%s capsule requires 'radius_mm' and 'length_mm'", where)
		}
		if *it.RadiusMM <= 0 || *it.LengthMM <= 0 {
			return fmt.Errorf("%s capsule radius_mm and length_mm must be > 0", where)
		}
	case "arrow":
		if it.RadiusMM == nil || it.LengthMM == nil {
			return fmt.Errorf("%s arrow requires 'radius_mm' and 'length_mm'", where)
		}
		if *it.RadiusMM <= 0 || *it.LengthMM <= 0 {
			return fmt.Errorf("%s arrow radius_mm and length_mm must be > 0", where)
		}
	case "point":
		// no shape config
	case "mesh":
		if it.MeshPath == "" {
			return fmt.Errorf("%s mesh requires 'mesh_path'", where)
		}
		meshFmt, err := visuals.InferMeshContentType(it.MeshPath)
		if err != nil {
			return fmt.Errorf("%s %w", where, err)
		}
		resolved := resolveAssetPath(it.MeshPath)
		if _, err := os.Stat(resolved); err != nil {
			return fmt.Errorf("%s mesh asset not found: %s", where, resolved)
		}
		if it.RawSTL && meshFmt != "stl" {
			return fmt.Errorf("%s mesh 'raw_stl' only valid on .stl assets; got %q (inferred %q)",
				where, it.MeshPath, meshFmt)
		}
	case "pointcloud":
		if it.PointcloudPath == "" {
			return fmt.Errorf("%s pointcloud requires 'pointcloud_path'", where)
		}
		resolved := resolveAssetPath(it.PointcloudPath)
		if _, err := os.Stat(resolved); err != nil {
			return fmt.Errorf("%s pointcloud asset not found: %s", where, resolved)
		}
		if it.ChunkSize < 0 {
			return fmt.Errorf("%s pointcloud chunk_size must be a positive integer", where)
		}
	}
	return nil
}

// resolveAssetPath: relative paths are interpreted relative to the
// module directory (assumed to be the parent of the running binary
// when packaged, or the CWD in development).
func resolveAssetPath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	// Prefer ModuleDir (set by main) if it exists; otherwise CWD.
	if ModuleDir != "" {
		return filepath.Join(ModuleDir, p)
	}
	return p
}

// ModuleDir is the directory where the module was extracted by
// viam-server — the tarball root. Assets live at <ModuleDir>/assets/.
//
// We resolve it from os.Executable() (parent of the binary's parent,
// since the binary is at <ModuleDir>/bin/example-visualizations-go).
// CWD when viam-server launches the module is NOT reliable; the
// framework can run the binary from anywhere. Using executable path
// is the only stable anchor.
//
// VIAM_MODULE_DATA, despite the name, points at a per-module data
// dir for *runtime* state (logs, persistence) — NOT where the
// module's bundled assets live. Don't use it for asset resolution.
var ModuleDir string

func init() {
	exe, err := os.Executable()
	if err == nil {
		// <ModuleDir>/bin/example-visualizations-go → <ModuleDir>
		ModuleDir = filepath.Dir(filepath.Dir(exe))
	}
	// Fallback for tests / scripts run from the repo root.
	if ModuleDir == "" || !dirHas(ModuleDir, "assets") {
		if d, e := os.Getwd(); e == nil && dirHas(d, "assets") {
			ModuleDir = d
		}
	}
}

func dirHas(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.IsDir()
}
