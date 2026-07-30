// publishing-export spec, "The artifact is validated on read by the
// build" + design.md D-1's read-side contract: "EXPORT_URL live / EXPORT_DIR
// fixture, digest check, Zod, exact version equality; failure exits build
// non-zero, previous deploy stays". Task 9a.1 (RED) / 9a.2 (GREEN).
//
// This is the loader's OWN behaviour under test -- sha256 verification and
// the dir/url mode switch -- distinct from `schema.test.ts`, which only
// exercises the Zod schemas directly against fixture bytes already read
// off disk. Both suites intentionally overlap on "an unsupported
// schema_version fails" since that scenario is explicitly required by both
// the write-side/read-side spec AND the loader's own "any failure exits
// non-zero" contract.
import { describe, expect, it, vi, afterEach } from "vitest";
import { mkdtemp, writeFile, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { loadExportArtifact, resolveLoadOptionsFromEnv } from "../../src/lib/export/loader";

const FIXTURES_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../fixtures/export");

/** Copies the real golden fixture into a fresh temp directory so a test can
 * corrupt ONE byte/field without mutating the checked-in fixture other
 * tests (and the real `astro build`) depend on. */
async function copyFixtureTo(dir: string): Promise<void> {
  const manifestBytes = await readFile(path.join(FIXTURES_DIR, "manifest.json"));
  await writeFile(path.join(dir, "manifest.json"), manifestBytes);
  const { mkdir } = await import("node:fs/promises");
  await mkdir(path.join(dir, "series"), { recursive: true });
  // Read the slug list off the manifest itself (slice 9b regenerated the
  // fixture with all six frozen slugs) so this helper never drifts from
  // whatever the checked-in fixture actually carries.
  const manifest = JSON.parse(manifestBytes.toString("utf-8")) as { series: string[] };
  for (const slug of manifest.series) {
    await writeFile(
      path.join(dir, "series", `${slug}.json`),
      await readFile(path.join(FIXTURES_DIR, "series", `${slug}.json`)),
    );
  }
}

const tempDirs: string[] = [];
afterEach(async () => {
  while (tempDirs.length > 0) {
    const dir = tempDirs.pop()!;
    await rm(dir, { recursive: true, force: true });
  }
});

async function freshFixtureCopy(): Promise<string> {
  const dir = await mkdtemp(path.join(tmpdir(), "concontexto-loader-test-"));
  tempDirs.push(dir);
  await copyFixtureTo(dir);
  return dir;
}

describe("loadExportArtifact — EXPORT_DIR (fixture) mode", () => {
  it("loads the real golden fixture: manifest + all six series, every sha256 verified", async () => {
    const { manifest, seriesBySlug } = await loadExportArtifact({ dir: FIXTURES_DIR });
    expect(manifest.schema_version).toBe(1);
    expect(seriesBySlug.size).toBe(6);
    expect(seriesBySlug.get("tasa-de-paro-epa")?.slug).toBe("tasa-de-paro-epa");
    expect(seriesBySlug.get("ocupados-epa")?.points.length).toBeGreaterThan(0);
    expect(seriesBySlug.get("poblacion-residente")?.points.length).toBeGreaterThan(0);
    // pib's export document is still slugged "pib-cvi" (the Go pipeline's
    // own series id) -- see content/indicators/pib.ts's artifactSlug doc
    // comment. The loader itself is slug-agnostic; the "pib" alias is
    // resolved one layer up, in [slug].astro.
    expect(seriesBySlug.get("pib-cvi")?.points.length).toBeGreaterThan(0);
  });

  it("rejects a series file whose bytes do not match the manifest's declared sha256 — refuses to build from a tampered artifact", async () => {
    const dir = await freshFixtureCopy();
    const corrupted = (await readFile(path.join(dir, "series", "tasa-de-paro-epa.json"), "utf-8")).replace(
      "9.93",
      "999.93",
    );
    await writeFile(path.join(dir, "series", "tasa-de-paro-epa.json"), corrupted);

    await expect(loadExportArtifact({ dir })).rejects.toThrow(/sha256 mismatch/);
  });

  it("rejects a schema_version the loader does not support — exact equality, never >=", async () => {
    const dir = await freshFixtureCopy();
    const manifestRaw = JSON.parse(await readFile(path.join(dir, "manifest.json"), "utf-8"));
    manifestRaw.schema_version = 2;
    await writeFile(path.join(dir, "manifest.json"), JSON.stringify(manifestRaw));

    await expect(loadExportArtifact({ dir })).rejects.toThrow(/schema_version/);
  });

  it("rejects a manifest that is not valid JSON, naming the failure", async () => {
    const dir = await freshFixtureCopy();
    await writeFile(path.join(dir, "manifest.json"), "{not json");

    await expect(loadExportArtifact({ dir })).rejects.toThrow(/not valid JSON/);
  });

  it("rejects a manifest listing a slug whose file the artifact's own document disagrees about", async () => {
    const dir = await freshFixtureCopy();
    const manifestRaw = JSON.parse(await readFile(path.join(dir, "manifest.json"), "utf-8"));
    // Force a slug/document mismatch while keeping the digest coherent, by
    // recomputing the digest for the swapped-in content — proves the
    // loader ALSO cross-checks slug identity, not only the byte digest.
    const otherDoc = await readFile(path.join(dir, "series", "ocupados-epa.json"), "utf-8");
    await writeFile(path.join(dir, "series", "tasa-de-paro-epa.json"), otherDoc);
    const { createHash } = await import("node:crypto");
    manifestRaw.digests["series/tasa-de-paro-epa.json"] = createHash("sha256")
      .update(otherDoc)
      .digest("hex");
    await writeFile(path.join(dir, "manifest.json"), JSON.stringify(manifestRaw));

    await expect(loadExportArtifact({ dir })).rejects.toThrow(/declares slug/);
  });

  // verify-report CRITICAL-4: the page state that drives PRD §6.1.3's
  // banners is now REQUIRED in the artifact. An artifact that lacks it must
  // fail the build — the alternative (treating the absence as "fresh") is
  // precisely the silent-suppression bug this remediation closes. Digests
  // are recomputed so the loader reaches the Zod stage and rejects on SHAPE,
  // not on a byte mismatch that would mask the real check.
  it("rejects a series document with no pageState — the field is required, never defaulted to fresh — mutation-checked", async () => {
    const dir = await freshFixtureCopy();
    const manifestRaw = JSON.parse(await readFile(path.join(dir, "manifest.json"), "utf-8"));
    const docRaw = JSON.parse(await readFile(path.join(dir, "series", "tasa-de-paro-epa.json"), "utf-8"));
    delete docRaw.pageState;
    const rewritten = JSON.stringify(docRaw);
    await writeFile(path.join(dir, "series", "tasa-de-paro-epa.json"), rewritten);
    const { createHash } = await import("node:crypto");
    manifestRaw.digests["series/tasa-de-paro-epa.json"] = createHash("sha256").update(rewritten).digest("hex");
    await writeFile(path.join(dir, "manifest.json"), JSON.stringify(manifestRaw));

    await expect(loadExportArtifact({ dir })).rejects.toThrow(/pageState/);
  });
});

describe("loadExportArtifact — EXPORT_URL (live) mode", () => {
  it("fetches manifest + series over HTTP and verifies the same sha256 digests", async () => {
    const manifestBytes = await readFile(path.join(FIXTURES_DIR, "manifest.json"));
    const manifest = JSON.parse(manifestBytes.toString("utf-8")) as { series: string[] };
    const seriesBytes = new Map<string, Buffer>();
    for (const slug of manifest.series) {
      seriesBytes.set(slug, await readFile(path.join(FIXTURES_DIR, "series", `${slug}.json`)));
    }

    const fetchMock = vi.fn(async (input: string | URL) => {
      const url = input.toString();
      const bytes = url.endsWith("manifest.json")
        ? manifestBytes
        : seriesBytes.get(url.replace(/^.*series\//, "").replace(/\.json$/, ""))!;
      // Copied into a plain `Uint8Array` rather than handed over as a Node
      // `Buffer`: `BodyInit` accepts `ArrayBufferView<ArrayBuffer>`, and a
      // `Buffer<ArrayBufferLike>` is not that (verify-report SUGGESTION-14 —
      // `astro check` ts2345, invisible until this file was first
      // type-checked). Byte-for-byte identical, which is the point: the
      // digests asserted below are computed over these exact bytes.
      return new Response(new Uint8Array(bytes), { status: 200 });
    });
    vi.stubGlobal("fetch", fetchMock);

    try {
      const { manifest: parsed, seriesBySlug } = await loadExportArtifact({ url: "https://concontexto.example/data-derived" });
      expect(parsed.schema_version).toBe(1);
      expect(seriesBySlug.size).toBe(manifest.series.length);
      expect(fetchMock).toHaveBeenCalledTimes(manifest.series.length + 1); // manifest + every series
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("fails loudly on a non-2xx HTTP response, naming the URL and status", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response("not found", { status: 404 })),
    );
    try {
      await expect(loadExportArtifact({ url: "https://concontexto.example/data-derived" })).rejects.toThrow(
        /HTTP 404/,
      );
    } finally {
      vi.unstubAllGlobals();
    }
  });
});

describe("resolveLoadOptionsFromEnv", () => {
  it("prefers EXPORT_URL when both are set (design.md D-1's live/fixture switch)", () => {
    expect(resolveLoadOptionsFromEnv({ EXPORT_URL: "https://x/data-derived", EXPORT_DIR: "test/fixtures/export" })).toEqual({
      url: "https://x/data-derived",
    });
  });

  it("falls back to EXPORT_DIR when only it is set", () => {
    expect(resolveLoadOptionsFromEnv({ EXPORT_DIR: "some/dir" })).toEqual({ dir: "some/dir" });
  });
});

// verify-report CRITICAL-15. The default this block replaces was
// `return { dir: "test/fixtures/export" }` — an unconditional fallback to a
// fixture whose history is SYNTHESISED (`web/test/fixtures/export/source.txt`:
// "MOST OF THE NUMBERS IN THIS DIRECTORY ARE SYNTHESISED. They were never
// published by INE"), reached by any build that set neither variable. The
// Dockerfile's own web-builder stage was exactly such a build, so the
// production image rendered invented numbers under INE attribution, with an
// origin identifier and an extraction timestamp, and disclosed nothing.
//
// The fallback is now an OPT-IN, and the unconfigured case is a hard failure.
// The distinction these tests defend is not stylistic: a silent default is
// indistinguishable, at the call site, from a correctly configured build,
// which is precisely how the defect survived every green suite.
describe("resolveLoadOptionsFromEnv — an unconfigured build must never silently reach the synthetic fixture", () => {
  it("throws when neither EXPORT_URL nor EXPORT_DIR is set, naming BOTH variables so the failure is actionable", () => {
    expect(() => resolveLoadOptionsFromEnv({})).toThrow(/EXPORT_URL/);
    expect(() => resolveLoadOptionsFromEnv({})).toThrow(/EXPORT_DIR/);
  });

  it("never returns the synthetic fixture directory as a default — the returned value cannot be the fixture path", () => {
    // Asserted as "it threw rather than returned" AND "nothing it could have
    // returned points at the fixture": a future edit that restores the
    // fallback fails here even if it also softens the message assertions
    // above.
    let returned: unknown = "the call did not throw";
    try {
      returned = resolveLoadOptionsFromEnv({});
    } catch {
      returned = undefined;
    }
    expect(returned).toBeUndefined();
  });

  it("explains WHY it refuses — the refusal must name the synthesised fixture, not just report a missing variable", () => {
    expect(() => resolveLoadOptionsFromEnv({})).toThrow(/synthes/i);
  });

  it("honours the deliberate opt-in: BUILD_WITH_SYNTHETIC_FIXTURE=1 selects the fixture directory", () => {
    expect(resolveLoadOptionsFromEnv({ BUILD_WITH_SYNTHETIC_FIXTURE: "1" })).toEqual({
      dir: "test/fixtures/export",
    });
  });

  it("requires the opt-in to be exactly \"1\" (the WORKBENCH convention astro.config.mjs already uses) — a truthy-looking value is not consent", () => {
    for (const value of ["true", "yes", "0", "", "TRUE"]) {
      expect(() => resolveLoadOptionsFromEnv({ BUILD_WITH_SYNTHETIC_FIXTURE: value })).toThrow(/EXPORT_URL/);
    }
  });

  it("lets a real artifact source win over the opt-in — a stale opt-in in the environment must not shadow EXPORT_URL/EXPORT_DIR", () => {
    expect(resolveLoadOptionsFromEnv({ BUILD_WITH_SYNTHETIC_FIXTURE: "1", EXPORT_URL: "https://x/data-derived" })).toEqual({
      url: "https://x/data-derived",
    });
    expect(resolveLoadOptionsFromEnv({ BUILD_WITH_SYNTHETIC_FIXTURE: "1", EXPORT_DIR: "some/dir" })).toEqual({
      dir: "some/dir",
    });
  });
});
