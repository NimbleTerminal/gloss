package document

import "nimbleterminal.dev/gloss/internal/theme"

// testTheme resolves a theme name as gloss's callers do before they load.
func testTheme(name string) Theme {
	return Theme{Name: name, Dark: theme.Dark(name), Palette: theme.Typeset(name)}
}
