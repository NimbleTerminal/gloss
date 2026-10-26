// gloss is a terminal pager for images, SVGs, PDFs, 3D meshes, and
// Markdown, Word, and Excel files.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/spf13/pflag"
	"nimbleterminal.dev/gloss/internal/app"
	"nimbleterminal.dev/gloss/internal/clipimage"
	"nimbleterminal.dev/gloss/internal/document"
	"nimbleterminal.dev/gloss/internal/handoff"
	"nimbleterminal.dev/gloss/internal/i18n"
	"nimbleterminal.dev/gloss/internal/theme"
	"nimbleterminal.dev/gloss/skills"
	"nimbleterminal.dev/nt3d"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		if !errors.As(err, new(reported)) {
			fmt.Fprintf(os.Stderr, "gloss: %v\n", err)
		}
		os.Exit(handoff.ExitCode(err))
	}
}

// options are the viewer's, and those of showing it somewhere else.
type options struct {
	themeExplicit bool
	fontExplicit  bool
	app.Options
	Serve, NoOpen bool
	PickWeb       bool // Experimental upload-only page, without a terminal viewer.
	Listen        string
	AdvertiseHost string
	Timeout       time.Duration
	Info, JSON    bool
	Apps          bool
	FetchAllowed  bool
	UserAgent     string            // What downloads send as their User-Agent; see userAgent.
	Origins       map[string]string // The address each downloaded argument came from, by file.
	Failed        map[string]string // Why an address among the arguments could not be downloaded, when --info goes on without it.
	Globs         []string
	Text          bool           // Take the text out, rather than draw.
	Grep          *regexp.Regexp // Search the text layer of PDF pages for this, and report the matches.
	Pages         pageRange      // What --page asked for; Page holds it when it is one.
	Resume        string         // Token of a detached pick whose answer is wanted.
	Status        string         // Token of a detached pick whose state is wanted, now.
	Cancel        string         // Token of a detached session to end, files and all.
	Detached      string         // Set on the server a detached start runs: its token.
	VisionProfile string         // A convenience alias for a max edge; --max-edge is the stable flag.
	EdgeReason    string         // Why the alias chose its edge, for the manifest.
	SkillInstall  string         // Where gloss skill install writes.
	SkillDetect   bool           // gloss skill install with no folder: write for the agents found here.

	QR string // Text to show as a QR code, in place of files.

	// fileArgs says which of the command line's arguments name files, not
	// options or their values: a detached start rewrites only those.
	fileArgs []bool

	Print                   bool // Draw one viewport to stdout without a terminal.
	PrintWidth, PrintHeight int
}

func parse(args []string, out io.Writer) (options, bool, error) {
	var opts options
	opts.Version = version
	whole := args
	name, args := splitVerb(args)
	f := pflag.NewFlagSet("gloss", pflag.ContinueOnError)
	f.SetOutput(out)
	f.StringVarP(&opts.Render, "render", "r", "auto", "terminal graphics: auto, kitty, glyph")
	f.StringVar(&opts.Render3D, "3d", "auto", "mesh renderer: auto, software, wireframe")
	f.StringVar(&opts.Font, "font", "readable", "Typeset font pair: readable, editorial, technical, compact")
	f.StringVar(&opts.Mono, "mono", "", "Override typeset code face: jetbrains")
	f.StringVar(&opts.Theme, "theme", "auto", "Colors: "+strings.Join(themeNames(), ", ")+", or theme JSON file")
	f.StringVar(&opts.Lang, "lang", "", "language of gloss's own words and number formats (BCP 47 or POSIX, e.g. es, zh-Hans, de_DE); options stay English")
	f.BoolVar(&opts.Typeset, "typeset", false, "Typeset Markdown, plain text, source code, and JSON as graphics")
	f.BoolVarP(&opts.LineNumbers, "line-numbers", "N", false, "number the lines of plain text, source code, JSON and Markdown source (# toggles, : goes to a line)")
	f.BoolVarP(&opts.ChopLines, "chop-long-lines", "S", false, "cut long lines at the right edge instead of wrapping them, as less -S does (S toggles, arrows scroll sideways)")
	f.StringVar(&opts.MarkdownMode, "markdown", "text", "Markdown renderer: text, typeset (direct graphics)")
	f.StringVarP(&opts.Type, "type", "t", "", "force input type: image, svg, pdf, stl, 3mf, obj, ply, docx, xlsx, grist, csv, json (or jsonl, ndjson), ipynb, html, text, markdown (or md), mermaid, video, geojson, gpx, pmtiles")
	accept := f.String("accept", "", "require these content formats for this session, comma-separated: image/*, pdf, svg, stl, 3mf, obj, ply, docx, xlsx, grist, csv, json (or jsonl, ndjson), ipynb, html, text, markdown, mermaid, video (or video/*)")
	page := f.StringP("page", "p", "1", "PDF page or workbook sheet, from 1; for an export or text into a folder, a range such as 2-5, or all")
	f.BoolVar(&opts.Text, "text", false, "take the text out: Markdown of a Word document, page, or notebook, text of a PDF page, CSV of a sheet, text and JSON as they are")
	grep := f.String("grep", "", "search the text layer of a PDF, every page unless --page says, for this regular expression (RE2), and print each match's page and a line around it; --json for objects")
	f.IntVarP(&opts.DPI, "dpi", "d", 150, "PDF rasterization DPI (36–600)")
	f.BoolVarP(&opts.Menu, "menu", "m", false, "start with the file-selection menu")
	f.BoolVarP(&opts.Preview, "preview", "P", false, "start with the file menu and a preview pane")
	view := f.String("view", "", "views of a mesh: front, back, left, right, top, bottom, iso, all; several, as front,top, export as one sheet")
	at := f.String("at", "", "moments of a video, as 1:23, 83.5 or 1m23s, comma-separated: the frames an export draws, several as one sheet or a file each with --output-dir; the viewer starts at the first")
	f.IntVar(&opts.FrameCount, "frames", 0, "export this many frames of a video, 1..16, spread through it: one sheet, or a file each with --output-dir")
	camera := f.String("camera", "", "camera for a mesh, as elevation,azimuth or elevation,azimuth,distance in degrees")
	projection := f.String("projection", "ortho", "projection of a mesh: ortho, perspective")
	f.StringVar(&opts.Prompt, "prompt", "", "show this request to the user in a box, to say what to pick or look at")
	promptLoc := f.String("prompt-loc", "bottom", "where the terminal prompt box goes: bottom or top (browser requests always appear above the viewer)")
	paint := f.String("color", "", "paint the faces of a mesh its file left plain: #rrggbb or a name such as orange")
	up := f.String("up", "", "the axis a mesh file treats as up: y or z; STL defaults to z, OBJ and PLY to y")
	parts := f.String("parts", "", "show only these parts of a 3MF, by name: name,name")
	partn := f.String("partn", "", "show only these parts of a 3MF, counted from 1: 2,4-6")
	cols := f.String("cols", "", "show only these columns of a table, by header or letter: name,name")
	coln := f.String("coln", "", "show only these columns of a table, counted from 1: 2,4-6")
	f.StringArrayVar(&opts.Globs, "glob", nil, "search the folders named, or the current one, for files: a glob, an extension, or a kind such as images (repeatable)")
	f.BoolVar(&opts.FetchAllowed, "fetch", false, "allow opening a dropped URL or a table address with Enter, and the map provider named in GLOSS_MAP_TILES and GLOSS_GEOCODER; an address named on the command line needs no option, and GLOSS_NO_FETCH forbids both")
	f.StringVar(&opts.UserAgent, "user-agent", "", "what downloads send as their User-Agent; else $GLOSS_USER_AGENT, else gloss and its version")
	f.BoolVar(&opts.Apps, "apps", false, "list the apps for each file kind, to open and to edit it, without opening a file; --json for scripts")
	f.StringVar(&opts.QR, "qr", "", "show this text, such as a URL, as a QR code instead of a file, to scan with a phone or tablet; with --serve, on a web page; nothing is fetched")
	f.BoolVar(&opts.Print, "print", false, "print one content viewport to stdout and exit; pictures use ANSI-colored half-blocks, no terminal required")
	f.IntVar(&opts.PrintWidth, "print-width", 0, "with --print, viewport columns (1–512; default terminal width or 80)")
	f.IntVar(&opts.PrintHeight, "print-height", 0, "with --print, viewport rows (1–256; default terminal height or 24)")
	f.BoolVar(&opts.Info, "info", false, "print what each file says about itself, and do not open the viewer")
	f.BoolVar(&opts.JSON, "json", false, "with --apps, --info, --text, --grep, an export, --pick-web, or --resume, print JSON")
	f.BoolVar(&opts.Pick, "pick", false, "wait for the user to hand over files: Enter prints their paths and quits")
	f.BoolVar(&opts.PickWeb, "pick-web", false, "experimental file request with Choose files and Send buttons, without a terminal; implies --serve --pick; takes no input files")
	f.StringVar(&opts.Listen, "listen", handoff.DefaultListen, "with --pick-web, listen on IP:port (IPv6: [IP]:port); port 0 chooses a free port; wildcard requires --advertise-host")
	f.StringVar(&opts.AdvertiseHost, "advertise-host", "", "with --pick-web, use this IP or DNS name in the shared URL (no scheme, port, or path); defaults to the specific listen IP")
	f.BoolVar(&opts.Serve, "serve", false, "show the viewer on a web page, from a temporary server on this machine")
	f.BoolVar(&opts.NoOpen, "no-open", false, "with --serve, print the page's address without opening a browser")
	f.DurationVar(&opts.Timeout, "timeout", 0, "hard deadline with --serve or --pick; wait limit with --resume (as 90s or 10m)")
	f.BoolVarP(&opts.KeepScreen, "no-alt-screen", "X", false, "draw on the main screen: scrollback is kept, and the last view stays after quitting")
	f.StringVarP(&opts.Output, "output", "o", "", "export one input as PNG; '-' writes PNG to stdout")
	f.StringVarP(&opts.OutputDir, "output-dir", "O", "", "export each input as a numbered PNG in this directory")
	f.IntVarP(&opts.MaxEdge, "max-edge", "s", 1536, "maximum exported image edge in pixels (1–4096)")
	f.StringVar(&opts.VisionProfile, "vision-profile", "", "alias for a --max-edge, which is stable where these go stale: "+strings.Join(document.VisionProfileNames(), ", "))
	f.StringVar(&opts.Resume, "resume", "", "print the answer of a pick that was started without a terminal, by the token it printed; with --timeout, stop waiting after that long")
	f.StringVar(&opts.Status, "status", "", "print, as JSON and at once, how a pick started without a terminal stands: waiting, picked, declined, timeout, closed, or failed; it waits for nothing and changes nothing")
	f.StringVar(&opts.Cancel, "cancel", "", "end a session started without a terminal, by its resume token: stop its server and delete its folder, dropped files and answer alike")
	f.StringVar(&opts.Detached, "detached", "", "")
	_ = f.MarkHidden("detached")
	showVersion := f.BoolP("version", "V", false, "print version")
	// The skill verb replaced these two options; they work, unlisted, for scripts
	// that have them.
	showSkill := f.Bool("skill", false, "")
	skillInstall := f.String("install", "", "")
	f.Lookup("install").NoOptDefVal = "auto"
	_ = f.MarkHidden("skill")
	_ = f.MarkHidden("install")
	showHelp := f.BoolP("help", "h", false, "show help")
	// The reference is written from these options; see docs.go.
	docsMan := f.String("docs-man", "", "")
	docsMarkdown := f.String("docs-markdown", "", "")
	docsHugo := f.Bool("docs-hugo", false, "")
	docsCompletions := f.String("docs-completions", "", "")
	for _, name := range []string{"docs-man", "docs-markdown", "docs-hugo", "docs-completions"} {
		_ = f.MarkHidden(name)
	}
	f.Usage = func() {
		fmt.Fprint(out, "Usage: "+usageLines[0]+"\n")
		for _, line := range usageLines[1:] {
			fmt.Fprint(out, "       "+line+"\n")
		}
		fmt.Fprint(out, "\n"+usageAbout+"\n\nCommands:\n")
		for _, v := range verbs {
			fmt.Fprintf(out, "  %-6s  %s\n", v.Name, v.Summary)
		}
		fmt.Fprint(out, "\nOptions of view:\n")
		f.PrintDefaults()
		fmt.Fprintln(out, "\nKeys: q quit · ? help · [/] files · n/p pages · +/- zoom · arrows pan/orbit")
		fmt.Fprintln(out, "Documentation: "+app.DocsURL)
	}
	switch name {
	case "skill":
		return parseSkill(args, opts, out)
	case "help":
		done, err := parseHelp(args, f.Usage, out)
		return opts, done, err
	}
	if err := f.Parse(args); err != nil {
		return opts, errors.Is(err, pflag.ErrHelp), err
	}
	if *showHelp {
		f.Usage()
		return opts, true, nil
	}
	if *showVersion {
		fmt.Fprintln(out, "gloss "+version)
		return opts, true, nil
	}
	if *docsMan != "" || *docsMarkdown != "" || *docsCompletions != "" {
		var wrote bytes.Buffer
		err := writeDocs(f, docsRequest{ManDir: *docsMan, MarkdownDir: *docsMarkdown, CompletionsDir: *docsCompletions, Hugo: *docsHugo}, &wrote)
		out.Write(wrote.Bytes())
		return opts, err == nil, err
	}
	if *showSkill {
		if f.Changed("install") {
			opts.SkillInstall = *skillInstall
			opts.SkillDetect = *skillInstall == ""
			return opts, false, nil
		}
		fmt.Fprint(out, skills.Gloss)
		return opts, true, nil
	}
	if f.Changed("install") {
		return opts, false, fmt.Errorf("--install goes with --skill: gloss skill install")
	}
	if (f.Changed("listen") || f.Changed("advertise-host")) && !opts.PickWeb {
		return opts, false, fmt.Errorf("--listen and --advertise-host require --pick-web; the full --serve viewer remains localhost-only")
	}
	if f.Changed("qr") {
		switch {
		case opts.QR == "":
			return opts, false, fmt.Errorf("--qr needs the text to encode")
		case f.NArg() > 0 || len(opts.Globs) > 0:
			return opts, false, fmt.Errorf("--qr shows its own code: no input files or --glob")
		case opts.Pick || opts.PickWeb:
			return opts, false, fmt.Errorf("--qr shows a code; it cannot be combined with --pick or --pick-web")
		}
	}
	if opts.PickWeb {
		opts.Serve, opts.Pick = true, true
		if f.NArg() > 0 || len(opts.Globs) > 0 || opts.FetchAllowed || opts.Menu || opts.Preview {
			return opts, false, fmt.Errorf("--pick-web accepts browser uploads only: no input files, --glob, --fetch, --menu, or --preview")
		}
		if f.Changed("advertise-host") && opts.AdvertiseHost == "" {
			return opts, false, fmt.Errorf("--advertise-host cannot be empty")
		}
		if err := handoff.CheckNetwork(opts.Listen, opts.AdvertiseHost); err != nil {
			return opts, false, err
		}
	}
	opts.fontExplicit = f.Changed("font") || f.Changed("mono")
	if _, err := document.ResolveFont(opts.Font, opts.Mono); err != nil {
		return opts, false, err
	}
	if f.Changed("lang") {
		tag, err := i18n.Parse(opts.Lang)
		if err != nil {
			return opts, false, fmt.Errorf("--lang %q: not a language tag", opts.Lang)
		}
		opts.Lang = tag.String()
	}
	opts.themeExplicit = f.Changed("theme")
	if ref, err := theme.Load(opts.Theme); err != nil {
		return opts, false, fmt.Errorf("--theme: %w", err)
	} else {
		opts.Theme = ref
	}
	if !slices.Contains([]string{"text", "typeset"}, opts.MarkdownMode) {
		return opts, false, fmt.Errorf("--markdown must be text or typeset")
	}
	if !slices.Contains([]string{"auto", "kitty", "glyph"}, opts.Render) {
		return opts, false, fmt.Errorf("--render must be auto, kitty, or glyph")
	}
	if !slices.Contains([]string{"auto", "software", "wireframe"}, opts.Render3D) {
		return opts, false, fmt.Errorf("--3d must be auto, software, or wireframe")
	}
	switch opts.Type {
	case "md":
		opts.Type = "markdown"
	case "jsonl", "ndjson":
		opts.Type = "json"
	}
	if !slices.Contains([]string{"", "image", "svg", "pdf", "stl", "3mf", "obj", "ply", "docx", "xlsx", "grist", "csv", "json", "ipynb", "html", "text", "markdown", "mermaid", "video", "geojson", "gpx", "pmtiles"}, opts.Type) {
		return opts, false, fmt.Errorf("--type must be image, svg, pdf, stl, 3mf, obj, ply, docx, xlsx, grist, csv, json, ipynb, html, text, markdown, mermaid, video, geojson, gpx, or pmtiles")
	}
	if f.Changed("grep") {
		if *grep == "" {
			return opts, false, fmt.Errorf("--grep needs a pattern")
		}
		pattern, err := regexp.Compile(*grep)
		if err != nil {
			return opts, false, fmt.Errorf("--grep: %w", err)
		}
		opts.Grep = pattern
		if !f.Changed("page") {
			*page = "all" // A search is of the whole document unless told otherwise.
		}
	}
	pages, err := parsePages(*page)
	if err != nil {
		return opts, false, err
	}
	if opts.Accept, err = document.ParseAccept(*accept); err != nil {
		return opts, false, err
	}
	opts.Pages = pages
	if single, ok := opts.Pages.single(); ok {
		opts.Page = single
	} else if opts.OutputDir == "" && !opts.JSON && opts.Grep == nil {
		return opts, false, fmt.Errorf("--page with a range or all needs --output-dir, or --json with --text")
	}
	if opts.DPI < 36 || opts.DPI > 600 {
		return opts, false, fmt.Errorf("--dpi must be between 36 and 600")
	}
	opts.Files = f.Args()
	opts.fileArgs = append(make([]bool, len(whole)-len(args)), fileArguments(f, args)...)
	if opts.Columns, err = document.ParseColumns(*cols, *coln); err != nil {
		return opts, false, err
	}
	if opts.Parts, err = document.ParseParts(*parts, *partn); err != nil {
		return opts, false, err
	}
	if opts.Up = strings.ToLower(*up); opts.Up != "" && opts.Up != "y" && opts.Up != "z" {
		return opts, false, fmt.Errorf("--up must be y or z")
	}
	if *paint != "" {
		c, err := document.ParseColor(*paint)
		if err != nil {
			return opts, false, err
		}
		opts.Color = &c
	}
	switch {
	case *promptLoc != "bottom" && *promptLoc != "top":
		return opts, false, fmt.Errorf("--prompt-loc must be bottom or top")
	case f.Changed("prompt-loc") && opts.Prompt == "":
		return opts, false, fmt.Errorf("--prompt-loc needs --prompt")
	case (opts.Output != "" || opts.OutputDir != "") && opts.Prompt != "":
		return opts, false, fmt.Errorf("--prompt cannot be combined with an export, which shows nothing")
	}
	opts.PromptTop = *promptLoc == "top"
	switch {
	case *view != "" && *camera != "":
		return opts, false, fmt.Errorf("choose --view or --camera")
	case f.Changed("view"):
		if opts.Views, err = document.ParseViews(*view); err == nil && len(opts.Views) == 0 {
			err = fmt.Errorf("--view names no view")
		}
	case f.Changed("camera"):
		var at document.View
		at, err = document.ParseCamera(*camera)
		opts.Views = []document.View{at}
	case f.Changed("projection"):
		// The camera there would have been, otherwise projected.
		at := nt3d.DefaultCamera()
		opts.Views = []document.View{{Alpha: at.Alpha, Beta: at.Beta, Distance: at.Distance}}
	}
	if err != nil {
		return opts, false, fmt.Errorf("--view/--camera: %w", err)
	}
	if !slices.Contains([]string{"ortho", "perspective"}, *projection) {
		return opts, false, fmt.Errorf("--projection must be ortho or perspective")
	}
	if f.Changed("at") {
		if opts.Times, err = parseTimes(*at); err != nil {
			return opts, false, err
		}
	}
	exporting := opts.Output != "" || opts.OutputDir != "" || opts.Print
	switch {
	case f.Changed("frames") && (opts.FrameCount < 1 || opts.FrameCount > document.MaxFrames):
		return opts, false, fmt.Errorf("--frames must be 1..%d", document.MaxFrames)
	case f.Changed("at") && f.Changed("frames"):
		return opts, false, fmt.Errorf("choose --at or --frames")
	case (f.Changed("at") || f.Changed("frames")) && (opts.Text || opts.Info || opts.Grep != nil):
		return opts, false, fmt.Errorf("--at and --frames choose frames to draw; --text gives a video's subtitles, with their times")
	case (opts.FrameCount > 0 || len(opts.Times) > 1) && !exporting:
		return opts, false, fmt.Errorf("several frames need --output, --output-dir or --print; the viewer starts at one --at")
	}
	for i := range opts.Views {
		if *projection == "perspective" {
			opts.Views[i].Projection = nt3d.Perspective
		}
	}
	if opts.PrintWidth < 0 || opts.PrintWidth > 512 || opts.PrintHeight < 0 || opts.PrintHeight > 256 || (f.Changed("print-width") && opts.PrintWidth == 0) || (f.Changed("print-height") && opts.PrintHeight == 0) {
		return opts, false, fmt.Errorf("--print-width must be 1..512 and --print-height 1..256")
	}
	if !opts.Print && (f.Changed("print-width") || f.Changed("print-height")) {
		return opts, false, fmt.Errorf("--print-width and --print-height require --print")
	}
	if opts.Print {
		if opts.Output != "" || opts.OutputDir != "" || opts.Text || opts.Info || opts.Grep != nil || opts.JSON || opts.Apps || opts.Serve || opts.Pick || opts.Menu || opts.Preview || opts.KeepScreen || opts.FetchAllowed || opts.Prompt != "" || opts.Resume != "" || opts.Status != "" || opts.Cancel != "" || opts.NoOpen || opts.Timeout != 0 {
			return opts, false, fmt.Errorf("--print cannot be combined with another output mode or interactive viewer options")
		}
		if opts.Render == "kitty" {
			return opts, false, fmt.Errorf("--print uses glyphs; --render kitty requires the interactive viewer")
		}
	}
	switch {
	case opts.Grep != nil && (opts.Output != "" || opts.OutputDir != "" || opts.Info || opts.Serve || opts.Pick || opts.Menu || opts.Preview || opts.KeepScreen || opts.FetchAllowed):
		return opts, false, fmt.Errorf("--grep prints the matches and exits; it cannot be combined with an export, --info, or the viewer's options")
	case opts.JSON && !opts.Apps && opts.Grep == nil && !opts.Info && !opts.Text && opts.Output == "" && opts.OutputDir == "" && opts.Resume == "" && !opts.PickWeb:
		return opts, false, fmt.Errorf("--json goes with --info, --text, --grep, an export, --pick-web, or --resume")
	case opts.Info && (opts.Output != "" || opts.OutputDir != "" || opts.Text || opts.Serve || opts.Pick || opts.Menu || opts.Preview || opts.KeepScreen):
		return opts, false, fmt.Errorf("--info prints and exits; it cannot be combined with the viewer's or export's options")
	case opts.Text && (opts.Serve || opts.Pick || opts.Menu || opts.Preview || opts.KeepScreen || opts.FetchAllowed):
		return opts, false, fmt.Errorf("--text prints and exits; it cannot be combined with the viewer's options")
	}
	if opts.VisionProfile != "" {
		if opts.MaxEdge, opts.EdgeReason, err = document.ResolveEdge(opts.MaxEdge, f.Changed("max-edge"), opts.VisionProfile); err != nil {
			return opts, false, err
		}
		if opts.Output == "" && opts.OutputDir == "" {
			return opts, false, fmt.Errorf("--vision-profile requires --output or --output-dir")
		}
	}
	if opts.MaxEdge < 1 || opts.MaxEdge > 4096 {
		return opts, false, fmt.Errorf("--max-edge must be 1..4096")
	}
	if opts.Output != "" && opts.OutputDir != "" {
		return opts, false, fmt.Errorf("choose --output or --output-dir")
	}
	if opts.Output != "" && len(opts.Files) > 1 {
		return opts, false, fmt.Errorf("--output accepts one input; use --output-dir for multiple inputs")
	}
	if (opts.Output != "" || opts.OutputDir != "") && (opts.Menu || opts.Preview) {
		return opts, false, fmt.Errorf("export cannot be combined with --menu or --preview")
	}
	if (opts.Output != "" || opts.OutputDir != "") && opts.KeepScreen {
		return opts, false, fmt.Errorf("export draws nothing; it cannot be combined with --no-alt-screen")
	}
	if (opts.Output != "" || opts.OutputDir != "") && (opts.Serve || opts.Pick || opts.FetchAllowed) {
		return opts, false, fmt.Errorf("export cannot be combined with --serve, --pick, or --fetch")
	}
	if opts.FetchAllowed && fetchDisabled() {
		return opts, false, fmt.Errorf("--fetch is disabled by %s", noFetchVariable)
	}
	if opts.UserAgent, err = userAgent(opts.UserAgent); err != nil {
		return opts, false, err
	}
	if opts.Serve && opts.KeepScreen {
		return opts, false, fmt.Errorf("--serve draws on a page; it cannot be combined with --no-alt-screen")
	}
	if opts.NoOpen && !opts.Serve {
		return opts, false, fmt.Errorf("--no-open requires --serve")
	}
	if opts.Timeout < 0 || (opts.Timeout > 0 && !opts.Serve && !opts.Pick && opts.Resume == "") {
		return opts, false, fmt.Errorf("--timeout must be positive, and requires --serve, --pick, or --resume")
	}
	switch {
	case opts.Resume != "" && (len(opts.Files) > 0 || opts.Serve || opts.Pick || opts.Info || opts.Text || opts.Output != "" || opts.OutputDir != "" || opts.Detached != "" || len(opts.Globs) > 0):
		return opts, false, fmt.Errorf("--resume only asks what a pick came to: it takes no files and no other mode (--json, --timeout excepted)")
	case opts.Status != "" && (len(opts.Files) > 0 || opts.Serve || opts.Pick || opts.Info || opts.Text || opts.JSON || opts.Output != "" || opts.OutputDir != "" || opts.Detached != "" || opts.Resume != "" || opts.Timeout > 0 || opts.Prompt != "" || len(opts.Globs) > 0):
		return opts, false, fmt.Errorf("--status only asks how a pick stands: it takes no files, no timeout, and no other mode")
	case opts.Cancel != "" && (len(opts.Files) > 0 || opts.Serve || opts.Pick || opts.Info || opts.Text || opts.JSON || opts.Output != "" || opts.OutputDir != "" || opts.Detached != "" || opts.Resume != "" || opts.Status != "" || opts.Timeout > 0 || opts.Prompt != "" || len(opts.Globs) > 0):
		return opts, false, fmt.Errorf("--cancel only ends a session: it takes no files, no timeout, and no other mode")
	case opts.Detached != "" && (!opts.Serve || !handoff.ValidToken(opts.Detached)):
		return opts, false, fmt.Errorf("--detached is for gloss's own use")
	}

	if opts.Apps && (len(opts.Files) > 0 || len(opts.Globs) > 0 || opts.Serve || opts.Pick || opts.PickWeb || opts.Info || opts.Text || opts.Grep != nil || opts.Output != "" || opts.OutputDir != "" || opts.Resume != "" || opts.Status != "" || opts.Cancel != "" || opts.Detached != "" || opts.Menu || opts.Preview || opts.FetchAllowed) {
		return opts, false, fmt.Errorf("--apps lists applications; it takes no files or other mode")
	}
	return opts, false, nil
}

func run(args []string) (err error) {
	// What a pick comes back with. The deferred report of a detached server
	// and the removal of downloads read it once run has returned.
	var picked pickAnswer
	opts, done, err := parse(args, os.Stdout)
	if done {
		return nil
	}
	if err != nil {
		return err
	}
	if opts.Apps {
		host, err := nativeOpenWith()
		if host == nil {
			return err
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "gloss: open-with preferences: %v\n", err)
		}
		return writeApps(os.Stdout, host, opts.JSON)
	}
	if opts.SkillDetect || opts.SkillInstall != "" {
		if term.IsTerminal(os.Stdout.Fd()) {
			// The diagnostic already includes the path. Emit bare paths only
			// when stdout is redirected for a script to consume.
			return installSkill(opts.SkillInstall, io.Discard, os.Stderr)
		}
		return installSkill(opts.SkillInstall, os.Stdout, os.Stderr)
	}
	if opts.Cancel != "" {
		return handoff.Cancel(handoffConfig(opts))
	}
	if opts.Status != "" {
		return handoff.Status(handoffConfig(opts), os.Stdout)
	}
	if opts.Resume != "" {
		return handoff.Resume(handoffConfig(opts), os.Stdout, os.Stderr)
	}
	if opts.Detached != "" {
		// The server of a detached start: it answers through its state.
		defer func() { handoff.Conclude(opts.Detached, err, picked.paths, opts.Pick, picked.message) }()
	}
	stdinTTY := term.IsTerminal(os.Stdin.Fd())
	opts.MarkdownBase, err = os.Getwd()
	if err != nil {
		return err
	}
	opts.Files = arguments(opts, stdinTTY)
	exporting := opts.Output != "" || opts.OutputDir != "" || opts.Text || opts.Grep != nil || opts.Print
	if opts.Output == "-" && term.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("redirect PNG stdout to a file or pipe")
	}
	// With no terminal to answer, a pick or a page must not hold its
	// caller: the server runs apart and the start says where it is.
	if (opts.Pick || opts.Serve) && !stdinTTY && opts.Detached == "" && !exporting && !opts.Info {
		return handoff.Detach(args, handoffConfig(opts), os.Stdout, os.Stderr)
	}
	lang := language(opts, stdinTTY, os.Getenv)
	opts.Options.Lang = lang
	i18n.SetLanguage(lang)
	// A page needs no terminal. A pick keeps standard output for its answer,
	// and draws on the terminal itself.
	var screen *os.File
	switch {
	case exporting || opts.Info || opts.Serve || term.IsTerminal(os.Stdout.Fd()):
	case opts.Pick:
		if screen, err = os.OpenFile("/dev/tty", os.O_WRONLY, 0); err != nil {
			return fmt.Errorf("--pick found no terminal to draw on; use --serve to show a page instead")
		}
		defer screen.Close()
	default:
		return fmt.Errorf("output must be a terminal; run gloss directly in your terminal, or use --print for a static view or --serve for a browser viewer")
	}
	qrPath := ""
	if opts.QR != "" {
		path, remove, err := qrFile(opts.QR)
		if err != nil {
			return err
		}
		defer remove()
		qrPath, opts.Files = path, []string{path}
		if !exporting && !opts.Info && opts.Prompt == "" {
			opts.Prompt = opts.QR // Say what the code holds, above it.
		}
	}
	stdinPath := ""
	defer func() {
		if stdinPath != "" {
			_ = os.Remove(stdinPath)
		}
	}()
	for i, path := range opts.Files {
		if path == "-" {
			if stdinTTY {
				return fmt.Errorf("'-' requires piped or redirected input")
			}
			if stdinPath == "" {
				data, err := document.ReadLimited(os.Stdin)
				if err != nil {
					return fmt.Errorf("stdin: %w", err)
				}
				f, err := os.CreateTemp("", "gloss-stdin-*")
				if err != nil {
					return err
				}
				stdinPath = f.Name()
				opts.Stdin = stdinPath
				_, writeErr := f.Write(data)
				closeErr := f.Close()
				if err := errors.Join(writeErr, closeErr); err != nil {
					return err
				}
			}
			opts.Files[i] = stdinPath
			continue
		}
	}
	// A web address among the arguments is downloaded first, and shown or
	// exported as the file it became. Downloads last the session; an
	// interrupt meanwhile ends the download, and so the session, in order.
	downloads := fetchFolder{agent: opts.UserAgent}
	defer func() { downloads.discard(picked.paths) }()
	var failed func(address string, err error)
	if opts.Info {
		failed = func(address string, err error) {
			if opts.Failed == nil {
				opts.Failed = map[string]string{}
			}
			opts.Failed[address] = err.Error()
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	opts.Files, opts.Origins, err = downloadArguments(ctx, opts.Files, &downloads, opts.Accept, failed)
	stop()
	if err != nil {
		return err
	}
	if len(opts.Globs) > 0 {
		if opts.Files, err = expandGlobs(opts.Files, opts.Globs, os.Stderr); err != nil {
			return err
		}
	}
	if opts.Info {
		return describe(opts, os.Stdout, os.Stderr)
	}
	for _, path := range opts.Files {
		if info, e := os.Stat(path); e == nil && info.IsDir() {
			continue
		}
		if err := opts.Accept.CheckFile(nil, path); err != nil {
			return fmt.Errorf("%q: %w", path, err)
		}
	}
	opts.Browse, opts.Files, err = inputs(opts.Files, opts.Type, exporting, os.Stderr)
	if err != nil {
		return err
	}

	if opts.Grep != nil {
		return grepFiles(opts, os.Stdout, os.Stderr)
	}
	if opts.Text {
		return textFiles(opts, os.Stdout, os.Stderr)
	}
	if opts.Print {
		w, h := printSize(opts, os.Stdout)
		return printFiles(opts, os.Stdout, w, h)
	}
	if exporting {
		return exportFiles(opts, os.Stdout, os.Stderr)
	}
	if opts.FetchAllowed {
		opts.Fetch = func(address string) (string, error) {
			return downloads.fetch(context.Background(), address, opts.Accept)
		}
		if !opts.Serve {
			if opts.Maps, err = mapProvider(os.Getenv, opts.UserAgent); err != nil {
				return err
			}
		}
	}
	if opts.Serve {
		picked, err = served(opts, os.Stdout, os.Stderr)
		return err
	}
	var scratch scratchFolder
	defer scratch.remove()
	err = configureOpenWith(&opts, nativeOpenWith)
	if err != nil {
		fmt.Fprintf(os.Stderr, "gloss: open-with preferences: %v\n", err)
	}
	opts.IsTemporary = func(path string) bool {
		if path == stdinPath && stdinPath != "" || path == qrPath && qrPath != "" {
			return true
		}
		return downloads.holds(path) || scratch.holds(path)
	}
	opts.OpenLink = browse
	opts.CopyImage = func(png []byte) error { return clipimage.Copy(context.Background(), png) }
	opts.Scratch = scratch.write
	opts.ThemeChoices = theme.Available()
	opts.SaveTheme = theme.SaveChoice
	opts.SaveFont = saveFontChoice
	if !opts.fontExplicit {
		if name, mono, err := savedFontChoice(); err == nil {
			opts.Font, opts.Mono = name, mono
		}
	}
	if !opts.themeExplicit {
		if ref, err := theme.SavedChoice(); err == nil {
			opts.Theme = ref
		}
	}
	if opts.Theme != "auto" {
		opts.ThemeChoices = append(opts.ThemeChoices, theme.Choice{Name: theme.Name(opts.Theme), Ref: opts.Theme})
	}
	m := app.New(opts.Options)
	defer m.Close()
	var drawn []tea.ProgramOption
	if screen != nil {
		drawn = append(drawn, tea.WithOutput(screen))
	}
	ended := make(chan error, 1)
	// Bubble Tea opens the controlling TTY automatically when stdin is a pipe.
	program := tea.NewProgram(m, drawn...)
	go func() {
		_, err := program.Run()
		ended <- err
	}()
	var late <-chan time.Time
	if opts.Timeout > 0 {
		late = time.After(opts.Timeout)
	}
	select {
	case err = <-ended:
	case <-late:
		program.Kill()
		<-ended
		return handoff.ErrTimeout
	}
	if err != nil {
		return err
	}
	picked, err = finish(m, opts, os.Stdout, os.Stderr)
	return err
}

// pickAnswer is what the user picked, which a fetched file may be among, and
// the message an upload-only request may send with it.
type pickAnswer struct {
	paths   []string
	message string
}

// discardFetched empties the folder of fetched files, except for what was
// picked, and removes the folder once nothing is left in it.
func discardFetched(dir string, picked []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if path := filepath.Join(dir, entry.Name()); !slices.Contains(picked, path) {
			os.Remove(path)
		}
	}
	if left, err := os.ReadDir(dir); err == nil && len(left) == 0 {
		os.Remove(dir)
	}
}

// arguments are the files named, or standard input when a document is piped
// in and none is. A program that starts gloss to ask for a file pipes in
// nothing: there, standard input is read only if it is named.
func arguments(opts options, terminal bool) []string {
	if len(opts.Files) == 0 && opts.QR == "" && !terminal && !opts.Serve && !opts.Pick {
		return []string{"-"}
	}
	return opts.Files
}

// fileArguments marks the arguments pflag takes as files: those that are not
// options, nor the value an option takes in the next argument, and all after
// "--".
func fileArguments(f *pflag.FlagSet, args []string) []bool {
	files := make([]bool, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			for j := i + 1; j < len(args); j++ {
				files[j] = true
			}
			return files
		case strings.HasPrefix(arg, "--"):
			if flag := f.Lookup(arg[2:]); flag != nil && flag.NoOptDefVal == "" {
				i++ // --name value
			}
		case strings.HasPrefix(arg, "-") && arg != "-":
			// A cluster of shorthands; the first that takes a value takes the
			// rest of the argument, or else the next one.
			for j := 1; j < len(arg); j++ {
				if flag := f.ShorthandLookup(arg[j : j+1]); flag != nil && flag.NoOptDefVal == "" {
					if j == len(arg)-1 {
						i++
					}
					break
				}
			}
		default:
			files[i] = true
		}
	}
	return files
}

// handoffConfig is what the handoff protocol takes from the command line. A
// detached start readies its arguments here, where downloads are made.
func handoffConfig(opts options) handoff.Config {
	return handoff.Config{
		Options: opts.Options, Listen: opts.Listen, AdvertiseHost: opts.AdvertiseHost,
		Timeout: opts.Timeout, JSON: opts.JSON, PickWeb: opts.PickWeb,
		Detached: opts.Detached, Resume: opts.Resume, Status: opts.Status, Cancel: opts.Cancel,
		Arguments: func(ctx context.Context, args []string, dir string, stdin io.Reader) ([]string, error) {
			return detachArguments(ctx, args, opts.fileArgs, dir, stdin, opts.Accept, opts.UserAgent)
		},
	}
}

// served shows the viewer on a page, and waits for it to be done with.
func served(opts options, stdout, stderr io.Writer) (pickAnswer, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, end := context.WithCancel(ctx)
	defer end()
	cfg := handoffConfig(opts)
	s, err := handoff.Start(ctx, end, cfg)
	if err != nil {
		return pickAnswer{}, err
	}
	defer s.Close()
	fmt.Fprintf(stderr, "gloss: viewer at %s\n", s.URL)
	if !opts.NoOpen {
		if err := browse(s.URL); err != nil {
			fmt.Fprintf(stderr, "gloss: no browser could be opened (%v); open the address yourself\n", err)
		}
	}
	if opts.PickWeb {
		paths, message, err := s.WaitWebPick(ctx, cfg, stderr)
		if err != nil {
			s.Discard()
			return pickAnswer{}, err
		}
		return pickAnswer{paths, message}, handoff.AnswerPicked(paths, message, opts.JSON, stdout)
	}
	var picked pickAnswer
	m, err := s.Wait(opts.Timeout)
	if err == nil && m == nil {
		err = handoff.ErrCancelled
	}
	if err == nil {
		defer m.Close()
		picked, err = finish(m, opts, stdout, stderr)
	}
	if err != nil || !opts.Pick {
		s.Discard()
	}
	return picked, err
}

// finish reports what came of the viewer once the screen is given back:
// what was skipped, and the paths a pick was waiting for.
func finish(m *app.Model, opts options, stdout, stderr io.Writer) (pickAnswer, error) {
	picked := pickAnswer{paths: m.Picked()}
	for _, line := range m.Skipped() {
		fmt.Fprintln(stderr, line)
	}
	if err := m.Err(); err != nil && !opts.Pick {
		return picked, err
	}
	if !opts.Pick {
		return picked, nil
	}
	if len(picked.paths) == 0 {
		return picked, handoff.ErrCancelled
	}
	for _, path := range picked.paths {
		fmt.Fprintln(stdout, path)
	}
	return picked, nil
}

var errNoInputs = errors.New("no supported input files")

// inputs sorts the arguments into files to show and a folder to browse.
// Given files, folders are skipped, as a glob needs. A folder is browsed only
// when there is nothing else to show, and with no arguments at all the viewer
// opens empty.
func inputs(paths []string, forced string, exporting bool, stderr io.Writer) (string, []string, error) {
	if exporting {
		if len(paths) == 0 {
			return "", nil, fmt.Errorf("no input: name a file to export")
		}
		files, err := supportedFiles(paths, forced, stderr)
		return "", files, err
	}
	if len(paths) == 0 {
		return "", nil, nil
	}
	var skipped bytes.Buffer
	files, err := supportedFiles(paths, forced, &skipped)
	if errors.Is(err, errNoInputs) {
		for i, path := range paths {
			if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
				// Report the rest, but not the folder that is put to use.
				if rest := slices.Delete(slices.Clone(paths), i, i+1); len(rest) > 0 {
					_, _ = supportedFiles(rest, forced, stderr)
				}
				return path, nil, nil
			}
		}
	}
	_, _ = stderr.Write(skipped.Bytes())
	return "", files, err
}

// expandGlobs searches each folder named, or the working folder when none
// is, for the files the patterns match, and puts them in the folder's
// place; files named stay as they are. The search keeps to the limits
// gloss sets for one, and says on stderr where it stopped short.
func expandGlobs(paths, patterns []string, stderr io.Writer) ([]string, error) {
	var out []string
	folders := 0
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			folders++
			found, err := document.Find(path, patterns, document.Limits{})
			if err != nil {
				return nil, err
			}
			if found.Note != "" {
				fmt.Fprintf(stderr, "gloss: %s: %s\n", path, found.Note)
			}
			if len(found.Paths) == 0 {
				fmt.Fprintf(stderr, "gloss: %s: no match for %s\n", path, strings.Join(patterns, " "))
			}
			out = append(out, found.Paths...)
			continue
		}
		out = append(out, path)
	}
	if folders == 0 && len(paths) == 0 {
		return expandGlobs([]string{"."}, patterns, stderr)
	}
	return out, nil
}

// Unsupported arguments are skipped so shell globs can contain unrelated files.
func supportedFiles(paths []string, forced string, stderr io.Writer) ([]string, error) {
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		_, err := document.Probe(path, forced)
		if errors.Is(err, document.ErrUnsupported) || errors.Is(err, document.ErrNotRegular) || errors.Is(err, document.ErrDirectory) {
			fmt.Fprintln(stderr, document.Skipped(path, err))
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%q: %w", path, err)
		}
		files = append(files, path)
	}
	if len(files) == 0 {
		return nil, errNoInputs
	}
	return files, nil
}
