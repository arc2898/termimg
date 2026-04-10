#!/usr/bin/env node
/**
 * termimg CLI Entry Point
 * Display images in any terminal
 */

import { Command } from 'commander';
import { display } from './commands/display.js';
import { version } from './lib/version.js';

const program = new Command();

program
  .name('termimg')
  .description('Display images in any terminal — ASCII art, ANSI color, or sixel graphics')
  .version(version);

program
  .argument('<image>', 'Path to the image file (PNG, JPEG, GIF, BMP, TIFF, WebP)')
  .option('-m, --mode <mode>', 'Rendering mode: auto, ascii, ansi, sixel', 'auto')
  .option('-w, --width <cols>', 'Output width in characters', '')
  .option('-H, --height <rows>', 'Output height in rows', '')
  .option('-c, --charset <charset>', 'ASCII charset: simple, detailed, blocks', 'detailed')
  .option('--color <mode>', 'Color mode: truecolor, 256, mono', 'truecolor')
  .option('--invert', 'Invert light/dark for ASCII mode')
  .option('-b, --batch', 'Batch mode for multiple files')
  .option('-o, --output <file>', 'Write to file instead of stdout', '')
  .option('-i, --info', 'Show image info without displaying')
  .action(display);

program.parseAsync(process.argv).catch(err => {
  console.error(`\n  ❌ Error: ${err.message}`);
  process.exit(1);
});
