package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/ntmaps/geocode"
	"nimbleterminal.dev/ntmaps/tile"
)

// The online map provider is the one a user names: gloss has none of its
// own, so it never sends a map server traffic nobody chose. Both need
// --fetch, and both are left out of every headless mode.
const (
	mapTilesVariable       = "GLOSS_MAP_TILES"       // An XYZ tile URL template, with {z}, {x} and {y}.
	mapAttributionVariable = "GLOSS_MAP_ATTRIBUTION" // The credit the tiles' terms ask for.
	geocoderVariable       = "GLOSS_GEOCODER"        // A Nominatim-compatible search endpoint.
)

// maxPlaces is how many place lookups a session remembers, so asking again
// asks no server.
const maxPlaces = 256

// mapProvider reads the provider from the environment for the viewer: tiles
// under its maps, and place lookup. It is nil when neither is named.
func mapProvider(getenv func(string) string, agent string) (*app.MapProvider, error) {
	tiles, finder := strings.TrimSpace(getenv(mapTilesVariable)), strings.TrimSpace(getenv(geocoderVariable))
	if tiles == "" && finder == "" {
		return nil, nil
	}
	var p app.MapProvider
	if tiles != "" {
		if !webAddress(tiles) {
			return nil, fmt.Errorf("%s must be an http(s) address", mapTilesVariable)
		}
		u, _ := url.Parse(tiles)
		host := u.Hostname()
		for _, part := range []string{"{z}", "{x}", "{y}"} {
			if !strings.Contains(tiles, part) {
				return nil, fmt.Errorf("%s must hold {z}, {x} and {y}", mapTilesVariable)
			}
		}
		credit := strings.TrimSpace(getenv(mapAttributionVariable))
		if credit == "" {
			credit = "Tiles: " + host
		}
		if err := validUserAgent(credit); err != nil {
			return nil, fmt.Errorf("%s %w", mapAttributionVariable, err)
		}
		p.Tiles = tile.Provider{Name: "gloss " + host, URL: tiles, Attribution: credit, UserAgent: agent}
	}
	if finder != "" {
		if !webAddress(finder) {
			return nil, fmt.Errorf("%s must be an http(s) address", geocoderVariable)
		}
		p.Geocoder = rememberPlaces(&geocode.Nominatim{Endpoint: finder, UserAgent: agent})
	}
	return &p, nil
}

// rememberPlaces asks g each query once a session: a place found, or not
// found, is answered again from memory. Nominatim's own pacing still spaces
// the questions it is asked.
func rememberPlaces(g geocode.Geocoder) geocode.Geocoder {
	type answer struct {
		place geocode.Result
		err   error
	}
	var mu sync.Mutex
	known := map[string]answer{}
	return geocode.Func(func(ctx context.Context, query string) (geocode.Result, error) {
		key := strings.ToLower(strings.Join(strings.Fields(query), " "))
		mu.Lock()
		a, ok := known[key]
		mu.Unlock()
		if ok {
			return a.place, a.err
		}
		place, err := g.Geocode(ctx, query)
		if err == nil || errors.Is(err, geocode.ErrNotFound) {
			mu.Lock()
			if len(known) >= maxPlaces {
				clear(known)
			}
			known[key] = answer{place, err}
			mu.Unlock()
		}
		return place, err
	})
}
