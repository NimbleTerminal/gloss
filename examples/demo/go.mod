module nimbleterminal.dev/gloss/examples/demo

go 1.26.9

require (
	charm.land/bubbletea/v2 v2.1.0
	github.com/NimbleMarkets/go-booba v0.10.3
	github.com/spf13/pflag v1.0.10
	nimbleterminal.dev/gloss v0.0.0
)

require (
	charm.land/bubbles/v2 v2.2.1 // indirect
	charm.land/glamour/v2 v2.0.1 // indirect
	charm.land/lipgloss/v2 v2.0.6 // indirect
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2 // indirect
	github.com/NimbleMarkets/oksvg v0.1.2 // indirect
	github.com/alecthomas/chroma/v2 v2.27.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261008173134-6b8d4baf91b4 // indirect
	github.com/charmbracelet/x/ansi v0.11.9 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250327172914-2fdc97757edf // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/gen2brain/h265 v0.2.3 // indirect
	github.com/gen2brain/jxl v0.2.0 // indirect
	github.com/go-text/render v0.2.1 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/go-webgpu/goffi v0.6.4 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.0 // indirect
	github.com/gogpu/naga v0.19.0 // indirect
	github.com/gogpu/wgpu v0.34.5 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/jolestar/go-commons-pool/v2 v2.1.2 // indirect
	github.com/klippa-app/go-pdfium v1.21.3 // indirect
	github.com/ledongthuc/pdf v0.0.0-20260907135840-6c8c28e0e8a0 // indirect
	github.com/lrstanley/bubblezone/v2 v2.0.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/neomantra/sqlittle v0.0.0-20261001025832-4a4bc3e212fa // indirect
	github.com/piglig/go-qr/v2 v2.7.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/srwiley/oksvg v0.0.0-20221011165216-be6e8873101c // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	github.com/xo/terminfo v1.2.0 // indirect
	github.com/yuin/goldmark v1.8.2 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	github.com/zkrebbekx/go-mermaid v0.1.6 // indirect
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	nimbleterminal.dev/nt3d v0.2.0 // indirect
	nimbleterminal.dev/ntcanvas v0.1.0 // indirect
	nimbleterminal.dev/ntfilepicker v0.1.0 // indirect
	nimbleterminal.dev/ntfonts v0.1.0 // indirect
	nimbleterminal.dev/ntgpuimage v0.3.0 // indirect
	nimbleterminal.dev/ntpdf v0.5.1 // indirect
	nimbleterminal.dev/ntpicture v0.1.1 // indirect
	nimbleterminal.dev/ntqrcode v0.3.1 // indirect
	nimbleterminal.dev/ntshim v0.6.0 // indirect
	nimbleterminal.dev/ntsvg v0.5.1 // indirect
	nimbleterminal.dev/nttypeset v0.1.1 // indirect
)

replace nimbleterminal.dev/gloss => ../..

replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 v2.0.0-20261014185126-aa0ac3f1b5fe

tool (
	github.com/NimbleMarkets/go-booba/cmd/booba-assets
	nimbleterminal.dev/ntshim/cmd/ntshim-assets
)
