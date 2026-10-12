//go:build race

package document

import "time"

// The race detector makes a parser worker, this test binary run again, several
// times slower, and CI runners share their cores between the packages under
// test: the product deadline then stops healthy parses. Tests that check the
// deadline itself set it explicitly.
func init() { parseDeadline = 2 * time.Minute }
