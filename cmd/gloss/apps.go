package main

import (
	"encoding/json"
	"fmt"
	"io"
	"nimbleterminal.dev/gloss/internal/openwith"
	"nimbleterminal.dev/gloss/internal/theme"
	svg "nimbleterminal.dev/ntsvg"
	"strings"
)

type listedApp struct {
	ID        string `json:"id" doc:"Stable app identifier: system, reveal, terminal-editor, mac:…, xdg:…, win:…."`
	Name      string `json:"name" doc:"Human-readable app name."`
	Available bool   `json:"available" doc:"Always true: only apps that can be launched are listed."`
	Terminal  bool   `json:"terminal" doc:"Runs in the terminal, which it takes over until it exits."`
}
type appsListing struct {
	Kind    string      `json:"kind" doc:"Canonical gloss format name."`
	Verb    string      `json:"verb" doc:"Which action the row is for." enum:"open,edit"`
	Default string      `json:"default" doc:"Effective default app identifier: system (open rows only), an app ID such as mac:…, xdg:… or win:…, a bare terminal-app ID such as vim, or empty when there is no default: E opens the chooser, or nothing can edit it."`
	Apps    []listedApp `json:"apps" doc:"Visible compatible apps in chooser order; open rows end with reveal, and edit rows of text kinds can hold terminal-editor or other terminal apps."`
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
	verbs := []openwith.Verb{openwith.Open, openwith.Edit}
	rows := make([]appsListing, 0, len(verbs)*len(openwith.Kinds))
	for _, kind := range openwith.Kinds {
		for _, verb := range verbs {
			choices, err := host.Choices(openwith.Target{Kind: kind}, verb)
			if err != nil {
				return err
			}
			row := appsListing{Kind: kind, Verb: string(verb), Apps: []listedApp{}}
			for _, c := range choices {
				if c.Default {
					row.Default = c.ID
				}
				row.Apps = append(row.Apps, listedApp{c.ID, c.Name, c.Available, c.Terminal})
			}
			rows = append(rows, row)
		}
	}
	if jsonOutput {
		return json.NewEncoder(out).Encode(rows)
	}
	// App IDs and names come from the operating system: data, not terminal input.
	var b strings.Builder
	for _, row := range rows {
		fmt.Fprintf(&b, "%s %s (default: %s)\n", row.Kind, row.Verb, svg.SanitizeForTerminal(row.Default))
		for _, a := range row.Apps {
			suffix := ""
			if !a.Available {
				suffix = " (unavailable)"
			}
			fmt.Fprintf(&b, "  %s — %s%s\n", svg.SanitizeForTerminal(a.ID), svg.SanitizeForTerminal(a.Name), suffix)
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
