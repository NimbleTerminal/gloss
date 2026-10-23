package openwith

// Kinds are the canonical preference keys, in listing order.
var Kinds = []string{"image/*", "svg", "pdf", "stl", "3mf", "obj", "ply", "docx", "xlsx", "grist", "csv", "json", "ipynb", "html", "text", "markdown", "mermaid", "geojson", "gpx"}

// Kind maps loader image kinds to their shared preference key.
func Kind(kind string) string {
	switch kind {
	case "png", "jpeg", "gif", "webp", "bmp", "tiff", "heic", "heif", "jxl", "image":
		return "image/*"
	}
	return kind
}

// Editable reports formats whose original file is text, not a converted view.
func Editable(kind string) bool {
	switch kind {
	case "text", "markdown", "json", "html", "svg", "mermaid", "obj", "csv", "ipynb", "geojson", "gpx":
		return true
	}
	return false
}

// ExtOf is the extension that stands for a kind when there is no file to ask
// about, or "" for a kind gloss does not list.
func ExtOf(kind string) string {
	switch kind {
	case "image/*":
		return ".png"
	case "text":
		return ".txt"
	case "markdown":
		return ".md"
	case "mermaid":
		return ".mmd"
	case "svg", "pdf", "stl", "3mf", "obj", "ply", "docx", "xlsx", "grist", "csv", "json", "ipynb", "html", "geojson", "gpx":
		return "." + kind
	}
	return ""
}
