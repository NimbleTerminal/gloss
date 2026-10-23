// Package theme defines the palettes shared by terminal text and typeset pages.
package theme

import "image/color"

type Palette struct {
	Background, Foreground, Muted, Border, Code, Header, Accent color.RGBA
	Selection                                                   color.RGBA // Behind selected typeset text.
	Hover                                                       color.RGBA // Behind the typeset link under the pointer; zero blends Selection into Background.
}

func rgb(n uint32) color.RGBA {
	return color.RGBA{R: uint8(n >> 16), G: uint8(n >> 8), B: uint8(n), A: 255}
}

// Resolve keeps exports deterministic: an unresolved or automatic theme is light.
// The interactive application resolves auto from the terminal, with a dark fallback.
func Resolve(name string) string {
	if _, ok := catalog.Load(name); ok {
		return name
	}
	if name == "dark" {
		return "dark"
	}
	return "light"
}

func Get(name string) Palette {
	if s, ok := catalog.Load(name); ok {
		return s.(snapshot).palette
	}
	if Resolve(name) == "dark" {
		return Palette{Background: rgb(0x181a1e), Foreground: rgb(0xe6edf3), Muted: rgb(0x9da7b3), Border: rgb(0x424a55), Code: rgb(0x242830), Header: rgb(0x303641), Accent: rgb(0xb9a5ff), Selection: rgb(0x264f78)}
	}
	return Palette{Background: rgb(0xffffff), Foreground: rgb(0x232a32), Muted: rgb(0x596575), Border: rgb(0xd2d2d2), Code: rgb(0xf5f7f9), Header: rgb(0xedf1f5), Accent: rgb(0x55408c), Selection: rgb(0xb4d5fe)}
}
