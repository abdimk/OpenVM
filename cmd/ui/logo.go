package ui

import (
	"bytes"
	"compress/zlib"
	_ "embed"
	"encoding/binary"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"os"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	sixel "github.com/mattn/go-sixel"
)

//go:embed assets/openvm.png
var openvmLogoPNG []byte

type logoPixel struct {
	r, g, b, a uint8
}

type logoImage struct {
	width, height int
	pix           []logoPixel
}

var logoCache struct {
	sync.Mutex
	width    int
	rendered string
}

func OpenVMLogo(maxWidth int) string {
	logoCache.Lock()
	defer logoCache.Unlock()

	if logoCache.width == maxWidth && logoCache.rendered != "" {
		return logoCache.rendered
	}

	out := renderSixelLogo(maxWidth)
	if out == "" {
		out = renderOpenVMLogo(maxWidth)
	}

	logoCache.width = maxWidth
	logoCache.rendered = out
	return out
}

func detectSixel() bool {

	if isWSL() {
		return false
	}
	term := os.Getenv("TERM")
	if strings.Contains(term, "sixel") {
		return true
	}
	termProgram := os.Getenv("TERM_PROGRAM")
	switch termProgram {
	case "WezTerm", "iTerm.app", "mintty", "contour", "Black Box":
		return true
	}

	if os.Getenv("WT_SESSION") != "" {
		return true
	}

	if os.Getenv("MSYSTEM") != "" && os.Getenv("TERM_PROGRAM") == "" {
		return true
	}
	return false
}

func isWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}
	if os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "Microsoft") || strings.Contains(string(data), "WSL")
}

func renderSixelLogo(maxWidth int) string {
	if !detectSixel() || maxWidth < 8 {
		return ""
	}

	img, _, err := image.Decode(bytes.NewReader(openvmLogoPNG))
	if err != nil {
		return ""
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW == 0 || srcH == 0 {
		return ""
	}

	const cellPx = 8
	targetW := maxWidth * cellPx
	if targetW > srcW {
		targetW = srcW
	}
	targetH := srcH * targetW / srcW
	if targetH < 1 {
		targetH = 1
	}

	scaled := resizeNearest(img, targetW, targetH)

	var buf bytes.Buffer
	if err := sixel.NewEncoder(&buf).Encode(scaled); err != nil {
		return ""
	}

	sixelData := buf.String()
	if sixelData == "" {
		return ""
	}

	sixelCells := targetW / cellPx
	padding := (maxWidth - sixelCells) / 2
	if padding > 0 {
		return strings.Repeat(" ", padding) + sixelData
	}
	return sixelData
}

func resizeNearest(src image.Image, w, h int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := bounds.Min.X + x*srcW/w
			sy := bounds.Min.Y + y*srcH/h
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

func renderOpenVMLogo(maxWidth int) string {
	img, err := decodePNG(openvmLogoPNG)
	if err != nil || maxWidth < 8 {
		return ""
	}

	cellW := maxWidth
	cellRows := img.height * cellW / (2 * img.width)

	const maxRows = 14
	if cellRows > maxRows {
		cellRows = maxRows
		cellW = cellRows * 2 * img.width / img.height
	}
	if cellW < 8 || cellRows < 1 {
		return ""
	}

	leftPad := strings.Repeat(" ", (maxWidth-cellW)/2)

	var b strings.Builder
	for cy := 0; cy < cellRows; cy++ {
		y0 := cy * img.height / cellRows
		y1 := (cy + 1) * img.height / cellRows
		mid := (y0 + y1) / 2

		b.WriteString(leftPad)
		for cx := 0; cx < cellW; cx++ {
			x0 := cx * img.width / cellW
			x1 := (cx + 1) * img.width / cellW

			tr, tg, tb, ta := img.avgBlock(x0, x1, y0, mid)
			br, bg, bb, ba := img.avgBlock(x0, x1, mid, y1)

			tr, tg, tb = blendToBlack(tr, tg, tb, ta)
			br, bg, bb = blendToBlack(br, bg, bb, ba)

			b.WriteString(halfCell(tr, tg, tb, ta, br, bg, bb, ba))
		}
		b.WriteString("\n")
	}

	out := strings.TrimRight(b.String(), "\n")
	return out
}

func halfCell(tr, tg, tb, ta, br, bg, bb, ba float64) string {
	const alphaMin = 0.45

	topStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(rgbHex(tr, tg, tb)))

	switch {
	case ta < alphaMin && ba < alphaMin:
		return " "
	case ta < alphaMin:
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color(rgbHex(br, bg, bb))).
			Render("▄")
	case ba < alphaMin:
		return topStyle.Render("▀")
	default:
		return topStyle.
			Background(lipgloss.Color(rgbHex(br, bg, bb))).
			Render("▀")
	}
}

func blendToBlack(r, g, b, a float64) (float64, float64, float64) {
	if a >= 1 {
		return r, g, b
	}
	return r * a, g * a, b * a
}

func rgbHex(r, g, b float64) string {
	to255 := func(v float64) int {
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		return int(v + 0.5)
	}
	return fmt.Sprintf("#%02x%02x%02x", to255(r), to255(g), to255(b))
}

func (img logoImage) avgBlock(x0, x1, y0, y1 int) (r, g, b, a float64) {
	if x1 <= x0 {
		x1 = x0 + 1
	}
	if y1 <= y0 {
		y1 = y0 + 1
	}

	var rs, gs, bs, as float64
	var n float64

	for y := y0; y < y1 && y < img.height; y++ {
		for x := x0; x < x1 && x < img.width; x++ {
			p := img.pix[y*img.width+x]
			af := float64(p.a) / 255
			rs += float64(p.r) * af
			gs += float64(p.g) * af
			bs += float64(p.b) * af
			as += af
			n++
		}
	}
	if n == 0 || as == 0 {
		return 0, 0, 0, 0
	}
	return rs / as, gs / as, bs / as, as / n
}

func decodePNG(data []byte) (logoImage, error) {
	var img logoImage

	sig := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if len(data) < 8 || !bytes.Equal(data[:8], sig) {
		return img, fmt.Errorf("not a PNG file")
	}

	var idat bytes.Buffer
	var (
		width, height, bitDepth, colorType int
		haveHeader                         bool
	)

	pos := 8
	for pos+8 <= len(data) {
		length := int(binary.BigEndian.Uint32(data[pos : pos+4]))
		ctype := string(data[pos+4 : pos+8])
		body := data[pos+8 : pos+8+length]
		pos += 12 + length

		switch ctype {
		case "IHDR":
			width = int(binary.BigEndian.Uint32(body[0:4]))
			height = int(binary.BigEndian.Uint32(body[4:8]))
			bitDepth = int(body[8])
			colorType = int(body[9])
			if body[12] != 0 {
				return img, fmt.Errorf("interlaced PNG not supported")
			}
			haveHeader = true
		case "IDAT":
			idat.Write(body)
		case "IEND":
			pos = len(data)
		}
	}

	if !haveHeader {
		return img, fmt.Errorf("missing IHDR")
	}
	if bitDepth != 8 || (colorType != 6 && colorType != 2) {
		return img, fmt.Errorf("unsupported PNG: depth=%d colorType=%d", bitDepth, colorType)
	}

	bpp := 4
	if colorType == 2 {
		bpp = 3
	}

	raw, err := zlib.NewReader(&idat)
	if err != nil {
		return img, fmt.Errorf("zlib: %w", err)
	}
	defer raw.Close()

	stride := width * bpp
	buf := make([]byte, height*(stride+1))
	if _, err := io.ReadFull(raw, buf); err != nil {
		return img, fmt.Errorf("image data truncated: %w", err)
	}

	img.width = width
	img.height = height
	img.pix = make([]logoPixel, width*height)

	prev := make([]byte, stride)
	cur := make([]byte, stride)

	for y := 0; y < height; y++ {
		filter := buf[y*(stride+1)]
		copy(cur, buf[y*(stride+1)+1:y*(stride+1)+1+stride])

		for i := 0; i < stride; i++ {
			left := 0
			if i >= bpp {
				left = int(cur[i-bpp])
			}
			up := int(prev[i])
			upleft := 0
			if i >= bpp {
				upleft = int(prev[i-bpp])
			}

			switch filter {
			case 0:
			case 1:
				cur[i] = byte(int(cur[i]) + left)
			case 2:
				cur[i] = byte(int(cur[i]) + up)
			case 3:
				cur[i] = byte(int(cur[i]) + (left+up)/2)
			case 4:
				cur[i] = byte(int(cur[i]) + paeth(left, up, upleft))
			default:
				return img, fmt.Errorf("bad filter %d", filter)
			}
		}

		row := img.pix[y*width : (y+1)*width]
		for x := 0; x < width; x++ {
			o := x * bpp
			if colorType == 6 {
				row[x] = logoPixel{cur[o], cur[o+1], cur[o+2], cur[o+3]}
			} else {
				row[x] = logoPixel{cur[o], cur[o+1], cur[o+2], 255}
			}
		}

		prev, cur = cur, prev
	}

	return img, nil
}

func paeth(a, b, c int) int {
	p := a + b - c
	pa, pb, pc := abs(p-a), abs(p-b), abs(p-c)
	if pa <= pb && pa <= pc {
		return a
	}
	if pb <= pc {
		return b
	}
	return c
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
