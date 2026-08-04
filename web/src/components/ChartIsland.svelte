<script lang="ts">
  // PRD §14.2: the ONE interactive island this product ships. Progressive
  // enhancement over the exact no-JS experience `IndicatorChart.astro`
  // (slice 7) already proves complete — this component is self-contained
  // (design.md D-5: "the island... redrawing client-side by calling the
  // *same* [geometry/transform] module"), so it renders the whole chart
  // experience itself (SVG, break bands, annotation groups, accessible data
  // table, textual description) via the SAME shared pure functions
  // `IndicatorChart.astro` calls — never a second, independently-authored
  // renderer. `IndicatorChart.astro` remains the workbench's/no-JS-gate's
  // static-only catalog entry; a page composing THIS component (slice 9a's
  // decision) gets an identical initial render (task 8.12's golden-parity
  // test) plus tooltips, keyboard navigation, range presets and
  // transformation toggles once JavaScript hydrates.
  //
  // indicator-page spec, P4 ("Series breaks are always visible and never
  // dismissible") + "A transformation never splices across a break":
  // `visibleBreaks` below is the ONLY gate on which bands render, and it is
  // driven exclusively by whether a break's period falls inside the
  // CURRENTLY VISIBLE window (after range + transform) — never by the
  // active toggle state itself. Changing range or transformation can never
  // drop a break that genuinely falls inside the new visible window.
  import {
    NARROW_VIEWPORT_QUERY,
    projectPoints,
    wideDimensions,
    type ChartDimensions,
    type ChartPoint,
    type ObservationStatus,
  } from "../lib/chart/geometry";
  import { narrowChartVariant, renderChartSVG } from "../lib/chart/svg";
  import { describeSeries } from "../lib/chart/description";
  import { periodFromCalendarDate, periodOrdinalIndex, type Frequency } from "../lib/chart/periods";
  import { computeIntraPeriodRate, computeYoY } from "../lib/transform/yoy";
  import { computePerCapita } from "../lib/transform/perCapita";
  import {
    CUSTOM_RANGE,
    GOVERNMENT_RANGE,
    availablePresets,
    periodStartCalendarDate,
    resolveCustomRange,
    sliceCustomRange,
    sliceRange,
    type CustomRangeRejection,
    type RangePreset,
    type RangeSelection,
  } from "../lib/transform/sliceRange";
  import {
    availableGovernmentTerms,
    deriveGovernmentTerms,
    type GovernmentTerm,
  } from "../lib/transform/governmentTerms";
  import { nearestPointIndexForX } from "../lib/chart/hitTest";
  import { nextFocusIndex, type ChartNavKey } from "../lib/chart/focusIndex";
  import { reduceTransform, type TransformKind, type ToggleableTransform } from "../lib/chart/toggleState";
  import { decodeChartState, encodeChartState } from "../lib/chart/permalink";
  import { toChartPoints } from "../lib/chart/chartPoints";
  import { visibleTransformControls, type IndicatorTransformConfig } from "../lib/chart/applicability";
  // The SAME formatter the static half uses. This component re-renders the
  // table and the point announcements client-side, so anything it formatted
  // differently would flip under the reader the moment the island hydrated.
  import { formatNumber } from "../lib/format/number";
  // Both period registers, for the same reason: this component re-renders
  // the table, the break list, the range disclosures and the point
  // announcements client-side, so a period it spelled differently from the
  // static half would flip under the reader the moment the island hydrated.
  // Which register goes where is `lib/format/period.ts`'s rule — compact in
  // the data table (a column), prose everywhere the label sits in a sentence
  // or is spoken aloud.
  import { formatPeriodCompact, formatPeriodProse } from "../lib/format/period";
  import { es } from "../i18n/es";
  import { onMount } from "svelte";

  export type AnnotationGroup = "governments" | "exogenous" | "milestones";

  interface IndicatorChartBreak {
    key: string;
    date: string;
    kind: string;
    noteMd: string;
    sourceUrl?: string | null;
  }

  interface IndicatorChartAnnotation {
    id: string;
    group: AnnotationGroup;
    name: string;
    dateStart: string;
    dateEnd?: string | null;
    href?: string | null;
  }

  interface Props {
    slug: string;
    name: string;
    unit: string;
    frequency: Frequency;
    points: ChartPoint[];
    breaks?: IndicatorChartBreak[];
    annotations?: IndicatorChartAnnotation[];
    decimals?: number;
    transforms: IndicatorTransformConfig;
    /** Population observations, same `{period,value,status}` shape as
     * `points` — only meaningful (and only need be supplied) when
     * `transforms.perCapita` is true. */
    population?: ChartPoint[];
    idSuffix?: string;
    /** Heading level for the break list's "Rupturas de la serie" heading.
     *
     * A component cannot know its own depth in the document, and this one is
     * rendered in two genuinely different places. In the workbench it sits
     * under an `<h3>` component-demo heading, so `<h4>` is right — hence the
     * default. On an indicator page the chart section carries no heading of
     * its own (it is labelled with `aria-label`), so this heading follows the
     * `<h1>` page title directly and must be `<h2>`; `IndicatorPage.astro`
     * passes that.
     *
     * Hard-coding `<h4>` produced a real axe `heading-order` violation on
     * every page whose series resolves a break (h1 → h4 skips two levels). It
     * went unnoticed for as long as it did only because the export fixture
     * carried no breaks at all, so the block never rendered and the gate had
     * nothing to judge. */
    breaksHeadingLevel?: 2 | 3 | 4;
    /** Explicit id overrides, bypassing the slug-derived defaults —
     * exists so a test can feed this component the exact same ids
     * `svg.test.ts`'s golden fixture was generated with (task 8.12's
     * island-parity proof), not a production concern. */
    titleId?: string;
    descriptionId?: string;
    tableId?: string;
  }

  let {
    slug,
    name,
    unit,
    frequency,
    points,
    breaks = [],
    annotations = [],
    decimals = 1,
    transforms,
    population = [],
    idSuffix,
    breaksHeadingLevel = 4,
    titleId: titleIdOverride,
    descriptionId: descriptionIdOverride,
    tableId: tableIdOverride,
  }: Props = $props();

  const idBase = $derived(idSuffix ? `${slug}-${idSuffix}` : slug);
  const titleId = $derived(titleIdOverride ?? `chart-title-${idBase}`);
  /** The narrow drawing's own `<title>` id. Both variants are in the
   * document at once (CSS shows one), and two nodes sharing an id is a
   * duplicate whether or not one of them is displayed. */
  const narrowTitleId = $derived(`${titleId}-narrow`);
  const descriptionId = $derived(descriptionIdOverride ?? `chart-description-${idBase}`);
  const tableId = $derived(tableIdOverride ?? `chart-table-${idBase}`);

  // ---- Toggle state (client-only; initialized from the permalink onMount,
  // never during SSR — `onMount`/`$effect` are documented Svelte no-ops on
  // the server, so the FIRST render — the one task 8.12 golden-tests — is
  // always the default "raw" / "full" view). ----
  let activeTransform: TransformKind = $state("raw");
  let activeRange: RangeSelection = $state("full");
  // ---- Custom range ("personalizado": series-transformations spec, "Range
  // presets"; verify-report WARNING-5). Four pieces of state, deliberately
  // separate:
  //
  //   `customDraft*`  what the reader has typed into the two date inputs
  //                   (ISO calendar dates — the only vocabulary a native
  //                   `<input type="date">` speaks), NOT yet applied.
  //   `customRange`   the committed, already-resolved-and-clamped bounds, as
  //                   PERIOD labels. Non-null only after a range was
  //                   accepted.
  //   `customClamped` whether that acceptance narrowed what was asked for,
  //                   so the UI can disclose it.
  //   `customError`   why the last attempt was refused, or null.
  //
  // Keeping draft and committed state apart is what makes a refusal
  // non-destructive: a rejected range leaves the chart exactly as it was,
  // with the reader's own input still in the fields to correct.
  let customDraftFrom = $state("");
  let customDraftTo = $state("");
  let customRange: { from: string; to: string } | null = $state(null);
  let customClamped = $state(false);
  let customError: CustomRangeRejection | null = $state(null);
  // ---- Government range (indicator-page spec, "Annotation layers per PRD
  // §6.1.1(a)"). ONE piece of state, unlike the custom range's four: the
  // selected government's editorial event id, or "" for no government filter
  // (the `<select>`'s neutral option). There is no draft/committed split
  // because there is nothing to refuse — the options offered are exactly the
  // terms that genuinely narrow this series, computed before the control is
  // rendered (`availableGovernmentTerms`), so a selection cannot fail the way
  // a typed range can.
  let governmentSelection = $state("");
  // The picker is rendered only once this flips (see the `onMount` below and
  // `chart-no-js.spec.ts` for the full rationale): with JavaScript disabled a
  // free-form range cannot work at all on a statically built page, so it must
  // be ABSENT rather than present-but-dead — the same discipline the spec
  // states for a preset that cannot apply ("absent, not disabled").
  let hydrated = $state(false);
  let openGroups: Record<AnnotationGroup, boolean> = $state({
    governments: false,
    exogenous: false,
    milestones: true,
  });
  let hoverIndex: number | null = $state(null);
  let focusIndex: number | null = $state(null);
  let rovingIndex = $state(0);

  const activeIndex = $derived(focusIndex ?? hoverIndex);

  const rawStatusByPeriod = $derived(new Map<string, ObservationStatus>(points.map((p) => [p.period, p.status])));

  const yoyPoints = $derived(computeYoY(points, frequency));
  const qoqPoints = $derived(computeIntraPeriodRate(points, frequency));
  const perCapitaResult = $derived(population.length > 0 ? computePerCapita(points, population) : { coverage: null, points: [] });

  const visible = $derived(
    visibleTransformControls({
      config: transforms,
      yoyPointCount: yoyPoints.length,
      qoqPointCount: qoqPoints.length,
      perCapitaCoverage: perCapitaResult.coverage,
    }),
  );

  // A transform the reader previously selected (via permalink) that is no
  // longer offered (e.g. this series' own config/data) silently falls back
  // to raw rather than rendering a control that "does not exist"
  // (series-transformations spec).
  const effectiveTransform = $derived(
    activeTransform === "raw" || (visible as string[]).includes(activeTransform) ? activeTransform : "raw",
  );

  const viewPoints = $derived.by((): ChartPoint[] => {
    switch (effectiveTransform) {
      case "yoy":
        return toChartPoints(yoyPoints, rawStatusByPeriod);
      case "qoq":
        return toChartPoints(qoqPoints, rawStatusByPeriod);
      case "perCapita":
        return toChartPoints(perCapitaResult.points, rawStatusByPeriod);
      default:
        return points;
    }
  });

  const firstPeriod = $derived(points[0]?.period ?? "");
  const latestPeriod = $derived(points[points.length - 1]?.period ?? "");
  const presets = $derived(availablePresets(frequency, firstPeriod, latestPeriod));

  // The RAW series' own observation labels. `resolveCustomRange` is fed these
  // and never `viewPoints`: a custom range is a statement about the series'
  // history, so toggling a transformation must not silently redefine — or
  // invalidate — a range the reader already committed. This is exactly how
  // the fixed presets already behave.
  const rawPeriods = $derived(points.map((p) => p.period));
  /** Offered whenever there is more than one observation to narrow between —
   * the custom range's own version of the presets' availability rule. */
  const customRangeAvailable = $derived(points.length > 1);
  const customActive = $derived(activeRange === CUSTOM_RANGE && customRange !== null);

  // ---- Government terms ----
  //
  // Derived from the SAME `annotations` prop the chips below already render,
  // so the control and the chips can never disagree about which governments
  // this artifact carries. `deriveGovernmentTerms` owns the succession
  // inference and marks it (see that module's header); this component owns
  // only the disclosure of it.
  //
  // Fed `rawPeriods`, never `viewPoints`: a government's term is a statement
  // about the series' history, so toggling a transformation must not silently
  // redefine which governments overlap it — exactly as for the fixed presets
  // and the custom range.
  const governmentTerms = $derived(deriveGovernmentTerms(annotations, frequency));
  const availableTerms = $derived(availableGovernmentTerms(governmentTerms, rawPeriods, frequency));
  /** The term currently in force, or null. A selection that names a term this
   * series does not offer (an old permalink, a hand-edited URL, an artifact
   * whose registry changed under a cached page) resolves to null and the view
   * falls back to the full range — the same silent, truthful degradation an
   * unrecognised preset already gets. */
  const activeTerm = $derived<GovernmentTerm | null>(
    activeRange === GOVERNMENT_RANGE && governmentSelection
      ? availableTerms.find((term) => term.id === governmentSelection) ?? null
      : null,
  );
  const governmentActive = $derived(activeTerm !== null);
  /** The preset in force when no custom range is. A preset that does not
   * apply to this series' span (e.g. reached via an old permalink) falls back
   * to "full", unchanged from before this slice. */
  const effectivePreset = $derived<RangePreset>(
    activeRange !== CUSTOM_RANGE && (presets as string[]).includes(activeRange) ? (activeRange as RangePreset) : "full",
  );
  const effectiveRange = $derived<RangeSelection>(
    customActive ? CUSTOM_RANGE : governmentActive ? GOVERNMENT_RANGE : effectivePreset,
  );
  /** Every range selection this component is currently offering — the gate
   * `decodeChartState` applies to the permalink, so a URL can never select a
   * range no control on the page could have produced. `GOVERNMENT_RANGE` joins
   * it only when at least one term genuinely narrows this series, which is
   * what makes `?range=government` inert on a page carrying no such control. */
  const rangeSelections = $derived<RangeSelection[]>([
    ...presets,
    ...(customRangeAvailable ? [CUSTOM_RANGE] : []),
    ...(availableTerms.length > 0 ? [GOVERNMENT_RANGE] : []),
  ]);

  const rangedPoints = $derived(
    customActive && customRange
      ? sliceCustomRange(viewPoints, frequency, customRange.from, customRange.to)
      : activeTerm
        ? // The SAME slicing primitive the custom range uses, deliberately:
          // a government term is a `[from, to]` window like any other once it
          // has been derived, and a second slicing path would be a second
          // place for the two to disagree. An OPEN term (the sitting
          // government) is sliced to the series' own last period — the honest
          // reading of "no end recorded", and not an invented end date.
          sliceCustomRange(viewPoints, frequency, activeTerm.startPeriod, activeTerm.endPeriod ?? latestPeriod)
        : sliceRange(viewPoints, frequency, effectivePreset),
  );
  const rangedPeriods = $derived(rangedPoints.map((p) => p.period));

  // The two date inputs' own bounds, in their own vocabulary. Advisory, not
  // load-bearing: `min`/`max` stop the native picker from OFFERING a date
  // outside the series, but a typed or pasted value can still exceed them and
  // a permalink bypasses them entirely — `resolveCustomRange` is what
  // actually decides, in every one of those paths.
  const customMinDate = $derived(firstPeriod ? periodStartCalendarDate(firstPeriod, frequency) : undefined);
  const customMaxDate = $derived(latestPeriod ? periodStartCalendarDate(latestPeriod, frequency) : undefined);

  const customFromId = $derived(`custom-range-from-${idBase}`);
  const customToId = $derived(`custom-range-to-${idBase}`);
  const customStatusId = $derived(`custom-range-status-${idBase}`);
  const governmentSelectId = $derived(`government-range-${idBase}`);
  const governmentStatusId = $derived(`government-range-status-${idBase}`);

  /** One option's visible label: the president's name (registry data, printed
   * verbatim — never translated, never reworded) plus the term's own years, so
   * a reader who does not remember when Aznar governed can still choose.
   *
   * The years describe the TERM, not the slice: Aznar's option reads
   * "1996–2004" even on a series beginning in 2002, because the option names a
   * government and the status line below names what was actually rendered. */
  function governmentOptionLabel(term: GovernmentTerm): string {
    const startYear = term.startDate.slice(0, 4);
    // An open term has no closing year to print, and printing the current one
    // would assert an end that has not happened.
    if (term.endPeriod === null) return es.chart.government.openOptionLabel(term.name, startYear);
    // Every period label — "2018-Q1", "2018-05", "2018" — begins with its
    // four-digit year, which is the only part this label needs.
    return es.chart.government.optionLabel(term.name, startYear, term.endPeriod.slice(0, 4));
  }

  /** The government control's own status line: what is on screen, plus — when
   * the term's end was INFERRED rather than configured — the sentence saying
   * so. The disclosure is not optional decoration: no government in the
   * editorial registry carries an end date, so without it every closed term
   * would present a derived boundary as a recorded one. */
  const governmentStatus = $derived.by((): string => {
    if (!activeTerm || rangedPoints.length === 0) return "";
    // Prose register, like every other sentence in this live region.
    const from = formatPeriodProse(rangedPoints[0].period);
    const to = formatPeriodProse(rangedPoints[rangedPoints.length - 1].period);
    const applied = es.chart.government.appliedNote(activeTerm.name, from, to);
    const endNote =
      activeTerm.endKind === "succession"
        ? es.chart.government.derivedEndNote
        : activeTerm.endKind === "open"
          ? es.chart.government.openEndNote
          : // "configured": the registry states the end, so there is no
            // inference to disclose. Unreachable today (no entry carries
            // `date_end`) and deliberately silent rather than defensive.
            "";
    return endNote ? `${applied} ${endNote}` : applied;
  });

  /** The single line of text under the picker: the refusal reason, or the
   * disclosure of what was actually applied. Reports the span REALLY
   * rendered (the first and last plotted period), not the bounds that were
   * typed — after clamping, and under an active transformation, those are not
   * always the same thing, and the honest statement is the one about what is
   * on screen. */
  const customRangeStatus = $derived.by((): string => {
    if (customError !== null) return es.chart.customRange.error[customError];
    if (!customActive || rangedPoints.length === 0) return "";
    // Prose register: this line is a sentence in a polite live region, read
    // aloud as often as it is read on screen. The bounds it names are for the
    // reader only — the ones written to the permalink below stay canonical.
    const from = formatPeriodProse(rangedPoints[0].period);
    const to = formatPeriodProse(rangedPoints[rangedPoints.length - 1].period);
    return customClamped ? es.chart.customRange.clampedNote(from, to) : es.chart.customRange.appliedNote(from, to);
  });

  const viewUnit = $derived(
    effectiveTransform === "raw"
      ? unit
      : effectiveTransform === "perCapita"
        ? es.chart.transforms.perCapitaUnit(unit)
        : es.chart.transforms.rateUnit,
  );
  const viewDecimals = $derived(effectiveTransform === "raw" || effectiveTransform === "perCapita" ? decimals : 1);
  const transformLabel = $derived(
    effectiveTransform === "raw"
      ? es.chart.transforms.raw
      : effectiveTransform === "yoy"
        ? es.chart.transforms.yoy
        : effectiveTransform === "qoq"
          ? es.chart.transforms.qoq(frequency)
          : es.chart.transforms.perCapita,
  );

  // indicator-page spec, P4: breaks are filtered ONLY by whether they fall
  // inside the currently-visible period window — never by the active
  // transform/range choice itself. Always passed to the SAME `renderChartSVG`
  // the static component calls, so a break inside the window is baked into
  // the SVG string exactly the same way regardless of who renders it.
  const visibleBreaks = $derived(
    breaks.filter((b) => {
      if (rangedPeriods.length === 0) return false;
      const target = periodOrdinalIndex(periodFromCalendarDate(b.date, frequency), frequency);
      const firstOrd = periodOrdinalIndex(rangedPeriods[0], frequency);
      const lastOrd = periodOrdinalIndex(rangedPeriods[rangedPeriods.length - 1], frequency);
      return target >= firstOrd && target <= lastOrd;
    }),
  );

  const chartInput = $derived({
    points: rangedPoints,
    breaks: visibleBreaks.map((b) => ({ key: b.key, date: b.date })),
    frequency,
    decimals: viewDecimals,
    unit: viewUnit,
    descriptionId,
    tableId,
  });

  const svgMarkup = $derived(renderChartSVG({ ...chartInput, titleId }));

  /** The SAME renderer and the SAME series in a phone-shaped box. A real
   * `/indicador/{slug}` page composes THIS component, not
   * `IndicatorChart.astro`, so this is the drawing a reader on a phone
   * actually receives — including a reader with JavaScript disabled, who
   * gets this server-rendered markup and nothing else. See
   * `lib/chart/geometry.ts` for why the wide box cannot be made legible by
   * enlarging its type. */
  const narrowVariant = $derived(narrowChartVariant(rangedPoints, viewDecimals));
  const narrowSvgMarkup = $derived(
    renderChartSVG({ ...chartInput, titleId: narrowTitleId, ...narrowVariant }),
  );

  /** Which drawing the reader is currently looking at.
   *
   * CSS decides which SVG is VISIBLE with zero JavaScript, and that is the
   * whole no-JS story. This flag exists only for the interactive layer on
   * top: the hover overlay, the roving-tabindex point buttons and the
   * tooltip are HTML positioned in percentages of the figure box, and those
   * percentages are computed from the plot area's margins — which differ
   * between the two boxes. Reading the SAME `NARROW_VIEWPORT_QUERY` the
   * markup toggles on is what keeps the hit targets on top of the points
   * rather than beside them.
   *
   * `false` during SSR (there is no viewport to ask), corrected on mount.
   * Nothing shifts when it flips: the interactive layer is invisible and
   * inert until hydration anyway, so its coordinates were never observable
   * before this runs. */
  let narrowViewport = $state(false);
  onMount(() => {
    const query = window.matchMedia(NARROW_VIEWPORT_QUERY);
    const sync = () => {
      narrowViewport = query.matches;
    };
    sync();
    query.addEventListener("change", sync);
    return () => query.removeEventListener("change", sync);
  });

  /** The wide box for THIS series, derived exactly as `renderChartSVG`
   * derives it when no `dims` is passed — same pure function, same inputs,
   * same result. It is spelled out here because the hit targets below are
   * positioned from the plot area's margins, and those margins now depend on
   * the series' own labels: a constant here would put the buttons beside the
   * points on any page whose y axis is wider than the old fixed gutter. */
  const wideDims: ChartDimensions = $derived(
    wideDimensions(rangedPeriods, rangedPoints, viewDecimals),
  );

  const activeDims: ChartDimensions = $derived(narrowViewport ? narrowVariant.dims : wideDims);

  const description = $derived(describeSeries({ points: rangedPoints, unit: viewUnit, decimals: viewDecimals }));

  const projected = $derived(projectPoints(rangedPoints, activeDims));
  const interactivePoints = $derived(
    projected
      .map((p, i) => ({ ...p, i }))
      .filter((p): p is (typeof projected)[number] & { i: number; y: number } => p.value !== null && p.y !== null),
  );

  function xPercent(x: number): number {
    return (x / activeDims.width) * 100;
  }
  function yPercent(y: number): number {
    return (y / activeDims.height) * 100;
  }

  function pointId(i: number): string {
    return `chart-point-${idBase}-${i}`;
  }

  function pointLabel(p: ChartPoint): string {
    // Same formatting as the static table below and as the page header:
    // a value a reader hears announced must be the value they can read.
    //
    // The PROSE register for the period, deliberately, even though the table
    // two elements down uses the compact one: this string is an
    // `aria-label` spoken by a screen reader and a tooltip floating over the
    // plot area. Neither is a column, so neither has any width to save — and
    // "jun 2026" read aloud is a saving that costs the listener the word.
    return es.chart.pointAnnouncement(
      formatPeriodProse(p.period),
      formatNumber(p.value as number, viewDecimals),
      viewUnit,
      p.status,
    );
  }

  function onPointFocus(i: number) {
    focusIndex = i;
    rovingIndex = i;
  }
  function onPointBlur() {
    focusIndex = null;
  }
  function onPointEnter(i: number) {
    hoverIndex = i;
  }
  function onPointLeave() {
    hoverIndex = null;
  }
  function onPointKeydown(e: KeyboardEvent, i: number) {
    const key = e.key;
    if (key !== "ArrowRight" && key !== "ArrowLeft" && key !== "Home" && key !== "End") return;
    e.preventDefault();
    const next = nextFocusIndex(i, key as ChartNavKey, interactivePoints.length);
    if (next === i) return;
    rovingIndex = next;
    document.getElementById(pointId(next))?.focus();
  }

  /** Pointer-move hit-testing over the WHOLE plot area (one shared overlay,
   * never per-point discrete hit boxes) — `nearestPointIndexForX` translates
   * the pointer's position (scaled from client to viewBox coordinates via
   * the overlay's own real rendered size) to the nearest interactive point. */
  function onOverlayPointerMove(e: PointerEvent) {
    const target = e.currentTarget as HTMLElement;
    const rect = target.getBoundingClientRect();
    const localX = ((e.clientX - rect.left) / rect.width) * activeDims.width;
    const nearest = nearestPointIndexForX(localX, interactivePoints.length, activeDims);
    hoverIndex = nearest === -1 ? null : nearest;
  }
  function onOverlayPointerLeave() {
    hoverIndex = null;
  }

  function selectTransform(kind: ToggleableTransform) {
    activeTransform = reduceTransform(effectiveTransform, kind);
  }
  function selectRange(preset: RangePreset) {
    activeRange = preset;
    // A fixed preset supersedes the custom range, so a stale refusal message
    // must not outlive the attempt that produced it. The committed bounds
    // themselves are kept, so the reader can return to their own range by
    // pressing the commit control again without re-typing it.
    customError = null;
    // The government select, however, is CLEARED rather than merely
    // deactivated: it is a control that displays its own state, so leaving a
    // president's name showing while the chart draws a preset would state
    // something false about the view. (The date inputs above are draft fields,
    // not a statement about what is rendered, which is why they are kept.)
    governmentSelection = "";
  }

  /** Selects one government's derived term, or — for the neutral option —
   * clears the filter back to the full series. "Todos los gobiernos" means
   * every period, so the full range is what it selects. */
  function selectGovernment(id: string) {
    customError = null;
    governmentSelection = id;
    activeRange = id === "" ? "full" : GOVERNMENT_RANGE;
  }

  /** Commits whatever is currently in the two date inputs, via the single
   * function that judges a reader-entered range (`resolveCustomRange`).
   *
   * The date inputs speak calendar dates and the chart speaks period labels,
   * so each bound is mapped through `periodFromCalendarDate` first — the same
   * function that already positions break bands and annotations against this
   * axis, so a custom range lands on the period axis exactly as everything
   * else does. A refusal returns without touching the view: nothing is
   * clamped into a different range, nothing is emptied, and the reason is
   * shown. */
  function commitCustomRange() {
    const from = customDraftFrom ? periodFromCalendarDate(customDraftFrom, frequency) : "";
    const to = customDraftTo ? periodFromCalendarDate(customDraftTo, frequency) : "";
    const resolution = resolveCustomRange(rawPeriods, frequency, from, to);
    if (resolution.status === "rejected") {
      customError = resolution.reason;
      return;
    }
    customError = null;
    customRange = { from: resolution.from, to: resolution.to };
    customClamped = resolution.clamped;
    activeRange = CUSTOM_RANGE;
    // Same reason as in `selectRange`: the government select states what the
    // chart is showing, and the chart is no longer showing a government's term.
    governmentSelection = "";
  }
  function toggleAnnotationGroup(group: AnnotationGroup) {
    openGroups = { ...openGroups, [group]: !openGroups[group] };
  }

  function annotationDateLabel(ann: IndicatorChartAnnotation): string {
    const startYear = ann.dateStart.slice(0, 4);
    if (!ann.dateEnd) return startYear;
    const endYear = ann.dateEnd.slice(0, 4);
    return startYear === endYear ? startYear : `${startYear}–${endYear}`;
  }

  const GROUPS: AnnotationGroup[] = ["governments", "exogenous", "milestones"];
  const groupedAnnotations = $derived(
    GROUPS.map((group) => ({
      group,
      label: es.chart.annotationGroupLabel[group],
      entries: annotations.filter((a) => a.group === group),
    })).filter((g) => g.entries.length > 0),
  );

  // `displayPeriod` is named for what it is and is now genuinely one: the
  // break's calendar date is snapped onto the series' own period axis
  // (canonical, because that is what the snap needs) and only then rendered.
  // Prose register — "Ruptura: junio de 2021" is a phrase in a band, not a
  // cell in a column.
  const breakDisplay = $derived(
    visibleBreaks.map((b) => ({ ...b, displayPeriod: formatPeriodProse(periodFromCalendarDate(b.date, frequency)) })),
  );

  // series-transformations spec, "Transformations recompute in the browser
  // with no network request": neither this initialization nor any toggle
  // handler above issues `fetch`/`XMLHttpRequest` — every recomputation
  // calls only the shared, pure `lib/chart`/`lib/transform` modules already
  // delivered with the page. `history.replaceState` is a same-document URL
  // update, not a network request.
  onMount(() => {
    hydrated = true;
    const decoded = decodeChartState(window.location.search, rangeSelections, ["raw", ...visible]);
    activeTransform = decoded.transform;

    if (decoded.range === CUSTOM_RANGE && decoded.custom) {
      // A hand-edited permalink is untrusted input: the bounds arrive
      // verbatim from the query string and are judged here, against this
      // series, exactly like a range typed into the inputs. On refusal the
      // range degrades SILENTLY to the full series — deliberately, and
      // unlike the UI path: there is no reader gesture to attach an error
      // to, an error about a URL the reader did not compose would be noise,
      // and the full series is a truthful view rather than a broken or empty
      // one. This is the same fallback `decodeChartState` already applies to
      // an unrecognised preset.
      const resolution = resolveCustomRange(rawPeriods, frequency, decoded.custom.from, decoded.custom.to);
      if (resolution.status === "ok") {
        customRange = { from: resolution.from, to: resolution.to };
        customClamped = resolution.clamped;
        // Prefill the inputs from the range actually applied, so the reader
        // arrives at a picker that agrees with the chart in front of them
        // instead of two blank fields under an already-narrowed view.
        customDraftFrom = periodStartCalendarDate(resolution.from, frequency);
        customDraftTo = periodStartCalendarDate(resolution.to, frequency);
        activeRange = CUSTOM_RANGE;
      } else {
        activeRange = "full";
      }
    } else if (decoded.range === GOVERNMENT_RANGE && decoded.government) {
      // The id arrives verbatim from the query string and is judged HERE,
      // against this series' own available terms — a government that does not
      // overlap this series (or one the registry no longer carries) is not
      // selectable by URL any more than it is by control. Degrades silently to
      // the full series, for the same reason the custom range does: there is no
      // reader gesture to attach an error to, and the full series is a truthful
      // view rather than an empty one.
      const term = availableTerms.find((t) => t.id === decoded.government);
      if (term) {
        governmentSelection = term.id;
        activeRange = GOVERNMENT_RANGE;
      } else {
        activeRange = "full";
      }
    } else {
      activeRange = decoded.range;
    }
  });

  $effect(() => {
    const search = encodeChartState({
      range: effectiveRange,
      transform: effectiveTransform,
      // The CLAMPED bounds, not the ones typed: a permalink must reproduce
      // the view it was copied from, and the clamped span is that view. It
      // reloads unclamped (nothing was narrowed the second time) and so
      // carries no clamping disclosure — correct, because the disclosure
      // answers "why is this narrower than what you asked for", a question
      // only the reader who typed it ever asked.
      custom: customActive ? customRange : null,
      // The government's ID, never its derived window: the window is an
      // inference this product re-derives on every load, so a link that froze
      // today's inference would quietly stop agreeing with the registry the
      // day the registry gains a confirmed end date. See
      // `lib/chart/permalink.ts`.
      government: governmentActive ? governmentSelection : null,
    });
    const url = `${window.location.pathname}${search}${window.location.hash}`;
    window.history.replaceState(null, "", url);
  });
</script>

<div class="indicator-chart-island" data-testid="indicator-chart-island" data-slug={slug}>
  <!--
    The box's SHAPE is chosen by CSS, not by the `narrowViewport` flag in
    the script above, and that ordering is deliberate: the aspect ratio is
    what reserves the chart's space in the layout, so making it depend on
    hydration would move everything below the chart the moment the island
    woke up — and would leave a no-JavaScript reader with a box that never
    matched its drawing at all. See this component's `<style>` block.
  -->
  <div class="indicator-chart-island__figure relative w-full">
    <!-- Two drawings, one shown — same `md` breakpoint, and same
         reasoning, as `IndicatorChart.astro`. -->
    <div class="indicator-chart-island__figure--narrow pointer-events-none absolute inset-0 md:hidden">
      {@html narrowSvgMarkup}
    </div>
    <div class="indicator-chart-island__figure--wide pointer-events-none absolute inset-0 hidden md:block">
      {@html svgMarkup}
    </div>
    <div
      class="absolute inset-0"
      role="group"
      aria-label={es.chart.controls.pointsGroupLabel}
      onpointermove={onOverlayPointerMove}
      onpointerleave={onOverlayPointerLeave}
    >
      {#each interactivePoints as p (p.period)}
        <button
          id={pointId(p.i)}
          type="button"
          class="absolute -translate-x-1/2 -translate-y-1/2 min-h-11 min-w-11 rounded-pill bg-transparent focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          style={`left: ${xPercent(p.x)}%; top: ${yPercent(p.y)}%;`}
          tabindex={rovingIndex === p.i ? 0 : -1}
          aria-label={pointLabel(p)}
          data-testid="chart-island-point"
          onfocus={() => onPointFocus(p.i)}
          onblur={onPointBlur}
          onmouseenter={() => onPointEnter(p.i)}
          onmouseleave={onPointLeave}
          onkeydown={(e) => onPointKeydown(e, p.i)}
        ></button>
      {/each}
    </div>
    {#if activeIndex !== null && interactivePoints[activeIndex]}
      <div
        class="chart-island-tooltip pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-control bg-tooltip-bg px-2 py-1 text-caption text-tooltip-ink shadow-popover"
        style={`left: ${xPercent(interactivePoints[activeIndex].x)}%; top: ${yPercent(interactivePoints[activeIndex].y) - 4}%;`}
        role="tooltip"
        data-testid="chart-island-tooltip"
      >
        {pointLabel(interactivePoints[activeIndex])}
      </div>
    {/if}
  </div>

  <p id={descriptionId} class="mt-3 text-body text-ink" data-testid="chart-description">{description}</p>

  {#if effectiveTransform !== "raw"}
    <p class="mt-1 text-caption text-ink-muted" data-testid="chart-derivation-note">
      {es.chart.transforms.derivationNote(transformLabel)}
    </p>
  {/if}

  {#if effectiveTransform === "perCapita" && perCapitaResult.coverage}
    <p class="mt-1 text-caption text-ink-muted" data-testid="chart-percapita-disclosure">
      {es.chart.perCapita.coverageDisclosure(
        formatPeriodProse(perCapitaResult.coverage.from),
        formatPeriodProse(perCapitaResult.coverage.to),
      )}
    </p>
  {/if}

  <ul class="chart-legend mt-2 flex flex-wrap gap-4 text-caption text-ink-muted" data-testid="chart-legend">
    <li class="flex items-center gap-1.5">
      <span aria-hidden="true" class="inline-block h-2.5 w-2.5 rounded-pill bg-accent"></span>
      {es.chart.statusLabel.D}
    </li>
    <li class="flex items-center gap-1.5">
      <span aria-hidden="true" class="inline-block h-2.5 w-2.5 rotate-45 border border-dotted border-provisional bg-provisional"></span>
      {es.chart.statusLabel.P}
    </li>
  </ul>

  <div class="chart-island-controls mt-4 flex flex-col gap-3">
    {#if presets.length > 1}
      <div role="group" aria-label={es.chart.controls.rangeGroupLabel} class="flex flex-wrap gap-2" data-testid="chart-range-controls">
        {#each presets as preset (preset)}
          <button
            type="button"
            class="min-h-11 min-w-11 rounded-control border px-3 py-2 text-body focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            class:border-accent={effectiveRange === preset}
            class:text-accent={effectiveRange === preset}
            class:border-ink={effectiveRange !== preset}
            class:text-ink={effectiveRange !== preset}
            aria-pressed={effectiveRange === preset}
            data-testid={`range-preset-${preset}`}
            onclick={() => selectRange(preset)}
          >
            {es.chart.range[preset]}
          </button>
        {/each}
      </div>
    {/if}

    <!-- The spec's sixth range entry, "personalizado" (verify-report
         WARNING-5). Rendered only once hydrated: a free-form range needs
         JavaScript that these statically built pages cannot provide any other
         way, and a control that does nothing must be absent rather than
         disabled-looking (the spec's own rule for a preset that cannot
         apply). `chart-no-js.spec.ts` gates that absence.
         Tab order is the reading order — presets, then "Desde", "Hasta" and
         the commit control, then the transformation toggles — with no focus
         management of any kind, so nothing here can trap the keyboard. -->
    {#if hydrated && customRangeAvailable}
      <div
        role="group"
        aria-label={es.chart.customRange.groupLabel}
        class="flex flex-wrap items-end gap-2"
        data-testid="chart-custom-range"
      >
        <div class="flex flex-col gap-1">
          <label class="text-caption text-ink-muted" for={customFromId} data-testid="custom-range-from-label">
            {es.chart.customRange.fromLabel}
          </label>
          <input
            id={customFromId}
            type="date"
            class="chart-custom-range__input min-h-11 rounded-control border border-ink bg-surface px-3 py-2 text-body text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            min={customMinDate}
            max={customMaxDate}
            aria-describedby={customStatusId}
            aria-invalid={customError !== null}
            data-testid="custom-range-from"
            bind:value={customDraftFrom}
          />
        </div>
        <div class="flex flex-col gap-1">
          <label class="text-caption text-ink-muted" for={customToId} data-testid="custom-range-to-label">
            {es.chart.customRange.toLabel}
          </label>
          <input
            id={customToId}
            type="date"
            class="chart-custom-range__input min-h-11 rounded-control border border-ink bg-surface px-3 py-2 text-body text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            min={customMinDate}
            max={customMaxDate}
            aria-describedby={customStatusId}
            aria-invalid={customError !== null}
            data-testid="custom-range-to"
            bind:value={customDraftTo}
          />
        </div>
        <button
          type="button"
          class="min-h-11 min-w-11 rounded-control border px-3 py-2 text-body focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          class:border-accent={customActive}
          class:text-accent={customActive}
          class:border-ink={!customActive}
          class:text-ink={!customActive}
          aria-pressed={customActive}
          data-testid="custom-range-apply"
          onclick={commitCustomRange}
        >
          {es.chart.customRange.applyLabel}
        </button>
      </div>
      <!-- One polite live region for both outcomes (refusal reason and
           applied/clamped disclosure), so a screen-reader reader learns what
           the chart now shows without re-reading it, and so the message is
           never announced as an interruption. Also the inputs' own
           `aria-describedby` target, which is what ties a refusal to the
           fields that caused it. -->
      <p
        id={customStatusId}
        class="text-caption text-ink-muted"
        aria-live="polite"
        data-testid="custom-range-status"
      >
        {customRangeStatus}
      </p>
    {/if}

    <!-- The government range (indicator-page spec, "Annotation layers per PRD
         §6.1.1(a)"): the one range this product offers that a reader cannot
         express with the controls above, because its bounds are not on the
         page — they are in the editorial registry, and its end is derived
         from the succession.

         `hydrated &&`, for the same reason the custom picker carries it: a
         `<select>` that redraws a client-side chart cannot work on a
         statically built page with no JavaScript, and a control that cannot
         work must be ABSENT rather than rendered as though it works (the
         spec's own "absent, not disabled" discipline for a preset that
         cannot apply). `island-ssr.test.ts` and the no-JS browser context
         both gate that absence.

         `availableTerms.length > 0` is the second gate: a series overlapping
         no government — or only one whose term covers the whole of it —
         offers no control at all rather than an empty or decorative one
         (`availableGovernmentTerms`).

         A `<select>` rather than the button row the presets use: up to six
         options carrying full presidential names would occupy more of a
         375 px phone than the chart they filter, and the choice is a single
         mutually-exclusive pick from a closed list — which is what a native
         select is. It inherits keyboard support, screen-reader semantics and
         the platform's own picker for free. -->
    {#if hydrated && availableTerms.length > 0}
      <div class="flex flex-col gap-1" data-testid="chart-government-range">
        <label class="text-caption text-ink-muted" for={governmentSelectId} data-testid="government-select-label">
          {es.chart.government.selectLabel}
        </label>
        <select
          id={governmentSelectId}
          class="min-h-11 rounded-control border border-ink bg-surface px-3 py-2 text-body text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
          class:border-accent={governmentActive}
          class:text-accent={governmentActive}
          aria-describedby={governmentStatusId}
          data-testid="government-select"
          bind:value={governmentSelection}
          onchange={(e) => selectGovernment(e.currentTarget.value)}
        >
          <option value="">{es.chart.government.allOption}</option>
          {#each availableTerms as term (term.id)}
            <option value={term.id}>{governmentOptionLabel(term)}</option>
          {/each}
        </select>
      </div>
      <!-- One polite live region, exactly like the custom range's: it states
           the span actually rendered AND — because no government in the
           registry carries an end date — that the end of a closed term is
           derived from the next investiture rather than recorded. That
           sentence is the whole reason this line exists; a filter that
           silently presented an inferred boundary as a configured one would
           be the P4 failure this project's `date_status: unconfirmed`
           machinery exists to prevent. -->
      <p
        id={governmentStatusId}
        class="text-caption text-ink-muted"
        aria-live="polite"
        data-testid="government-range-status"
      >
        {governmentStatus}
      </p>
    {/if}

    {#if visible.length > 0}
      <div role="group" aria-label={es.chart.controls.transformGroupLabel} class="flex flex-wrap gap-2" data-testid="chart-transform-controls">
        {#each visible as kind (kind)}
          <button
            type="button"
            class="min-h-11 min-w-11 rounded-control border px-3 py-2 text-body focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            class:border-accent={effectiveTransform === kind}
            class:text-accent={effectiveTransform === kind}
            class:border-ink={effectiveTransform !== kind}
            class:text-ink={effectiveTransform !== kind}
            aria-pressed={effectiveTransform === kind}
            data-testid={`transform-toggle-${kind}`}
            onclick={() => selectTransform(kind)}
          >
            {kind === "yoy" ? es.chart.transforms.yoy : kind === "qoq" ? es.chart.transforms.qoq(frequency) : es.chart.transforms.perCapita}
          </button>
        {/each}
      </div>
    {/if}
  </div>

  {#if breakDisplay.length > 0}
    <div class="chart-breaks mt-4" data-testid="chart-breaks">
      <svelte:element this={`h${breaksHeadingLevel}`} class="mb-2 text-caption font-semibold text-ink-muted">
        {es.chart.breaksHeading}
      </svelte:element>
      <div class="flex flex-col gap-2">
        {#each breakDisplay as b (b.key)}
          <!-- This band is a hand-written COPY of `BreakBand.astro`'s markup:
               the island re-renders the break list rather than hydrating it
               (the Astro/Svelte boundary forces it), so the same classes and
               `data-testid`s are emitted twice, by two files. Any change here
               must be made there too — including the `<button>` trigger below
               (verify-report WARNING-7; see BreakBand.astro's header for why a
               button and not a focusable span).
               `test/design-system/break-band-parity.test.ts` fails loudly if
               the two ever diverge, and `src/styles/components.css` — not a
               scoped `<style>` — owns the rules they share. -->
          <div class="break-band relative inline-flex items-center gap-2 rounded-control border-l-4 border-ink bg-surface px-3 py-2 shadow-card">
            <svg aria-hidden="true" class="h-5 w-5 shrink-0 text-ink" viewBox="0 0 16 16" fill="none">
              <path d="M8 1v5.5L4 9l4 6 4-6-4-2.5V1" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" />
            </svg>
            <button
              type="button"
              aria-describedby={`break-tooltip-${idBase}-${b.key}`}
              class="break-band__trigger inline-flex min-h-11 min-w-11 items-center rounded-control px-1 text-body font-medium text-ink focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              {es.breakBand.rupturaLabel} {b.displayPeriod} <span class="ml-1 text-caption text-ink-muted">({b.kind})</span>
            </button>
            <span
              id={`break-tooltip-${idBase}-${b.key}`}
              role="tooltip"
              data-testid="break-band-tooltip"
              class="break-band__tooltip absolute left-0 top-full z-10 mt-2 w-72 rounded-control bg-tooltip-bg p-3 text-caption text-tooltip-ink shadow-popover"
            >
              {b.noteMd}
              <a href={b.sourceUrl ?? `#ruptura-${b.key}`} class="ml-1 inline-flex min-h-11 items-center underline">{es.breakBand.noteLinkLabel}</a>
            </span>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if groupedAnnotations.length > 0}
    <div class="chart-annotations mt-4 flex flex-col gap-3" data-testid="chart-annotations">
      {#each groupedAnnotations as { group, label, entries } (group)}
        <div class="annotation-group" data-testid={`annotation-group-${group}`}>
          <button
            type="button"
            class="inline-flex min-h-11 min-w-11 items-center gap-2 rounded-control border border-ink/15 px-3 py-2 text-body text-ink hover:border-accent focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            aria-expanded={openGroups[group]}
            aria-controls={`ann-content-${idBase}-${group}`}
            data-testid={`annotation-toggle-${group}`}
            onclick={() => toggleAnnotationGroup(group)}
          >
            {openGroups[group] ? es.chart.annotationToggleHide(label) : es.chart.annotationToggleShow(label)}
          </button>
          {#if openGroups[group]}
            <div id={`ann-content-${idBase}-${group}`} class="mt-2 flex flex-wrap gap-2" data-testid={`annotation-group-content-${group}`}>
              {#each entries as entry (entry.id)}
                <span class="inline-flex items-center gap-1.5 rounded-pill border border-ink/15 bg-surface px-3 py-1 text-caption text-ink" data-testid="annotation-chip">
                  <span class="font-medium text-ink-muted">{annotationDateLabel(entry)}:</span>
                  {entry.name}
                </span>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}

  <table id={tableId} class="mt-4 w-full border-collapse text-body text-ink" data-testid="accessible-data-table">
    <caption class="mb-2 text-left text-caption text-ink-muted">{es.chart.tableCaption(name)} ({viewUnit})</caption>
    <thead>
      <tr class="border-b border-ink/15">
        <th scope="col" class="px-2 py-1 text-left font-semibold">{es.table.periodHeader}</th>
        <th scope="col" class="px-2 py-1 text-right font-semibold">{es.table.valueHeader}</th>
        <th scope="col" class="px-2 py-1 text-left font-semibold">{es.table.statusHeader}</th>
      </tr>
    </thead>
    <tbody>
      {#each rangedPoints as p (p.period)}
        <tr class="border-b border-ink/10" data-status={p.status}>
          <!-- Compact register, matching `AccessibleDataTable.astro`'s own
               cell byte for byte: this is the same column, re-rendered by the
               other half of the same chart. The `{#each}` key above is still
               `p.period` — the CANONICAL label — because a keyed list is
               identity, not display. -->
          <td class="px-2 py-1">{formatPeriodCompact(p.period)}</td>
          <td
            class="font-numeric px-2 py-1 text-right tabular-nums"
            class:border-b-2={p.status === "P"}
            class:border-dotted={p.status === "P"}
            class:border-provisional={p.status === "P"}
            class:text-provisional={p.status === "P"}
            data-testid={`table-value-${p.status}`}
          >
            {p.value === null ? "—" : formatNumber(p.value, viewDecimals)}
          </td>
          <td class="px-2 py-1">{es.chart.statusLabel[p.status]}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  /* The figure reserves the WIDE box by default and the NARROW box below
     the `md` breakpoint. Kept in CSS rather than in the inline style the
     component used to compute, because this is the one thing that must be
     right on the very first paint: it is what stops the page reflowing when
     the island hydrates, and what stops it reflowing at all for a reader
     who never runs the script.

     The two ratios are the two `ChartDimensions` this component renders
     (960x360 and 560x420). They are literals here because a Svelte
     `<style>` block cannot read a module constant; `test/chart/island-ssr`
     and `geometry.test.ts` pin the numbers on the other side. */
  .indicator-chart-island__figure {
    aspect-ratio: 560 / 420;
  }

  @media (min-width: 48rem) {
    .indicator-chart-island__figure {
      aspect-ratio: 960 / 360;
    }
  }

  .indicator-chart-island__figure :global(svg.indicator-chart-svg) {
    width: 100%;
    height: 100%;
  }

  /* The one part of a native date input the design tokens cannot reach: its
     browser-drawn calendar affordance. Chromium paints that glyph according
     to the element's `color-scheme`, which defaults to light — so on a dark
     surface it would render a near-black icon on a near-black background,
     invisible, while every token-driven colour around it measured fine.
     Declaring the scheme per theme is what makes the widget follow the page
     rather than the platform. `:global()` is required because the theme
     attribute lives on an ancestor outside this component (the page's own
     `<html data-theme>`, or the workbench's per-section wrapper). */
  :global([data-theme="light"]) .chart-custom-range__input {
    color-scheme: light;
  }
  :global([data-theme="dark"]) .chart-custom-range__input {
    color-scheme: dark;
  }
</style>
