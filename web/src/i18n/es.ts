// Central Spanish strings module (indicator-page spec, "Reader-facing copy
// is Spanish and externalised": "A Spanish string MUST NOT be inlined in a
// component, so the i18n architecture PRD §13 requires exists from day
// one"). This slice (7) is this module's first real content: the
// build-time-generated textual description's own vocabulary, plus
// `IndicatorChart.astro`'s own annotation-toggle and table-caption copy.
//
// Disclosed, not silently narrowed: slice 6's seven static components
// (IndicatorCard, FreshnessSemaphore, BreakBand, AnnotationChip, ActionBar,
// MethodologySheet) still inline their own Spanish strings, predating this
// module's existence. Task 9a.11 ("no-inlined-copy scan") is the slice that
// closes that gap for every component at once; retrofitting it here would
// pre-empt that slice's own explicit narrowing step.
//
// Slice 8 (`ChartIsland.svelte`) adds this module's second real content
// batch: range-preset labels, transformation toggle labels/units, the
// per-capita coverage disclosure (**resolves D6** — tasks.md 8.4: the exact
// copy the reconciliation table flagged as missing, drafted here with the
// same "flagged for editorial sign-off" convention every other new
// reader-facing string in this change already carries, e.g.
// `FreshnessSemaphore`'s "Al día") and the keyboard-focused-point
// announcement.
export const es = {
  chart: {
    statusLabel: {
      P: "Provisional",
      D: "Definitivo",
    } as const,
    /** Roving-tabindex keyboard navigation (web-accessibility-gates spec,
     * "each focused point exposes its period, value and status to
     * assistive technology"). */
    pointAnnouncement: (period: string, value: string, unit: string, status: "P" | "D") =>
      `${period}: ${value} ${unit}, ${es.chart.statusLabel[status]}`,
    range: {
      full: "Todo el periodo",
      "5y": "5 años",
      "10y": "10 años",
      "since-2008": "Desde 2008",
      "since-2018": "Desde 2018",
    } as const,
    /** The spec's sixth range entry, "personalizado" (series-transformations
     * spec, "Range presets"; verify-report WARNING-5). Kept in its own key
     * rather than added to `range` above, because `range` is a pure
     * preset-key → label map the island indexes by `RangePreset`, and the
     * custom range is not a preset key — it is a small form with two date
     * inputs, a commit control and its own status line.
     *
     * ALL NEW READER-FACING COPY — flagged for editorial sign-off, per this
     * project's established convention. The register deliberately matches
     * the rest of this module: impersonal, no second person, and every
     * message states a fact about the series rather than instructing the
     * reader. */
    customRange: {
      groupLabel: "Rango personalizado",
      fromLabel: "Desde",
      toLabel: "Hasta",
      applyLabel: "Aplicar rango",
      /** Announced (via the polite live region) whenever a custom range is
       * committed, so a screen-reader reader learns what the chart now
       * shows without having to re-read it. */
      appliedNote: (from: string, to: string) => `Rango personalizado aplicado: se muestra de ${from} a ${to}.`,
      /** The clamping disclosure. `resolveCustomRange` narrows a range that
       * runs past either end of the series to the span that genuinely
       * exists; this sentence is what stops that narrowing from being
       * silent — the free-form equivalent of the spec's "absent, not
       * disabled" rule for the fixed presets. */
      clampedNote: (from: string, to: string) =>
        `El intervalo solicitado se extiende más allá del periodo con datos. Se muestra de ${from} a ${to}, ` +
        `el tramo que la serie cubre realmente.`,
      /** One message per `CustomRangeRejection`. A rejected range leaves the
       * chart exactly as it was: nothing is invented and nothing is
       * silently substituted. */
      error: {
        incomplete: "Falta la fecha de inicio o la de fin.",
        unparseable: "Las fechas indicadas no son válidas.",
        inverted: "La fecha de inicio debe ser anterior a la de fin.",
        "no-observations": "La serie no tiene ningún dato en el intervalo indicado.",
      } as const,
    },
    /** The government range (indicator-page spec, "Annotation layers per PRD
     * §6.1.1(a)" — the `governments` event group, offered as a range control
     * rather than only as a chip). Its own key, beside `customRange` and for
     * the same reason: `range` above is a pure preset-key → label map, and a
     * government selection is not a preset key — it is a small form (one
     * select, one status line) whose options are DATA, not vocabulary. The
     * president's name is never translated or rephrased here: it arrives from
     * `config/gobiernos.yaml` and is printed verbatim.
     *
     * ALL NEW READER-FACING COPY — flagged for editorial sign-off, per this
     * project's established convention. Register matched to the rest of this
     * module: impersonal, no second person, every message a statement of fact
     * about the series rather than an instruction.
     *
     * `derivedEndNote` and `openEndNote` are the two that matter most, and
     * they are not decoration. No government in the registry carries an end
     * date, so the end of every closed term is INFERRED from the next
     * government's investiture (`lib/transform/governmentTerms`). This project
     * does not present an inference as a configured fact — `date_status:
     * unconfirmed` exists precisely to stop that — so the sentence that says
     * so travels with the view it produced, in the same polite live region
     * that already discloses a clamped custom range. */
    government: {
      selectLabel: "Gobierno",
      /** The neutral option: no government filter. Names the whole set rather
       * than saying "ninguno", because choosing it shows every period, not no
       * period. */
      allOption: "Todos los gobiernos",
      optionLabel: (name: string, startYear: string, endYear: string) => `${name} (${startYear}–${endYear})`,
      /** The sitting government: no closing year exists, so none is printed. */
      openOptionLabel: (name: string, startYear: string) => `${name} (desde ${startYear})`,
      /** Reports the span REALLY rendered — the first and last plotted period
       * — not the term's own bounds, which can extend beyond the series at
       * either end. Same rule the custom range's disclosure already follows:
       * the honest statement is the one about what is on screen. */
      appliedNote: (name: string, from: string, to: string) => `Gobierno de ${name}: se muestra de ${from} a ${to}.`,
      derivedEndNote:
        "El registro editorial no recoge la fecha de fin de este gobierno: el final del intervalo se deduce de la " +
        "toma de posesión del gobierno siguiente.",
      openEndNote:
        "El registro editorial no recoge la fecha de fin de este gobierno: se muestra hasta el último dato " +
        "disponible de la serie.",
      /** ── The on-chart marker's own copy (ALL NEW reader-facing strings,
       * flagged for editorial sign-off like every other string in this
       * module). ────────────────────────────────────────────────────────────
       *
       * `markerTitle` is the marker's `<title>`. The rule now prints the
       * president's name ON the drawing, turned onto its side beside it, so
       * this string is no longer the only way a pointer reader can identify
       * it — it survives because it adds the YEAR, which the on-drawing label
       * deliberately leaves out: the x axis under the mark is already a
       * calendar and the chip below already prints the year, so spending the
       * label's scarce room beside a rule on it would buy nothing. The YEAR
       * and not the full date, still, because that is exactly how
       * `AnnotationChip` already labels a government: one event, one label, in
       * every place it appears.
       *
       * `markerLegendLabel` is what teaches the code. An unexplained vertical
       * rule is a decoration; the legend entry beside "Definitivo" and
       * "Provisional" is what makes it a mark with a meaning — and it says
       * what the marks have IN COMMON, which no per-mark name can.
       *
       * `changesNote` is the same information as prose, in a polite live
       * region beside the chart (it moved out of the chart's description
       * paragraph when the markers became selection-gated: it now states
       * transient selection state rather than a permanent property of the
       * series). The labels are painted inside a single `role="img"`, which
       * prunes its own children from the accessibility tree, so a screen reader
       * reaches not one glyph of them; without this sentence the feature would
       * exist for sighted readers only. It is also the complete list when a
       * name was too long to be drawn whole on the plot, which is refused
       * rather than truncated.
       *
       * "REGISTRADOS en el periodo representado" is doing real work: it says
       * these are the changes the editorial registry records inside the span
       * on screen, and stops short of claiming there were no others. That
       * matters for `poblacion-residente`, whose first decade (1971-1980)
       * carries no marker because the only government of that period,
       * `gobierno-suarez-1976`, is `date_status: unconfirmed` and is never
       * projected. A sentence claiming completeness would turn that honest
       * silence into a false assertion. */
      markerTitle: (name: string, year: string) => `Cambio de gobierno: ${name} (${year})`,
      markerLegendLabel: "Cambio de gobierno",
      changesNote: (list: string) => `Se marcan los cambios de gobierno registrados en el periodo representado: ${list}.`,
      /** One marked change, as it reads inside `changesNote`'s list. */
      changeListItem: (name: string, year: string) => `${name} (${year})`,
      /** Spanish joins the last item of a list with "y", not with a comma. */
      listConjunction: "y",
    },
    /** ── The editorial event SPAN projected onto the drawing (PRD §6.1.1(b)/
     * (c)'s `exogenous` and `milestones` groups) — the chart's fourth
     * annotation treatment, and the first one that is about an interval.
     *
     * ALL NEW READER-FACING COPY — flagged for editorial sign-off, per this
     * project's established convention. Register matched to the rest of this
     * module: impersonal, no second person, every message a statement of fact
     * about the series rather than an instruction. The event's own name is
     * never translated or rephrased: it arrives from `config/eventos.yaml` and
     * is printed verbatim.
     *
     * `railTitle` is the rail's `<title>`. A rail now prints its event's name
     * beside it whenever that name fits — "Crisis financiera global y crisis de
     * deuda soberana europea" is 58 glyphs, which fits the 960-unit drawing and
     * cannot fit the 560-unit one at any size this chart sets type in, and is
     * refused there rather than cut. So this string is both the rail's PERIOD
     * RANGE (which no label carries) and the only identification left on a
     * phone. It is NOT the accessibility answer — the drawing is a single
     * `role="img"`, which prunes its own descendants — which is what
     * `projectedNote` below is for.
     *
     * `projectedNote` IS THE CONSTRAINED ONE. "que el registro editorial acota
     * con fecha de inicio y de fin" is not padding: `shock-energetico-2022`
     * and `ngeu-primer-desembolso` carry a start and no end, so nothing is
     * drawn for them, and this clause is what accounts for the difference
     * between the chips a reader counts and the rails they see. Without it the
     * sentence would read as a complete inventory of the group.
     *
     * `clampedListItem` carries the other disclosure. A rail whose event runs
     * past the window is drawn UNCAPPED at that end, which says "this
     * continues beyond what is on screen" to a sighted reader and nothing at
     * all to a listener — so the words have to make the same distinction. */
    eventSpan: {
      railTitle: (name: string, from: string, to: string) => `${name}: de ${from} a ${to}`,
      /** The same, for an event only partly inside the window: it names what
       * is drawn and says the event itself is wider. */
      railTitleClamped: (name: string, from: string, to: string) =>
        `${name}: visible de ${from} a ${to}, se extiende fuera del periodo representado`,
      /** Teaches the code. An unexplained horizontal rail is a decoration; the
       * legend entry beside "Definitivo", "Provisional" and "Cambio de
       * gobierno" is what makes it a mark with a meaning. */
      legendLabel: "Periodo de un acontecimiento",
      projectedNote: (list: string) =>
        `Se señalan sobre el gráfico los periodos de los acontecimientos que el registro editorial acota con ` +
        `fecha de inicio y de fin: ${list}.`,
      listItem: (name: string, from: string, to: string) => `${name} (de ${from} a ${to})`,
      clampedListItem: (name: string, from: string, to: string) =>
        `${name} (visible de ${from} a ${to}; se extiende fuera del periodo representado)`,
      /** Spanish joins the last item of a list with "y", not with a comma.
       * Deliberately its own key rather than a reference to the government
       * layer's identical one: these are two separate vocabularies that happen
       * to share a conjunction, and reaching across would tie one sentence's
       * wording to the other's. */
      listConjunction: "y",
    },
    /** ── The POLICY MEASURE marked on the time axis (the `measures`
     * annotation group, config/medidas.yaml) — the chart's fifth annotation
     * treatment, and the only one drawn outside the plot area.
     *
     * ALL NEW READER-FACING COPY — flagged for editorial sign-off, per this
     * project's established convention. Register matched to the rest of this
     * module: impersonal, no second person, every message a statement of fact
     * rather than an instruction. The instrument's own name is never
     * translated, shortened or rephrased: it arrives from config/medidas.yaml
     * and is printed verbatim.
     *
     * THIS IS THE MOST CONSTRAINED VOCABULARY IN THE FILE, and the constraint
     * is a product decision the owner stated outright: the site shows WHICH
     * measures were taken and WHEN, and says nothing whatever about whether
     * they worked. Prose is where that is easiest to break, because a
     * sentence can imply by ordering what a drawing only suggests by
     * position. So every string below states an instrument and a date and
     * stops — there is no verb of change, no comparison, no "since then", no
     * period-after figure, and nothing to fill one with: the registry, the
     * table and the artifact carry no such field.
     *
     * `noClaimNote` IS THE ONE THAT EARNS ITS PLACE MOST. It is a statement
     * about what the DRAWING does, not about the measures — and it is the
     * only device available that forecloses the reading a mark beside a
     * falling curve otherwise invites. Silence on that question is not
     * neutrality when the layout itself poses it.
     *
     * `markTitle` is the stub's `<title>`, pointer-only: the drawing is a
     * single `role="img"`, which prunes its own descendants from the
     * accessibility tree. It prints the FULL calendar date rather than the
     * year, because for a measure the day is the whole content of the
     * annotation — a law's date of entry into force is not an approximation
     * to a year the way a government's term is.
     *
     * `legendLabel` teaches the code. An unexplained stub under the axis is a
     * smudge; the legend entry beside "Definitivo", "Provisional", "Cambio de
     * gobierno" and "Periodo de un acontecimiento" is what makes it a mark
     * with a meaning — and it names the mark as a date of ENTRY INTO FORCE,
     * which is the fact, rather than as "a measure", which would leave the
     * reader to supply the rest. */
    measure: {
      markTitle: (name: string, date: string) => `Entrada en vigor: ${name} (${date})`,
      legendLabel: "Entrada en vigor de una medida",
      /** One marked measure, as it reads inside `markedNote`'s list. */
      listItem: (name: string, date: string) => `${name} (${date})`,
      /** Spanish joins the last item of a list with "y", not with a comma.
       * Its own key rather than a reference to the government or event-span
       * layers' identical ones, for the reason those two already give: these
       * are separate vocabularies that happen to share a conjunction, and
       * reaching across would tie one sentence's wording to another's. */
      listConjunction: "y",
      /** "registradas para esta serie" is doing real work, and it is the same
       * work `government.changesNote`'s "REGISTRADOS en el periodo
       * representado" does: it says these are the measures the editorial
       * registry records for this indicator inside the span on screen, and
       * stops short of claiming they are all the measures there were. The
       * registry is a small, deliberately partial seed; wording that implied
       * completeness would turn an honest gap into a false assertion. */
      markedNote: (list: string) =>
        `Se señalan en el eje de tiempo las fechas de entrada en vigor de las medidas registradas para esta ` +
        `serie: ${list}. ${es.chart.measure.noClaimNote}`,
      noClaimNote: "El gráfico no representa ninguna relación entre esas medidas y la evolución de la serie.",
    },
    transforms: {
      raw: "Serie original",
      yoy: "Variación interanual",
      /** "Intra-annual rate" per series-transformations spec's own generic
       * naming — the visible label differs by the series' own cadence:
       * quarter-on-quarter for a quarterly series (pib's PRD §7 #14
       * mandatory rate), month-on-month for a monthly one (ipc-general/
       * ipc-subyacente). */
      qoq: (frequency: "M" | "Q" | "A") =>
        frequency === "M" ? "Variación intermensual" : "Variación intertrimestral",
      perCapita: "Por habitante",
      rateUnit: "% de variación",
      perCapitaUnit: (unit: string) => `${unit} por habitante`,
      /** series-transformations spec, "An active transformation relabels
       * the view": "the methodology sheet MUST state what the displayed
       * values now are". Rendered by the island itself (the only place
       * that knows the CURRENT toggle state) rather than the static,
       * build-time `MethodologySheet` component. */
      derivationNote: (label: string) => `Los valores mostrados son la ${label.toLowerCase()}, derivada de la serie original.`,
    },
    perCapita: {
      /** series-transformations spec, "An active per-capita view discloses
       * its covered span": "the covered span and the reason for the
       * restriction are disclosed on the page in Spanish" (**resolves
       * D6**). Population is semiannual 1971-Q1–2020-Q3 and quarterly from
       * 2021-Q1 (`poblacion-residente`'s own cadence segments) — the
       * "periodo con datos de población para cada fecha" phrasing below
       * describes that constraint honestly without inventing a specific
       * boundary date in the copy itself (the actual `from`/`to` are
       * computed, never hand-typed). */
      coverageDisclosure: (from: string, to: string) =>
        `Los datos por habitante solo se muestran entre ${from} y ${to}, el tramo en el que existe un dato de ` +
        `población para cada periodo de la serie. Fuera de ese tramo no se calcula el valor por habitante para ` +
        `no usar una población estimada, repetida o de otra fecha.`,
      /** series-transformations spec, "The attribution rule MUST be stated
       * in the methodology sheet" — this exact sentence is the rule the
       * methodology sheet states (per-capita series only). */
      attributionRule:
        "El valor por habitante se calcula dividiendo cada dato de la serie entre la población residente del " +
        "mismo periodo, nunca entre la población más reciente ni entre un valor interpolado.",
    },
    /** Textual-description vocabulary (web-accessibility-gates spec, "Every
     * chart has a textual description of its main pattern" — PRD §12.5's
     * own worked example, "el paro sube de X a Y entre A y B, luego
     * desciende"). */
    description: {
      noData: "No hay datos disponibles para esta serie.",
      singlePoint: (value: string, period: string) => `Serie con un único dato disponible: ${value} en ${period}.`,
      simple: (verb: string, startValue: string, startPeriod: string, endValue: string, endPeriod: string) =>
        `El valor ${verb} de ${startValue} en ${startPeriod} a ${endValue} en ${endPeriod}.`,
      withTurningPoint: (
        firstVerb: string,
        startValue: string,
        startPeriod: string,
        turningValue: string,
        turningPeriod: string,
        secondVerb: string,
        endValue: string,
        endPeriod: string,
      ) =>
        `El valor ${firstVerb} de ${startValue} en ${startPeriod} a ${turningValue} en ${turningPeriod}, ` +
        `y después ${secondVerb} hasta ${endValue} en ${endPeriod}.`,
      verbRise: "sube",
      verbFall: "desciende",
      verbStable: "se mantiene estable",
    },
    tableCaption: (name: string) => `Datos de ${name}`,
    annotationGroupLabel: {
      governments: "gobiernos",
      exogenous: "shocks exógenos",
      milestones: "hitos normativos",
      /** NEW reader-facing copy — flagged for editorial sign-off. "de
       * política pública" rather than the bare "medidas": on a page about
       * unemployment, "medidas" alone would as easily be read as units of
       * measurement. */
      measures: "medidas de política pública",
    } as const,
    /** Zero-JS-honest phrasing (correct whether the group is currently
     * shown or hidden) — used by the static toggle label. */
    annotationToggleLabel: (label: string) => `Mostrar/ocultar ${label}`,
    /** State-specific verbs reserved for slice 8's JS-enhanced island, which
     * can update the visible label live as the toggle's checked state
     * changes. Unused by the zero-JS static chart on purpose. */
    annotationToggleShow: (label: string) => `Mostrar ${label}`,
    annotationToggleHide: (label: string) => `Ocultar ${label}`,
    breaksHeading: "Rupturas de la serie",
    controls: {
      pointsGroupLabel: "Puntos de la serie, navegables con las flechas del teclado",
      rangeGroupLabel: "Rango del gráfico",
      transformGroupLabel: "Transformaciones",
    },
  },
  /** Slice 9a's own content: the indicator page template (`IndicatorPage.astro`),
   * indicator-page spec's "The three page states of PRD §6.1.3" and related
   * header/methodology copy that is genuinely new this slice (as opposed to
   * every field ALREADY sourced from the export artifact or from
   * `content/indicators/methodology.ts`'s own per-slug editorial prose, which
   * is data, not vocabulary). */
  page: {
    /** Spec-mandated EXACT string (indicator-page spec, "A validation failure
     * serves the last valid datum with a banner": the banner reads
     * "Última actualización correcta: {fecha}. ..." verbatim) — flagged for
     * editorial sign-off like every other new reader-facing string in this
     * change, per this project's own established convention. */
    validationFailureBanner: (fecha: string) =>
      `Última actualización correcta: ${fecha}. La fuente ha publicado un dato que no ha superado nuestra validación automática; estamos revisándolo`,
    /** verify-report CRITICAL-4 remediation. The export artifact now drives
     * this banner, and it deliberately reports a validation failure with NO
     * last-correct-update date when a series' very first ingestion run
     * failed validation: there has never been a correct update to name.
     * `validationFailureBanner` above cannot be reused with a placeholder —
     * it would either print "Última actualización correcta: null" or assert
     * a previous correct update that never happened (principle P4).
     *
     * This string is therefore the spec-mandated sentence with its
     * date-bearing clause REMOVED, not a rewrite: the second half is
     * byte-for-byte the second half of `validationFailureBanner`, so both
     * banners state the identical fact and only the unavailable one is
     * dropped. The trailing period is likewise omitted to match the
     * spec-fixed string's own punctuation. NEW reader-facing copy — flagged
     * for editorial sign-off, per this project's established convention. */
    validationFailureBannerNoDate:
      "La fuente ha publicado un dato que no ha superado nuestra validación automática; estamos revisándolo",
    /** The spec fixes only the validation-failure banner verbatim; the
     * discontinued-series banner's exact wording is this session's own
     * proposed copy (indicator-page spec: "a permanent banner explains the
     * discontinuation... a successor link is shown when a successor is
     * configured") — flagged for editorial sign-off. */
    discontinuedBanner: "Esta serie ha sido descontinuada por la fuente y ya no se actualiza.",
    discontinuedSuccessorLink: (name: string) => `Consultar la serie que la sustituye: ${name}`,
    relatedHeading: "Indicadores relacionados",
    methodologyHeading: "Ficha metodológica",
    /** Human periodicity label from the artifact's own `frequency` code
     * ("M"/"Q"/"A") — never a per-series hand-typed string, so it can never
     * drift from the data it describes. */
    periodicityLabel: { M: "Mensual", Q: "Trimestral", A: "Anual" } as const,
    variationNotAvailable: "—",
    nextPublicationFallback: "Consultar el calendario de publicaciones de la fuente",
    ingestionScriptLinkLabel: "Ver script de ingesta en el repositorio",
    latestValueLabel: "Último valor",
    latestPeriodLabel: "Periodo",
    /** Milestone 1.2: until `/` listed the indicators, an indicator page was
     * a dead end — a reader who arrived on one had no link anywhere else on
     * the site. NEW reader-facing copy, flagged for editorial sign-off.
     * "Volver al inicio" rather than the bare "Inicio" because the control
     * is a return path from a page the reader is already on, and the longer
     * label is also what makes the link wide enough to be a comfortable
     * touch target without padding invented for its own sake. */
    backToHomeLabel: "Volver al inicio",
  },
  /** Date vocabulary. `lib/format/date.ts` produces the date and the clock
   * through `Intl` (the `es-ES` convention `lib/format/number.ts` already
   * established for numerals); the CONNECTOR and the timezone attribution
   * are copy, so they live here.
   *
   * NEW reader-facing copy — flagged for editorial sign-off, per this
   * project's established convention.
   *
   * "(hora peninsular)" rather than `Intl`'s own `timeZoneName`: that option
   * emits "CEST"/"CET" or "GMT+2"/"GMT+1", both of which are machine
   * vocabulary, and both of which CHANGE TWICE A YEAR — a reader who saw
   * "GMT+2" in July and "GMT+1" in January would reasonably wonder which of
   * the two the site had got wrong. The phrase below is true in both halves
   * of the year because the formatter resolves in `Europe/Madrid`, which IS
   * peninsular time. It is deliberately an attribution and not a
   * disclaimer: naming the clock is what lets a reader in the Canary
   * Islands subtract the hour they already know they have to subtract. */
  dates: {
    instant: (date: string, time: string) => `${date} a las ${time} (hora peninsular)`,
  },
  /** Period vocabulary. `lib/format/period.ts` produces month names and years
   * through `Intl` (the same `es-ES` authority `lib/format/date.ts` already
   * uses for "29 de julio de 2026"); the QUARTER has no `Intl` field of its
   * own, so its one authored letter lives here.
   *
   * NEW reader-facing copy — flagged for editorial sign-off, per this
   * project's established convention.
   *
   * "T" for *trimestre*, and it is not this project's invention: INE's own
   * API returns `T3_Periodo` with the values "T1"–"T4", so every quarterly
   * figure on this site arrives already labelled T and was being re-labelled
   * Q on the way to the screen. `Q` is the English abbreviation for
   * *quarter*; Spanish official statistics do not use it.
   *
   * ORDINAL FIRST, YEAR SECOND ("T2 2026") rather than the storage order
   * ("2026-T2"): the reader's question at a glance is which quarter, and the
   * form is the one INE prints in its own tables. It is also exactly seven
   * glyphs, like the "2026-Q2" it replaces — which is what keeps the chart's
   * derived x-axis margins and the data table's Periodo column unchanged. */
  periods: {
    quarter: (ordinal: number, year: string) => `T${ordinal} ${year}`,
  },
  /** verify-report CRITICAL-3 remediation: the shared column-header
   * vocabulary for every rendered data table (`AccessibleDataTable.astro`'s
   * own table AND `ChartIsland.svelte`'s island-rendered copy of the same
   * table) — same three words, same meaning, one definition so the two can
   * never drift apart. */
  table: {
    periodHeader: "Periodo",
    valueHeader: "Valor",
    statusHeader: "Estado",
  },
  /** verify-report CRITICAL-3 remediation: `ActionBar.astro`'s own reader-
   * facing copy, moved here verbatim (byte-for-byte identical to what was
   * previously inlined — this is a relocation, not a rewrite). */
  actionBar: {
    defaultAriaLabel: "Acciones del indicador",
    permalinkLabel: "Enlace permanente",
    csvLabel: "Exportar CSV",
    jsonLabel: "Exportar JSON",
  },
  /** verify-report CRITICAL-3 remediation: `FreshnessSemaphore.astro`'s two
   * states, moved here verbatim. "Pendiente de actualización por la fuente"
   * is the spec's own exact quoted string (indicator-page spec, "A pending
   * source period yields amber with its Spanish copy") — preserved
   * byte-for-byte, since any drift here IS itself a spec violation. */
  freshness: {
    fresh: "Al día",
    sourcePending: "Pendiente de actualización por la fuente",
  },
  /** verify-report CRITICAL-3 remediation: `MethodologySheet.astro`'s own
   * "Qué mide / qué no mide" mobile-summary teaser, moved here verbatim.
   * The section heading itself already existed as `page.methodologyHeading`
   * ("Ficha metodológica") and `MethodologySheetFields.astro`'s ingestion-
   * script link already existed as `page.ingestionScriptLinkLabel` — both
   * reused as-is rather than duplicated under this key. */
  methodologySheet: {
    teaser: "Qué mide / qué no mide",
    measuresHeading: "Qué mide",
    doesNotMeasureHeading: "Qué NO mide",
    sourceLabel: "Fuente",
    operationLabel: "Operación estadística",
    originLabel: "Identificador de origen",
    periodicityLabel: "Periodicidad",
    nextPublicationLabel: "Próxima publicación",
    extractedAtLabel: "Última extracción",
    unitLabel: "Unidad",
    baseLabel: "Base",
    vintageLabel: "Vintage mostrado",
    revisionHistoryLinkLabel: "historial de revisiones",
    perCapitaAttributionHeading: "Atribución per cápita",
  },
  /** verify-report CRITICAL-3 remediation: `BreakBand.astro`'s own inlined
   * copy, moved here verbatim. `ChartIsland.svelte`'s own duplicated
   * break-band markup (task 8.x's static-vs-island parity) reuses the SAME
   * keys, so both copies read the identical Spanish text by construction. */
  breakBand: {
    rupturaLabel: "Ruptura:",
    noteLinkLabel: "Ver nota completa",
  },
  /** verify-report CRITICAL-3 remediation (found DURING remediation, not
   * named in the report's own 9-component/37-string count):
   * `AnnotationChip.astro`'s own short group-prefix labels — distinct from
   * `chart.annotationGroupLabel`'s longer group NAMES ("gobiernos", "shocks
   * exógenos", "hitos normativos") used elsewhere for toggle labels; this is
   * the chip's own terse per-entry prefix ("Gobierno:", "Shock:", "Hito:"). */
  annotationChip: {
    groupPrefix: {
      governments: "Gobierno",
      exogenous: "Shock",
      milestones: "Hito",
      /** NEW reader-facing copy — flagged for editorial sign-off. The terse
       * per-entry prefix, matching its three siblings' register; the longer
       * group NAME lives in `chart.annotationGroupLabel`. */
      measures: "Medida",
    } as const,
  },
  /** verify-report CRITICAL-3 remediation: `IndicatorCard.astro`'s own
   * inlined "Periodo:" label, moved here verbatim. Distinct key from
   * `page.latestPeriodLabel` ("Periodo", no colon) — that key's own render
   * site (`IndicatorPage.astro`'s header) already composes its own colon
   * from surrounding markup; this key keeps the colon baked in to match
   * `IndicatorCard`'s original exact bytes ("Periodo: {latestPeriod}"). */
  indicatorCard: {
    periodLabel: "Periodo:",
  },
  /** verify-report WARNING-18 remediation: `src/pages/index.astro`'s own
   * tagline, moved here verbatim. The no-inlined-copy scan (task 9a.11)
   * originally scanned a hand-maintained file list that never included this
   * page — it predates the indicator-page template and was never added —
   * so this string went unnoticed until the scan was widened to glob every
   * `.astro`/`.svelte` file under `src/`. */
  home: {
    /** REWRITTEN in milestone 1.2, when `/` stopped being a placeholder and
     * became the site's only entry point to the six indicators. The previous
     * value ended "— deploy smoke target for milestone 0.1.": an English
     * build-process note printed to Spanish-speaking readers on the first
     * page they see. It described the page's role in this repository, not
     * anything the site does for them.
     *
     * NEW reader-facing copy — flagged for editorial sign-off, per this
     * project's established convention. Every clause is a fact the site
     * already keeps: each indicator page carries a latest value, a named
     * source and a methodology sheet. Deliberately promises nothing the
     * product does not yet do (no forecasts, no analysis, no coverage
     * claims beyond the six). */
    tagline:
      "Portal de Datos Económicos de España. Cada indicador se publica con su último dato, su fuente y su " +
      "ficha metodológica.",
    /** Heading of the one section `/` has. Named for what it lists rather
     * than for how many there are: the count is frozen at six today, and a
     * heading that said so would have to be edited the day that changes. */
    indicatorsHeading: "Indicadores",
  },
  /** The site footer's copy. ALL NEW reader-facing strings — flagged for
   * editorial sign-off, per this project's established convention. The
   * register matches the rest of this module: impersonal, no second person,
   * every sentence a statement of fact rather than an instruction.
   *
   * `dataTermsNote` IS THE CONSTRAINED ONE, and it is constrained by a hard
   * requirement rather than by taste (spec `source-attribution-licensing`,
   * "No blanket data-licence claim exists in the repository": "The
   * repository MUST NOT assert a single licence over all derived data ...
   * `LICENSE-DATA` MUST defer to `sources/{source}.yaml` rather than
   * override it").
   *
   * The footer any site would write by default — "Datos bajo CC BY 4.0" —
   * would be false here. `config/sources/eurostat.yaml` records that
   * Commission Decision 2011/833/EU authorises reuse of Eurostat's OWN
   * material with acknowledgement, that the permission does NOT extend to
   * third-party material Eurostat republishes, and that some commercial
   * redissemination is separately restricted; `ine.yaml` and
   * `seg-social.yaml` carry different terms again. One sentence covering all
   * three would have to drop whichever restriction did not fit.
   *
   * So this sentence states the ABSENCE of a blanket licence and stops
   * there. It deliberately summarises nothing: the per-source link beside it
   * goes to `config/sources/`, the record the spec makes authoritative, and
   * NOT to `LICENSE-DATA` — that file itself defers to those YAML files, and
   * routing the reader through a deferring summary is the hop this footer
   * exists to remove.
   *
   * Also deliberately absent: ConContexto's own editorial text is offered
   * under CC BY 4.0 *where a source's terms permit redistribution*
   * (LICENSE-DATA). That is a conditional claim about authored prose, not
   * about the values, and printing it in a footer beside the data is exactly
   * how it would be read as covering the data. It stays in LICENSE-DATA,
   * where its condition travels with it. */
  footer: {
    dataTermsNote:
      "Los datos publicados aquí no están cubiertos por una licencia única: cada fuente fija sus propias " +
      "condiciones de reutilización.",
    repositoryLabel: "Repositorio del proyecto",
    /** MIT is named here and only here, because the code IS covered by one
     * licence — a true single claim, unlike anything that could be said
     * about the data. */
    codeLicenceLabel: "Licencia del código (MIT)",
    sourceTermsLabel: "Condiciones de reutilización de cada fuente",
    /** Honest about the destination. A reader who follows this link gets a
     * `text/plain` listing — one line per archived raw file, each carrying
     * its SHA-256, its source, its download timestamp and the exact URL it
     * came from. Naming it "transparencia" or "procedencia de los datos"
     * would promise a page this project does not have. */
    rawFilesLabel: "Hashes SHA-256 de los ficheros originales (texto plano)",
  },
  /** verify-report WARNING-18 remediation: `src/workbench/pages/index.astro`'s
   * own intro paragraph, moved here verbatim. Same widened-scan discovery as
   * `home.tagline` above — the workbench route was never in the
   * hand-maintained file list either. */
  workbench: {
    intro:
      "Los siete componentes estáticos del sistema de diseño (design-system spec), con al menos una " +
      "variante de estado cada uno, en ambos temas.",
  },
};
