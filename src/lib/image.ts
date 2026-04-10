import { Jimp } from 'jimp';
import { statSync } from 'fs';

export async function loadImage(path: string): Promise<any> {
  return await Jimp.read(path);
}

export async function getImageInfo(path: string) {
  const image = await Jimp.read(path);
  const stats = statSync(path);

  return {
    width: image.width,
    height: image.height,
    format: image.mime || 'unknown',
    filePath: path,
    fileSize: stats.size,
  };
}

export function getTerminalSize(): { width: number; height: number } {
  const width = parseInt(process.env.COLUMNS || '80', 10);
  const height = parseInt(process.env.LINES || '24', 10);
  return { width, height };
}
