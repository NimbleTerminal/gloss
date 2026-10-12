package main

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/NimbleMarkets/ntcharts-svg/svg"

	"nimbleterminal.dev/gloss/internal/document"
)

// fetchTimeout bounds one download, from the command line or a document.
const fetchTimeout = time.Minute

// noFetchVariable, when set and not empty, keeps gloss off the network:
// web addresses on the command line are errors, and --fetch is refused.
const noFetchVariable = "GLOSS_NO_FETCH"

func fetchDisabled() bool { return os.Getenv(noFetchVariable) != "" }

// userAgentVariable names the User-Agent downloads send when --user-agent
// does not; without either, it is gloss and its version.
const userAgentVariable = "GLOSS_USER_AGENT"

// userAgent resolves what downloads send as their User-Agent: the option,
// else the environment, else gloss and its version. A value is one line of
// at most 1 KiB without control characters, which a header cannot carry.
func userAgent(option string) (string, error) {
	if option != "" {
		if err := validUserAgent(option); err != nil {
			return "", fmt.Errorf("--user-agent %w", err)
		}
		return option, nil
	}
	if env := os.Getenv(userAgentVariable); env != "" {
		if err := validUserAgent(env); err != nil {
			return "", fmt.Errorf("%s %w", userAgentVariable, err)
		}
		return env, nil
	}
	return "gloss/" + version, nil
}

func validUserAgent(agent string) error {
	if len(agent) > 1024 {
		return fmt.Errorf("must be at most 1 KiB")
	}
	if strings.ContainsFunc(agent, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return fmt.Errorf("must be one line without control characters")
	}
	return nil
}

// webAddress reports whether an argument names a web address rather than a
// file: an http or https URL with a host. Nothing else is one, so a path
// with a drive letter or a dot-slash in front stays a path.
func webAddress(arg string) bool {
	u, err := url.Parse(arg)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// otherScheme reports a URL of some other scheme, such as ftp, which the
// user is told is not fetched, rather than looked for as a file.
func otherScheme(arg string) bool {
	scheme, _, found := strings.Cut(arg, "://")
	return found && len(scheme) > 1 && strings.IndexFunc(scheme, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '+' || r == '-' || r == '.')
	}) < 0
}

// fetchFolder is the session's folder of downloads: addresses named on the
// command line, and with --fetch those chosen inside documents. It is made
// when a download first needs it and emptied when the session ends, except
// for a download the user picked, which is the caller's to delete. A fetch
// runs as a command of the viewer while the model asks what is a download,
// so the folder is locked.
type fetchFolder struct {
	mu    sync.Mutex
	dir   string
	agent string // What downloads send as their User-Agent.
}

// path makes the folder if it does not yet exist.
func (f *fetchFolder) path() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dir == "" {
		dir, err := os.MkdirTemp("", "gloss-fetch-")
		if err != nil {
			return "", err
		}
		f.dir = dir
	}
	return f.dir, nil
}

// holds reports whether a path is in the folder, that is, a download.
func (f *fetchFolder) holds(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dir == "" {
		return false
	}
	rel, err := filepath.Rel(f.dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// discard empties the folder of downloads, except for what was picked, and
// removes the folder once nothing is left in it.
func (f *fetchFolder) discard(picked []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dir != "" {
		discardFetched(f.dir, picked)
	}
}

// fetch downloads one address into the folder, within the time allowed.
func (f *fetchFolder) fetch(ctx context.Context, address string, accept document.AcceptFilter) (string, error) {
	if fetchDisabled() {
		return "", fmt.Errorf("fetching is disabled by %s", noFetchVariable)
	}
	dir, err := f.path()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	return document.Fetch(ctx, address, dir, f.agent, accept)
}

// downloadArguments puts, in the place of each web address among the
// arguments, the file it downloads, and says in origins which address each
// such file came from. Naming an address is asking for it, so no option is
// needed; GLOSS_NO_FETCH forbids it all the same. A download that fails is
// an error, as a file that cannot be opened is, unless failed is given: then
// it is told, and the address stays among the arguments for it to account
// for, as --info does with the files it cannot describe.
func downloadArguments(ctx context.Context, args []string, folder *fetchFolder, accept document.AcceptFilter, failed func(address string, err error)) (files []string, origins map[string]string, err error) {
	files = slices.Clone(args)
	for i, arg := range args {
		var err error
		switch {
		case webAddress(arg):
			var path string
			if path, err = folder.fetch(ctx, arg, accept); err == nil {
				files[i] = path
				if origins == nil {
					origins = map[string]string{}
				}
				origins[path] = arg
				continue
			}
		case otherScheme(arg):
			if u, e := url.Parse(arg); e == nil && (u.Scheme == "http" || u.Scheme == "https") {
				err = fmt.Errorf("the address has no host")
			} else {
				err = fmt.Errorf("only http and https addresses are fetched")
			}
		default:
			continue
		}
		if failed == nil {
			return nil, nil, fmt.Errorf("%s: %w", redacted(arg), err)
		}
		failed(arg, err) // The address is known to it; the error need not repeat it.
	}
	return files, origins, nil
}

// redacted is an address as it may be printed: without any password it
// carries, and without anything the terminal would act on.
func redacted(address string) string {
	if u, err := url.Parse(address); err == nil && u.User != nil {
		address = u.Redacted()
	}
	return svg.SanitizeForTerminal(address)
}

// detachArguments prepares the arguments of a detached server: standard
// input, when named, is kept in the session folder, and so is what each web
// address names, so the server starts at once with files, and a download
// that fails is the caller's error rather than a server that never comes up.
func detachArguments(ctx context.Context, args []string, dir string, stdin io.Reader, accept document.AcceptFilter, agent string) ([]string, error) {
	child := slices.Clone(args)
	for i, arg := range child {
		if arg != "-" {
			continue
		}
		data, err := document.ReadLimited(stdin)
		if err != nil {
			return nil, fmt.Errorf("stdin: %w", err)
		}
		kept, err := document.WriteNew(filepath.Join(dir, "stdin"), data)
		if err != nil {
			return nil, err
		}
		for j := i; j < len(child); j++ {
			if child[j] == "-" {
				child[j] = kept
			}
		}
		break
	}
	child, _, err := downloadArguments(ctx, child, &fetchFolder{dir: dir, agent: agent}, accept, nil)
	return child, err
}
