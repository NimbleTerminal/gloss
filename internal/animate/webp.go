package animate

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"time"

	"golang.org/x/image/webp"
)

// Animated WebP is a RIFF container: a VP8X header with the animation flag,
// an ANIM chunk with the background color and loop count, and one ANMF chunk
// per frame holding its placement, duration, blend and disposal, and the
// frame's own ALPH/VP8/VP8L chunks. Only the container is parsed here; the
// frame pixels are decoded by golang.org/x/image/webp from a synthetic
// single-image file, so no second codec is involved.

const (
	webpAnimationFlag = 1 << 1
	webpAlphaFlag     = 1 << 4
)

// IsAnimatedWebP reports whether data starts like a WebP file whose extended
// header declares an animation. It reads only the first 30 bytes.
func IsAnimatedWebP(data []byte) bool {
	return len(data) >= 30 && string(data[:4]) == "RIFF" && string(data[8:16]) == "WEBPVP8X" && binary.LittleEndian.Uint32(data[16:20]) == 10 && data[20]&webpAnimationFlag != 0
}

type webpFrameInfo struct {
	rect     image.Rectangle
	delay    time.Duration
	blend    draw.Op
	disposal byte
	alpha    bool   // The frame data carries an ALPH chunk.
	data     []byte // The frame's bitstream chunks inside data.
}

type webpAnimation struct {
	bounds     image.Rectangle
	alpha      bool
	background color.NRGBA
	loop       int // As stored: 0 forever, otherwise the number of plays.
	frames     []webpFrameInfo
}

// parseWebPAnimation walks the container's chunk headers, checking declared
// lengths against the file and every frame against the canvas and the frame
// budgets, without decoding any pixels.
func parseWebPAnimation(data []byte) (*webpAnimation, error) {
	if !IsAnimatedWebP(data) {
		return nil, fmt.Errorf("webp: not an animation")
	}
	a := &webpAnimation{}
	chunks, err := webpChunks(data[12:])
	if err != nil {
		return nil, err
	}
	seenANIM := false
	pixels := 0
	for _, c := range chunks {
		switch c.id {
		case "VP8X":
			if len(c.payload) != 10 {
				return nil, fmt.Errorf("webp: invalid VP8X chunk")
			}
			a.alpha = c.payload[0]&webpAlphaFlag != 0
			w, h := int(webpUint24(c.payload[4:]))+1, int(webpUint24(c.payload[7:]))+1
			if w > MaxPixels/h {
				return nil, fmt.Errorf("WebP canvas exceeds %d pixels", MaxPixels)
			}
			a.bounds = image.Rect(0, 0, w, h)
		case "ANIM":
			if len(c.payload) != 6 || seenANIM {
				return nil, fmt.Errorf("webp: invalid ANIM chunk")
			}
			seenANIM = true
			a.background = color.NRGBA{R: c.payload[2], G: c.payload[1], B: c.payload[0], A: c.payload[3]}
			a.loop = int(binary.LittleEndian.Uint16(c.payload[4:]))
		case "ANMF":
			if !seenANIM || a.bounds.Empty() || len(c.payload) < 16 {
				return nil, fmt.Errorf("webp: invalid ANMF chunk")
			}
			if len(a.frames) >= MaxGIFFrames {
				return nil, fmt.Errorf("WebP exceeds %d frames", MaxGIFFrames)
			}
			x, y := 2*int(webpUint24(c.payload[0:])), 2*int(webpUint24(c.payload[3:]))
			w, h := int(webpUint24(c.payload[6:]))+1, int(webpUint24(c.payload[9:]))+1
			f := webpFrameInfo{rect: image.Rect(x, y, x+w, y+h), delay: normalizedDelay(time.Duration(webpUint24(c.payload[12:])) * time.Millisecond), blend: draw.Over, disposal: DisposalNone}
			if !f.rect.In(a.bounds) {
				return nil, fmt.Errorf("WebP has an out-of-canvas frame")
			}
			if w > (MaxGIFPixels-pixels)/4/h {
				return nil, fmt.Errorf("WebP frames exceed %d decoded pixels", MaxGIFPixels/4)
			}
			pixels += w * h * 4
			if c.payload[15]&2 != 0 {
				f.blend = draw.Src
			}
			if c.payload[15]&1 != 0 {
				f.disposal = DisposalBackground
			}
			f.data = c.payload[16:]
			sub, err := webpChunks(f.data)
			if err != nil {
				return nil, err
			}
			bitstream := false
			for _, s := range sub {
				switch s.id {
				case "ALPH":
					f.alpha = true
				case "VP8 ", "VP8L":
					bitstream = true
				}
			}
			if !bitstream {
				return nil, fmt.Errorf("webp: frame without image data")
			}
			a.frames = append(a.frames, f)
		}
	}
	if len(a.frames) == 0 {
		return nil, fmt.Errorf("webp: animation without frames")
	}
	return a, nil
}

type webpChunk struct {
	id      string
	payload []byte
}

// webpChunks splits RIFF chunks, failing on one that overruns the data.
func webpChunks(b []byte) ([]webpChunk, error) {
	var out []webpChunk
	for len(b) > 0 {
		if len(b) < 8 {
			return nil, fmt.Errorf("webp: truncated chunk header")
		}
		n := binary.LittleEndian.Uint32(b[4:8])
		if uint64(n) > uint64(len(b)-8) {
			return nil, fmt.Errorf("webp: chunk %q overruns the file", b[:4])
		}
		out = append(out, webpChunk{id: string(b[:4]), payload: b[8 : 8+n]})
		b = b[8+n:]
		if n%2 == 1 && len(b) > 0 {
			b = b[1:]
		}
	}
	return out, nil
}

func webpUint24(b []byte) uint32 { return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 }

// PreflightWebP sizes an animation from its chunk headers alone. A still
// WebP counts as one frame of its own size.
func PreflightWebP(data []byte) (Stats, error) {
	if len(data) > MaxFileBytes {
		return Stats{}, ErrTooLarge
	}
	if !IsAnimatedWebP(data) {
		cfg, err := webp.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return Stats{}, err
		}
		if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > MaxPixels/cfg.Height {
			return Stats{}, fmt.Errorf("WebP canvas exceeds %d pixels", MaxPixels)
		}
		return Stats{Bounds: image.Rect(0, 0, cfg.Width, cfg.Height), Frames: 1, Bytes: cfg.Width * cfg.Height * 4}, nil
	}
	a, err := parseWebPAnimation(data)
	if err != nil {
		return Stats{}, err
	}
	stats := Stats{Bounds: a.bounds, Frames: len(a.frames)}
	for _, f := range a.frames {
		stats.Bytes += f.rect.Dx() * f.rect.Dy() * 4
	}
	return stats, nil
}

// DecodeWebP decodes a WebP file. An animation becomes a Player when animate
// is set; otherwise, as for previews and exports, only its first frame is
// composited onto the canvas. A still returns its image and no Player.
func DecodeWebP(data []byte, animate bool) (image.Image, *Player, error) {
	if len(data) > MaxFileBytes {
		return nil, nil, ErrTooLarge
	}
	if !IsAnimatedWebP(data) {
		if _, err := PreflightWebP(data); err != nil {
			return nil, nil, err
		}
		img, err := webp.Decode(bytes.NewReader(data))
		return img, nil, err
	}
	a, err := parseWebPAnimation(data)
	if err != nil {
		return nil, nil, err
	}
	frames := a.frames
	if !animate {
		frames = frames[:1]
	}
	// Transparent animations use a transparent canvas, as browsers do; opaque
	// ones use the declared background, like an opaque GIF.
	clip := &Clip{Bounds: a.bounds, LoopCount: webpLoopCount(a.loop), Background: a.background}
	if a.alpha {
		clip.Background = color.Transparent
	}
	for _, f := range frames {
		img, err := decodeWebPFrame(f)
		if err != nil {
			return nil, nil, err
		}
		clip.Frames = append(clip.Frames, ClipFrame{Image: img, Delay: f.delay, Disposal: f.disposal, Blend: f.blend})
	}
	p := newPlayer(clip)
	if len(frames) == 1 {
		return p.Image(), nil, nil
	}
	return p.Image(), p, nil
}

// decodeWebPFrame hands the frame's own chunks to the still decoder as a
// file of their own, which needs an extended header when alpha is present.
func decodeWebPFrame(f webpFrameInfo) (*image.NRGBA, error) {
	var file bytes.Buffer
	file.WriteString("RIFF\x00\x00\x00\x00WEBP")
	if f.alpha {
		vp8x := [18]byte{'V', 'P', '8', 'X', 10, 0, 0, 0, webpAlphaFlag}
		webpPut24(vp8x[12:], uint32(f.rect.Dx()-1))
		webpPut24(vp8x[15:], uint32(f.rect.Dy()-1))
		file.Write(vp8x[:])
	}
	file.Write(f.data)
	binary.LittleEndian.PutUint32(file.Bytes()[4:], uint32(file.Len()-8))
	src, err := webp.Decode(&file)
	if err != nil {
		return nil, err
	}
	if src.Bounds().Size() != f.rect.Size() {
		return nil, fmt.Errorf("WebP frame is %v, not its declared %v", src.Bounds().Size(), f.rect.Size())
	}
	dst := image.NewNRGBA(f.rect)
	draw.Draw(dst, f.rect, src, src.Bounds().Min, draw.Src)
	return dst, nil
}

func webpPut24(b []byte, v uint32) { b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16) }

// webpLoopCount maps WebP's play count (0 forever) onto Clip's GIF-style
// count: -1 once, 0 forever, otherwise repeats after the first pass.
func webpLoopCount(plays int) int {
	switch plays {
	case 0:
		return 0
	case 1:
		return -1
	}
	return plays - 1
}
