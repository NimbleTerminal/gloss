# Animated GIFs inside Markdown

Designed with the owner on 2026-10-10. The trigger: `gloss README.md` shows
the demo recording as a still of its first frame.

## Goal

A GIF embedded in a Markdown document plays in place, in both the default
text view and the typeset view, looping as the file says, with the bounds and
cancellation discipline the standalone GIF viewer already has. Previews, the
file-list thumbnails, and headless PNG exports keep the first frame.

The playback machinery becomes an internal package shaped like a NimbleTerminal
component, so the owner can extract it later (beside `ntqrcode`) as a
move rather than a redesign.

## What exists

- `internal/document/gif.go`: bounded decoding (preflight before `DecodeAll`:
  canvas up to 32 MP, 1,000 frames, 64 Mi total decoded frame pixels) and
  `GIFPlayer`, an immutable playback position with disposal, loop counts, and
  seek. Tied to `*gif.GIF`.
- `internal/app/animation.go`: playback for the one main picture. Timer and
  epoch, one pending command at a time, Kitty pacing (the next delay starts
  after the previous frame is transmitted), a fresh image ID per frame from a
  reserved block, the retained front placement that is retired only after the
  replacement's grid is shown (ghostty-web keeps a texture per ID), speed,
  pause, finish, and seek through `scrubber.go`. Reads and writes the model's
  `pic`, `picID`, `source`, `animationFront`, `animationPlacement`.
- `internal/document/markdown.go`: decodes each image once with the still
  decoders, so a GIF is its first frame. Budgets: 32 references, 16 Mi
  decoded pixels after the 1600-edge resize, 128 MiB encoded input.
- `internal/app/markdown.go` (text view): one `picture.Model` per image at a
  fixed Kitty ID, placed as rows among the text lines.
- `internal/document/typeset_*.go`: the whole viewport is one raster from the
  layout's drawing operations; image operations carry the asset and its page
  rectangle. The app shows the raster through the main picture and the
  standalone animation path.
- `Request.Animate` already says whether bounded playback is wanted; exports
  and previews leave it false.

## Decisions

1. **New package `internal/animate`**, no dependency on `internal/app` or
   `internal/document`. It holds:
   - `Clip`: a decoded animation, format-neutral: canvas bounds, paletted or
     RGBA frames with their rectangles, delays, disposal, blend mode, loop
     count, and background. The frame budget is 64 MiB of decoded frame
     storage, which for paletted GIF frames is the existing 64 Mi pixels.
     The GIF decoder (moved from `document/gif.go`, with its preflight) is
     the only producer in this change; animated WebP is the planned second
     (a RIFF demuxer over the still decoder already in use, with RGBA frames
     at four bytes per pixel), and APNG would only have to produce a `Clip`.
     The formats guide keeps saying neither is supported until it is.
   - `Player`: today's `GIFPlayer` over a `Clip`: immutable positions,
     `Next`, `Seek`, `Restart`, `Delay`, `Frame`, `Frames`, `Image`.
   - `Source`: the frame-source interface the animator drives:
     `Next() (image.Image, delay time.Duration, more bool)`, plus `Restart()`.
     `PlayerSource` wraps one `Player` and carries speed (powers of two,
     −2..2) and the seek target.
   - `Animator`: the state now in `animation.go`, owned per picture rather
     than by the model: timer cancel, epoch, pending, paused, finished,
     awaiting-transmission, the fresh-ID chain from a host-supplied allocator
     (`func() int`, gloss passes `nextKittyID`; each call reserves 1,000 IDs),
     the front picture and in-flight placement, and the scheduling rules
     (one timer or composition command at a time; Kitty waits for the
     acknowledgement; hidden animators cancel without retaining frames).
     Messages (`Tick`, `Frame`, `Presented`) name their animator and epoch.
     Its API is commands over a `picture.Model` the host hands it:
     `Schedule(visible bool)`, `Update(msg) (cmd, handled)`, `Show(img)` (the
     fresh-picture dance now in `showPicture`), `View()`, `Pause/Toggle`,
     `SetSpeed`, `Stop`, `Clear`, `Keep` (keep-screen quit), `Resize`.
   The standalone GIF viewer, the text view's inline images, and the typeset
   compositor are its three callers. `scrubber.go` stays in `internal/app`
   and calls `Player.Seek` through the animator's source.
2. **Loader keeps a player per GIF.** `MarkdownImage` gains `Animation
   *animate.Player`; the typeset asset map keeps one per destination. When
   `Request.Animate` is set and the detected kind is `gif`, the image is
   decoded with the animated decoder; otherwise as today. The first frame is
   still resized to the 1600 edge and counted in the 16 Mi still budget; the
   player keeps the full canvas. Two document budgets, checked before
   `DecodeAll` from the preflight's counts: at most 8 animated images and
   64 Mi decoded frame pixels in total. A GIF over either budget keeps its
   first frame and is counted in the info box ("Animated: 2 of 3 GIFs;
   1 over budget"). Duplicate destinations share one player set.
3. **Text view.** Each image block whose image has a player gets its own
   `Animator` and `PlayerSource` at 1× with no controls. Only images with at
   least one row inside the visible window are scheduled; `scroll` and
   `layout` re-evaluate, cancelling timers of images that left. Each frame is
   scaled once to the asset's bounds (the resized first frame's size) before
   `Show`. The animator's fresh pictures replace the fixed-ID pictures for
   animated images; still images keep theirs. `view()` reads the animator's
   retained front grid, as the main picture does. Finished GIFs stay on
   their last frame; `R`/`r` restart them.
4. **Typeset view.** `renderTypeset` also returns `Animated
   []TypesetRegion{Rect image.Rectangle; Player *animate.Player; Destination
   string}`: the on-raster rectangles (after the viewport offset and scale)
   of image operations that have a player and overlap the viewport. The
   main picture's animator gets a `CompositeSource`: the base raster, the
   regions, and a position per region. `Next` picks the region whose next
   frame is due soonest, advances it, draws its frame scaled with CatmullRom
   into a copy of the base at the rectangle (every region at its current
   frame, so the copy is always complete), and returns that copy with the
   delay to the next due region. A reflow or scroll replaces the base and
   regions; positions are kept by destination so playback survives.
   Headless typeset export is unchanged: `Animate` is false.
5. **Controls.** Unchanged for standalone GIFs (Space, `<`/`>`, Backspace,
   `,`/`.`, Home/End, the seek bar). Inline GIFs have none: Space keeps
   scrolling the text view, and the typeset keys keep their meaning. The
   info box (`i`) says how many GIFs are playing. `e` exports the frame on
   screen in either view.
6. **Lifecycle.** Leaving the document, reloading, toggling source (`s`) or
   typeset (`t`), suspending, help, the menu, and the preview pane cancel
   through the animators' epochs and the loader's context, as the standalone
   path does. Search indexing and highlights read the text layout only and
   are unchanged. Glyph fallback mode works through the same animators with
   no ID dance.
7. **Bounds that stay as they are.** 32 image references, 16 Mi still pixels,
   128 MiB encoded input, the per-GIF preflight, and the typeset raster and
   layout caps. The typeset compositor adds one viewport-sized copy per
   frame, already the cost of a standalone GIF frame.

## Testing

- `internal/animate`: the GIF decoder tests move with it. Animator tests
  port the current `animation_test.go` cases against a fake picture and a
  fake clock: one pending command at a time, epoch invalidation, Kitty
  pacing on `Presented`, fresh IDs and retirement order, pause/finish/
  restart, speed, keep-screen quit, resize.
- Loader: a player is present with `Animate` and absent without; the 8-image
  and 64 Mi budgets leave the first frame and are reported; duplicates share.
  The existing "GIF must show first frame" test becomes the `Animate: false`
  case.
- Text view: only on-screen images are scheduled; scrolling off cancels and
  scrolling back resumes; source toggle stops all; frames reach the right
  picture.
- Typeset: a region's rectangle matches the raster; after two composed
  frames a pixel inside the region differs and one outside is unchanged;
  two regions advance on their own delays; reflow keeps positions.
- Model: `R` restarts inline GIFs; leaving the file cancels; headless export
  of Markdown with a GIF is still the first frame; `--info` names the count.

## Documentation

The Markdown guide (embedded GIFs play; what pauses them; the two budgets),
the formats guide if any limit changes, AGENTS.md (the new package and the
Markdown limits), the README key table note that the GIF keys are for
standalone GIFs, the skill only if an agent-visible limit changes, and
CHANGELOG.

## Out of scope

Animated WebP, APNG, and HEIF sequences; controls for inline GIFs; Kitty
overlay placements; animating inline images that are labels in the text
view; extracting the package to its own module (the owner will do that
later).
