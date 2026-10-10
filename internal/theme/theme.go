// Package theme defines the two palettes shared by terminal text and typeset pages.
package theme

import "image/color"

type Palette struct {
	Background, Foreground, Muted, Border, Code, Header, Accent color.RGBA
	Selection                                                   color.RGBA // Behind selected typeset text.
}

func rgb(n uint32) color.RGBA {
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
}

// Resolve keeps exports deterministic: an unresolved or automatic theme is light.
// The interactive application resolves auto from the terminal, with a dark fallback.
func Resolve(name string) string {
	if name == "dark" {
		return "dark"
	}
	return "light"
}

func Get(name string) Palette {
	if Resolve(name) == "dark" {
		return Palette{rgb(0x181a1e), rgb(0xe6edf3), rgb(0x9da7b3), rgb(0x424a55), rgb(0x242830), rgb(0x303641), rgb(0xb9a5ff), rgb(0x264f78)}
	}
	return Palette{rgb(0xffffff), rgb(0x232a32), rgb(0x596575), rgb(0xd2d2d2), rgb(0xf5f7f9), rgb(0xedf1f5), rgb(0x55408c), rgb(0xb4d5fe)}
}
