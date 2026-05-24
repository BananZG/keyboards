package main

import (
	keyboard "github.com/sago35/tinygo-keyboard"
	"github.com/sago35/tinygo-keyboard/keycodes"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
)

// Vial layer indices — one combined layer per key set.
// Layers 0-2 also define the rotary encoder mode (VOL / SCROLL / BRIGHT).
const (
	LayerAG  = 0 // a–g + Shift  [rotary: Volume]
	LayerHN  = 1 // h–n + Shift  [rotary: Scroll]
	LayerOU  = 2 // o–u + Shift  [rotary: Brightness]
	LayerVZ  = 3 // v–z, 0–1 + Shift
	LayerNum = 4 // 2–9
	LayerNav = 5 // Navigation (Home/Up/End/PgUp · Left/Down/Right/PgDn)
)

// Physical key index layout (row-major, 4 cols × 3 rows):
//
//	Row 0 (idx 0-3):  K1  K2  K3  K4    — letter / number / nav keys
//	Row 1 (idx 4-7):  Sft K6  K7  K8    — Shift in col 0, letter in cols 1-3
//	Row 2 (idx 8-11): FN  Del MsL MsR   — always fixed across every layer

// layerKeys holds firmware-default keycodes for all six layers.
// The first 8 indices (rows 0-1) vary per layer; the last 4 (row 2) are
// identical on every layer (FN=0, Backspace, MouseLeft, MouseRight).
// Layers 0-3 place Left Shift at index 4; layer 4 uses index 4 for "6".
var layerKeys = [keyboard.LayerCount][12]keyboard.Keycode{
	LayerAG: {
		jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyD, // row 0: a b c d
		jp.KeyLeftShift, jp.KeyE, jp.KeyF, jp.KeyG, // row 1: Sft e f g
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight, // row 2: FN Del MsL MsR
	},
	LayerHN: {
		jp.KeyH, jp.KeyI, jp.KeyJ, jp.KeyK,
		jp.KeyLeftShift, jp.KeyL, jp.KeyM, jp.KeyN,
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
	},
	LayerOU: {
		jp.KeyO, jp.KeyP, jp.KeyQ, jp.KeyR,
		jp.KeyLeftShift, jp.KeyS, jp.KeyT, jp.KeyU,
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
	},
	LayerVZ: {
		jp.KeyV, jp.KeyW, jp.KeyX, jp.KeyY,
		jp.KeyLeftShift, jp.KeyZ, jp.Key0, jp.Key1,
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
	},
	LayerNum: {
		jp.Key2, jp.Key3, jp.Key4, jp.Key5,
		jp.Key6, jp.Key7, jp.Key8, jp.Key9,
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
	},
	LayerNav: {
		jp.KeyHome, jp.KeyUp, jp.KeyEnd, jp.KeyPageUp,
		jp.KeyLeft, jp.KeyDown, jp.KeyRight, jp.KeyPageDown,
		0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
	},
}

// layerDisplayNames maps each Vial layer to a centred OLED title string.
// Layers 0-2 include the rotary mode abbreviation (knob acts as VOL/SCR/BRT);
// layers 3-5 show just the key set name.
var layerDisplayNames = [keyboard.LayerCount]string{
	LayerAG:  "L-0  VOL",
	LayerHN:  "L-1  SCR",
	LayerOU:  "L-2  BRT",
	LayerVZ:  "L-3     ",
	LayerNum: "L-4     ",
	LayerNav: "L-5     ",
}

// layerRotaryLabel is a short label for the rotary encoder's bound action on
// each layer.  Non-empty → comet LED effect + the label is meaningful.
// Empty → rotary is unbound on this layer; turning shows a warning on screen.
var layerRotaryLabel = [keyboard.LayerCount]string{
	LayerAG:  "VOL",
	LayerHN:  "SCR",
	LayerOU:  "BRT",
	LayerVZ:  "",
	LayerNum: "",
	LayerNav: "",
}

// rkLayerKeys holds firmware-default keycodes for the rotary encoder keyboard
// (kbIndex 1).  Layers without a named mode use {0,0} (send nothing).
var rkLayerKeys = [keyboard.LayerCount][2]keyboard.Keycode{
	LayerAG:  {jp.KeyMediaVolumeDec, jp.KeyMediaVolumeInc},
	LayerHN:  {jp.WheelDown, jp.WheelUp},
	LayerOU:  {jp.KeyMediaBrightnessDown, jp.KeyMediaBrightnessUp},
	LayerVZ:  {0, 0},
	LayerNum: {0, 0},
	LayerNav: {0, 0},
}

// gkLayerKeys holds firmware-default keycodes for the GPIO keyboard (kbIndex 2).
// Index 0 = joystick button (no keycode; game mode handled by callback).
// Index 1 = rotary encoder button (cycles to the next Vial layer in order).
var gkLayerKeys = [keyboard.LayerCount][2]keyboard.Keycode{
	LayerAG:  {0, jp.KeyTo1},
	LayerHN:  {0, jp.KeyTo2},
	LayerOU:  {0, jp.KeyTo3},
	LayerVZ:  {0, jp.KeyTo4},
	LayerNum: {0, jp.KeyTo5},
	LayerNav: {0, jp.KeyTo0},
}

// fnRow2Keys are the keycodes installed on row-2 indices 9-11 while FN is
// held.  They replace Del/MouseL/MouseR with more useful input keys.
var fnRow2Keys = [3]keyboard.Keycode{
	jp.KeyTab,   // idx 9  (normally Backspace/Del)
	jp.KeySpace, // idx 10 (normally MouseL)
	jp.KeyEnter, // idx 11 (normally MouseR)
}

// Keycode is an alias for the tinygo-keyboard Keycode type, exposed so
// display.go can reference it without importing tinygo-keyboard directly.
type Keycode = keyboard.Keycode

// shiftMask is used by keycodeToRune in display.go for display hint decoding.
const shiftMask = keycodes.ShiftMask
