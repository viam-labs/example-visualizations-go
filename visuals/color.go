package visuals

// Color is an RGB color with channel values in [0, 255]. The wire
// format encodes each channel as a uint8; the integer type here
// keeps construction syntax light (no casts at call sites).
//
// Future versions can grow named-color tables, hex parsing, and
// HSV conversion here without affecting the wire format.
type Color struct {
	R, G, B int
}

// BoxDims is the per-axis box size in millimeters.
type BoxDims struct{ X, Y, Z float64 }
