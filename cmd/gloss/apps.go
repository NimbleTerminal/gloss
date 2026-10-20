package main

import (
	"encoding/json"
	"fmt"
	"io"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/gloss/internal/theme"
	"strings"
)

type listedApp struct {
	ID        string `json:"id" doc:"Stable compiled-in app identifier."`
	Name      string `json:"name" doc:"Human-readable app name."`
	Available bool   `json:"available" doc:"Whether the native opener is installed."`
}
type appsListing struct {
	Kind    string      `json:"kind" doc:"Canonical gloss format name."`
	Default string      `json:"default" doc:"Effective default app identifier."`
	Apps    []listedApp `json:"apps" doc:"Visible compatible apps in chooser order."`
}

func nativeOpenWith() (openwith.Host, error) {
	dir, err := theme.Dir()
	if err != nil {
		return nil, err
	}
	prefs, invalid, err := openwith.LoadPreferences(dir)
	if err != nil {
		return nil, err
	}
	// A damaged file is reported, not fatal; saving then refuses to replace it.
	return openwith.NewHost(openwith.Source(), prefs, openwith.Saver(dir, invalid)), invalid
}
func writeApps(out io.Writer, host openwith.Host, jsonOutput bool) error {
	rows := make([]appsListing, 0, len(openwith.Kinds))
	for _, kind := range openwith.Kinds {
		choices, err := host.Choices(openwith.Target{Kind: kind}, openwith.Open)
		if err != nil {
			return err
		}
		row := appsListing{Kind: kind, Apps: []listedApp{}}
		for _, c := range choices {
			if c.Default {
				row.Default = c.ID
			}
			row.Apps = append(row.Apps, listedApp{c.ID, c.Name, c.Available})
		}
		rows = append(rows, row)
	}
	if jsonOutput {
		return json.NewEncoder(out).Encode(rows)
	}
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s (default: %s)\n", row.Kind, row.Default)
		for _, a := range row.Apps {
			suffix := ""
			if !a.Available {
				suffix = " (unavailable)"
			}
			fmt.Fprintf(&b, "  %s — %s%s\n", a.ID, a.Name, suffix)
		}
	}
	_, err := io.WriteString(out, b.String())
	return err
}

// configureOpenWith keeps native launch capability out of noninteractive hosts.
func configureOpenWith(opts *options, discover func() (openwith.Host, error)) error {
	if opts.Serve || opts.PickWeb || opts.Info || opts.Text || opts.Grep != nil || opts.Output != "" || opts.OutputDir != "" {
		opts.OpenWith = nil
		return nil
	}
	host, err := discover()
	opts.OpenWith = host
	return err
}
