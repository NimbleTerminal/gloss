package browse_test

import (
	"testing"

	"github.com/NimbleTerminal/gloss/internal/browse/browsecfg"
	"github.com/NimbleTerminal/gloss/internal/browse/browsetest"
)

func TestScripts(t *testing.T) {
	browsetest.RunDir(t, browsecfg.Config(), "testdata/scripts")
}
