package document

import "nimbleterminal.dev/gloss/internal/chart"

// chartFields describes a chart document for the info box.
func chartFields(doc chart.Document) []Field {
	format := map[chart.Kind]string{chart.KindFlint: "Flint chart", chart.KindEnvelope: "Compiled chart", chart.KindSpec: "ntcharts spec", chart.KindError: "Chart error"}[doc.Kind]
	data := "inline rows"
	if doc.Unbound {
		data = "none; needs a table"
	}
	return Section("Chart", Field{"Format", format}, Field{"Data", data}, Field{"Size", byteSize(len(doc.Raw))})
}
