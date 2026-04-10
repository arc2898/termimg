import { Jimp } from 'jimp';
import { getTerminalSize } from '../lib/image.js';
import { imageToAscii, imageToAnsi, imageToSixel, resizeImage } from '../lib/renderer.js';
import { writeFileSync } from 'fs';
export async function display(imagePath, options) {
    try {
        const image = await Jimp.read(imagePath);
        const termSize = getTerminalSize();
        let targetWidth = options.width ? parseInt(options.width, 10) : termSize.width;
        let targetHeight = options.height ? parseInt(options.height, 10) : termSize.height;
        if (options.info) {
            console.log(`\n  📷 ${imagePath}`);
            console.log(`     ${image.width} × ${image.height}px`);
            console.log(`     Format: ${image.mime || 'unknown'}\n`);
            return;
        }
        const mode = options.mode || 'auto';
        let output;
        if (mode === 'ascii' || mode === 'auto') {
            const asciiOpts = {
                width: targetWidth,
                height: targetHeight,
                charset: options.charset || 'detailed',
                invert: options.invert || false,
                color: options.color === 'truecolor',
            };
            const resized = resizeImage(image, targetWidth, Math.floor(targetHeight * 0.5));
            const lines = imageToAscii(resized, asciiOpts);
            output = lines.join('\n');
        }
        else if (mode === 'ansi') {
            const lines = imageToAnsi(image, targetWidth, targetHeight);
            output = lines.join('\n');
        }
        else if (mode === 'sixel') {
            output = imageToSixel(image, targetWidth, targetHeight);
        }
        else {
            throw new Error(`Unknown mode: ${mode}. Use: auto, ascii, ansi, sixel`);
        }
        if (options.output) {
            writeFileSync(options.output, output, 'utf-8');
            console.log(`\n  ✅ Written to ${options.output}\n`);
        }
        else {
            console.log('\n' + output + '\n');
        }
    }
    catch (error) {
        throw new Error(`Failed to display image: ${error.message}`);
    }
}
