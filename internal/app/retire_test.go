package app

import "github.com/NimbleTerminal/gloss/internal/animate"

func init() { animate.RetireDelay = 0 } // Tests drain commands synchronously; a wait would only slow them.
