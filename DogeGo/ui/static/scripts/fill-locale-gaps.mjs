#!/usr/bin/env node
/**
 * Deep-merge translated gap keys into locale JSON files.
 * Run from repo root: node ui/static/scripts/fill-locale-gaps.mjs
 */
import fs from "fs";
import path from "path";
import { fileURLToPath } from "url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const LOCALES_DIR = path.join(__dirname, "..", "locales");
const GAPS_DIR = path.join(LOCALES_DIR, "gaps");

function deepMerge(base, overlay) {
  if (!overlay) return base;
  const out = JSON.parse(JSON.stringify(base));
  function merge(into, from) {
    for (const [k, v] of Object.entries(from)) {
      if (v && typeof v === "object" && !Array.isArray(v) && into[k] && typeof into[k] === "object" && !Array.isArray(into[k])) {
        merge(into[k], v);
      } else {
        into[k] = v;
      }
    }
  }
  merge(out, overlay);
  return out;
}

const codes = ["fr", "de", "pt-PT", "zh", "ja"];
for (const code of codes) {
  const gapPath = path.join(GAPS_DIR, `${code}.json`);
  const localePath = path.join(LOCALES_DIR, `${code}.json`);
  if (!fs.existsSync(gapPath)) {
    console.warn(`skip ${code}: no gaps file`);
    continue;
  }
  const gaps = JSON.parse(fs.readFileSync(gapPath, "utf8"));
  const locale = JSON.parse(fs.readFileSync(localePath, "utf8"));
  const merged = deepMerge(locale, gaps);
  fs.writeFileSync(localePath, JSON.stringify(merged, null, 2) + "\n");
  console.log(`merged ${code}.json`);
}
