// The site footer: the one place on this site that says, at SITE level, what
// may be done with what it publishes and where the raw bytes behind it are.
//
// THE TWO DEFECTS THIS DRIVES.
//
// 1. AN UNREACHABLE PROVENANCE ARTIFACT. Every ingest republishes
//    `STATIC_ROOT/transparencia/raw-files.sha256` (app/cmd/concontexto/
//    ingest_cmd.go's `publicHashPath`) — the SHA-256 of every archived raw
//    source file, with its source, its download timestamp and the exact URL
//    it came from. It is served, it returns 200, and NOTHING in `src/`
//    linked to it: `grep -rn transparencia src/` returned nothing. For a
//    project whose whole premise is that a published figure is traceable to
//    the bytes it came from (PRD §14.2, spec raw-file-archive), an
//    unreachable listing is the claim without the evidence.
//
// 2. NO SITE-LEVEL LICENSING STATEMENT AT ALL. The methodology sheet names
//    each SERIES' source and licence, per series, and that stays the
//    authoritative place for it. What no page said is the site-level fact: the
//    code is MIT, and the data is NOT under one licence.
//
// WHY THE NEGATIVE ASSERTIONS BELOW ARE THE POINT. The obvious footer — "©
// ConContexto · Datos bajo CC BY 4.0" — would violate a hard requirement:
//
//   openspec/.../source-attribution-licensing/spec.md
//   ### Requirement: No blanket data-licence claim exists in the repository
//   "The repository MUST NOT assert a single licence over all derived data.
//    ... `LICENSE-DATA` MUST defer to `sources/{source}.yaml` rather than
//    override it."
//
// Eurostat is the concrete reason it would be false: Commission Decision
// 2011/833/EU authorises reuse of Eurostat's OWN material with
// acknowledgement, that permission does NOT extend to third-party material
// Eurostat republishes, and some commercial redissemination is separately
// restricted (config/sources/eurostat.yaml's own `redistribution` block).
// INE and Seguridad Social carry their own, different terms. A footer that
// flattened those three into one sentence would be the overclaim the spec
// forbids — so the tests below assert what the footer must NOT say as
// carefully as what it must.
import { afterEach, describe, expect, it, vi } from "vitest";
import { experimental_AstroContainer as AstroContainer } from "astro/container";
import { globSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import Home from "../../src/pages/index.astro";
import SiteFooter from "../../src/templates/SiteFooter.astro";
import { es } from "../../src/i18n/es";

const WEB_ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");
const SRC_DIR = path.join(WEB_ROOT, "src");
const REPO_ROOT = path.resolve(WEB_ROOT, "..");

/** Every `.astro` file under `src/` that opens its own document — the same
 * discovery `favicon.test.ts` uses, and for the same reason its own header
 * argues at length: a hand-maintained list of route files goes stale
 * SILENTLY the first time a fourth route is added. A footer is site-level
 * furniture; "site-level" means every document, discovered the way a browser
 * discovers one. */
function documentEntryPoints(): string[] {
  return globSync("**/*.astro", { cwd: SRC_DIR })
    .filter((file) => readFileSync(path.join(SRC_DIR, file), "utf8").includes("<html"))
    .sort();
}

async function renderHome(): Promise<string> {
  vi.stubEnv("BUILD_WITH_SYNTHETIC_FIXTURE", "1");
  const container = await AstroContainer.create();
  return container.renderToString(Home);
}

async function renderFooter(): Promise<string> {
  const container = await AstroContainer.create();
  return container.renderToString(SiteFooter);
}

/** Every `href` the footer emits, in document order. */
function footerHrefs(html: string): string[] {
  const footer = /<footer[\s\S]*?<\/footer>/.exec(html);
  if (!footer) return [];
  return [...footer[0].matchAll(/href="([^"]+)"/g)].map((m) => m[1]);
}

/** The footer's visible text, tags stripped and whitespace collapsed — the
 * string a reader actually reads, which is what the no-blanket-claim
 * assertions have to be made against (a claim hidden in an attribute would
 * still be a claim, so the raw markup is checked separately below). */
function footerText(html: string): string {
  const footer = /<footer[\s\S]*?<\/footer>/.exec(html);
  if (!footer) return "";
  return footer[0]
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

afterEach(() => {
  vi.unstubAllEnvs();
});

describe("Site footer — placement", () => {
  it("discovers every route entry point, so this scan cannot cover fewer pages than exist", () => {
    const entries = documentEntryPoints();
    // Same floor and same rationale as `favicon.test.ts`: three documents
    // carry their own `<html>` today (`/`, `/indicador/{slug}`,
    // `/workbench`). "At least", not "exactly", so a genuinely new route
    // does not require editing this test — only shipping a footer on it.
    expect(entries.length).toBeGreaterThanOrEqual(3);
    expect(entries).toContain("pages/index.astro");
    expect(entries).toContain("pages/indicador/[slug].astro");
    expect(entries).toContain("workbench/pages/index.astro");
  });

  it("renders the footer in EVERY document that has a body, the workbench included", () => {
    for (const entry of documentEntryPoints()) {
      const source = readFileSync(path.join(SRC_DIR, entry), "utf8");
      expect(source, `${entry} opens an <html> document but renders no <SiteFooter />`).toContain("<SiteFooter />");
    }
  });

  it("places the footer OUTSIDE <main>, so it is a contentinfo landmark and not part of the page's main content", () => {
    for (const entry of documentEntryPoints()) {
      const source = readFileSync(path.join(SRC_DIR, entry), "utf8");
      const mainClose = source.indexOf("</main>");
      const footerTag = source.indexOf("<SiteFooter />");
      expect(mainClose, `${entry} has no </main> to place the footer after`).toBeGreaterThan(-1);
      expect(
        footerTag,
        `${entry} renders <SiteFooter /> inside <main>, where it is ordinary content rather than a contentinfo landmark`,
      ).toBeGreaterThan(mainClose);
    }
  });

  it("really reaches the built homepage, not merely the source file", async () => {
    const html = await renderHome();

    expect(html).toContain('data-testid="site-footer"');
    expect(html.indexOf("</main>")).toBeLessThan(html.indexOf("<footer"));
  });
});

describe("Site footer — what it says about licensing, and what it must never say", () => {
  it("states that no single licence covers the data, rather than naming one", async () => {
    const text = footerText(await renderFooter());

    expect(text).toContain(es.footer.dataTermsNote);
  });

  it("asserts no blanket licence over all derived data (spec: 'The repository MUST NOT assert a single licence over all derived data')", async () => {
    const html = await renderFooter();
    const text = footerText(html);

    // The exact overclaims this footer could plausibly have been written as.
    // `CC BY 4.0` is a real, correct licence for ConContexto's OWN editorial
    // text where a source permits redistribution (LICENSE-DATA) — and stating
    // it in a site footer, next to the data, is precisely the flattening that
    // would make it read as covering Eurostat-derived series too.
    for (const forbidden of [/cc\s*by/i, /creative\s*commons/i, /todos los datos/i, /licencia de los datos/i]) {
      expect(text, `the footer's visible text matches ${forbidden}, which claims a licence over data`).not.toMatch(
        forbidden,
      );
      expect(html, `the footer's markup matches ${forbidden} in an attribute`).not.toMatch(forbidden);
    }
  });

  it("claims MIT for the code only — the one licence that IS a true single claim here", async () => {
    const text = footerText(await renderFooter());

    expect(text).toContain(es.footer.codeLicenceLabel);
    // MIT is named next to "código", never next to "datos".
    expect(es.footer.codeLicenceLabel.toLowerCase()).toContain("código");
    expect(es.footer.codeLicenceLabel).toContain("MIT");
  });

  it("sends the reader to the per-source terms themselves, which the spec makes authoritative", async () => {
    const hrefs = footerHrefs(await renderFooter());

    // Not `LICENSE-DATA`: that file DEFERS to these, and putting a deferring
    // summary between the reader and the authority is the hop this footer
    // exists to remove (spec: "these per-source terms MUST be authoritative
    // over any site-wide statement", settled decision D2).
    expect(hrefs.some((href) => href.includes("config/sources"))).toBe(true);
  });

  it("names a real licence file for the code claim it does make", async () => {
    const hrefs = footerHrefs(await renderFooter());

    expect(hrefs.some((href) => href.endsWith("/LICENSE"))).toBe(true);
  });
});

describe("Site footer — the raw-file listing nothing linked to", () => {
  it("links the published SHA-256 listing", async () => {
    const hrefs = footerHrefs(await renderFooter());

    expect(hrefs).toContain("/transparencia/raw-files.sha256");
  });

  it("points at the path the ingest command really publishes, not a second hard-coded guess", () => {
    // The lesson of verify-report CRITICAL-1, applied to a different writer:
    // `ActionBar`'s CSV href was `/data-derived/{slug}.csv` while the Go
    // exporter wrote `/data-derived/csv/{slug}.csv`, and every one of the six
    // pages linked to a 404 because the two strings were maintained
    // independently. This asserts against the WRITER's own source rather than
    // restating the path here.
    const ingestCmd = readFileSync(path.join(REPO_ROOT, "app", "cmd", "concontexto", "ingest_cmd.go"), "utf8");
    const publishedPath = /publicHashPath\s*=\s*filepath\.Join\(([^)]*)\)/.exec(ingestCmd);
    expect(publishedPath, "ingest_cmd.go no longer joins a publicHashPath — this guard has gone blind").not.toBeNull();
    expect(publishedPath![1]).toContain('"transparencia"');
    expect(publishedPath![1]).toContain('"raw-files.sha256"');
  });

  it("describes it as the plain-text hash listing it is, not as a page", async () => {
    const text = footerText(await renderFooter());

    // A reader who clicks this gets 12 lines of `<sha256>  <source>
    // <timestamp>  <url>` served as `text/plain`. Link text that promised
    // "transparencia" or "procedencia de los datos" would be describing a
    // page this project does not have.
    expect(text).toContain(es.footer.rawFilesLabel);
    expect(es.footer.rawFilesLabel).toContain("SHA-256");
  });
});

describe("Site footer — accessibility and copy constraints the gates enforce", () => {
  it("renders a <footer> element, the contentinfo landmark axe expects", async () => {
    expect(await renderFooter()).toMatch(/<footer[\s>]/);
  });

  it("contains no heading at all, so it can never skip a level below whatever precedes it", async () => {
    // The footer follows an h3 on `/`, an h2 on an indicator page and an h2
    // on the workbench. Any fixed heading level would skip on at least one
    // of them; the footer needs no heading to be navigable, since
    // contentinfo is itself a landmark a screen reader jumps to.
    expect(await renderFooter()).not.toMatch(/<h[1-6][\s>]/);
  });

  it("sizes every link as a 44 px touch target, which the 375 px e2e sweep will measure for real", async () => {
    const html = await renderFooter();
    const links = [...html.matchAll(/<a\b[^>]*>/g)].map((m) => m[0]);

    expect(links.length).toBeGreaterThan(0);
    for (const link of links) {
      // A structural check, deliberately: the authoritative measurement is
      // the rendered-geometry sweep in `tests/e2e/footer/site-footer.spec.ts`,
      // which runs in a real browser at 375 px. This one fails FASTER and
      // names the file, which is worth having at this layer too.
      expect(link, `a footer link with no 44 px floor: ${link}`).toContain("min-h-11");
      expect(link, `a footer link with no 44 px floor: ${link}`).toContain("min-w-11");
    }
  });

  it("takes every reader-facing string from `es.ts`", async () => {
    const source = readFileSync(path.join(SRC_DIR, "templates", "SiteFooter.astro"), "utf8");
    const template = source.replace(/^---\r?\n[\s\S]*?\r?\n---/, "");

    // The same shape as the widened no-inlined-copy scan in
    // `indicator-page.container.test.ts` (which already globs this file too);
    // restated here so a failure names the footer directly.
    const suspicious = [...template.matchAll(/>([^<>{}]{4,})</g)]
      .map((m) => m[1].replace(/\s+/g, " ").trim())
      .filter((text) => text.length > 0 && /[a-záéíóúñ]{4,}/i.test(text));
    expect(suspicious, `bare text nodes in SiteFooter.astro: ${JSON.stringify(suspicious)}`).toEqual([]);
  });

  it("does not repeat, at site level, what the methodology sheet already gives per series", async () => {
    const text = footerText(await renderFooter());

    // Source name, licence link, origin identifier and extraction timestamp
    // are per-SERIES facts, and `MethodologySheet.astro` is their
    // authoritative home. A footer that repeated them would be stating them
    // once per site for data that differs per series.
    for (const perSeriesLabel of [
      es.methodologySheet.sourceLabel,
      es.methodologySheet.originLabel,
      es.methodologySheet.extractedAtLabel,
    ]) {
      expect(text, `the footer repeats the per-series "${perSeriesLabel}" the methodology sheet owns`).not.toContain(
        perSeriesLabel,
      );
    }
  });
});

describe("Site footer — one repository, not two", () => {
  it("shares its repository root with the methodology sheet's ingestion-script links", async () => {
    // The footer introduces this project's SECOND hard-coded repository URL
    // under `src/`. `workbench/fixtures.ts` already carries a third that
    // points at a repository that does not exist
    // (`github.com/concontexto/concontexto`) — harmless in a fixture, and
    // exactly the drift this pins for the two REAL reader-facing ones.
    const hrefs = footerHrefs(await renderFooter());
    const repoHref = hrefs.find((href) => href.startsWith("https://github.com/"));
    expect(repoHref, "the footer links to no repository at all").toBeDefined();

    const repoRoot = /^https:\/\/github\.com\/[^/]+\/[^/]+/.exec(repoHref!)![0];
    const methodology = readFileSync(path.join(SRC_DIR, "content", "indicators", "methodology.ts"), "utf8");
    const methodologyRepos = [...methodology.matchAll(/https:\/\/github\.com\/[^/"]+\/[^/"]+/g)].map((m) => m[0]);
    expect(methodologyRepos.length, "methodology.ts links to no repository — this guard has gone blind").toBeGreaterThan(
      0,
    );
    for (const other of methodologyRepos) {
      expect(other, `the footer links ${repoRoot} while methodology.ts links ${other}`).toBe(repoRoot);
    }
  });
});
