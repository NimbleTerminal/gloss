//go:build !(js && wasm)

package document

import "io/fs"

// videoURL is the browser's; a native program plays a video from its path,
// so one held in memory, as in the embedded gallery, cannot be played.
func videoURL(fs.FS, string) (string, func(), error) { return "", nil, ErrVideoNative }

// blobURL is the browser's too; natively nothing comes this far.
func blobURL([]byte, string) (string, func()) { return "", func() {} }
