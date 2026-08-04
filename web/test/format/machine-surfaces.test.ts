// The other side of `lib/format/period.ts`: the surfaces a period label must
// reach UNCHANGED, stated as executable assertions rather than as a comment
// nobody re-reads.
//
// WHY THIS FILE EXISTS AT ALL. A reader-facing formatter is a one-line change
// at every call site, and every call site looks alike. The three that must
// NOT take it are indistinguishable at a glance from the twelve that must:
//
//   - the published CSV and JSON carry digests and external consumers, and a
//     "T2 2026" in a `period` column breaks every one of them silently;
//   - the permalink's `from`/`to` are PARSED BACK by `resolveCustomRange`, so
//     a display label there is a shared link that quietly resolves to the
//     full series on every reopen;
//   - anything sorted, compared or joined by period ("T2 2026" sorts after
//     "T10 …" would, does not parse, and does not match a map key written as
//     "2026-Q2").
//
// Getting one of those wrong corrupts a download or breaks a shared link,
// which is a far worse failure than an ugly label — and neither shows up in a
// screenshot. So each boundary gets a test that fails loudly when it is
// crossed. `test/format/period.test.ts` proves the formatter is right; this
// file proves it is not applied where it must not be.
import { createHash } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import { decodeChartState, encodeChartState } from "../../src/lib/chart/permalink";
import { periodOrdinalIndex, previousPeriod, addYears } from "../../src/lib/chart/periods";
import { resolveCustomRange, sliceRange } from "../../src/lib/transform/sliceRange";
import { computeYoY } from "../../src/lib/transform/yoy";
import type { ChartPoint } from "../../src/lib/chart/geometry";

const HERE = path.dirname(fileURLToPath(import.meta.url));
const FIXTURE_DIR = path.resolve(HERE, "../fixtures/export");
const SRC_DIR = path.resolve(HERE, "../../src");

/** The canonical storage shapes, exactly as migration 0001 defines them.
 * Written out here rather than imported, so this guard keeps meaning what it
 * means even if the production regex is ever loosened. */
const CANONICAL_PERIOD = /^(\d{4}-Q[1-4]|\d{4}-(0[1-9]|1[0-2])|\d{4})$/;

const QUARTERLY_POINTS: ChartPoint[] = [
  { period: "2019-Q1", value: 10.2, status: "D" },
  { period: "2019-Q2", value: 10.5, status: "D" },
  { period: "2019-Q3", value: 10.1, status: "D" },
  { period: "2019-Q4", value: 9.8, status: "D" },
  { period: "2020-Q1", value: 14.4, status: "D" },
  { period: "2020-Q2", value: 15.3, status: "D" },
];

describe("the published CSV and JSON are the pipeline's own bytes, and carry only canonical periods", () => {
  // The boundary is STRUCTURAL first: `app/internal/publishing` writes these
  // files and the Astro build only reads them, so no reader-facing formatter
  // can reach them by construction. These assertions pin that the structure
  // is still what the comment claims.

  it("every published series document still hashes to the digest its own manifest declares", () => {
    // The strongest available statement of "byte-unchanged": the artifact
    // carries a sha256 per file, written by the producer, and `loader.ts`
    // already refuses any file that does not match. Recomputing them here
    // means this test fails the moment ANY byte of a published JSON moves —
    // whether from a formatter, a hand edit, or a regeneration — without this
    // file needing a hand-copied constant that could itself be updated to fit
    // the damage.
    const manifest = JSON.parse(readFileSync(path.join(FIXTURE_DIR, "manifest.json"), "utf-8")) as {
      digests: Record<string, string>;
    };
    const entries = Object.entries(manifest.digests);
    expect(entries.length).toBeGreaterThanOrEqual(6);
    for (const [relativePath, declared] of entries) {
      const bytes = readFileSync(path.join(FIXTURE_DIR, relativePath));
      expect(createHash("sha256").update(bytes).digest("hex"), `${relativePath}: digest`).toBe(declared);
    }
  });

  it("every `period` in every published series document is a canonical storage label", () => {
    const seriesDir = path.join(FIXTURE_DIR, "series");
    const files = readdirSync(seriesDir).filter((f) => f.endsWith(".json"));
    expect(files.length).toBeGreaterThanOrEqual(6);
    for (const file of files) {
      const doc = JSON.parse(readFileSync(path.join(seriesDir, file), "utf-8")) as {
        points: { period: string }[];
      };
      for (const point of doc.points) {
        expect(point.period, `${file}: "${point.period}" is not a canonical period label`).toMatch(
          CANONICAL_PERIOD,
        );
      }
    }
  });

  it("every `period` cell in every published CSV is a canonical storage label", () => {
    const csvDir = path.join(FIXTURE_DIR, "csv");
    const files = readdirSync(csvDir).filter((f) => f.endsWith(".csv"));
    expect(files.length).toBeGreaterThanOrEqual(6);
    for (const file of files) {
      const lines = readFileSync(path.join(csvDir, file), "utf-8").split("\n");
      const headerIndex = lines.findIndex((line) => line.startsWith("period,"));
      expect(headerIndex, `${file}: no period header row`).toBeGreaterThanOrEqual(0);
      const rows = lines.slice(headerIndex + 1).filter((line) => line.trim().length > 0);
      expect(rows.length).toBeGreaterThan(0);
      for (const row of rows) {
        const cell = row.split(",")[0];
        expect(cell, `${file}: "${cell}" is not a canonical period label`).toMatch(CANONICAL_PERIOD);
      }
    }
  });

  it("no module in the web tree writes a CSV or a series JSON, which is what makes the boundary structural", () => {
    // If this ever stops being true, the two assertions above stop being a
    // guarantee and become a coincidence — so the structural claim gets its
    // own test rather than living only in a doc comment.
    const offenders: string[] = [];
    const walk = (dir: string) => {
      for (const entry of readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          walk(full);
          continue;
        }
        if (!/\.(ts|astro|svelte)$/.test(entry.name)) continue;
        const source = readFileSync(full, "utf-8");
        if (/\bwriteFile(Sync)?\b|\bcreateWriteStream\b/.test(source)) {
          offenders.push(path.relative(SRC_DIR, full));
        }
      }
    };
    walk(SRC_DIR);
    expect(offenders, `web sources that write files: ${offenders.join(", ")}`).toEqual([]);
  });
});

describe("permalink query parameters stay canonical, byte for byte", () => {
  it("encodes the custom range's bounds exactly as the series stores them", () => {
    // The exact bytes, not a `toContain`: this string is what lands in the
    // address bar and in whatever a reader pastes into a message.
    expect(
      encodeChartState({ range: "custom", transform: "raw", custom: { from: "2019-Q1", to: "2020-Q4" } }),
    ).toBe("?range=custom&from=2019-Q1&to=2020-Q4");
    expect(
      encodeChartState({ range: "custom", transform: "yoy", custom: { from: "2002-01", to: "2026-06" } }),
    ).toBe("?range=custom&from=2002-01&to=2026-06&transform=yoy");
  });

  it("round-trips a shared link back to the same bounds, and those bounds still resolve against the series", () => {
    const search = encodeChartState({
      range: "custom",
      transform: "raw",
      custom: { from: "2019-Q2", to: "2020-Q1" },
    });
    const decoded = decodeChartState(search, ["full", "custom"], ["raw"]);
    expect(decoded.custom).toEqual({ from: "2019-Q2", to: "2020-Q1" });

    // The half that actually matters to a reader: the decoded bounds are
    // still something `resolveCustomRange` accepts. A display label decodes
    // just as happily and is then silently rejected, which is how a shared
    // link degrades to the full series with no visible error.
    const resolution = resolveCustomRange(
      QUARTERLY_POINTS.map((p) => p.period),
      "Q",
      decoded.custom!.from,
      decoded.custom!.to,
    );
    expect(resolution.status).toBe("ok");
  });

  it("refuses a display-formatted bound, which is why it must never be written into a permalink", () => {
    // The negative that gives the two assertions above their meaning.
    const resolution = resolveCustomRange(
      QUARTERLY_POINTS.map((p) => p.period),
      "Q",
      "T2 2019",
      "T1 2020",
    );
    expect(resolution.status).toBe("rejected");
  });
});

describe("period arithmetic keeps computing on the canonical label", () => {
  it("orders quarters chronologically, which a display label cannot be made to do", () => {
    const ordered = ["2019-Q4", "2020-Q1", "2020-Q2"].map((p) => periodOrdinalIndex(p, "Q"));
    expect(ordered).toEqual([...ordered].sort((a, b) => a - b));
    expect(periodOrdinalIndex("2020-Q1", "Q") - periodOrdinalIndex("2019-Q4", "Q")).toBe(1);
  });

  it("steps and shifts periods, and both throw on a display label rather than inventing one", () => {
    expect(previousPeriod("2020-Q1", "Q")).toBe("2019-Q4");
    expect(addYears("2020-Q1", "Q", -1)).toBe("2019-Q1");
    // Fail-closed, exactly as `parsePeriod` documents: a display label
    // reaching this arithmetic is a defect, and a silent coercion would hide
    // it behind a plausible-looking chart.
    expect(() => previousPeriod("T1 2020", "Q")).toThrow();
    expect(() => periodOrdinalIndex("T1 2020", "Q")).toThrow();
  });

  it("slices a range and joins a year-on-year divisor by the canonical label", () => {
    expect(sliceRange(QUARTERLY_POINTS, "Q", "full")).toHaveLength(QUARTERLY_POINTS.length);
    const yoy = computeYoY(QUARTERLY_POINTS, "Q");
    // 2020-Q1 over 2019-Q1 — a join that only happens because both sides are
    // still spelled the way the artifact spells them.
    expect(yoy.map((p) => p.period)).toContain("2020-Q1");
  });
});
