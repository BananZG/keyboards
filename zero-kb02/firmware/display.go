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
	showPageInfo    = iota // briefly show page layout after a page change
	showLayerInfo          // briefly show rotary layer name after a layer change
	showFNOverlay          // FN key held, waiting for page selection
	showFNPreview          // FN held + page selected: show page name briefly
	showScreensaver        // bouncing-pixel screensaver
)

// ---- Colours ----

var (
	textWhite = color.RGBA{255, 255, 255, 255}
	textBlack = color.RGBA{0, 0, 0, 255}
)

// ---- Render helpers ----

// rotaryLayerNames maps vial layer index to a short display label.
var rotaryLayerNames = [3]string{"VOL", "CURSOR", "BRIGHT"}

// renderPageInfo writes the current page name and key layout to the LCD.
// Called once on page change; no heap allocations.
func renderPageInfo(display *ssd1306.Device, buf *DisplayBuffer, page int) {
	display.ClearDisplay()

	// Line 1: page name centred
	name := GetPageName(page)
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, "PAGE "+name)
	tinyfont.WriteLine(display, &freemono.Regular12pt7b,
		int16(128-w)/2, 20, "PAGE "+name, textWhite)

	// Line 2: key hints for rows 0-1
	if page < TotalPages {
		var hint [9]byte
		row0 := keyPages[page][0]
		for i, kc := range row0 {
			hint[i] = keycodeToRune(kc)
		}
		hint[4] = ' '
		row1 := keyPages[page][1]
		for i, kc := range row1 {
			hint[5+i] = keycodeToRune(kc)
		}
		tinyfont.WriteLine(display, &freemono.Regular12pt7b, 4, 50, string(hint[:]), textWhite)
	}

	display.Display()
	_ = buf
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

// renderLayerInfo shows the active rotary layer name on the LCD.
func renderLayerInfo(display *ssd1306.Device, layer int) {
	display.ClearDisplay()
	name := "?"
	if layer >= 0 && layer < len(rotaryLayerNames) {
		name = rotaryLayerNames[layer]
	}
	_, w := tinyfont.LineWidth(&freemono.Regular12pt7b, name)
	tinyfont.WriteLine(display, &freemono.Regular12pt7b,
		int16(128-w)/2, 40, name, textWhite)
	display.Display()
}

// renderScreensaver advances the bouncing pixel screensaver.
func renderScreensaver(display *ssd1306.Device, buf *DisplayBuffer, dispx, dispy, deltaX, deltaY *int16) {
	pixel := buf.GetPixel(*dispx, *dispy)
	c := textWhite
	if pixel {
		c = textBlack
	}
	buf.SetPixel(*dispx, *dispy, c)
	*dispx += *deltaX
	*dispy += *deltaY
	if *dispx == 0 || *dispx == 127 {
		*deltaX = -*deltaX
	}
	if *dispy == 0 || *dispy == 63 {
		*deltaY = -*deltaY
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
	case kc == jp.KeySpace:
		return '_'
	case kc == jp.KeyEnter:
		return 'E'
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
