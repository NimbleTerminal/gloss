//go:build js && wasm

package document

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall/js"
)

// videoURL makes a blob: URL of a video the page holds in memory, for the
// browser's own <video> element, through ntvideo's DOM backend. revoke frees
// it; the viewer calls it once the video is closed.
func videoURL(files fs.FS, path string) (url string, revoke func(), err error) {
	data, err := readFileFrom(files, path)
	if err != nil {
		return "", nil, err
	}
	bytes := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(bytes, data)
	options := js.Global().Get("Object").New()
	options.Set("type", videoMIME(path))
	blob := js.Global().Get("Blob").New([]any{bytes}, options)
	url = js.Global().Get("URL").Call("createObjectURL", blob).String()
	return url, func() { js.Global().Get("URL").Call("revokeObjectURL", url) }, nil
}

// videoMIME is the type a browser is told a video is, by its name; MP4 when
// the name says nothing.
func videoMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	case ".mkv":
		return "video/x-matroska"
	}
	return "video/mp4"
}
