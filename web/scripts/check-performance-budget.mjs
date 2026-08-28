import { readdir, stat } from "node:fs/promises";
import { join } from "node:path";
import { gzipSync } from "node:zlib";

const distDir = new URL("../dist/", import.meta.url);
const assetsDir = new URL("../dist/assets/", import.meta.url);
const maxInitialJavaScriptGzip = 150 * 1024;
const maxInitialCssGzip = 80 * 1024;
const maxFontAssetBytes = 64 * 1024;

const files = await readdir(assetsDir);
const entryScripts = files.filter((file) => /^index-[^/]+\.js$/.test(file));
const entryStyles = files.filter((file) => /^index-[^/]+\.css$/.test(file));
if (entryScripts.length !== 1 || entryStyles.length !== 1) {
  throw new Error(
    `expected one entry script and stylesheet, got ${entryScripts.length} scripts and ${entryStyles.length} styles`,
  );
}

const checks = [
  await checkGzip(join(distDir.pathname, "assets", entryScripts[0]), maxInitialJavaScriptGzip),
  await checkGzip(join(distDir.pathname, "assets", entryStyles[0]), maxInitialCssGzip),
];

for (const file of files.filter((name) => /\.woff2?$/.test(name))) {
  const bytes = (await stat(join(distDir.pathname, "assets", file))).size;
  if (bytes > maxFontAssetBytes) {
    throw new Error(`font asset ${file} is ${bytes} bytes; limit is ${maxFontAssetBytes}`);
  }
}

for (const result of checks) {
  console.log(`${result.label}: ${result.gzipBytes} bytes gzip (limit ${result.limit})`);
}
console.log("performance budget passed");

async function checkGzip(path, limit) {
  const bytes = await stat(path);
  const content = await import("node:fs/promises").then(({ readFile }) => readFile(path));
  const gzipBytes = gzipSync(content, { level: 9 }).byteLength;
  if (gzipBytes > limit) {
    throw new Error(`${path} is ${gzipBytes} bytes gzip; limit is ${limit}`);
  }
  return { label: path.split("/").at(-1), gzipBytes, limit, bytes: bytes.size };
}
