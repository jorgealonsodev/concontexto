// design-system spec, "No prefabricated component kit" — tasks.md 5.9 (RED)
// / 5.10 (GREEN). ADR-6: Flowbite, shadcn/ui, Bootstrap, DaisyUI, Material
// UI, Chakra and equivalents are forbidden, directly or transitively.
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const WEB_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

// Package-name fragments (npm scope/name), not full literal matches, so
// naming variants (`flowbite-svelte`, `@shadcn/ui`, `bootstrap-vue`, ...)
// are still caught.
const DENYLIST = [
  "flowbite",
  "shadcn",
  "bootstrap",
  "daisyui",
  "@mui/material",
  "@material-ui/core",
  "@chakra-ui/react",
  "@chakra-ui/core",
];

describe("component-kit dependency guard (ADR-6): no prefabricated kit, direct or transitive", () => {
  const pkg = JSON.parse(readFileSync(path.join(WEB_DIR, "package.json"), "utf-8"));
  const lockfile = readFileSync(path.join(WEB_DIR, "package-lock.json"), "utf-8");
  const declaredDeps = { ...pkg.dependencies, ...pkg.devDependencies };

  it.each(DENYLIST)("package.json does not directly depend on %s", (fragment) => {
    const hit = Object.keys(declaredDeps).find((name) => name.includes(fragment));
    expect(hit, `package.json declares a forbidden dependency: ${hit}`).toBeUndefined();
  });

  it.each(DENYLIST)("the lockfile carries no resolved package matching %s (direct or transitive)", (fragment) => {
    // npm lockfile v3 keys every resolved package as `"node_modules/<name>"`
    // (nested copies as `"node_modules/x/node_modules/<name>"`), so a
    // substring search over that JSON text also catches transitive pulls.
    const pattern = new RegExp(`"node_modules/[^"]*${fragment.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}[^"]*"\\s*:`);
    expect(pattern.test(lockfile), `lockfile carries a resolved package matching "${fragment}"`).toBe(false);
  });
});
