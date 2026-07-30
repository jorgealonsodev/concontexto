// indicator-page spec, "A slug change ships a permanent redirect". Task
// 9b.2 (RED-first, per this slice's own boundary — unlike the browser/budget
// gates below it, this is a pure function). Proves the redirect MECHANISM
// via a test fixture, per this project's explicit instruction, since this
// change's own production map is empty (all six slugs are frozen here).
import { describe, expect, it } from "vitest";
import { buildAstroRedirects, SLUG_REDIRECTS } from "../../src/lib/indicator/redirects";
import { INDICATOR_CONTENT } from "../../src/content/indicators";

describe("SLUG_REDIRECTS / buildAstroRedirects", () => {
  it("is empty in production — this change freezes all six slugs, none renamed", () => {
    expect(SLUG_REDIRECTS).toEqual({});
    expect(buildAstroRedirects()).toEqual({});
  });

  it("translates a fixture from->to slug map into Astro's `redirects` config shape, anchored under /indicador/", () => {
    const fixture = { "paro-epa-legacy": "tasa-de-paro-epa" };
    expect(buildAstroRedirects(fixture)).toEqual({
      "/indicador/paro-epa-legacy": "/indicador/tasa-de-paro-epa",
    });
  });

  it("supports more than one entry at once", () => {
    const fixture = { "indice-precios-antiguo": "ipc-general", pib_legacy: "pib" };
    expect(buildAstroRedirects(fixture)).toEqual({
      "/indicador/indice-precios-antiguo": "/indicador/ipc-general",
      "/indicador/pib_legacy": "/indicador/pib",
    });
  });

  it("every fixture redirect target resolves to a real, currently frozen slug — a redirect must never point nowhere", () => {
    const fixture = { "paro-epa-legacy": "tasa-de-paro-epa", "old-pib": "pib" };
    for (const to of Object.values(fixture)) {
      expect(Object.keys(INDICATOR_CONTENT)).toContain(to);
    }
  });
});
