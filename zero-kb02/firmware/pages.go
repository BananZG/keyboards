package main

import (
	keyboard "github.com/sago35/tinygo-keyboard"
	"github.com/sago35/tinygo-keyboard/keycodes"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
)

// Page indices - order determines layout
const (
	PageLowerA  = iota // row1: a-d, row2: e-h
	PageLowerI         // row1: i-l, row2: m-p
	PageLowerQ         // row1: q-t, row2: u-x
	PageLowerY         // row1: y-z + punctuation, row2: more punctuation
	PageUpperA         // row1: A-D, row2: E-H
	PageUpperI         // row1: I-L, row2: M-P
	PageUpperQ         // row1: Q-T, row2: U-X
	PageUpperY         // row1: Y-Z + symbols, row2: more symbols
	PageNumbers        // row1: 1-4, row2: 5-8
	PageSymbols        // row1: !@#$, row2: %&*(
	PageNav            // row1: arrows, row2: home/end/pgup/pgdn
	TotalPages         // sentinel = 11

	// FNLayerID is active when FN key is held.
	// row1: page shortcuts 1-4 | row2: page shortcuts 5-8 | row3: page shortcuts 9-11
	FNLayerID = TotalPages
)

// PageManager tracks current page and FN state.
// Allocated on stack in run() - no heap usage.
type PageManager struct {
	currentPage   uint8
	fnPressed     bool
	displayUpdate bool
}

func (pm *PageManager) Init() {
	pm.currentPage = PageLowerA
	pm.fnPressed = false
	pm.displayUpdate = true // show current page on startup
}

// EffectivePage returns FNLayerID when FN is held, otherwise currentPage.
func (pm *PageManager) EffectivePage() int {
	if pm.fnPressed {
		return FNLayerID
	}
	return int(pm.currentPage)
}

func (pm *PageManager) SetPage(page int) {
	if page >= 0 && page < TotalPages {
		changed := uint8(page) != pm.currentPage
		if changed {
			pm.currentPage = uint8(page)
		}
		// Always refresh when FN is held so re-selecting the same page
		// resets the preview timer.
		if changed || pm.fnPressed {
			pm.displayUpdate = true
		}
	}
}

func (pm *PageManager) SetFN(pressed bool) {
	if pm.fnPressed != pressed {
		pm.fnPressed = pressed
		pm.displayUpdate = true
	}
}

// ConsumeDisplayUpdate returns true once if a display refresh is needed.
func (pm *PageManager) ConsumeDisplayUpdate() bool {
	if pm.displayUpdate {
		pm.displayUpdate = false
		return true
	}
	return false
}

// pageNames maps page index to LCD display name.
var pageNames = [TotalPages]string{
	PageLowerA:  "a-h",
	PageLowerI:  "i-p",
	PageLowerQ:  "q-x",
	PageLowerY:  "y-z",
	PageUpperA:  "A-H",
	PageUpperI:  "I-P",
	PageUpperQ:  "Q-X",
	PageUpperY:  "Y-Z",
	PageNumbers: "0-9",
	PageSymbols: "9-0",
	PageNav:     "NAV",
}

// GetPageName returns the LCD display name for a page.
func GetPageName(page int) string {
	if page == FNLayerID {
		return "FN"
	}
	if page >= 0 && page < TotalPages {
		return pageNames[page]
	}
	return "?"
}

// Keycode is an alias for the tinygo-keyboard Keycode type.
type Keycode = keyboard.Keycode

// Row 3 fixed keys - same on every page
//
//	[0] FN key (handled by callback, not here)
//	[1] Backspace
//	[2] Mouse Left  (handled by mouse.Port)
//	[3] Mouse Right (handled by mouse.Port)
const (
	KeyRow3FN        = jp.KeyMod0 // FN - triggers page layer
	KeyRow3Backspace = jp.KeyBackspace
	// MouseLeft / MouseRight handled outside keymap
)

// shiftMask is kept for keycodeToRune detection; do NOT use it in keyPages.
// In the regular matrix key-press path the library ignores ShiftMask bits —
// use lsft() instead, which encodes Shift+key via the TypeLxxx modifier path.
const shiftMask = keycodes.ShiftMask

// lsft encodes a keycode as LeftShift+key using the tinygo-keyboard TypeLxxx
// modifier mechanism. Unlike key|shiftMask (only works in macros), this is
// handled in the normal noneToPress / pressToRelease paths.
func lsft(k Keycode) Keycode {
	return Keycode(keycodes.TypeXSft) | (k & 0xFF)
}

// keyPages contains all keycodes for rows 0 and 1 of each page.
// Row 2 (index 2) is always fixed (FN, Backspace, MouseL, MouseR) and handled
// separately in main.go - the zero values here are never read for row 2.
//
// Layout: keyPages[page][row][col]  - row 0 or 1, col 0-3
var keyPages = [TotalPages][2][4]Keycode{
	// PageLowerA
	{
		{jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD},
		{jp.KeyE, jp.KeyF, jp.KeyG, jp.KeyH},
	},
	// PageLowerI
	{
		{jp.KeyI, jp.KeyJ, jp.KeyK, jp.KeyL},
		{jp.KeyM, jp.KeyN, jp.KeyO, jp.KeyP},
	},
	// PageLowerQ
	{
		{jp.KeyQ, jp.KeyR, jp.KeyS, jp.KeyT},
		{jp.KeyU, jp.KeyV, jp.KeyW, jp.KeyX},
	},
	// PageLowerY: y-z then common punctuation
	{
		{jp.KeyY, jp.KeyZ, jp.KeySpace, jp.KeyEnter},
		{jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyMinus},
	},
	// PageUpperA - lsft() produces A-H via TypeLxxx Shift modifier path
	{
		{lsft(jp.KeyA), lsft(jp.KeyB), lsft(jp.KeyC), lsft(jp.KeyD)},
		{lsft(jp.KeyE), lsft(jp.KeyF), lsft(jp.KeyG), lsft(jp.KeyH)},
	},
	// PageUpperI
	{
		{lsft(jp.KeyI), lsft(jp.KeyJ), lsft(jp.KeyK), lsft(jp.KeyL)},
		{lsft(jp.KeyM), lsft(jp.KeyN), lsft(jp.KeyO), lsft(jp.KeyP)},
	},
	// PageUpperQ
	{
		{lsft(jp.KeyQ), lsft(jp.KeyR), lsft(jp.KeyS), lsft(jp.KeyT)},
		{lsft(jp.KeyU), lsft(jp.KeyV), lsft(jp.KeyW), lsft(jp.KeyX)},
	},
	// PageUpperY - Y-Z then symbols
	{
		{lsft(jp.KeyY), lsft(jp.KeyZ), jp.KeySpace, jp.KeyEnter},
		{jp.KeyColon, jp.KeySemicolon, lsft(jp.KeyMinus), jp.KeyHat},
	},
	// PageNumbers
	{
		{jp.Key1, jp.Key2, jp.Key3, jp.Key4},
		{jp.Key5, jp.Key6, jp.Key7, jp.Key8},
	},
	// PageNumbers2: 9, 0 + calculation helpers (JIS: 8|shift=(, 9|shift=), minus|shift==)
	{
		{jp.Key9, jp.Key0, jp.KeyMinus, jp.KeyPeriod},
		{lsft(jp.Key8), lsft(jp.Key9), lsft(jp.KeyMinus), jp.KeyComma},
	},
	// PageNav: arrows + home/end/pgup/pgdn
	{
		{jp.KeyHome, jp.KeyUp, jp.KeyEnd, jp.KeyPageUp},
		{jp.KeyLeft, jp.KeyDown, jp.KeyRight, jp.KeyPageDown},
	},
}

// fnPageKeys maps FN row/col to the page index to jump to.
// Row 0 col 0-3  => pages 0-3
// Row 1 col 0-3  => pages 4-7
// Row 2 col 1-3  => pages 8-10  (col 0 = FN key itself, skip)
var fnPageKeys = [3][4]int{
	{PageLowerA, PageLowerI, PageLowerQ, PageLowerY},
	{PageUpperA, PageUpperI, PageUpperQ, PageUpperY},
	{-1, PageNumbers, PageSymbols, PageNav}, // -1 = FN key position, no action
}

// GetPageKeycode returns the keycode for rows 0-1 on the given page.
// Returns 0 for row 2 (fixed row, handled separately).
func GetPageKeycode(page int, row int, col int) Keycode {
	if row == 2 || col < 0 || col > 3 {
		return 0
	}
	if page >= 0 && page < TotalPages {
		return keyPages[page][row][col]
	}
	return 0
}

// GetFNPageTarget returns which page index to jump to when a key is pressed
// while FN is held. Returns -1 if no page jump for that position.
func GetFNPageTarget(row int, col int) int {
	if row < 0 || row > 2 || col < 0 || col > 3 {
		return -1
	}
	return fnPageKeys[row][col]
}
