// Zod schemas for the publishing-export artifact (design.md D-1, "Export
// artifact schema (as-built, schema_version: 1)"). This is the anti-drift
// device design.md commits to: the SAME schemas the real loader
// (`web/src/lib/export/loader.ts`, slice 9a) will use to validate the live
// artifact also validate the Go-generated golden fixture
// (`web/test/fixtures/export/`) in `web/test/export/schema.test.ts`, so a
// silent shape mismatch between the two ecosystems fails a fast unit test
// instead of a build days later.
//
// `SUPPORTED_SCHEMA_VERSION` is checked with EXACT integer equality by the
// loader, never `>=` (design.md D-1: "requires schema_version to equal the
// one version the loader supports").
import { z } from "zod";

export const SUPPORTED_SCHEMA_VERSION = 1 as const;

export const ObservationStatusSchema = z.enum(["P", "D"]);
export type ObservationStatus = z.infer<typeof ObservationStatusSchema>;

export const FrequencySchema = z.enum(["M", "Q", "A"]);
export type Frequency = z.infer<typeof FrequencySchema>;

export const ArtifactFreshnessSchema = z.enum(["fresh", "source-pending"]);
export type ArtifactFreshness = z.infer<typeof ArtifactFreshnessSchema>;

export const SourceRefSchema = z.object({
  id: z.string().min(1),
  name: z.string().min(1),
  attribution: z.string().min(1),
  licenceName: z.string().min(1),
  licenceUrl: z.string().min(1),
});
export type SourceRef = z.infer<typeof SourceRefSchema>;

export const OriginRefSchema = z.object({
  kind: z.string().min(1),
  ref: z.string().min(1),
  requestUrl: z.string().min(1),
});
export type OriginRef = z.infer<typeof OriginRefSchema>;

export const VintageRefSchema = z.object({
  ingestionRunId: z.number().int().positive(),
  extractedAt: z.string().min(1),
});
export type VintageRef = z.infer<typeof VintageRefSchema>;

/** One entry of the per-run `vintages` lookup (design.md D-1's resolution
 * of "provenance survives the export" — keyed by `ingestion_run_id`,
 * giving every point's full provenance chain with zero further queries).
 *
 * `rawFileSha256` is validated as a non-empty string, not a strict
 * 64-hex-character regex: the slice-3 golden fixture
 * (`web/test/fixtures/export/`) was generated from throwaway seed data
 * whose `raw_file.sha256` column holds a human-readable placeholder
 * (`"fixture0000...aa"`, not a genuine hex digest) rather than a real
 * ingested file's hash. Production `publishing.ValidateArtifact` computes
 * genuine hex digests ("digests computed last", design.md D-1); a stricter
 * format check can be reinstated once the fixture is regenerated from real
 * ingested data — disclosed here rather than silently loosened without a
 * reason. */
export const VintageProvenanceSchema = z.object({
  extractedAt: z.string().min(1),
  rawFileSha256: z.string().min(1),
  requestUrl: z.string().min(1),
});
export type VintageProvenance = z.infer<typeof VintageProvenanceSchema>;

/** The page state PRD §6.1.3's three banners are driven by, carried by the
 * artifact itself (verify-report CRITICAL-4 remediation: this was previously
 * a hand-maintained constant in `content/indicators/methodology.ts`, so a
 * real validation failure produced no banner without a source edit and a
 * redeploy).
 *
 * Modelled as a DISCRIMINATED UNION rather than one flat object with three
 * loose fields, because the Go half's contract is not just "three optional
 * fields": `lastCorrectUpdate` is non-null ONLY on the validation-failure
 * arm and `successorSlug` ONLY on the discontinued arm. Encoding that here
 * means the page can never render a banner from a field its own state has
 * no claim to — a drifted writer fails the build loudly (loader contract:
 * "An unsupported version or a shape mismatch MUST fail the build loudly")
 * instead of quietly producing a banner nobody specified.
 *
 * `lastCorrectUpdate` is NULLABLE on its own arm, and this is deliberate,
 * not defensive slack. The Go half emits `kind: "validation-failure"` with
 * `lastCorrectUpdate: null` when a series' very FIRST ingestion run failed
 * validation: there has never been a correct update to name. Substituting
 * any stand-in date (run start, extraction instant, today) would make the
 * banner assert a provenance fact that never happened (principle P4), and
 * downgrading the state to "fresh" would silently suppress a recorded
 * failure. The artifact carries the fact; `lib/indicator/pageState.ts`
 * owns the wording that names no date.
 *
 * The date is required to be a BARE `YYYY-MM-DD` calendar date rather than
 * any parseable timestamp: the banner prints this value verbatim into
 * reader-facing Spanish prose (the same convention `MethodologySheetFields`
 * already uses for `extractedAt`), so a writer that started emitting a full
 * RFC 3339 instant would otherwise leak `2026-07-29T11:04:00Z` into a
 * sentence. Failing the build is the intended outcome of that drift. */
export const PageStateSchema = z.discriminatedUnion("kind", [
  z.object({
    kind: z.literal("fresh"),
    lastCorrectUpdate: z.null(),
    successorSlug: z.null(),
  }),
  z.object({
    kind: z.literal("validation-failure"),
    lastCorrectUpdate: z.string().regex(/^\d{4}-\d{2}-\d{2}$/).nullable(),
    successorSlug: z.null(),
  }),
  z.object({
    kind: z.literal("discontinued"),
    lastCorrectUpdate: z.null(),
    successorSlug: z.string().min(1).nullable(),
  }),
]);
export type ArtifactPageState = z.infer<typeof PageStateSchema>;

/** A calendar DAY, `YYYY-MM-DD`.
 *
 * Break and event dates are calendar days, never series periods — the Go
 * writer says so explicitly and enforces it by construction: `dateOnly()`
 * (`app/internal/publishing/export.go`) formats every one of them with
 * `time.Time.UTC().Format("2006-01-02")`. `Point.period` is the OTHER
 * format ("2002-Q1"), and the two are not interchangeable:
 * `IndicatorPage.astro` feeds `break.date` to `periodFromCalendarDate()`,
 * which needs a real calendar day to derive the period label the reader
 * sees. A writer that started emitting periods or RFC 3339 instants here
 * must fail the build rather than render a mangled label.
 */
const CalendarDateSchema = z.string().regex(/^\d{4}-\d{2}-\d{2}$/);

/** The three annotation groups the page is able to render.
 *
 * Deliberately TIGHTER than the Go half, which types `EventRef.Group` as a
 * bare `string` and never constrains its value — `config/loader.go` copies
 * it straight out of `config/eventos.yaml` (defaulting only
 * `config/gobiernos.yaml`'s entries to "governments"), so a typo in that
 * YAML travels all the way into the artifact unchallenged.
 *
 * Tightening it HERE is the only place the mistake becomes visible.
 * `ChartIsland.svelte`'s `groupedAnnotations` walks the three known groups
 * and filters each one's entries out of `annotations`; an event carrying a
 * fourth value matches no group, renders nowhere, and reports nothing. An
 * editorial event silently missing from a page is precisely the failure
 * mode the loader contract ("a shape mismatch MUST fail the build loudly")
 * exists to prevent — so the build fails instead. It also means
 * `IndicatorPage.astro` can pass `event.group` straight through, rather
 * than asserting the union with a cast the compiler cannot check.
 */
export const AnnotationGroupSchema = z.enum(["governments", "exogenous", "milestones"]);
export type AnnotationGroup = z.infer<typeof AnnotationGroupSchema>;

/** One entry of a series document's `breaks` section — the Go writer's
 * `BreakRef` (`app/internal/publishing/artifact.go`).
 *
 * `sourceUrl` is `.optional()` and NOT `.nullable()`, which mirrors the Go
 * declaration exactly rather than hedging: the field is
 * `SourceURL *string \`json:"sourceUrl,omitempty"\``, and `omitempty` on a
 * pointer OMITS THE KEY for a nil value — `encoding/json` never writes
 * `"sourceUrl": null` from that declaration. Accepting `null` as well would
 * quietly widen the contract to a shape the writer cannot produce, which is
 * the opposite of what this schema is for. `IndicatorPage.astro` narrows
 * the resulting `string | undefined` to the `string | null` its consumers
 * take, at the one boundary that owns that decision.
 *
 * Every other field is required and non-empty, because every one of them is
 * rendered verbatim: `key` keys Svelte's `{#each}`, `date` becomes the
 * reader-facing period label, `kind` is printed in parentheses beside it,
 * and `noteMd` is the tooltip's entire body. `undefined` reaching any of
 * them prints "undefined" into Spanish prose on a public page.
 */
export const BreakRefSchema = z.object({
  key: z.string().min(1),
  date: CalendarDateSchema,
  kind: z.string().min(1),
  noteMd: z.string().min(1),
  sourceUrl: z.string().min(1).optional(),
});
export type BreakRef = z.infer<typeof BreakRefSchema>;

/** One entry of a series document's `events` section — the Go writer's
 * `EventRef`. Same optional-not-nullable reasoning as `BreakRefSchema` for
 * `dateEnd` and `noteMd`, both `*string` + `omitempty` on the Go side.
 *
 * `dateEnd` absent is the ordinary case, not an edge one: it is how an
 * open-ended entry (a sitting government, an ongoing episode) is written.
 * `toEventRefs` additionally collapses an empty `noteMd` or `sourceUrl` to
 * nil before marshalling, so the key is absent rather than `""` — hence
 * `.min(1)` under the `.optional()`, which rejects an empty string a drifted
 * writer would have to have gone out of its way to emit.
 *
 * `sourceUrl` is the document the registry recorded for the entry, where it
 * recorded one. Optional, because a change of government has none to point
 * at; the same `.optional()`-never-`.nullable()` contract every other
 * optional field on this type keeps.
 */
export const EventRefSchema = z.object({
  id: z.string().min(1),
  group: AnnotationGroupSchema,
  name: z.string().min(1),
  dateStart: CalendarDateSchema,
  dateEnd: CalendarDateSchema.optional(),
  noteMd: z.string().min(1).optional(),
  sourceUrl: z.string().min(1).optional(),
});
export type EventRef = z.infer<typeof EventRefSchema>;

/**
 * Rejects an array whose entries do not carry distinct values of `field`.
 *
 * Mirrors the Go writer's `validateBreaks`/`validateEvents`
 * (`app/internal/publishing/validate.go`), which refuse a duplicated
 * `key`/`id` as "an unresolved (ambiguous) reference" before an artifact is
 * ever written. Re-checking it on this side is not defensive duplication:
 * both sections are rendered with a KEYED `{#each ... (b.key)}` in
 * `ChartIsland.svelte`, and Svelte throws on duplicate keys at runtime.
 * Without this, a drifted writer converts a loud build-time rejection into
 * a client-side crash on an already-hydrated public page.
 */
function uniqueBy<T extends Record<string, unknown>>(field: keyof T & string) {
  return (entries: T[]): boolean => new Set(entries.map((entry) => entry[field])).size === entries.length;
}

export const SeriesPointSchema = z.object({
  period: z.string().min(1),
  value: z.number().nullable(),
  status: ObservationStatusSchema,
  version: z.number().int().positive(),
  ingestionRunId: z.number().int().positive(),
});
export type SeriesPoint = z.infer<typeof SeriesPointSchema>;

export const SeriesDocSchema = z.object({
  schema_version: z.literal(SUPPORTED_SCHEMA_VERSION),
  slug: z.string().min(1),
  name: z.string().min(1),
  unit: z.string().min(1),
  frequency: FrequencySchema,
  decimals: z.number().int().nonnegative(),
  geo: z.string().min(1),
  operation: z.string(), // Disclosed gap (design.md): populated from dataset.id today, not yet a real config field.
  base: z.string().nullable(), // Disclosed gap: always null until a future slice adds index-base config.
  source: SourceRefSchema,
  origin: OriginRefSchema,
  vintage: VintageRefSchema,
  points: z.array(SeriesPointSchema),
  sourceStatus: z.record(z.string(), z.string()),
  withdrawn: z.array(z.unknown()),
  vintages: z.record(z.string(), VintageProvenanceSchema),
  breaks: z
    .array(BreakRefSchema)
    .refine(uniqueBy<BreakRef>("key"), { message: "duplicated break key — an unresolved (ambiguous) reference" }),
  events: z
    .array(EventRefSchema)
    .refine(uniqueBy<EventRef>("id"), { message: "duplicated event id — an unresolved (ambiguous) reference" }),
  freshness: ArtifactFreshnessSchema,
  // REQUIRED, never `.optional()`/`.default()`: a document without it must
  // fail the loader rather than be silently treated as fresh (that silent
  // treatment IS verify-report CRITICAL-4). Added WITHOUT a
  // `schema_version` bump because the Go writer and this loader ship
  // together in the same change — no already-published artifact lacking the
  // field is ever read by a loader that requires it.
  pageState: PageStateSchema,
});
export type SeriesDoc = z.infer<typeof SeriesDocSchema>;

export const ManifestSchema = z.object({
  schema_version: z.literal(SUPPORTED_SCHEMA_VERSION),
  generated_at: z.string().min(1),
  series: z.array(z.string().min(1)),
  digests: z.record(z.string(), z.string()),
});
export type Manifest = z.infer<typeof ManifestSchema>;

/**
 * Parses and validates a manifest, requiring EXACT `schema_version`
 * equality with the one version this loader supports (design.md D-1).
 */
export function parseManifest(data: unknown): Manifest {
  const manifest = ManifestSchema.parse(data);
  if (manifest.schema_version !== SUPPORTED_SCHEMA_VERSION) {
    throw new Error(
      `Unsupported manifest schema_version ${manifest.schema_version}; this loader supports exactly ${SUPPORTED_SCHEMA_VERSION}`,
    );
  }
  return manifest;
}

/** Parses and validates one series document, same exact-version contract. */
export function parseSeriesDoc(data: unknown): SeriesDoc {
  const doc = SeriesDocSchema.parse(data);
  if (doc.schema_version !== SUPPORTED_SCHEMA_VERSION) {
    throw new Error(
      `Unsupported series schema_version ${doc.schema_version} for "${doc.slug}"; this loader supports exactly ${SUPPORTED_SCHEMA_VERSION}`,
    );
  }
  return doc;
}
