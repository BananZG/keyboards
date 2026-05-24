package main

import (
	"errors"
	"image/color"

	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	"tinygo.org/x/drivers/ssd1306"
	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freemono"
)

// ---- Display state constants ----

const (
	showPageInfo      = iota // show current layer name + key hints
	showFNOverlay            // FN key held, waiting for layer selection
	showFNPreview            // FN held + layer selected: show preview briefly
	showShiftHint            // Shift key physically held
	showRotaryWarning        // rotary turned on a layer with no bound action
	showScreensaver          // bouncing-pixel screensaver
)

// ---- Colours ----

var (
	textWhite = color.RGBA{255, 255, 255, 255}
	textBlack = color.RGBA{0, 0, 0, 255}
)

// ---- Render helpers ----

// renderLayerInfo writes the current layer name and a key-hint row to the OLED.
// Line 1 (y=20): layer display name (e.g. "a-g  VOL").
// Line 2 (y=50): 9-char hint "xxxx yyyy" built from firmware-default keycodes.
// Called once per layer change; no heap allocations.
func renderLayerInfo(display *ssd1306.Device, buf *DisplayBuffer, layer int) {
	display.ClearDisplay()

	// Line 1: layer name (centred)
	name := layerDisplayNames[layer]
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, name)
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, int16(128-w)/2, 20, name, textWhite)

	// Line 2: key hints — row 0 (idx 0-3) then space then row 1 (idx 4-7)
	var hint [9]byte
	for i := 0; i < 4; i++ {
		hint[i] = keycodeToRune(layerKeys[layer][i])
	}
	hint[4] = ' '
	for i := 0; i < 4; i++ {
		hint[5+i] = keycodeToRune(layerKeys[layer][4+i])
	}
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, 4, 50, string(hint[:]), textWhite)

	display.Display()
	_ = buf
}

// renderShiftHint shows "SHIFT" on line 1 and uppercased key hints on line 2.
// Called while the Shift key is physically held.
func renderShiftHint(display *ssd1306.Device, buf *DisplayBuffer, layer int) {
	display.ClearDisplay()
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, "SHIFT")
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, int16(128-w)/2, 20, "SHIFT", textWhite)
	// Key hints uppercased — shows what Shift+key produces on this layer.
	var hint [9]byte
	upcase := func(b byte) byte {
		if b >= 'a' && b <= 'z' {
			return b - 32
		}
		return b
	}
	for i := 0; i < 4; i++ {
		hint[i] = upcase(keycodeToRune(layerKeys[layer][i]))
	}
	hint[4] = ' '
	for i := 0; i < 4; i++ {
		hint[5+i] = upcase(keycodeToRune(layerKeys[layer][4+i]))
	}
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, 4, 50, string(hint[:]), textWhite)
	display.Display()
	_ = buf
}

// renderRotaryWarning shows a brief "unbound" indicator when the rotary
// encoder is turned on a layer without a dedicated action.
func renderRotaryWarning(display *ssd1306.Device) {
	display.ClearDisplay()
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, "ROTARY")
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, int16(128-w)/2, 20, "ROTARY", textWhite)
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, 4, 50, "no bind", textWhite)
	display.Display()
}

// renderFNInfo shows the FN-held overlay: "FN" + short instruction.
func renderFNInfo(display *ssd1306.Device) {
	display.ClearDisplay()
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, "FN")
	tinyfont.WriteLine(display, &freemono.Regular12pt7b,
		int16(128-w)/2, 20, "FN", textWhite)
	tinyfont.WriteLine(display, &freemono.Regular12pt7b, 4, 50, "select:", textWhite)
	display.Display()
}

// screensaverState holds the mutable position and velocity of the bouncing
// pixel. Grouping them in a struct keeps the state together and means only
// one pointer needs to be passed (and escape-analysed) by renderScreensaver.
type screensaverState struct {
	x, y   int16
	dx, dy int16
}

// renderScreensaver advances the bouncing pixel screensaver.
func renderScreensaver(display *ssd1306.Device, buf *DisplayBuffer, ss *screensaverState) {
	pixel := buf.GetPixel(ss.x, ss.y)
	c := textWhite
	if pixel {
		c = textBlack
	}
	buf.SetPixel(ss.x, ss.y, c)
	ss.x += ss.dx
	ss.y += ss.dy
	if ss.x == 0 || ss.x == 127 {
		ss.dx = -ss.dx
	}
	if ss.y == 0 || ss.y == 63 {
		ss.dy = -ss.dy
	}
	display.SetBuffer(buf.GetBuffer())
	display.Display()
}

// keycodeToRune converts a single character keycode to a printable ASCII byte.
// Returns '.' for non-printable / unknown codes.
func keycodeToRune(kc Keycode) byte {
	// Detect whether the key is shifted and its base HID usage code.
	// Two encodings:
	//   TypeLxxx | TypeXSft (0x02xx) — lsft() encoding, used for uppercase.
	//   TypeNormal | ShiftMask (0xF4xx) — legacy raw ShiftMask (not in use).
	var shifted bool
	var hid Keycode
	if kc&0xF000 == 0x0000 && kc&0x0F00 != 0 {
		// TypeLxxx modifier-combo (e.g. lsft(jp.KeyA) = 0x0204)
		shifted = (kc & 0x0200) != 0 // TypeXSft bit
		hid = kc & 0xFF
	} else {
		// TypeNormal or other encoding
		shifted = (kc & shiftMask) != 0
		hid = (kc &^ shiftMask) & 0xFF
	}
	switch {
	case hid >= 0x04 && hid <= 0x1D: // a-z
		c := byte('a' + hid - 0x04)
		if shifted {
			c -= 32
		}
		return c
	case hid >= 0x1E && hid <= 0x27: // 1-0
		if hid == 0x27 {
			return '0'
		}
		return byte('1' + hid - 0x1E)
	case kc == jp.KeyLeftShift || kc == jp.KeyRightShift:
		return 'S'
	case kc == jp.KeyLeft:
		return '<'
	case kc == jp.KeyRight:
		return '>'
	case kc == jp.KeyUp:
		return '^'
	case kc == jp.KeyDown:
		return 'v'
	case kc == jp.KeyHome:
		return 'H'
	case kc == jp.KeyEnd:
		return 'E'
	case kc == jp.KeyPageUp:
		return 'U'
	case kc == jp.KeyPageDown:
		return 'D'
	case kc == jp.KeySpace:
		return '_'
	case kc == jp.KeyEnter:
		return 'N'
	case kc == jp.KeyComma:
		return ','
	case kc == jp.KeyPeriod:
		return '.'
	case kc == jp.KeySlash:
		return '/'
	case kc == jp.KeyMinus:
		return '-'
	default:
		return '.'
	}
}

// ---- DisplayBuffer ----

// DisplayBuffer mirrors the SSD1306 framebuffer in RAM for pixel-level ops
// (needed by the screensaver to read back individual pixel states).
type DisplayBuffer struct {
	buffer []byte
	width  int16
	height int16
}

func NewDisplayBuffer(width, height int16) *DisplayBuffer {
	return &DisplayBuffer{
		buffer: make([]byte, width*height/8),
		width:  width,
		height: height,
	}
}

func (d *DisplayBuffer) SetPixel(x, y int16, c color.RGBA) {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return
	}
	idx := x + (y/8)*d.width
	if c.R != 0 || c.G != 0 || c.B != 0 {
		d.buffer[idx] |= 1 << uint8(y%8)
	} else {
		d.buffer[idx] &^= 1 << uint8(y%8)
	}
}

func (d *DisplayBuffer) GetPixel(x, y int16) bool {
	if x < 0 || x >= d.width || y < 0 || y >= d.height {
		return false
	}
	idx := x + (y/8)*d.width
	return (d.buffer[idx]>>uint8(y%8))&0x1 == 1
}

func (d *DisplayBuffer) GetBuffer() []byte { return d.buffer }

func (d *DisplayBuffer) SetBuffer(src []byte) error {
	if len(src) != len(d.buffer) {
		return errBufferSize
	}
	copy(d.buffer, src)
	return nil
}

var errBufferSize = errors.New("invalid buffer size")
