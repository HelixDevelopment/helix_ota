// §11.4.170 oracle (i): golden image-diff. Pixel-diffs two PNGs with pixelmatch.
import { readFile, writeFile } from "node:fs/promises";
import { PNG } from "pngjs";
import pixelmatch from "pixelmatch";

export async function diffPng(aPath, bPath, diffOut) {
  const a = PNG.sync.read(await readFile(aPath));
  const b = PNG.sync.read(await readFile(bPath));
  if (a.width !== b.width || a.height !== b.height) {
    throw new Error(`dimension mismatch ${a.width}x${a.height} vs ${b.width}x${b.height}`);
  }
  const { width, height } = a;
  const diff = new PNG({ width, height });
  const mismatch = pixelmatch(a.data, b.data, diff.data, width, height, { threshold: 0.1 });
  if (diffOut) await writeFile(diffOut, PNG.sync.write(diff));
  const total = width * height;
  return { mismatch, total, ratio: mismatch / total };
}
