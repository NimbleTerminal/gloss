package app

import "nimbleterminal.dev/gloss/internal/i18n"

// infoLabel is a heading or label of the details box in the language in
// use. document writes them in English, and --info --json keeps them so as
// its keys; only the box translates them, as it draws them. A label with no
// translation is shown as it came.
func infoLabel(label string) string {
	switch label {
	case "Animated":
		return i18n.T("app.info.label.animated", "Animated")
	case "Application":
		return i18n.T("app.info.label.application", "Application")
	case "Areas":
		return i18n.T("app.info.label.areas", "Areas")
	case "Artist":
		return i18n.T("app.info.label.artist", "Artist")
	case "Attribution":
		return i18n.T("app.info.label.attribution", "Attribution")
	case "Audio":
		return i18n.T("app.info.label.audio", "Audio")
	case "Author":
		return i18n.T("app.info.label.author", "Author")
	case "Body font":
		return i18n.T("app.info.label.bodyFont", "Body font")
	case "Bounds":
		return i18n.T("app.info.label.bounds", "Bounds")
	case "Camera":
		return i18n.T("app.info.label.camera", "Camera")
	case "Cells":
		return i18n.T("app.info.label.cells", "Cells")
	case "Changed":
		return i18n.T("app.info.label.changed", "Changed")
	case "Changed by":
		return i18n.T("app.info.label.changedBy", "Changed by")
	case "Characters":
		return i18n.T("app.info.label.characters", "Characters")
	case "Code font":
		return i18n.T("app.info.label.codeFont", "Code font")
	case "Color":
		return i18n.T("app.info.label.color", "Color")
	case "Columns":
		return i18n.T("app.info.label.columns", "Columns")
	case "Company":
		return i18n.T("app.info.label.company", "Company")
	case "Copyright":
		return i18n.T("app.info.label.copyright", "Copyright")
	case "Created":
		return i18n.T("app.info.label.created", "Created")
	case "Creator":
		return i18n.T("app.info.label.creator", "Creator")
	case "Declared size":
		return i18n.T("app.info.label.declaredSize", "Declared size")
	case "Description":
		return i18n.T("app.info.label.description", "Description")
	case "Designer":
		return i18n.T("app.info.label.designer", "Designer")
	case "Details":
		return i18n.T("app.info.label.details", "Details")
	case "Diagram":
		return i18n.T("app.info.label.diagram", "Diagram")
	case "Dimensions":
		return i18n.T("app.info.label.dimensions", "Dimensions")
	case "Document":
		return i18n.T("app.info.label.document", "Document")
	case "Drawing":
		return i18n.T("app.info.label.drawing", "Drawing")
	case "Drawn":
		return i18n.T("app.info.label.drawn", "Drawn")
	case "Duration":
		return i18n.T("app.info.label.duration", "Duration")
	case "Elements":
		return i18n.T("app.info.label.elements", "Elements")
	case "Exposure":
		return i18n.T("app.info.label.exposure", "Exposure")
	case "Extent":
		return i18n.T("app.info.label.extent", "Extent")
	case "Features":
		return i18n.T("app.info.label.features", "Features")
	case "File":
		return i18n.T("app.info.label.file", "File")
	case "Font fallback":
		return i18n.T("app.info.label.fontFallback", "Font fallback")
	case "Format":
		return i18n.T("app.info.label.format", "Format")
	case "Frames":
		return i18n.T("app.info.label.frames", "Frames")
	case "Grist document":
		return i18n.T("app.info.label.gristDocument", "Grist document")
	case "Headings":
		return i18n.T("app.info.label.headings", "Headings")
	case "Height":
		return i18n.T("app.info.label.height", "Height")
	case "Image":
		return i18n.T("app.info.label.image", "Image")
	case "Images":
		return i18n.T("app.info.label.images", "Images")
	case "JSON":
		return i18n.T("app.info.label.json", "JSON")
	case "Kernel":
		return i18n.T("app.info.label.kernel", "Kernel")
	case "Keys":
		return i18n.T("app.info.label.keys", "Keys")
	case "Keywords":
		return i18n.T("app.info.label.keywords", "Keywords")
	case "Kitty graphics":
		return i18n.T("app.info.label.kittyGraphics", "Kitty graphics")
	case "Language":
		return i18n.T("app.info.label.language", "Language")
	case "Layout":
		return i18n.T("app.info.label.layout", "Layout")
	case "Lens":
		return i18n.T("app.info.label.lens", "Lens")
	case "License":
		return i18n.T("app.info.label.license", "License")
	case "Lines":
		return i18n.T("app.info.label.lines", "Lines")
	case "Links":
		return i18n.T("app.info.label.links", "Links")
	case "Location":
		return i18n.T("app.info.label.location", "Location")
	case "Map":
		return i18n.T("app.info.label.map", "Map")
	case "Mesh":
		return i18n.T("app.info.label.mesh", "Mesh")
	case "Meshes":
		return i18n.T("app.info.label.meshes", "Meshes")
	case "Missing glyphs":
		return i18n.T("app.info.label.missingGlyphs", "Missing glyphs")
	case "Model":
		return i18n.T("app.info.label.model", "Model")
	case "Modified":
		return i18n.T("app.info.label.modified", "Modified")
	case "Multiplexer":
		return i18n.T("app.info.label.multiplexer", "Multiplexer")
	case "Name":
		return i18n.T("app.info.label.name", "Name")
	case "Names":
		return i18n.T("app.info.label.names", "Names")
	case "Not laid out":
		return i18n.T("app.info.label.notLaidOut", "Not laid out")
	case "Not shown":
		return i18n.T("app.info.label.notShown", "Not shown")
	case "Note":
		return i18n.T("app.info.label.note", "Note")
	case "Notebook":
		return i18n.T("app.info.label.notebook", "Notebook")
	case "Objects":
		return i18n.T("app.info.label.objects", "Objects")
	case "Orientation":
		return i18n.T("app.info.label.orientation", "Orientation")
	case "Outputs":
		return i18n.T("app.info.label.outputs", "Outputs")
	case "Page":
		return i18n.T("app.info.label.page", "Page")
	case "Page size":
		return i18n.T("app.info.label.pageSize", "Page size")
	case "Pages":
		return i18n.T("app.info.label.pages", "Pages")
	case "Parts":
		return i18n.T("app.info.label.parts", "Parts")
	case "Path":
		return i18n.T("app.info.label.path", "Path")
	case "Photo":
		return i18n.T("app.info.label.photo", "Photo")
	case "Pictures":
		return i18n.T("app.info.label.pictures", "Pictures")
	case "Places":
		return i18n.T("app.info.label.places", "Places")
	case "Points":
		return i18n.T("app.info.label.points", "Points")
	case "Positions":
		return i18n.T("app.info.label.positions", "Positions")
	case "Producer":
		return i18n.T("app.info.label.producer", "Producer")
	case "Program":
		return i18n.T("app.info.label.program", "Program")
	case "Record":
		return i18n.T("app.info.label.record", "Record")
	case "Record keys":
		return i18n.T("app.info.label.recordKeys", "Record keys")
	case "Records":
		return i18n.T("app.info.label.records", "Records")
	case "Rows":
		return i18n.T("app.info.label.rows", "Rows")
	case "Schema version":
		return i18n.T("app.info.label.schemaVersion", "Schema version")
	case "Screen":
		return i18n.T("app.info.label.screen", "Screen")
	case "Sheets":
		return i18n.T("app.info.label.sheets", "Sheets")
	case "Shown":
		return i18n.T("app.info.label.shown", "Shown")
	case "Size":
		return i18n.T("app.info.label.size", "Size")
	case "Software":
		return i18n.T("app.info.label.software", "Software")
	case "Subject":
		return i18n.T("app.info.label.subject", "Subject")
	case "Subtitle streams":
		return i18n.T("app.info.label.subtitleStreams", "Subtitle streams")
	case "Subtitles":
		return i18n.T("app.info.label.subtitles", "Subtitles")
	case "Surface area":
		return i18n.T("app.info.label.surfaceArea", "Surface area")
	case "Table":
		return i18n.T("app.info.label.table", "Table")
	case "Tables":
		return i18n.T("app.info.label.tables", "Tables")
	case "Taken":
		return i18n.T("app.info.label.taken", "Taken")
	case "Terminal":
		return i18n.T("app.info.label.terminal", "Terminal")
	case "Text":
		return i18n.T("app.info.label.text", "Text")
	case "Text theme":
		return i18n.T("app.info.label.textTheme", "Text theme")
	case "Textured parts":
		return i18n.T("app.info.label.texturedParts", "Textured parts")
	case "Thumbnail":
		return i18n.T("app.info.label.thumbnail", "Thumbnail")
	case "Tile count":
		return i18n.T("app.info.label.tileCount", "Tile count")
	case "Tiles":
		return i18n.T("app.info.label.tiles", "Tiles")
	case "Time zone":
		return i18n.T("app.info.label.timeZone", "Time zone")
	case "Title":
		return i18n.T("app.info.label.title", "Title")
	case "Top level":
		return i18n.T("app.info.label.topLevel", "Top level")
	case "Triangles":
		return i18n.T("app.info.label.triangles", "Triangles")
	case "Type":
		return i18n.T("app.info.label.type", "Type")
	case "Up axis":
		return i18n.T("app.info.label.upAxis", "Up axis")
	case "Video":
		return i18n.T("app.info.label.video", "Video")
	case "View box":
		return i18n.T("app.info.label.viewBox", "View box")
	case "Warning":
		return i18n.T("app.info.label.warning", "Warning")
	case "Width":
		return i18n.T("app.info.label.width", "Width")
	case "Words":
		return i18n.T("app.info.label.words", "Words")
	case "Workbook":
		return i18n.T("app.info.label.workbook", "Workbook")
	case "Zooms":
		return i18n.T("app.info.label.zooms", "Zooms")
	}
	return label
}
