#!/usr/bin/env node
/**
 * Fill missing locale keys from en.json and drop keys absent in en.json.
 * Preserves existing translations; uses English for new keys only.
 */
const fs = require("fs");
const path = require("path");

const locDir = path.join(__dirname, "..", "locales");
const en = JSON.parse(fs.readFileSync(path.join(locDir, "en.json"), "utf8"));

function pruneToEnShape(enNode, locNode) {
  if (enNode === null || typeof enNode !== "object" || Array.isArray(enNode)) {
    return locNode !== undefined ? locNode : enNode;
  }
  const out = {};
  for (const key of Object.keys(enNode)) {
    const enChild = enNode[key];
    const locChild = locNode && locNode[key];
    if (typeof enChild === "object" && enChild !== null && !Array.isArray(enChild)) {
      out[key] = pruneToEnShape(enChild, locChild);
    } else {
      out[key] = locChild !== undefined ? locChild : enChild;
    }
  }
  return out;
}

for (const lang of ["de", "fr", "ja", "pt-PT", "zh"]) {
  const file = path.join(locDir, lang + ".json");
  const loc = JSON.parse(fs.readFileSync(file, "utf8"));
  const merged = pruneToEnShape(en, loc);
  fs.writeFileSync(file, JSON.stringify(merged, null, 2) + "\n", "utf8");
  console.log("synced", lang + ".json");
}
