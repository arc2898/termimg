# tview-go

`tview-go` is a small terminal-based image and video viewer for Linux. It renders images using truecolor ANSI escape sequences and Unicode half-block characters, so it works in ordinary 24-bit-capable terminals without opening a graphical window. Videos are decoded one frame at a time through `ffmpeg`, which keeps the Go program compact while supporting the formats available in the local ffmpeg build.

## Requirements

| Requirement | Purpose |
|---|---|
| Linux terminal | Raw keyboard input and ANSI rendering |
| Go 1.22 or newer | Building from source |
| `ffmpeg` | Extracting video frames |
| `ffprobe` | Reading duration and frame-rate metadata |
| Truecolor terminal | Best image quality; 256-color terminals may show degraded colors |

On Debian or Ubuntu, install the runtime dependencies with:

```sh
sudo apt install ffmpeg
```

## Build

```sh
go build -trimpath -ldflags='-s -w' -o tview-go .
```

You can optionally install it into your local executable path:

```sh
install -Dm755 tview-go "$HOME/.local/bin/tview-go"
```

## Usage

Open one or more files:

```sh
./tview-go photo.jpg
./tview-go clip.mp4
./tview-go photo.jpg clip.mp4 another.png
```

Open all supported media files in a directory, in lexicographic order:

```sh
./tview-go ./media
```

Useful options:

```sh
./tview-go --loop clip.mp4
./tview-go --fps 30 clip.mp4
./tview-go --mode ascii photo.jpg
./tview-go --mode color256 photo.jpg
./tview-go --mode color photo.jpg
```

`--loop` repeats a video and wraps the previous/next navigation at either end. `--fps` overrides the probed frame rate for playback timing, which can be useful for unusual or variable-rate files. The renderer defaults to `auto`: terminals that advertise truecolor use compact full-color half-blocks, standard `xterm-256color` terminals use 256-color half-blocks, and basic terminals use visible grayscale ASCII. Use `--mode color256` to force 256-color ANSI output, `--mode color` to force ANSI truecolor output, or `--mode ascii` when escape sequences are being printed literally.

## Controls

| Key | Action |
|---|---|
| `q`, `Esc` | Quit |
| `h`, `p` | Previous media |
| `l`, `n`, `Enter` | Next media |
| `Space` | Play or pause the current video |
| `0` | Restart the current video |
| `+`, `=` | Increase playback speed, up to 4× |
| `-`, `_` | Decrease playback speed, down to 0.25× |
| `r` | Re-read the terminal dimensions |

Images are displayed once and remain paused. The image is fitted inside the current terminal window while preserving its aspect ratio. Videos are sampled with ffmpeg and refreshed in the terminal; seeking is implemented by decoding the requested timestamp for each displayed frame.

## Supported formats

The viewer recognizes common extensions for JPEG, PNG, GIF, BMP, TIFF, MP4, MKV, WebM, MOV, AVI, M4V, FLV, WMV, MPEG, TS, 3GP, and OGV. The actual video codec support is provided by the installed ffmpeg build.

## Notes

The viewer deliberately avoids external Go dependencies. This makes the binary easy to build and audit. The automatic renderer uses the richest color mode advertised by the terminal: truecolor, then 256-color, then ASCII. This prevents invisible output on basic terminals while still giving ordinary Linux terminals colored images. If your terminal has unusual dimensions or does not report its size through `stty`, the viewer falls back to an 80×24 canvas.

For best performance, use a terminal with a fast renderer and avoid very large videos when seeking through long GOP intervals. The current implementation prioritizes portability and simple deployment over hardware-accelerated playback.
