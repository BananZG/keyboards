package main

// hsvToGRB converts a hue (0-255), full saturation & value, to the packed
// GRB uint32 that WS2812B expects.  No heap allocs, integer-only math.
func hsvToGRB(hue uint8) uint32 {
	h := uint32(hue)
	const v = 96 // half-ish brightness — easy on the eyes
	switch h / 43 {
	case 0: // red → yellow
		r := uint32(v)
		g := uint32(v) * (h * 6) / 256
		return (g << 24) | (r << 16)
	case 1: // yellow → green
		h -= 43
		r := uint32(v) * (255 - h*6) / 256
		g := uint32(v)
		return (g << 24) | (r << 16)
	case 2: // green → cyan
		h -= 86
		g := uint32(v)
		b := uint32(v) * (h * 6) / 256
		return (g << 24) | (b << 8)
	case 3: // cyan → blue
		h -= 129
		g := uint32(v) * (255 - h*6) / 256
		b := uint32(v)
		return (g << 24) | (b << 8)
	case 4: // blue → magenta
		h -= 172
		r := uint32(v) * (h * 6) / 256
		b := uint32(v)
		return (r << 16) | (b << 8)
	default: // magenta → red
		h -= 215
		r := uint32(v)
		b := uint32(v) * (255 - h*6) / 256
		return (r << 16) | (b << 8)
	}
}
