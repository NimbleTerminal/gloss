//go:build race

package document

import "time"

// The race detector makes a parser worker several
// times slower, and CI runners share their cores between the packages under
// test: the product deadline then stops healthy parses. This applies to all
// race builds, including packages importing document. Tests that check the
// deadline itself set it explicitly.
func init() { parseDeadline = 2 * time.Minute }
