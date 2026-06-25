#!/usr/bin/env node
// fix-locale-links.mjs — post-build locale-link repair for the static export.
//
// Nextra v4 i18n emits some sidebar/nav links without the locale prefix
// (e.g. `/operators/security` instead of `/en/operators/security`). At runtime
// Nextra relies on locale middleware to add the prefix, but `output: 'export'`
// cannot run middleware, so those links 404 on direct navigation and the lychee
// internal link-check flags them. This rewrites lang-less internal content links
// to the locale of the containing page's directory (out/<lang>/...), leaving
// already-prefixed links, assets (/_next, /images), and external URLs untouched.
//
// SPEC-DOC-001 REQ-DOC-012 (link integrity for static i18n export).
import { readdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";

const OUT = "out";
const LOCALES = ["en", "ko"];
// Top-level content sections (mirrors content/<lang>/* + generated CONTRIBUTING).
const SECTIONS = [
  "end-users",
  "getting-started",
  "introduction",
  "legal",
  "operators",
  "reference",
  "troubleshooting",
  "CONTRIBUTING",
];
const linkRe = new RegExp(`href="/(${SECTIONS.join("|")})(/[^"]*)?"`, "g");

async function* walkHtml(dir) {
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const p = join(dir, entry.name);
    if (entry.isDirectory()) yield* walkHtml(p);
    else if (entry.name.endsWith(".html")) yield p;
  }
}

let fileCount = 0;
let linkCount = 0;
for (const lang of LOCALES) {
  for await (const file of walkHtml(join(OUT, lang))) {
    const html = await readFile(file, "utf8");
    let n = 0;
    const fixed = html.replace(linkRe, (_m, section, rest = "") => {
      n += 1;
      return `href="/${lang}/${section}${rest}"`;
    });
    if (n > 0) {
      await writeFile(file, fixed);
      fileCount += 1;
      linkCount += n;
    }
  }
}
console.log(
  `fix-locale-links: rewrote ${linkCount} lang-less links across ${fileCount} files`,
);
