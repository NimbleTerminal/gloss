package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"nimbleterminal.dev/ntmaps/geocode"
	"nimbleterminal.dev/ntmaps/tile"
)

func TestMapProviderSettings(t *testing.T) {
	env := func(vars map[string]string) func(string) string {
		return func(k string) string { return vars[k] }
	}
	if p, err := mapProvider(env(nil), "gloss/test"); p != nil || err != nil {
		t.Fatalf("nothing named: %v %v", p, err)
	}
	p, err := mapProvider(env(map[string]string{mapTilesVariable: "https://tiles.example/{z}/{x}/{y}.png"}), "gloss/test")
	if err != nil {
		t.Fatal(err)
	}
	tiles := p.Tiles.(tile.Provider)
	if tiles.Attribution != "Tiles: tiles.example" || tiles.UserAgent != "gloss/test" || p.Geocoder != nil {
		t.Fatalf("%+v", tiles)
	}
	for name, vars := range map[string]map[string]string{
		"no {y}":           {mapTilesVariable: "https://tiles.example/{z}/{x}.png"},
		"another scheme":   {mapTilesVariable: "file:///tiles/{z}/{x}/{y}.png"},
		"a control":        {mapTilesVariable: "https://tiles.example/{z}/{x}/{y}.png", mapAttributionVariable: "a\x1bb"},
		"a geocoder path":  {geocoderVariable: "/search"},
		"a geocoder space": {geocoderVariable: "nominatim example"},
	} {
		if _, err := mapProvider(env(vars), "gloss/test"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	p, err = mapProvider(env(map[string]string{geocoderVariable: "https://geo.example/search", mapAttributionVariable: "ignored"}), "gloss/test")
	if err != nil || p.Tiles != nil || p.Geocoder == nil {
		t.Fatalf("geocoder only: %+v %v", p, err)
	}
}

func TestPlacesAreAskedOnce(t *testing.T) {
	asked := map[string]int{}
	g := rememberPlaces(geocode.Func(func(_ context.Context, q string) (geocode.Result, error) {
		asked[q]++
		switch q {
		case "nowhere":
			return geocode.Result{}, geocode.ErrNotFound
		case "offline":
			return geocode.Result{}, errors.New("no network")
		}
		return geocode.Result{Lat: 1, Lng: 2}, nil
	}))
	for _, q := range []string{"Paris", "  paris ", "PARIS", "nowhere", "nowhere", "offline", "offline"} {
		_, _ = g.Geocode(context.Background(), q)
	}
	if asked["Paris"] != 1 || asked["nowhere"] != 1 {
		t.Fatalf("asked %v", asked)
	}
	if asked["offline"] != 2 {
		t.Fatal("a failure was remembered; it should be asked again")
	}
	if _, err := g.Geocode(context.Background(), "nowhere"); !strings.Contains(err.Error(), "not found") {
		t.Fatalf("remembered not-found: %v", err)
	}
}
