// Type aliases — re-export the public schema from the visuals
// subpackage under the existing names. Lets the rest of this module
// (service.go, animation.go's tick code, config.go's ItemConfig)
// keep referencing the unqualified names while the canonical
// definitions live in the library subpackage.
//
// These aliases exist only to scope the diff of the
// example-visualizations-go OO extraction. They can be removed in a
// follow-up that updates each call site to use the qualified
// names directly.
package exampleviz

import "exampleviz/visuals"

type (
	Pose      = visuals.Pose
	Color     = visuals.Color
	BoxDims   = visuals.BoxDims
	Item      = visuals.Item
	Animation = visuals.Animation
	Overrides = visuals.Overrides
)

// Re-exported constants / functions (forward declarations so that
// existing code's bare references stay valid).
var (
	IdentityPose = visuals.IdentityPose
	PoseXYZ      = visuals.PoseXYZ
	PoseAt       = visuals.PoseAt
	IsAnimated   = visuals.IsAnimated
	ToItems      = visuals.ToItems
)

// Re-exported OO types — shape constructors and animation specs.
type (
	Visual        = visuals.Visual
	AnimationSpec = visuals.AnimationSpec
	Box           = visuals.Box
	Sphere        = visuals.Sphere
	Capsule       = visuals.Capsule
	Point         = visuals.Point
	Arrow         = visuals.Arrow
	Mesh          = visuals.Mesh
	PointCloud    = visuals.PointCloud
	Static        = visuals.Static
	Spin          = visuals.Spin
	Swing         = visuals.Swing
	Oscillate     = visuals.Oscillate
	Orbit         = visuals.Orbit
	Pulse         = visuals.Pulse
	Breathe       = visuals.Breathe
	Flicker       = visuals.Flicker
	Lifecycle     = visuals.Lifecycle
	ForceVector   = visuals.ForceVector
	Trajectory    = visuals.Trajectory
	// Composites.
	Composite       = visuals.Composite
	CoordinateFrame = visuals.CoordinateFrame
	Line            = visuals.Line
	BoundingBox     = visuals.BoundingBox
)

var (
	ArrowFromTo = visuals.ArrowFromTo
)

// Re-exported lifecycle convention.
var (
	LifecycleColorAppearing    = visuals.LifecycleColorAppearing
	LifecycleColorAlive        = visuals.LifecycleColorAlive
	LifecycleColorDisappearing = visuals.LifecycleColorDisappearing
	LifecycleOpacityAppearing  = visuals.LifecycleOpacityAppearing
	LifecycleOpacityAlive      = visuals.LifecycleOpacityAlive
	LifecycleOpacityDispearing = visuals.LifecycleOpacityDispearing
)

// Re-exported field-mask path constants.
const (
	PathTheta         = visuals.PathTheta
	PathX             = visuals.PathX
	PathY             = visuals.PathY
	PathZ             = visuals.PathZ
	PathOX            = visuals.PathOX
	PathOY            = visuals.PathOY
	PathOZ            = visuals.PathOZ
	PathSphereRadius  = visuals.PathSphereRadius
	PathCapsuleRadius = visuals.PathCapsuleRadius
	PathCapsuleLength = visuals.PathCapsuleLength
	PathBoxDimsX      = visuals.PathBoxDimsX
	PathBoxDimsY      = visuals.PathBoxDimsY
	PathBoxDimsZ      = visuals.PathBoxDimsZ
	PathMetadataColor = visuals.PathMetadataColor
	PathMetadataOpac  = visuals.PathMetadataOpac
)

var (
	SupportedModes = visuals.SupportedModes
	SupportedAxes  = visuals.SupportedAxes
)
