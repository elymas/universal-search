import { test } from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const webRoot = resolve(__dirname, "..");

test("web/app/layout.tsx exists", () => {
  const layoutPath = resolve(webRoot, "app", "layout.tsx");
  const srcLayoutPath = resolve(webRoot, "src", "app", "layout.tsx");
  const exists = existsSync(layoutPath) || existsSync(srcLayoutPath);
  assert.ok(exists, `Expected layout.tsx at ${layoutPath} or ${srcLayoutPath}`);
});

test("web/app/page.tsx exists", () => {
  const pagePath = resolve(webRoot, "app", "page.tsx");
  const srcPagePath = resolve(webRoot, "src", "app", "page.tsx");
  const exists = existsSync(pagePath) || existsSync(srcPagePath);
  assert.ok(exists, `Expected page.tsx at ${pagePath} or ${srcPagePath}`);
});

test("package.json lists Next.js 16.x", () => {
  const pkgPath = resolve(webRoot, "package.json");
  assert.ok(existsSync(pkgPath), "package.json must exist");
  const pkg = JSON.parse(readFileSync(pkgPath, "utf-8"));
  const deps = { ...pkg.dependencies, ...pkg.devDependencies };
  const nextVersion: string = deps["next"] ?? "";
  assert.match(
    nextVersion,
    /^[\^~]?16\./,
    `Expected Next.js 16.x, got: ${nextVersion}`,
  );
});

test("shadcn/ui marker exists (dependency or components.json)", () => {
  const pkgPath = resolve(webRoot, "package.json");
  const componentsJsonPath = resolve(webRoot, "components.json");
  const hasPkg = existsSync(pkgPath);
  assert.ok(hasPkg, "package.json must exist");
  const pkg = JSON.parse(readFileSync(pkgPath, "utf-8"));
  const deps = { ...pkg.dependencies, ...pkg.devDependencies };
  const hasShadcnDep =
    Object.keys(deps).some((k) => k.startsWith("@shadcn/")) ||
    Object.keys(deps).includes("shadcn");
  const hasComponentsJson = existsSync(componentsJsonPath);
  assert.ok(
    hasShadcnDep || hasComponentsJson,
    "Expected shadcn/ui dependency or components.json",
  );
});
