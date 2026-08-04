# Spec: series-transformations

Baseline capability specification — the source of truth for `series-transformations`.

Sources: PRD §6.1.2, §6.1.1, §7, principle P4; settled decision D4.
Contributing changes: `phase-1-indicator-page` (archived 2026-08-05) — established this capability.

## Requirements

### Requirement: A transformation that does not apply does not exist

A transformation control MUST be rendered only when the transformation is computable for that series over
the rendered span. A non-applicable transformation MUST NOT be rendered disabled, greyed, hidden behind a
menu or announced to assistive technology (PRD §6.1.2).

#### Scenario: A non-applicable transformation has no control at all

- GIVEN the `tasa-de-paro-epa` page, where per capita does not apply to a rate
- WHEN the page is rendered
- THEN no per-capita control exists in the markup or in the accessibility tree

#### Scenario: No disabled transformation control exists anywhere

- GIVEN all six indicator pages
- WHEN every transformation control is enumerated
- THEN none is in a disabled state

### Requirement: Transformation applicability for the six milestone-1.2 series

The applicable transformations MUST be exactly as follows. **"Real" applies to none of the six** — none is
a monetary magnitude: a rate, a headcount, two price indices, an already-real chained volume index and a
population. **"Median" applies to none of the six** — no source publishes both mean and median for them.

| Slug | Real | Median | Year-on-year | Intra-annual rate | Per capita |
|---|---|---|---|---|---|
| `tasa-de-paro-epa` | absent | absent | offered (difference in percentage points) | offered | absent |
| `ocupados-epa` | absent | absent | offered | offered | offered, over the covered span |
| `ipc-general` | absent | absent | **mandatory** | offered | absent |
| `ipc-subyacente` | absent | absent | **mandatory** | offered | absent |
| `pib` | absent | absent | **mandatory** | **mandatory** (quarter-on-quarter) | offered, over the covered span |
| `poblacion-residente` | absent | absent | offered | offered | absent |

Year-on-year is **mandatory, not optional**, wherever the configuration stores an index while PRD §7's
headline indicator is an annual rate: `ipc-general` (§7 #10), `ipc-subyacente` (§7 #11) and `pib` (§7 #14).
`pib` additionally requires the quarter-on-quarter rate because §7 #14 names both. That is the set of four
mandatory derived rates.

#### Scenario: An index-backed series offers its headline rate

- GIVEN the `ipc-general` page, whose configuration stores the price index
- WHEN the page is rendered
- THEN a year-on-year transformation control is present
- AND the methodology sheet states that the stored series is the index and the rate is derived

#### Scenario: `pib` offers both mandatory rates

- GIVEN the `pib` page
- WHEN it is rendered
- THEN both year-on-year and quarter-on-quarter transformation controls are present

#### Scenario: No page offers a real or median transformation

- GIVEN all six pages
- WHEN transformation controls are enumerated
- THEN no real/nominal control and no mean/median control exists on any of them

### Requirement: Per capita uses the resident population at each observation's own date

A per-capita transformation MUST divide each observation by the resident population recorded **at that
observation's own period**, never by the current population retroprojected across history. It MUST NOT
interpolate, extrapolate or otherwise invent a missing denominator period (PRD §6.1.2).

#### Scenario: Each observation uses its own period's denominator

- GIVEN an observation at period `P` and a population observation at the same period `P`
- WHEN per capita is computed
- THEN the divisor is the population value at `P`
- AND it is not the latest population value

#### Scenario: A missing denominator period is never invented

- GIVEN a quarterly aggregate series and a population series whose historical cadence is semiannual
- WHEN per capita is computed
- THEN periods with no population observation produce no per-capita point
- AND no interpolated, carried-forward or averaged denominator is used

### Requirement: Per capita exists only where the denominator genuinely exists

The per-capita control MUST be present only when the denominator covers a non-empty sub-span of the
series. When active, the transformation MUST render only over the covered sub-span and MUST disclose that
span to the reader. The attribution rule MUST be stated in the methodology sheet and is subject to
editorial sign-off (settled decision D4).

#### Scenario: A series with no denominator coverage has no control

- GIVEN an aggregate series for which no population observation shares any of its periods
- WHEN the page is rendered
- THEN the per-capita control is absent, not disabled

#### Scenario: An active per-capita view discloses its covered span

- GIVEN an aggregate series whose denominator covers only part of its history
- WHEN the per-capita transformation is activated
- THEN the chart renders only the covered sub-span
- AND the covered span and the reason for the restriction are disclosed on the page in Spanish
- AND the methodology sheet states the denominator attribution rule

### Requirement: Range presets

The chart MUST offer the full series as the default range plus these presets: 5 años, 10 años, desde 2008,
desde 2018, personalizado. A preset whose start precedes the series' first observation MUST be absent, not
disabled. The selected preset MUST be encoded in the permalink.

#### Scenario: A preset earlier than the series start is absent

- GIVEN a series whose first observation is in 2010
- WHEN the page is rendered
- THEN the "desde 2008" preset is absent from the controls
- AND it is not shown disabled

#### Scenario: Selecting a preset updates the permalink

- GIVEN a rendered page
- WHEN a range preset is selected
- THEN the permalink encodes that preset
- AND loading that permalink reproduces the same range

### Requirement: Transformations never invent data

A derived value MUST NOT be produced where its inputs do not exist. A year-on-year value MUST NOT be
emitted for periods with no same-period-prior-year observation.

#### Scenario: The first year of a series yields no year-on-year points

- GIVEN a quarterly series with 40 observations
- WHEN year-on-year is computed
- THEN the first four periods produce no point
- AND they are rendered as absent, not as zero

### Requirement: Transformations recompute in the browser with no network request

Activating a range preset or a transformation MUST redraw from data already delivered with the page. It
MUST NOT issue a network request and MUST NOT trigger a server-side computation (PRD §14.2, §9.2).

#### Scenario: Toggling a transformation issues no request

- GIVEN a loaded indicator page
- WHEN a transformation or range preset is activated
- THEN zero network requests are issued
- AND the chart redraws from the already-delivered series data

### Requirement: An active transformation relabels the view

Activating a transformation MUST update the axis label and the unit shown, and the methodology sheet MUST
state what the displayed values now are. When two encodings appear in one view, they MUST be distinguished
by a legend and by a non-colour channel (PRD §6.1.2, §12.5).

#### Scenario: The axis relabels when a rate is shown

- GIVEN a page showing an index
- WHEN the year-on-year transformation is activated
- THEN the axis label and the unit change to the annual rate
- AND the methodology sheet states that the displayed values are derived from the stored index
