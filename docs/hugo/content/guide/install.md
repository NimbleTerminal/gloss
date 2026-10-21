---
title: "Install"
weight: 10
bookToC: false
---

# Install

With [Homebrew](https://brew.sh), on macOS or Linux:

```sh
brew install --cask nimbleterminal/tap/gloss
```

The Homebrew installation note includes `gloss skill install` for AI agents,
and two tools for video, which gloss needs only for that:
[mpv](https://formulae.brew.sh/formula/mpv), whose library plays videos in the
viewer, and [FFmpeg](https://formulae.brew.sh/formula/ffmpeg), whose `ffprobe`
and `ffmpeg` describe them and draw their frames for previews, thumbnails and
exports. Without mpv a video does not play; without FFmpeg it plays, without
details or a still. On Debian or Ubuntu, install the `libmpv2` (or `mpv`) and
`ffmpeg` packages; on Windows, put `libmpv-2.dll` and FFmpeg on your `PATH`.

```sh
brew install mpv       # video playback
brew install ffmpeg    # video details and frames
```

On Debian or Ubuntu, download the `.deb` for your machine from the
[latest release](https://github.com/NimbleTerminal/gloss/releases/latest) and install it:

```sh
sudo apt install ./gloss_*_linux_amd64.deb     # or _arm64
```

The release also has archives for macOS, Linux, and Windows (amd64 and arm64),
each a single binary with nothing else to install. With Go, clone the
repository and run `go install ./cmd/gloss` in it to build from source.

## For agents

If you are installing gloss for an AI agent, also install its skill, which
teaches the agent the headless interface (`--text`, `--output`, `--info`,
`--pick`, `--serve`):

```sh
gloss skill install
```

See [For agents](../agents/) for what the skill does and other ways to install it.
