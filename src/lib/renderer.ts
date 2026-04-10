/**
 * ASCII art renderer
 * Converts images to ASCII character representations
 */

import { Jimp } from 'jimp';

const CHARSETS: Record<string, string> = {
  simple: ' .:-=+*#%@',
  detailed: " .'-\":;Il!i><~+_?][}{1)(|/\\tfjrxncpoahkbdqwmXYUJCLOQZmwW&8%B@#",
  blocks: '█▓▒░ ',
};

export interface AsciiOptions {
  width: number;
  height: number;
  charset: string;
  invert: boolean;
  color: boolean;
}

// Jimp v1 has complex generic types — using any for pragmatic interop
/* eslint-disable @typescript-eslint/no-explicit-any */
export function resizeImage(image: any, targetWidth: number, targetHeight: number): any {
  const adjustedHeight = Math.floor(targetHeight * 0.5);
  return image.resize({ w: targetWidth, h: adjustedHeight });
}

export function imageToAscii(image: any, options: AsciiOptions): string[] {
  const { width, height } = image.bitmap;
  const charset = CHARSETS[options.charset] || CHARSETS.detailed;
  const invert = options.invert;
  const lines: string[] = [];

  for (let y = 0; y < height; y++) {
    let line = '';
    for (let x = 0; x < width; x++) {
      const idx = (y * width + x) * 4;
      const r = image.bitmap.data[idx];
      const g = image.bitmap.data[idx + 1];
      const b = image.bitmap.data[idx + 2];
      const a = image.bitmap.data[idx + 3];

      if (a < 128) {
        line += ' ';
        continue;
      }

      const brightness = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
      const charIndex = Math.floor(brightness * (charset.length - 1));
      const char = invert
        ? charset[charset.length - 1 - charIndex]
        : charset[charIndex];

      line += char;
    }
    lines.push(line);
  }

  return lines;
}

export function imageToAnsi(image: any, targetWidth: number, targetHeight: number): string[] {
  const adjustedHeight = Math.floor(targetHeight * 0.5);
  const resized = resizeImage(image, targetWidth, adjustedHeight);
  const { width, height } = resized.bitmap;
  const lines: string[] = [];

  for (let y = 0; y < height; y++) {
    let line = '';
    for (let x = 0; x < width; x++) {
      const idx = (y * width + x) * 4;
      const r = resized.bitmap.data[idx];
      const g = resized.bitmap.data[idx + 1];
      const b = resized.bitmap.data[idx + 2];
      const a = resized.bitmap.data[idx + 3];

      if (a < 128) {
        line += ' ';
        continue;
      }

      line += `\x1b[38;2;${r};${g};${b}m▀`;
    }
    lines.push(line);
  }

  return lines.map(line => line + '\x1b[0m');
}

export function imageToSixel(image: any, targetWidth: number, targetHeight: number): string {
  const resized = image.resize({ w: targetWidth, h: targetHeight });
  const { width, height } = resized.bitmap;

  let sixel = '\x1b[?25l';
  sixel += `\x1bPq"1;1;${width};${height};4`;

  const DENSITY = 2;
  const data: number[][] = [];

  for (let y = 0; y < height; y++) {
    const row: number[] = [];
    for (let x = 0; x < width; x++) {
      const idx = (y * width + x) * 4;
      const r = resized.bitmap.data[idx];
      const g = resized.bitmap.data[idx + 1];
      const b = resized.bitmap.data[idx + 2];
      row.push((r << 16) | (g << 8) | b);
    }
    data.push(row);
  }

  for (let y = 0; y < height; y += DENSITY) {
    for (let dy = 0; dy < DENSITY && y + dy < height; dy++) {
      let runChar = -1;
      let runCount = 0;

      for (let x = 0; x < width; x++) {
        const color = data[y + dy][x];

        if (runChar === color) {
          runCount++;
        } else {
          if (runChar >= 0) {
            sixel += encodeSixelRun(runChar, runCount);
          }
          runChar = color;
          runCount = 1;
        }
      }
      if (runChar >= 0) {
        sixel += encodeSixelRun(runChar, runCount);
      }
      sixel += '$';
    }
    sixel += '-';
  }

  sixel += '\x1b\\';
  sixel += '\x1b[?25h';

  return sixel;

  function encodeSixelRun(color: number, count: number): string {
    let out = '';
    const r = (color >> 16) & 0xFF;
    const g = (color >> 8) & 0xFF;
    const b = color & 0xFF;

    out += '#2;' + [r, g, b].join(';') + 'l';

    if (count > 1) {
      out += '!' + count;
    }

    return out;
  }
}
