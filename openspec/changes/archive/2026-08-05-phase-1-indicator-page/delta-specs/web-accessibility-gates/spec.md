# Spec for web-accessibility-gates

New capability. Slice **9**, with the harness wired from slice **5**. Sources: PRD §12.5, §12.3, §13,
§14.2.

§12.5 requires accessibility **verified by audit**, not asserted. This capability is the verification
apparatus: it turns each accessibility and budget claim made elsewhere in this change into a blocking
gate.

## Requirements

### Requirement: WCAG 2.1 AA is verified by automated audit on every indicator page

An automated axe-core audit MUST run against every built indicator page, in both light and dark themes,
as a blocking CI gate. Serious and critical violations MUST fail the build.

#### Scenario: A serious violation fails CI

- GIVEN a built site
- WHEN the axe-core audit runs over all six indicator pages in both themes
- THEN any serious or critical violation fails the job, naming the page, the theme and the rule
- AND the merge is blocked

#### Scenario: A clean build passes

- GIVEN a built site with no serious or critical violations
- WHEN the audit runs
- THEN the job passes and records the audited page and theme count

### Requirement: Every chart has an accessible data table

Every chart MUST have an associated data table exposing period, value and provisional/definitive status
for every rendered observation. The table MUST be reachable by assistive technology and MUST be
programmatically associated with its chart.

#### Scenario: The data table matches the chart

- GIVEN a rendered chart with N observations in the default range
- WHEN its data table is read
- THEN it has N rows
- AND each row carries the period, the value and the provisional/definitive status of the corresponding
  point

#### Scenario: The table is programmatically associated with the chart

- GIVEN a rendered page
- WHEN the accessibility tree is inspected
- THEN the chart references its data table through an accessible relationship

### Requirement: Every chart has a textual description of its main pattern

A textual description of the series' principal pattern MUST be generated at build time and exposed in the
accessibility tree. It MUST describe direction and turning points in Spanish, in the shape PRD §12.5
gives ("el paro sube de X a Y entre A y B, luego desciende…").

#### Scenario: The description is present and non-generic

- GIVEN each of the six built pages
- WHEN the textual description is read
- THEN it names at least the start value, the end value, their periods and the principal direction change
- AND it differs between series

#### Scenario: The description exists without JavaScript

- GIVEN a page loaded with JavaScript disabled
- WHEN the description is read
- THEN it is present in the served HTML

### Requirement: Every chart point is reachable by keyboard

Chart points MUST be navigable by keyboard alone, with a visible focus indicator, and each focused point
MUST announce its period, value and provisional/definitive status.

#### Scenario: Keyboard traversal reaches every point

- GIVEN a rendered chart with JavaScript enabled
- WHEN the chart is focused and the arrow keys traverse it
- THEN every point can be reached
- AND each focused point exposes its period, value and status to assistive technology
- AND focus is visible at every step

#### Scenario: No keyboard trap

- GIVEN focus inside the chart
- WHEN Tab and Escape are pressed
- THEN focus can leave the chart without a pointer

### Requirement: Touch targets are at least 44 px

Every interactive control on an indicator page MUST present a hit area of at least 44 × 44 CSS pixels at
a mobile viewport (PRD §12.3), verified in the browser rather than assumed from styles.

#### Scenario: Measured hit areas meet the minimum

- GIVEN an indicator page rendered at a 375 px-wide viewport
- WHEN each interactive control's bounding box is measured in the browser
- THEN every control measures at least 44 × 44 CSS pixels

### Requirement: The no-JavaScript baseline is a blocking gate

A browser context with `javaScriptEnabled: false` MUST assert that the SVG chart, every break band, the
data table, the textual description and the methodology sheet are present and legible on every indicator
page (PRD §14.2).

#### Scenario: The no-JS context finds a complete page

- GIVEN a Playwright context created with `javaScriptEnabled: false`
- WHEN each of the six indicator pages is loaded
- THEN the SVG chart, break bands, data table, textual description and methodology sheet are all present
- AND no loading state, empty placeholder or error region is rendered

### Requirement: The transferred-bytes budget is a blocking gate

Lighthouse CI MUST run against every indicator page as a blocking gate and MUST assert **explicitly** that
total transferred bytes, including default-range data and excluding the typeface, are under 300 KB. The
gate MUST be wired from the first web slice, not added at the end of the change.

#### Scenario: An over-budget page fails the merge

- GIVEN an indicator page whose transferred bytes exceed 300 KB excluding the typeface
- WHEN the Lighthouse CI gate runs
- THEN the job fails, reporting the measured total and the budget
- AND the merge is blocked

#### Scenario: The budget assertion is explicit, not implied by a performance score

- GIVEN the CI configuration
- WHEN the Lighthouse assertions are inspected
- THEN a named transferred-bytes assertion exists with the 300 KB threshold and the typeface exclusion

### Requirement: Contrast is verified in both themes as part of the gate

The blocking gate MUST include the measured contrast verification of `design-system`, covering text,
tooltips and chart strokes, in light and in dark.

#### Scenario: A dark-theme-only contrast regression fails CI

- GIVEN a token change that meets AA in light but falls below it in dark
- WHEN the gate runs
- THEN it fails, naming the pairing and the dark theme

### Requirement: Web test commands are declared and blocking

`openspec/config.yaml` MUST declare the web unit-test and end-to-end commands in place of the current
`TBD`, and CI MUST run both as blocking jobs.

#### Scenario: No TBD web test command remains

- GIVEN `openspec/config.yaml`
- WHEN the testing and apply sections are read
- THEN the web test command is a runnable command, not `TBD`
- AND CI runs it as a blocking job

### Requirement: Test discipline is stated honestly

Strict TDD MUST apply red-first to the Go publishing layer and to pure render and transform functions.
Browser-rendered accessibility audits and budget gates MUST be written alongside the work they verify
rather than before it, because a red-first browser audit proves nothing.

#### Scenario: Pure transform functions are developed red-first

- GIVEN a new transformation or render function
- WHEN it is implemented
- THEN a failing unit test for it exists in the history before the implementation

#### Scenario: Acceptance gates are present by the end of their slice

- GIVEN a web slice that ships user-visible output
- WHEN the slice closes
- THEN its axe, keyboard, no-JS and budget assertions run in CI as blocking jobs
