package main

// outerRing maps a ring position (0-9, clockwise from top-left going down the
// left side) to the physical WS2812B LED index.
// LED wiring is column-major: col0=LEDs 0,1,2 | col1=3,4,5 | col2=6,7,8 | col3=9,10,11
// Inner LEDs 4 (col1,row1) and 7 (col2,row1) are NOT on the outer ring.
var outerRing = [10]uint8{0, 1, 2, 5, 8, 11, 10, 9, 6, 3}

// wsSpinPos is the comet head position in outerRing (0-9).
var wsSpinPos uint8

// wsSpinTimer is the number of 32ms LED ticks to hold the spin display.
// Set by RotarySpinAdvance; decremented each tick; 0 = inactive (rainbow mode).
var wsSpinTimer uint8

// RotarySpinAdvance advances the white comet one step around the outer ring
// and refreshes its hold timer. Call once per rotary notch.
// dir: +1 = clockwise, -1 = counter-clockwise.
func RotarySpinAdvance(dir int) {
	if dir >= 0 {
		wsSpinPos = (wsSpinPos + 9) % 10 // CW rotary → move backward in ring (ring is CCW-ordered)
	} else {
		wsSpinPos = (wsSpinPos + 1) % 10
	}
	wsSpinTimer = 25 // ~800 ms hold after the last notch
}

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
