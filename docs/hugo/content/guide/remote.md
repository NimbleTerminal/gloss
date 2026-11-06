---
title: "Steering a running viewer"
weight: 35
bookToC: false
---

# Steering a running viewer

A gloss that is already open can be told what to show by another program on
the same machine: an editor, a terminal multiplexer that keeps a preview pane,
a build script, or another gloss. Start the viewer with a socket name, and
point it at files from anywhere:

```sh
gloss --socket preview README.md          # in one pane: a viewer that takes requests
gloss --remote preview notes.pdf --page 3 # from another: show this, on page 3
gloss --remote preview --reload           # the file was rewritten: load it again
gloss --remote preview                    # which file is it showing? (prints its path)
gloss --remote preview --quit             # close it
```

`--socket NAME` makes the viewer listen on a Unix domain socket,
`gloss-NAME.sock` in the temporary folder (`$TMPDIR`, or `/tmp`); a name with a
slash in it is used as the socket's path. A name is letters, digits, dots,
dashes and underscores. The socket is readable by your user alone, and a stale
one left by a viewer that crashed is replaced; a name another viewer is still
listening on is an error.

`--remote NAME` is the other side: it sends the request and exits at once, 0
when the viewer carried it out and 1 when it did not, saying why: no viewer
listening on that name (it was quit), a file that is not there, a file gloss
cannot show. Scripts use that exit status to know whether to start a new
viewer. The files are made absolute before they are sent, so the two sides
may run in different folders; an `https://` address is sent as it is, and
opens only if the viewer runs with `--fetch`. A folder opens the file browser
there. Several files open as a drop does: the menu lists them.

The request lands whatever the viewer is doing. Help, the details box, a
search prompt, a picker, the actions menu, a text selection, the file browser
and the file menu are put away first, and the file is shown as if it had just
been opened: zoom and pan reset, the page is the one asked for or the first.
The viewer treats what arrives as paths to probe, exactly as it does files
dropped on it: `--type` and `--accept` still apply, and a file it cannot show
is skipped and named in the answer.

With `--json`, `--remote` prints the viewer's whole answer:

```sh
gloss --remote preview --json
# {"ok":true,"file":"/home/me/notes.pdf","kind":"pdf","page":3,"pages":12,"files":2}
gloss --remote preview a.png missing.png --json
# {"ok":true,"added":1,"skipped":["missing.png: no such file"],"file":"/home/me/a.png","files":3}
```

`gloss skill schema` describes these objects (`remote_request` and
`remote_response`). Any program can speak to the socket itself: connect, write
the request as one line of JSON (`{"action":"open","paths":["/abs/path"]}`),
read one line back. The actions are `open`, `reload`, `quit` and `status`.

On Unix, `SIGUSR1` has a viewer reload its file too, socket or no socket:
`pkill -USR1 gloss` refreshes every gloss after a build rewrote what they show.

Previews, `--serve` pages and the headless modes take no socket; `--remote`
takes no other mode.
