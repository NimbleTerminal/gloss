module nimbleterminal.dev/gloss/examples/demo

go 1.26.10

require (
	charm.land/bubbletea/v2 v2.1.1
	github.com/spf13/pflag v1.0.10
	nimbleterminal.dev/booba v0.12.1
	nimbleterminal.dev/gloss v0.0.0
)

require (
	charm.land/bubbles/v2 v2.2.1 // indirect
	charm.land/glamour/v2 v2.0.1 // indirect
	charm.land/lipgloss/v2 v2.0.6 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2 // indirect
	github.com/NimbleMarkets/oksvg v0.1.6 // indirect
	github.com/alecthomas/chroma/v2 v2.27.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/charmbracelet/ultraviolet v0.0.0-20261023060817-a45b7c86333f // indirect
	github.com/charmbracelet/x/ansi v0.11.9 // indirect
	github.com/charmbracelet/x/exp/slice v0.1.0 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dlclark/regexp2/v2 v2.12.0 // indirect
	github.com/dustin/go-humanize v1.1.0 // indirect
	github.com/ebitengine/purego v0.11.3 // indirect
	github.com/gen2brain/h265 v0.2.4 // indirect
	github.com/gen2brain/jxl v0.2.0 // indirect
	github.com/go-text/typesetting v0.3.5 // indirect
	github.com/go-webgpu/goffi v0.6.8 // indirect
	github.com/go-webgpu/webgpu v0.5.5 // indirect
	github.com/gogpu/gpucontext v0.31.3 // indirect
	github.com/gogpu/gputypes v0.8.2 // indirect
	github.com/gogpu/naga v0.21.2 // indirect
	github.com/gogpu/wgpu v0.34.7 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/jolestar/go-commons-pool/v2 v2.1.2 // indirect
	github.com/klippa-app/go-pdfium v1.21.4 // indirect
	github.com/ledongthuc/pdf v0.0.0-20261020125853-5e46805f718e // indirect
	github.com/lrstanley/bubblezone/v2 v2.0.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/neomantra/sqlittle v0.0.0-20261001025832-4a4bc3e212fa // indirect
	github.com/nicksnyder/go-i18n/v2 v2.7.0 // indirect
	github.com/piglig/go-qr/v2 v2.7.0 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/srwiley/rasterx v0.0.0-20220730225603-2ab79fcdd4ef // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	github.com/xo/terminfo v1.3.1 // indirect
	github.com/yuin/goldmark v1.8.7 // indirect
	github.com/yuin/goldmark-emoji v1.0.6 // indirect
	github.com/zkrebbekx/go-mermaid v0.1.6 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/image v0.47.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	nimbleterminal.dev/nt3d v0.8.1 // indirect
	nimbleterminal.dev/ntcanvas v0.1.4 // indirect
	nimbleterminal.dev/ntconfine v0.1.1 // indirect
	nimbleterminal.dev/ntconfine/pkg/confine v0.1.0 // indirect
	nimbleterminal.dev/ntfilepicker v0.2.1 // indirect
	nimbleterminal.dev/ntfonts v0.3.1 // indirect
	nimbleterminal.dev/ntgpuimage v0.6.0 // indirect
	nimbleterminal.dev/nthtml v0.2.3 // indirect
	nimbleterminal.dev/ntmaps v0.6.3 // indirect
	nimbleterminal.dev/ntmediabox v0.1.0 // indirect
	nimbleterminal.dev/ntmediabox/pkg/mediabox v0.1.0 // indirect
	nimbleterminal.dev/ntopenwith v0.1.2 // indirect
	nimbleterminal.dev/ntpdf v0.6.0 // indirect
	nimbleterminal.dev/ntpicture v0.1.5 // indirect
	nimbleterminal.dev/ntqrcode v0.4.0 // indirect
	nimbleterminal.dev/ntshim v0.6.1 // indirect
	nimbleterminal.dev/ntsvg v0.6.0 // indirect
	nimbleterminal.dev/nttypeset v0.1.9 // indirect
	nimbleterminal.dev/ntvideo v0.5.0 // indirect
)

replace nimbleterminal.dev/gloss => ../..

replace charm.land/bubbletea/v2 => github.com/neomantra/bubbletea/v2 v2.0.0-20261020122444-536722b0bdb6

tool (
	nimbleterminal.dev/booba/cmd/booba-assets
	nimbleterminal.dev/ntshim/cmd/ntshim-assets
	nimbleterminal.dev/ntvideo/cmd/ntvideo-assets
)
