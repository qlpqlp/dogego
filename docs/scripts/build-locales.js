#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");

const dir = path.join(__dirname, "..", "locales");

for (const file of fs.readdirSync(dir).filter((f) => f.endsWith(".json"))) {
  const code = file.replace(/\.json$/, "");
  const data = JSON.parse(fs.readFileSync(path.join(dir, file), "utf8"));
  const js =
    "(function(g){g.DogeGoSiteLocaleBundles=g.DogeGoSiteLocaleBundles||{};" +
    "g.DogeGoSiteLocaleBundles[" +
    JSON.stringify(code) +
    "]=" +
    JSON.stringify(data, null, 2) +
    ";})(window);\n";
  fs.writeFileSync(path.join(dir, code + ".js"), js);
  console.log("wrote locales/" + code + ".js");
}
