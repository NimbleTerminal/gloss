package browse_test

import (
	"testing"

	"nimbleterminal.dev/gloss/internal/browse/browsecfg"
	"nimbleterminal.dev/gloss/internal/browse/browsetest"
)

func TestScripts(t *testing.T) {
	browsetest.RunDir(t, browsecfg.Config(), "testdata/scripts")
}
