# termimg

> Display images in any terminal — ASCII art, ANSI color, or sixel graphics

![Demo](https://raw.githubusercontent.com/arc2898/termimg/main/demo.gif)

## Features

- **ASCII Art** — Classic character-based rendering with customizable charset
- **Truecolor ANSI** — Photo-realistic output using 24-bit ANSI colors (most modern terminals)
- **Sixel Graphics** — Native graphics protocol for compatible terminals (iTerm2, mlterm)
- **Batch Mode** — Display multiple images in sequence
- **Smart Resize** — Auto-fit to terminal dimensions or specify exact size
- ** Formats** — PNG, JPEG, GIF, BMP, TIFF, WebP

## Installation

```bash
npm install -g termimg
```

## Usage

```bash
# Display an image (auto-detects best method)
termimg photo.jpg

# Force ASCII art mode
termimg photo.jpg --mode ascii

# Force ANSI truecolor mode
termimg photo.jpg --mode ansi

# Force sixel mode
termimg photo.jpg --mode sixel

# Resize to specific dimensions
termimg photo.jpg --width 120 --height 40

# Batch display from directory
termimg ./screenshots/*.png --batch

# Output to file instead of stdout
termimg photo.jpg --output ascii.txt

# Show image info without displaying
termimg photo.jpg --info
```

## Options

| Option | Short | Description | Default |
|--------|-------|-------------|---------|
| `--mode` | `-m` | Rendering mode: `auto`, `ascii`, `ansi`, `sixel` | `auto` |
| `--width` | `-w` | Output width in characters/columns | Terminal width |
| `--height` | `-H` | Output height in rows | Auto |
| `--charset` | `-c` | ASCII charset: `simple`, `detailed`, `blocks` | `detailed` |
| `--color` | | Color mode: `truecolor`, `256`, `mono` | `truecolor` |
| `--invert` | | Invert light/dark for ASCII mode | `false` |
| `--batch` | `-b` | Batch mode for multiple files | `false` |
| `--output` | `-o` | Write to file instead of stdout | stdout |
| `--info` | `-i` | Show image metadata only | `false` |

## Supported Terminals

| Terminal | ANSI Truecolor | Sixel |
|----------|---------------|-------|
| iTerm2 | ✅ | ✅ |
| Windows Terminal | ✅ | ❌ |
| macOS Terminal | ✅ | ❌ |
| Konsole | ✅ | ✅ |
| mlterm | ✅ | ✅ |
| Foot | ✅ | ✅ |
| Kitty | ✅ | ❌ (uses its own protocol) |

## Demo

```bash
# See it in action
termimg --demo
```

## License

MIT
