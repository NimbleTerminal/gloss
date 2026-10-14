// Package animate adapts ntpicture animation data to Gloss's document regions
// and terminal playback policy. Decoding and immutable players live in ntpicture.
package animate

import "nimbleterminal.dev/ntpicture/animation"

// Player is shared by the presentation adapter and document loaders.
type Player = animation.Player
