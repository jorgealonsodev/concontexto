// Per-page editorial/methodology configuration (indicator-page spec, "The
// methodology sheet carries full traceability") — presentation-layer
// content, deliberately separate from the export artifact's own pipeline
// data (principle P1, design.md D-5: "pipeline data and editorial
// presentation stay separate").
//
// Every field here is genuinely hand-maintained EDITORIAL prose: what the
// series measures, what it does not, and where a reader can find the
// source's own calendar and this project's ingestion code. What used to sit
// alongside them and did NOT belong — PRD §6.1.3's page state, hard-coded to
// `{ kind: "fresh" }` for all six series — moved to the data path in the
// verify-report CRITICAL-4 remediation: it is a recorded pipeline OUTCOME,
// not an editorial choice, so the export artifact now carries it
// (`SeriesDoc.pageState`) and `lib/indicator/pageState.ts` derives it. A
// real validation failure now reaches readers with no source edit and no
// redeploy. Do not reintroduce a page-state field here.
//
// Slice 9b closed slice 9a's own disclosed, scoped narrowing: this module
// now carries all SIX frozen slugs (indicator-page spec, "Six indicator
// routes with frozen slugs"). `relatedSlugs` on every entry resolves to a
// real, now-built page — no related card 404s any more (verified by
// `indicator-page.container.test.ts`'s own related-card-resolution test,
// widened this slice to check every card's target actually renders).

export interface MethodologyContent {
  slug: string;
  /** "Qué mide" — indicator-page spec, "The methodology sheet carries full
   * traceability". */
  measures: string;
  /** "Qué NO mide". */
  doesNotMeasure: string;
  /** Link to this series' own ingestion adapter in the repository. */
  ingestionScriptHref: string;
  /** Link to the SOURCE's own published release calendar. No per-series
   * publication calendar is configured anywhere in this project yet (a
   * disclosed gap, not a fabricated date) — the page renders this href
   * alongside `es.page.nextPublicationFallback`'s honest generic text
   * rather than inventing a specific next-publication date this codebase
   * has no fact backing. */
  nextPublicationHref: string;
  /** 3–5 entries from the full six-slug catalog (see module doc comment). */
  relatedSlugs: string[];
}

const INE_INGESTION_SCRIPT_HREF = "https://github.com/jorgealonsodev/concontexto/tree/main/app/internal/adapters/ine";
const INE_CALENDAR_HREF = "https://www.ine.es/dyngs/INEbase/es/calendario.htm";

export const METHODOLOGY_CONTENT: Record<string, MethodologyContent> = {
  "tasa-de-paro-epa": {
    slug: "tasa-de-paro-epa",
    measures:
      "El porcentaje de población activa que se encuentra en desempleo, según la Encuesta de Población Activa (EPA) del INE.",
    doesNotMeasure:
      "No es lo mismo que el paro registrado del SEPE: la EPA es una encuesta muestral trimestral; el paro registrado cuenta inscripciones administrativas en las oficinas de empleo.",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["ocupados-epa", "poblacion-residente", "ipc-general", "pib"],
  },
  "ocupados-epa": {
    slug: "ocupados-epa",
    measures: "El número de personas ocupadas (con empleo), según la Encuesta de Población Activa (EPA) del INE.",
    doesNotMeasure:
      "No mide horas trabajadas ni tipo de contrato; es un recuento de personas ocupadas, no un indicador de calidad del empleo.",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["tasa-de-paro-epa", "poblacion-residente", "pib", "ipc-general"],
  },
  "poblacion-residente": {
    slug: "poblacion-residente",
    measures:
      "El total de población residente en España, según la Estadística Continua de Población (ECP) del INE.",
    doesNotMeasure:
      "No mide población empadronada por municipio ni nacionalidad; es el total nacional, con la cadencia real de la fuente (semestral hasta 2020, trimestral desde 2021 — ver ficha de rupturas).",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["tasa-de-paro-epa", "ocupados-epa", "ipc-general", "pib"],
  },
  "ipc-general": {
    slug: "ipc-general",
    measures:
      "La variación de precios de la cesta de bienes y servicios de consumo representativa del gasto de los hogares, según el Índice de Precios de Consumo (IPC) general del INE.",
    doesNotMeasure:
      "No mide el coste de la vida de un hogar concreto ni el poder adquisitivo individual; es un índice agregado nacional sobre una cesta representativa, base 2021=100.",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["ipc-subyacente", "tasa-de-paro-epa", "pib", "ocupados-epa"],
  },
  "ipc-subyacente": {
    slug: "ipc-subyacente",
    measures:
      "La variación de precios excluyendo alimentos no elaborados y productos energéticos — la medida de referencia para la tendencia de fondo de la inflación, según el INE.",
    doesNotMeasure:
      "No sustituye al IPC general para calcular la revalorización de rentas o pensiones; excluye deliberadamente los componentes más volátiles.",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["ipc-general", "tasa-de-paro-epa", "pib", "ocupados-epa"],
  },
  pib: {
    slug: "pib",
    measures:
      "El Producto Interior Bruto en volumen encadenado (sin efecto precios), según la Contabilidad Nacional Trimestral de España (CNTR) del INE.",
    doesNotMeasure:
      "No es una medida de bienestar ni de distribución de la renta; es el valor agregado de la producción, ajustado por precios pero no por estacionalidad ni calendario en esta serie.",
    ingestionScriptHref: INE_INGESTION_SCRIPT_HREF,
    nextPublicationHref: INE_CALENDAR_HREF,
    relatedSlugs: ["ipc-general", "ipc-subyacente", "tasa-de-paro-epa", "poblacion-residente"],
  },
};
