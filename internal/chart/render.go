package chart

import (
	"fmt"
	"strings"

	"github.com/NimbleMarkets/ntcharts/v2/spec"
)

// RenderText draws s as terminal text, s.Height lines of at most s.Width
// cells, the way flint-tui does.
func RenderText(s spec.Spec) (string, error) {
	model, err := spec.Build(s)
	if err != nil {
		return "", err
	}
	v, ok := model.(interface{ View() string })
	if !ok {
		return "", fmt.Errorf("chart model %T has no view", model)
	}
	lines := strings.Split(strings.TrimRight(v.View(), "\n"), "\n")
	for len(lines) < s.Height {
		lines = append(lines, "")
	}
	return strings.Join(lines[:s.Height], "\n"), nil
}
