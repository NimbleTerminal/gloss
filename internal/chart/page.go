package chart

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// echartsScript is Apache ECharts, vendored by scripts/fetch-echarts.sh.
//
//go:embed echarts/echarts.min.js
var echartsScript []byte

// echartsHash names the bundle to the page's policy; made once.
var echartsHash = scriptHash(echartsScript)

// Page is a self-contained HTML page that draws option with the embedded
// ECharts: no script, style or image is fetched. The canvas is laid out at
// OptionWidth by OptionHeight and scaled to the window; tooltips are drawn
// on the canvas, never as HTML. The policy returned beside the page is in
// its meta element and is for a server to send as the
// Content-Security-Policy header: it lets only the page's own two scripts
// run, named by hash, so nothing an option carries can.
func Page(title string, option json.RawMessage) (page []byte, csp string) {
	var boot bytes.Buffer
	boot.WriteString("\nvar option = ")
	boot.WriteString(inlineJSON(option))
	boot.WriteString(";\noption.tooltip = Object.assign({}, option.tooltip, {renderMode: 'richText'});\n")
	boot.WriteString("var el = document.getElementById('chart');\nvar chart = echarts.init(el, null, {renderer: 'canvas'});\nchart.setOption(option);\n")
	fmt.Fprintf(&boot, "function fit() { var s = Math.min(window.innerWidth / %d, window.innerHeight / %d, 2); el.style.transform = 'scale(' + s + ')'; }\nfit(); window.addEventListener('resize', fit);\n", OptionWidth, OptionHeight)
	csp = "default-src 'none'; script-src " + echartsHash + " " + scriptHash(boot.Bytes()) + "; style-src 'unsafe-inline'; img-src data:"

	var b bytes.Buffer
	b.Grow(len(echartsScript) + boot.Len() + 2048)
	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n")
	b.WriteString("<meta http-equiv=\"Content-Security-Policy\" content=\"" + csp + "\">\n<meta name=\"referrer\" content=\"no-referrer\">\n")
	b.WriteString("<title>" + html.EscapeString(clean(title)) + "</title>\n")
	fmt.Fprintf(&b, "<style>html,body{margin:0;height:100%%;background:#fff}body{display:grid;place-items:center;overflow:hidden}#chart{width:%dpx;height:%dpx;transform-origin:center}</style>\n", OptionWidth, OptionHeight)
	b.WriteString("</head><body><div id=\"chart\"></div>\n<script>")
	b.Write(echartsScript)
	b.WriteString("</script>\n<script>")
	b.Write(boot.Bytes())
	b.WriteString("</script></body></html>\n")
	return b.Bytes(), csp
}

// scriptHash is a script's name in a Content-Security-Policy.
func scriptHash(script []byte) string {
	sum := sha256.Sum256(script)
	return "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
}

// inlineJSON makes JSON safe inside a script element: "</" cannot end the
// element, and the line separators JSON allows but JavaScript did not are
// escaped. JSON that does not parse becomes an empty option.
func inlineJSON(raw json.RawMessage) string {
	if !json.Valid(raw) {
		return "{}"
	}
	return scriptSafe.Replace(string(raw))
}

// scriptSafe is the replacer inlineJSON uses. The separators are spelled
// by code point so that no editor turns them into the runes themselves.
var scriptSafe = strings.NewReplacer("</", `<\/`, string(rune(0x2028)), `\u`+"2028", string(rune(0x2029)), `\u`+"2029")
