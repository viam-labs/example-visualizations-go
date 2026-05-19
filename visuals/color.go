package visuals

// Color is an RGB color with channel values in [0, 255]. The wire
// format encodes each channel as a uint8; the integer type here
// keeps construction syntax light (no casts at call sites).
type Color struct {
	R, G, B int
}

// BoxDims is the per-axis box size in millimeters.
type BoxDims struct{ X, Y, Z float64 }

// HSVToRGB converts HSV (each in [0, 1]) to an 8-bit RGB Color.
// Useful for animations that cycle through the rainbow. Hue wraps:
// HSVToRGB(1.5, 1, 1) == HSVToRGB(0.5, 1, 1).
//
// Example: cycle a sphere through the spectrum at 1 cycle per 5
// seconds in your SceneTick:
//
//	c := visuals.HSVToRGB(math.Mod(t/5.0, 1.0), 1, 1)
//	sphere.Color = &c
//	return scene.Update(sphere)
func HSVToRGB(h, s, v float64) Color {
	h = h - float64(int(h)) // h mod 1
	if h < 0 {
		h += 1
	}
	h6 := h * 6.0
	i := int(h6) % 6
	f := h6 - float64(int(h6))
	p, q, tval := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	var r, g, b float64
	switch i {
	case 0:
		r, g, b = v, tval, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, tval
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = tval, p, v
	default:
		r, g, b = v, p, q
	}
	return Color{R: int(r * 255), G: int(g * 255), B: int(b * 255)}
}
