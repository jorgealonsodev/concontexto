# Spec for indicator-page

New capability. Slices **7**, **8** and **9**. Sources: PRD §6.1.1, §6.1.3, §12.2, §12.3, §14.2, §9.2,
Anexo E.1, principles P1, P2, P4; ADR-7.

**Language boundary.** This capability introduces the product's first reader-facing copy. Every string a
reader sees is **Spanish** (PRD §13), externalised in a strings module. Scenarios below that assert
Spanish text quote it verbatim; everything else in this spec is English. Indicator slugs are Spanish
because the PRD fixes them as data.

## Requirements

### Requirement: Six indicator routes with frozen slugs

The build MUST produce `/indicador/{slug}` for exactly these six slugs, which are **frozen by this
change** because permalinks are a permanent commitment (Anexo E.1): `tasa-de-paro-epa`, `ocupados-epa`,
`ipc-general`, `ipc-subyacente`, `pib`, `poblacion-residente`.

#### Scenario: All six routes exist in the build

- GIVEN a completed site build from a valid export artifact
- WHEN the output tree is inspected
- THEN a static page exists at `/indicador/{slug}` for each of the six slugs

#### Scenario: A slug change ships a permanent redirect

- GIVEN a published slug
- WHEN it is ever changed
- THEN a permanent redirect from the previous path is published alongside the new one
- AND no published permalink returns a not-found response

### Requirement: The homepage lists all six indicators or the build fails

Added in the record pass that closed verify-report pass-7 CRITICAL-46, resolving pass-7 **WARNING-47**'s
homepage half. Slice 21 (`ac69a29`) shipped `/` as the site's front door and nothing pinned it.

The build MUST produce a homepage at `/` that lists **exactly** the six frozen slugs above, each linking to
its own `/indicador/{slug}` route, and every indicator page MUST offer a link back to it. The listing MUST
be derived from the same all-or-nothing route resolution the six routes are built from, so a slug the
export artifact does not carry fails the build rather than being omitted from the list.

An indicator missing from `/` is a strictly worse failure than a missing route and that asymmetry is the
reason for the all-or-nothing rule: a missing route returns a not-found response to anyone holding its
permalink, while a missing **row** is invisible — the site reads as complete, and the absent indicator is
precisely the one no other surface mentions, because `/` is the only place a reader learns it exists. A
homepage deriving its own list would also be a second derivation of the route set, which is the defect
shape this change has already been bitten by once.

Each listed indicator MUST carry only measured, published facts — its editorial name and unit, its latest
published value and period, and its freshness state — resolved from the same export artifact the routes are
built from. The homepage MUST NOT carry search, filtering, category navigation or any indicator beyond the
six; those belong to later milestones and are out of scope for this change (`proposal.md`, "Out of scope").

#### Scenario: The homepage lists exactly the six frozen slugs

- GIVEN a completed site build from a valid export artifact
- WHEN `/` is inspected
- THEN it lists exactly the six frozen slugs, and no others
- AND each entry links to that slug's own `/indicador/{slug}` route

#### Scenario: A slug the artifact does not carry fails the build

- GIVEN an export artifact missing a document for one of the six frozen slugs
- WHEN the site is built
- THEN the build fails naming the absent slug
- AND no homepage is emitted listing the remaining five

#### Scenario: The homepage and the routes resolve the same list

- GIVEN the homepage and the indicator route sources
- WHEN their route resolution is traced
- THEN both reach the same resolution function
- AND no second derivation of the indicator list exists

#### Scenario: Each listed indicator carries its published figures

- GIVEN a homepage built from a valid export artifact
- WHEN one indicator's entry is read
- THEN it carries that indicator's editorial name and unit, its latest published value, that value's period
  and its freshness state
- AND a series the artifact carries with no observations fails the build rather than rendering a placeholder

#### Scenario: Every indicator page offers a way back

- GIVEN any of the six indicator pages
- WHEN it is rendered
- THEN it offers a link to `/` in Spanish, "Volver al inicio"

### Requirement: Page anatomy per PRD §6.1.1

Each page MUST render, in this order: header (indicator name, latest value with its date, year-on-year
variation, intra-annual variation, freshness semaphore); main interactive chart at full width on mobile;
action bar; methodology sheet; related indicators.

#### Scenario: Every anatomical section is present

- GIVEN any of the six built pages
- WHEN its rendered markup is inspected
- THEN header, chart, action bar, methodology sheet and related-indicators sections are each present
- AND the header carries the latest value, its period, and both variation figures

#### Scenario: The methodology sheet is expanded on desktop and collapsed but visible on mobile

- GIVEN a page rendered at a desktop viewport
- THEN the methodology sheet is expanded
- GIVEN the same page at a 375 px viewport
- THEN it is collapsed with its first line, "Qué mide / qué no mide", visible without interaction

### Requirement: Related indicators

Each page MUST present between 3 and 5 related-indicator cards resolved from configuration.

#### Scenario: Related cards are within range and resolvable

- GIVEN any of the six pages
- WHEN the related-indicators section is inspected
- THEN it contains between 3 and 5 cards
- AND every card links to an existing indicator route

### Requirement: Zero database queries and zero computation at request time

Every page MUST be a fully pre-rendered static file. The request path MUST NOT query a database, MUST NOT
compute page content, and MUST NOT call any external source (PRD §14.2, §9.2).

#### Scenario: The server import graph reaches no database or source adapter

- GIVEN the HTTP server package
- WHEN its transitive import graph is resolved
- THEN it reaches no database adapter, no source adapter and no database driver
- AND the existing import guard fails if one is introduced

#### Scenario: Serving a page issues no outbound request

- GIVEN a running server and a built site
- WHEN an indicator page is requested
- THEN the response is served from a static file
- AND no outbound network request and no database connection occurs during the request

### Requirement: The freshness semaphore has exactly two reader-facing states

The semaphore MUST express **green** — the latest period published by the source is the latest period
shown — or **amber** — *the source has not published the expected period yet*. Amber MUST carry exactly
that meaning and no other.

The page MUST NOT display, compute, poll for or otherwise report whether the build is older than data
held in the database. A statically built page cannot honestly report a fact that postdates its own
generation, and discovering it at request time would require a network call the golden rule forbids. That
condition is an **operations alert** (see `pipeline-operations`) and is never rendered to a reader.

#### Scenario: The latest published period yields green

- GIVEN an artifact whose series freshness state is fresh
- WHEN the page is built and rendered
- THEN the semaphore shows the green state

#### Scenario: A pending source period yields amber with its Spanish copy

- GIVEN an artifact whose series freshness state is source-pending
- WHEN the page is rendered
- THEN the semaphore shows the amber state
- AND its accessible label reads "Pendiente de actualización por la fuente"

#### Scenario: The page never determines freshness over the network

- GIVEN a built indicator page loaded in a browser with JavaScript enabled
- WHEN the page is fully loaded and idle
- THEN zero network requests are issued to any origin for the purpose of determining freshness
- AND the rendered markup contains no element whose meaning is "this page is older than the data we hold"

### Requirement: The default range is the full series

The chart MUST render the complete series by default. Range presets MUST be offered as visible controls,
never inside a hidden menu, and the selected preset MUST be encoded in the permalink.

#### Scenario: The initial render spans the whole series

- GIVEN any of the six pages
- WHEN it is first rendered
- THEN the chart's domain runs from the series' first observation to its last

### Requirement: Series breaks are always visible and never dismissible

Every break resolving for the series MUST render as a shaded vertical band with an icon, always visible.
**No affordance to hide a break MUST exist** — no control, no attribute, no keyboard action and no URL
parameter (principle P4).

#### Scenario: Breaks render for every resolved break

- GIVEN a series with two resolved breaks
- WHEN the page is rendered
- THEN two break bands are present at the corresponding periods

#### Scenario: No dismissal affordance exists

- GIVEN a rendered page
- WHEN its markup, controls, keyboard handlers and accepted URL parameters are enumerated
- THEN none of them hides, collapses or toggles a break band

#### Scenario: Breaks survive with JavaScript disabled

- GIVEN a page loaded with JavaScript disabled
- WHEN the chart is inspected
- THEN every break band is present and visible

#### Scenario: A break explains itself in one sentence and links to the full note

- GIVEN a break band
- WHEN it is hovered or focused
- THEN a tooltip states the change in one sentence in Spanish
- AND it links to the full methodological note

#### Scenario: A transformation never splices across a break

- GIVEN a chart with an active transformation whose span crosses a break
- WHEN the transformed series is rendered
- THEN the break band remains visible at the same period
- AND the transformed series is not silently joined across the break

### Requirement: Every point discloses provisional or definitive

Each point's tooltip and each data-table row MUST state whether the observation is provisional or
definitive. Provisional observations MUST additionally render with the reserved dotted-grey encoding.

#### Scenario: A provisional point discloses its status

- GIVEN an observation whose status is provisional
- WHEN its tooltip is opened and its table row is read
- THEN both state "Provisional"
- AND the point is rendered with the dotted-grey encoding

#### Scenario: A definitive point discloses its status

- GIVEN an observation whose status is definitive
- WHEN its tooltip is opened and its table row is read
- THEN both state "Definitivo"

#### Scenario: The source's verbatim token is preserved as provenance

- GIVEN an observation carrying a `source_status` token
- WHEN the methodology sheet's provenance section is read
- THEN the source's verbatim token is shown alongside the binary status
- AND the binary status shown to the reader is never inferred by defaulting an unknown token to definitive

### Requirement: Three separately toggleable annotation groups, two off by default

The chart MUST offer three annotation groups: **(a) gobiernos**, **(b) shocks exógenos**, **(c) hitos
normativos**. Groups (a) and (b) MUST be off by default; the reader enables them consciously. The layer
MUST render exactly the events that exist with a confirmed date; an absent event MUST NOT be an error. A
group with no applicable entries for the series MUST NOT present a control.

#### Scenario: Groups (a) and (b) are off by default

- GIVEN a freshly loaded page
- WHEN the chart is rendered
- THEN no government annotation and no exogenous-shock annotation is displayed
- AND controls for enabling each group are present

#### Scenario: Enabling a group renders only confirmed events

- GIVEN an editorial configuration containing both confirmed and unconfirmed government dates
- WHEN group (a) is enabled
- THEN only entries with a confirmed date are rendered
- AND the page renders successfully with no error, warning or placeholder for the unconfirmed entries

#### Scenario: A group with no entries has no control

- GIVEN a series for which no normative-milestone entry applies
- WHEN the page is rendered
- THEN no control for group (c) exists — it is absent, not disabled

#### Scenario: Pending editorial entries are operator-visible only

- GIVEN unconfirmed editorial entries
- WHEN a reader loads any indicator page
- THEN no count, badge, warning or placeholder discloses their existence to the reader

### Requirement: Data and interpretation are structurally separate

The header MUST contain only measured figures and their identifiers — indicator name, latest value, its
date, variation figures and the freshness semaphore. **No editorialised headline MUST appear above the
chart.** Interpretive prose MUST live only inside the methodology sheet, in its own labelled region
(principle P1).

#### Scenario: The header carries no interpretive prose

- GIVEN a rendered page
- WHEN the header region is inspected
- THEN it contains only the indicator name, the latest value, its period, variation figures and the
  freshness semaphore
- AND it contains no free prose beyond those labelled fields

#### Scenario: The methodology sheet is its own landmark region

- GIVEN a rendered page
- WHEN its landmark regions are enumerated
- THEN the methodology sheet is a distinct region with its own accessible name
- AND it is not nested inside the chart region

### Requirement: The methodology sheet carries full traceability

The sheet MUST carry: what the indicator measures and what it does **not** measure; source, statistical
operation and origin series identifier with a direct link; periodicity, next-publication calendar and the
extraction timestamp; unit and base; the list of the series' breaks with dates and notes; the vintage
shown and access to the revision history; a link to the ingestion script in the repository. Where a
per-capita transformation exists, the sheet MUST state the denominator attribution rule (principle P2).

#### Scenario: Every traceability field is present on all six pages

- GIVEN each of the six built pages
- WHEN the methodology sheet is inspected
- THEN every field listed above is present and non-empty
- AND the origin-identifier link resolves to the source's own page for that series

#### Scenario: Every figure on the page traces to its origin

- GIVEN any figure rendered on the page
- WHEN its provenance is followed
- THEN it resolves to a source, an origin series identifier, an extraction timestamp and a vintage

### Requirement: The three page states of PRD §6.1.3

The page MUST render three states: **fresh data** (normal); **source failure or validation not passed**
— the last valid datum is served with a banner naming the date of the last correct update; and
**discontinued series** — a permanent banner with an explanation and, where one exists, a link to the
successor series. **The chart MUST NOT be hidden in any state, and a suspect datum MUST NOT be published.**

#### Scenario: A validation failure serves the last valid datum with a banner

- GIVEN a series whose latest run failed validation
- WHEN the page is rendered
- THEN the banner reads "Última actualización correcta: {fecha}. La fuente ha publicado un dato que no ha
  superado nuestra validación automática; estamos revisándolo"
- AND the previously published value is the latest value shown
- AND the suspect value appears nowhere on the page

#### Scenario: The chart is never hidden

- GIVEN any of the three page states
- WHEN the page is rendered
- THEN the chart is present and rendered

#### Scenario: A discontinued series shows a permanent banner

- GIVEN a series marked discontinued by its source
- WHEN the page is rendered
- THEN a permanent banner explains the discontinuation
- AND a successor link is shown when a successor is configured

### Requirement: The page works with JavaScript disabled

With JavaScript disabled, the pre-rendered SVG chart, all break bands, the accessible data table, the
generated textual description and the methodology sheet MUST all be present and legible (PRD §14.2).

#### Scenario: The no-JS baseline is complete

- GIVEN a page loaded with JavaScript disabled
- WHEN it is inspected
- THEN the SVG chart, break bands, data table, textual description and methodology sheet are all present
- AND no empty placeholder or loading state is shown

### Requirement: Under 300 KB transferred per page

Each indicator page MUST transfer under 300 KB, including the data for the default range and excluding
the typeface (PRD §12.3).

#### Scenario: The default-range page stays within budget

- GIVEN any of the six pages loaded at its default range
- WHEN total transferred bytes excluding the typeface are measured
- THEN the total is below 300 KB

### Requirement: Reader-facing copy is Spanish and externalised

Every reader-facing string MUST be Spanish and MUST be resolved from a strings module. A Spanish string
MUST NOT be inlined in a component, so the i18n architecture PRD §13 requires exists from day one.

#### Scenario: No component inlines reader-facing copy

- GIVEN the component and page sources
- WHEN they are scanned for reader-facing string literals
- THEN every such string is resolved through the strings module

#### Scenario: Technical identifiers are not translated

- GIVEN indicator slugs, configuration filenames, source names and origin series identifiers
- WHEN they are rendered
- THEN they appear verbatim as data and are not routed through translation
