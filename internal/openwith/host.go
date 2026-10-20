//go:build !js

package openwith

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sync"

	"nimbleterminal.dev/ntopenwith"
)

const (
	// revealID is the chooser's last Open row, the file shown in its folder.
	revealID = "reveal"
	// copyID is the pager's Copy path row, never an app.
	copyID = "copy"
)

type nativeHost struct {
	src  ntopenwith.Source
	save func(ntopenwith.Preferences) error

	mu sync.Mutex // serializes changes and their saves
	r  *ntopenwith.Resolver
}

// NewHost answers gloss's chooser from src and the saved prefs. Each change
// is passed to save, and kept only when save succeeds. The returned Host is
// also a Manager.
func NewHost(src ntopenwith.Source, prefs ntopenwith.Preferences, save func(ntopenwith.Preferences) error) Host {
	return &nativeHost{src: src, save: save, r: ntopenwith.NewResolver(src, prefs)}
}

func (h *nativeHost) resolver() *ntopenwith.Resolver {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.r
}

// target is t as ntopenwith sees it; a kind alone is asked about by extension.
func target(t Target) ntopenwith.Target {
	nt := ntopenwith.Target{Path: t.Path, Kind: t.Kind, Directory: t.Directory, Text: Editable(t.Kind)}
	if t.Path == "" {
		nt.Ext = ExtOf(t.Kind)
	}
	return nt
}

func revealName() string {
	switch runtime.GOOS {
	case "darwin":
		return "Reveal in Finder"
	case "windows":
		return "Reveal in Explorer"
	}
	return "Open containing folder"
}

func (h *nativeHost) Choices(t Target, v Verb) ([]Choice, error) {
	var out []Choice
	if !t.Directory {
		apps, err := h.resolver().Choices(context.Background(), target(t), v)
		if err != nil {
			return nil, err
		}
		for _, a := range apps {
			out = append(out, Choice{ID: a.ID, Name: a.Name, Default: a.Default, Available: true, Terminal: a.Terminal})
		}
	}
	if v == Open {
		out = append(out, Choice{ID: revealID, Name: revealName(), Available: true})
	}
	return out, nil
}

func (h *nativeHost) Launch(t Target, v Verb, id string) (Launched, error) {
	if id == revealID && v == Open {
		if err := ntopenwith.Reveal(t.Path); err != nil {
			return Launched{}, err
		}
		return Launched{Name: revealName()}, nil
	}
	if t.Directory {
		return Launched{}, errors.New("directories can only be revealed")
	}
	ctx, r, nt := context.Background(), h.resolver(), target(t)
	var app ntopenwith.App
	fallback := false
	if id == "" {
		choice, err := r.Resolve(ctx, nt, v)
		if err != nil {
			return Launched{}, err
		}
		app, fallback = choice.App, choice.Fallback
	} else {
		apps, err := r.Choices(ctx, nt, v)
		if err != nil {
			return Launched{}, err
		}
		i := slices.IndexFunc(apps, func(a ntopenwith.App) bool { return a.ID == id })
		if i < 0 {
			return Launched{}, fmt.Errorf("app %q is unavailable for this file: %w", id, ntopenwith.ErrNotOffered)
		}
		app = apps[i]
	}
	cmd, err := r.Launch(ctx, nt, v, app)
	if err != nil {
		return Launched{}, err
	}
	l := Launched{Name: app.Name}
	if app.Terminal {
		l.Cmd = cmd
	}
	if fallback {
		l.Name += " (saved app unavailable)"
	}
	return l, nil
}

// change applies edit to a resolver over a copy of the preferences and keeps
// it only when the result is saved.
func (h *nativeHost) change(edit func(*ntopenwith.Resolver) error) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	next := ntopenwith.NewResolver(h.src, h.r.Preferences())
	if err := edit(next); err != nil {
		return err
	}
	if err := h.save(next.Preferences()); err != nil {
		return err
	}
	h.r = next
	return nil
}

// SetDefault saves id under t.Kind, after checking that it is offered for t
// itself: the file the chooser listed, which can be offered apps its kind's
// stand-in extension is not.
func (h *nativeHost) SetDefault(t Target, v Verb, id string) error {
	if !slices.Contains(Kinds, t.Kind) || t.Directory || id == revealID || id == copyID {
		return fmt.Errorf("app cannot be the default for %s", t.Kind)
	}
	return h.change(func(r *ntopenwith.Resolver) error {
		return r.SetDefault(context.Background(), target(t), v, id)
	})
}

func (h *nativeHost) Apps() []AppPreference {
	r := h.resolver()
	types := make(map[string]string, len(Kinds))
	for _, kind := range Kinds {
		types[kind] = ExtOf(kind)
	}
	apps, err := r.AllApps(context.Background(), types)
	if err != nil {
		return nil
	}
	out := make([]AppPreference, 0, len(apps))
	for _, a := range apps {
		if a.Terminal {
			continue // Terminal editors cannot be hidden.
		}
		out = append(out, AppPreference{ID: a.ID, Name: a.Name, Enabled: !r.IsHidden(a.ID)})
	}
	return out
}

func (h *nativeHost) SetEnabled(id string, enabled bool) error {
	if id == revealID || id == ntopenwith.SystemID {
		return fmt.Errorf("app %q cannot be managed", id)
	}
	return h.change(func(r *ntopenwith.Resolver) error { return r.SetHidden(id, !enabled) })
}
