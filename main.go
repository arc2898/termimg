package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type mediaKind int

type renderMode int

const (
	kindImage mediaKind = iota
	kindVideo
)

const (
	modeColor renderMode = iota
	modeColor256
	modeASCII
)

type media struct {
	path     string
	kind     mediaKind
	duration float64
	fps      float64
}

type viewer struct {
	items       []media
	index       int
	frame       image.Image
	position    float64
	playing     bool
	loop        bool
	speed       float64
	lastFrameAt time.Time
	termWidth   int
	termHeight  int
	mode        renderMode
}

type probeResult struct {
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
	Streams []struct {
		CodecType string `json:"codec_type"`
		AvgRate   string `json:"avg_frame_rate"`
	} `json:"streams"`
}

var (
	imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".bmp": true, ".tif": true, ".tiff": true}
	videoExts = map[string]bool{".mp4": true, ".mkv": true, ".webm": true, ".mov": true, ".avi": true, ".m4v": true, ".flv": true, ".wmv": true, ".mpeg": true, ".mpg": true, ".ts": true, ".3gp": true, ".ogv": true}
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "tview-go — terminal image and video viewer\n\nUsage:\n  tview-go [options] FILE...\n  tview-go [options] DIRECTORY\n\nOptions:\n")
		flag.PrintDefaults()
		fmt.Fprintln(os.Stderr, "\nControls:\n  q / Esc  quit     h / l or n / p  previous/next\n  Space    play/pause video        0  restart video\n  + / -    change playback speed    r  refresh terminal size")
	}
	loop := flag.Bool("loop", false, "loop videos and wrap around at the end")
	fps := flag.Float64("fps", 0, "override video playback rate (frames per second)")
	renderer := flag.String("mode", "auto", "renderer: auto, color256, truecolor, or ascii")
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}

	items, err := discover(flag.Args())
	if err != nil {
		fatal(err)
	}
	if len(items) == 0 {
		fatal(errors.New("no supported media files found"))
	}
	if err := checkDependencies(); err != nil {
		fatal(err)
	}

	v := &viewer{items: items, loop: *loop, speed: 1}
	v.mode, err = chooseRenderMode(*renderer)
	if err != nil {
		fatal(err)
	}
	if *fps > 0 {
		for i := range v.items {
			if v.items[i].kind == kindVideo {
				v.items[i].fps = *fps
			}
		}
	}
	if err := v.refreshSize(); err != nil {
		fatal(err)
	}
	if err := v.loadCurrent(); err != nil {
		fatal(err)
	}

	restore, err := rawTerminal()
	if err != nil {
		fatal(err)
	}
	defer restore()
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h\x1b[0m\x1b[2J\x1b[H")

	v.render()
	v.run()
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "tview-go: %v\n", err)
	os.Exit(1)
}

func chooseRenderMode(value string) (renderMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ascii", "plain":
		return modeASCII, nil
	case "color", "truecolor", "ansi":
		return modeColor, nil
	case "color256", "256", "ansi256":
		return modeColor256, nil
	case "auto", "":
		// Prefer truecolor when explicitly advertised, then standard 256-color
		// output, and finally plain ASCII for basic terminals and captured logs.
		colorterm := strings.ToLower(os.Getenv("COLORTERM"))
		term := strings.ToLower(os.Getenv("TERM"))
		if strings.Contains(colorterm, "truecolor") || strings.Contains(colorterm, "24bit") || strings.Contains(term, "direct") {
			return modeColor, nil
		}
		if strings.Contains(term, "256color") {
			return modeColor256, nil
		}
		return modeASCII, nil
	default:
		return modeASCII, fmt.Errorf("unknown renderer %q (choose auto, color256, color, or ascii)", value)
	}
}

func checkDependencies() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return errors.New("ffmpeg is required for video playback; install it with your Linux distribution's package manager")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return errors.New("ffprobe is required for video metadata; install it with your Linux distribution's package manager")
	}
	return nil
}

func discover(args []string) ([]media, error) {
	var paths []string
	for _, arg := range args {
		info, err := os.Stat(arg)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arg, err)
		}
		if info.IsDir() {
			entries, err := os.ReadDir(arg)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", arg, err)
			}
			for _, entry := range entries {
				if !entry.IsDir() {
					paths = append(paths, filepath.Join(arg, entry.Name()))
				}
			}
		} else {
			paths = append(paths, arg)
		}
	}
	sort.Strings(paths)
	items := make([]media, 0, len(paths))
	for _, path := range paths {
		ext := strings.ToLower(filepath.Ext(path))
		switch {
		case imageExts[ext]:
			items = append(items, media{path: path, kind: kindImage})
		case videoExts[ext]:
			m, err := probe(path)
			if err != nil {
				return nil, err
			}
			items = append(items, m)
		}
	}
	return items, nil
}

func probe(path string) (media, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration:stream=codec_type,avg_frame_rate", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		return media{}, fmt.Errorf("probe %s: %w", path, err)
	}
	var result probeResult
	if err := json.Unmarshal(out, &result); err != nil {
		return media{}, fmt.Errorf("parse metadata for %s: %w", path, err)
	}
	duration, _ := strconv.ParseFloat(result.Format.Duration, 64)
	fps := 25.0
	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			fps = parseRate(stream.AvgRate)
			break
		}
	}
	if fps <= 0 || math.IsNaN(fps) || math.IsInf(fps, 0) {
		fps = 25
	}
	return media{path: path, kind: kindVideo, duration: duration, fps: fps}, nil
}

func parseRate(rate string) float64 {
	parts := strings.Split(rate, "/")
	if len(parts) == 2 {
		n, _ := strconv.ParseFloat(parts[0], 64)
		d, _ := strconv.ParseFloat(parts[1], 64)
		if d != 0 {
			return n / d
		}
	}
	v, _ := strconv.ParseFloat(rate, 64)
	return v
}

func (v *viewer) current() media { return v.items[v.index] }

func (v *viewer) loadCurrent() error {
	v.position = 0
	v.playing = false
	item := v.current()
	if item.kind == kindImage {
		file, err := os.Open(item.path)
		if err != nil {
			return fmt.Errorf("open %s: %w", item.path, err)
		}
		defer file.Close()
		decoded, _, err := image.Decode(file)
		if err != nil {
			return fmt.Errorf("decode %s: %w", item.path, err)
		}
		v.frame = decoded
		return nil
	}
	v.playing = true
	return v.loadVideoFrame()
}

func (v *viewer) loadVideoFrame() error {
	item := v.current()
	position := v.position
	if item.duration > 0 && position >= item.duration {
		if v.loop {
			position = 0
			v.position = 0
		} else {
			v.position = math.Max(0, item.duration-0.001)
			v.playing = false
			position = v.position
		}
	}
	args := []string{"-hide_banner", "-loglevel", "error", "-ss", fmt.Sprintf("%.3f", position), "-i", item.path, "-frames:v", "1", "-f", "image2pipe", "-vcodec", "png", "pipe:1"}
	out, err := exec.Command("ffmpeg", args...).Output()
	if err != nil {
		return fmt.Errorf("decode frame at %.2fs from %s: %w", position, item.path, err)
	}
	decoded, _, err := image.Decode(strings.NewReader(string(out)))
	if err != nil {
		return fmt.Errorf("decode video frame: %w", err)
	}
	v.frame = decoded
	return nil
}

func (v *viewer) run() {
	lastTick := time.Now()
	for {
		if key, ok := readKey(); ok {
			if v.handleKey(key) {
				return
			}
		}
		now := time.Now()
		if v.playing && now.Sub(lastTick) >= 25*time.Millisecond {
			lastTick = now
			item := v.current()
			frameStep := 1.0 / math.Max(item.fps, 1)
			v.position += frameStep * v.speed
			if err := v.loadVideoFrame(); err != nil {
				v.playing = false
				v.renderError(err)
			} else {
				v.render()
			}
		} else if now.Sub(lastTick) >= 250*time.Millisecond {
			lastTick = now
			v.render()
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (v *viewer) handleKey(key byte) bool {
	switch key {
	case 'q', 'Q', 27:
		return true
	case 'h', 'H', 'p', 'P':
		v.previous()
	case 'l', 'L', 'n', 'N', '\r', '\n':
		v.next()
	case ' ':
		if v.current().kind == kindVideo {
			v.playing = !v.playing
		}
	case '0':
		if v.current().kind == kindVideo {
			v.position = 0
			v.playing = true
			_ = v.loadVideoFrame()
		}
	case '+', '=':
		v.speed = math.Min(4, v.speed+0.25)
	case '-', '_':
		v.speed = math.Max(0.25, v.speed-0.25)
	case 'r', 'R':
		_ = v.refreshSize()
	}
	v.render()
	return false
}

func (v *viewer) previous() {
	v.index--
	if v.index < 0 {
		v.index = len(v.items) - 1
	}
	if err := v.loadCurrent(); err != nil {
		v.renderError(err)
	}
}

func (v *viewer) next() {
	v.index++
	if v.index >= len(v.items) {
		v.index = 0
	}
	if err := v.loadCurrent(); err != nil {
		v.renderError(err)
	}
}

func (v *viewer) refreshSize() error {
	out, err := exec.Command("stty", "size").Output()
	if err != nil {
		v.termWidth, v.termHeight = 80, 24
		return nil
	}
	parts := strings.Fields(string(out))
	if len(parts) != 2 {
		v.termWidth, v.termHeight = 80, 24
		return nil
	}
	h, errH := strconv.Atoi(parts[0])
	w, errW := strconv.Atoi(parts[1])
	if errH != nil || errW != nil || w < 10 || h < 5 {
		v.termWidth, v.termHeight = 80, 24
		return nil
	}
	v.termWidth, v.termHeight = w, h
	return nil
}

func (v *viewer) render() {
	if v.frame == nil {
		return
	}
	fmt.Print("\x1b[H\x1b[2J")
	item := v.current()
	availableRows := v.termHeight - 2
	if availableRows < 1 {
		availableRows = 1
	}
	renderImage(os.Stdout, v.frame, v.termWidth, availableRows, v.mode)
	status := fmt.Sprintf(" %d/%d  %s", v.index+1, len(v.items), filepath.Base(item.path))
	if item.kind == kindVideo {
		state := "paused"
		if v.playing {
			state = "playing"
		}
		status += fmt.Sprintf("  %s  %s / %s  %.2fx", state, formatTime(v.position), formatTime(item.duration), v.speed)
	} else {
		status += "  image"
	}
	fmt.Printf("\x1b[0m\x1b[7m%-*s\x1b[0m", v.termWidth, truncate(status, v.termWidth))
	fmt.Printf("\n\x1b[2m q quit · h/l previous/next · space play/pause · +/- speed · 0 restart\x1b[0m")
}

func (v *viewer) renderError(err error) {
	fmt.Print("\x1b[H\x1b[2J")
	fmt.Printf("\n\x1b[31m%s\x1b[0m\n", err)
}

func formatTime(seconds float64) string {
	if seconds < 0 || math.IsNaN(seconds) {
		return "--:--"
	}
	minutes := int(seconds) / 60
	secs := int(seconds) % 60
	return fmt.Sprintf("%02d:%02d", minutes, secs)
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) > width {
		if width <= 1 {
			return string(runes[:width])
		}
		return string(runes[:width-1]) + "…"
	}
	return s
}

func renderImage(w io.Writer, img image.Image, cols, rows int, mode renderMode) {
	if mode == modeASCII {
		renderASCII(w, img, cols, rows)
		return
	}
	if mode == modeColor256 {
		renderColor256(w, img, cols, rows)
		return
	}
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	if imgW <= 0 || imgH <= 0 {
		return
	}
	maxPixelH := rows * 2
	scale := math.Min(float64(cols)/float64(imgW), float64(maxPixelH)/float64(imgH))
	if scale <= 0 {
		return
	}
	pixelW := max(1, int(math.Round(float64(imgW)*scale)))
	pixelH := max(1, int(math.Round(float64(imgH)*scale)))
	pixelW = min(pixelW, cols)
	pixelH = min(pixelH, maxPixelH)
	startX := max(0, (cols-pixelW)/2)
	startY := max(0, (rows*2-pixelH)/2)
	for cellY := 0; cellY < rows; cellY++ {
		upperY := cellY*2 - startY
		lowerY := upperY + 1
		for x := 0; x < cols; x++ {
			if x < startX || x >= startX+pixelW || upperY < 0 || upperY >= pixelH {
				fmt.Fprint(w, " ")
				continue
			}
			imageX := int(float64(x-startX) / float64(pixelW) * float64(imgW))
			if imageX >= imgW {
				imageX = imgW - 1
			}
			top := pixelColor(img, imageX, int(float64(upperY)/float64(pixelH)*float64(imgH)))
			if lowerY >= 0 && lowerY < pixelH {
				bottom := pixelColor(img, imageX, int(float64(lowerY)/float64(pixelH)*float64(imgH)))
				fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm▀", top[0], top[1], top[2], bottom[0], bottom[1], bottom[2])
			} else {
				fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dm▀", top[0], top[1], top[2])
			}
		}
		fmt.Fprint(w, "\x1b[0m\n")
	}
}

func renderColor256(w io.Writer, img image.Image, cols, rows int) {
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	if imgW <= 0 || imgH <= 0 {
		return
	}
	maxPixelH := rows * 2
	scale := math.Min(float64(cols)/float64(imgW), float64(maxPixelH)/float64(imgH))
	if scale <= 0 {
		return
	}
	pixelW := max(1, int(math.Round(float64(imgW)*scale)))
	pixelH := max(1, int(math.Round(float64(imgH)*scale)))
	pixelW = min(pixelW, cols)
	pixelH = min(pixelH, maxPixelH)
	startX := max(0, (cols-pixelW)/2)
	startY := max(0, (rows*2-pixelH)/2)
	for cellY := 0; cellY < rows; cellY++ {
		upperY := cellY*2 - startY
		lowerY := upperY + 1
		for x := 0; x < cols; x++ {
			if x < startX || x >= startX+pixelW || upperY < 0 || upperY >= pixelH {
				fmt.Fprint(w, " ")
				continue
			}
			imageX := int(float64(x-startX) / float64(pixelW) * float64(imgW))
			if imageX >= imgW {
				imageX = imgW - 1
			}
			top := pixelColor(img, imageX, int(float64(upperY)/float64(pixelH)*float64(imgH)))
			if lowerY >= 0 && lowerY < pixelH {
				bottom := pixelColor(img, imageX, int(float64(lowerY)/float64(pixelH)*float64(imgH)))
				fmt.Fprintf(w, "\x1b[38;5;%dm\x1b[48;5;%dm▀", rgbToANSI256(top), rgbToANSI256(bottom))
			} else {
				fmt.Fprintf(w, "\x1b[38;5;%dm▀", rgbToANSI256(top))
			}
		}
		fmt.Fprint(w, "\x1b[0m\n")
	}
}

func rgbToANSI256(rgb [3]uint8) int {
	// Map to the 6×6×6 color cube, which preserves more image color than the
	// 24-entry grayscale ramp for typical photographs.
	red := int(math.Round(float64(rgb[0]) / 255 * 5))
	green := int(math.Round(float64(rgb[1]) / 255 * 5))
	blue := int(math.Round(float64(rgb[2]) / 255 * 5))
	cube := 16 + 36*red + 6*green + blue
	gray := int(math.Round((float64(rgb[0]) + float64(rgb[1]) + float64(rgb[2])) / 3))
	grayIndex := int(math.Round(float64(gray-8) / 10))
	grayIndex = max(0, min(23, grayIndex))
	grayValue := 8 + grayIndex*10
	cubeDistance := colorDistance(rgb, ansiRGB(cube))
	grayDistance := colorDistance(rgb, [3]uint8{uint8(grayValue), uint8(grayValue), uint8(grayValue)})
	if grayDistance < cubeDistance {
		return 232 + grayIndex
	}
	return cube
}

func ansiRGB(index int) [3]uint8 {
	if index < 16 {
		palette := [16][3]uint8{{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0}, {0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192}, {128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255}}
		return palette[max(0, index)]
	}
	if index >= 232 {
		v := uint8(8 + (index-232)*10)
		return [3]uint8{v, v, v}
	}
	index -= 16
	return [3]uint8{uint8(colorCubeValue(index / 36)), uint8(colorCubeValue((index / 6) % 6)), uint8(colorCubeValue(index % 6))}
}

func colorCubeValue(value int) int {
	if value == 0 {
		return 0
	}
	return 55 + value*40
}

func colorDistance(a, b [3]uint8) float64 {
	dr := float64(a[0]) - float64(b[0])
	dg := float64(a[1]) - float64(b[1])
	db := float64(a[2]) - float64(b[2])
	return dr*dr + dg*dg + db*db
}

func renderASCII(w io.Writer, img image.Image, cols, rows int) {
	bounds := img.Bounds()
	imgW, imgH := bounds.Dx(), bounds.Dy()
	if imgW <= 0 || imgH <= 0 || cols <= 0 || rows <= 0 {
		return
	}
	scale := math.Min(float64(cols)/float64(imgW), float64(rows*2)/float64(imgH))
	pixelW := max(1, min(cols, int(math.Round(float64(imgW)*scale))))
	pixelH := max(1, min(rows*2, int(math.Round(float64(imgH)*scale))))
	startX := max(0, (cols-pixelW)/2)
	startY := max(0, (rows*2-pixelH)/2)
	const ramp = " .:-=+*#%@"
	for cellY := 0; cellY < rows; cellY++ {
		pixelY := cellY*2 - startY
		for x := 0; x < cols; x++ {
			if x < startX || x >= startX+pixelW || pixelY < 0 || pixelY >= pixelH {
				fmt.Fprint(w, " ")
				continue
			}
			imageX := int(float64(x-startX) / float64(pixelW) * float64(imgW))
			imageY := int(float64(pixelY) / float64(pixelH) * float64(imgH))
			if imageX >= imgW {
				imageX = imgW - 1
			}
			if imageY >= imgH {
				imageY = imgH - 1
			}
			c := pixelColor(img, imageX, imageY)
			luminance := (0.2126*float64(c[0]) + 0.7152*float64(c[1]) + 0.0722*float64(c[2])) / 255
			idx := int(luminance * float64(len(ramp)-1))
			fmt.Fprintf(w, "%c", ramp[idx])
		}
		fmt.Fprint(w, "\n")
	}
}

func pixelColor(img image.Image, x, y int) [3]uint8 {
	c := img.At(img.Bounds().Min.X+x, img.Bounds().Min.Y+y)
	r, g, b, a := c.RGBA()
	if a != 0xffff {
		// Composite transparent media over the terminal's typical dark background.
		r = (r*a + 0x101*(0xffff-a)) / 0xffff
		g = (g*a + 0x101*(0xffff-a)) / 0xffff
		b = (b*a + 0x101*(0xffff-a)) / 0xffff
	}
	return [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func rawTerminal() (func(), error) {
	stateCmd := exec.Command("stty", "-g")
	stateCmd.Stdin = os.Stdin
	state, err := stateCmd.Output()
	if err != nil {
		return nil, errors.New("stdin is not a terminal")
	}
	configure := exec.Command("stty", "-icanon", "-echo", "min", "0", "time", "0")
	configure.Stdin = os.Stdin
	if err := configure.Run(); err != nil {
		return nil, fmt.Errorf("configure terminal: %w", err)
	}
	restore := func() {
		cmd := exec.Command("stty", strings.TrimSpace(string(state)))
		cmd.Stdin = os.Stdin
		_ = cmd.Run()
	}
	return restore, nil
}

func readKey() (byte, bool) {
	var fds syscall.FdSet
	fds.Bits[0] = 1
	tv := syscall.Timeval{}
	ready, err := syscall.Select(1, &fds, nil, nil, &tv)
	if err != nil || ready == 0 {
		return 0, false
	}
	var b [1]byte
	n, err := os.Stdin.Read(b[:])
	return b[0], n == 1 && err == nil
}

// Keep bufio linked in builds where a platform's image decoder needs it through
// an indirect interface; it also documents that all input is intentionally byte-oriented.
var _ = bufio.ErrInvalidUnreadByte
