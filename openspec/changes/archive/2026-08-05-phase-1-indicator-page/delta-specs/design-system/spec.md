# Spec for design-system

New capability. Slices **5** and **6**. Sources: ADR-6, ADR-8, PRD §12.1, §12.3, §12.5.

ADR-8 lifted the party-colour prohibition. This spec therefore contains **no vetoed-hue list and no
neutrality analysis**. What survives is accessibility, not preference: colour is never the sole channel,
AA contrast is measured in both themes, and amber and dotted grey keep their reserved meanings.

## Requirements

### Requirement: The Tailwind theme is the design-token file and stock defaults are zeroed

A single CSS-first `@theme` block MUST be the project's design-token file. It MUST explicitly zero
Tailwind's stock scales — colours, type scale, border radii and shadows — so that a stock utility class
generates **no CSS rule at all**. The constraint is on Tailwind's *default values*, not on any hue.

#### Scenario: A stock palette class emits no rule

- GIVEN the built stylesheet
- WHEN a template uses a Tailwind stock class such as `bg-blue-500`
- THEN the built stylesheet contains no rule for that class
- AND the design-token guard test fails, naming the stock class

#### Scenario: A project token class emits a rule

- GIVEN the same build
- WHEN a template uses a class backed by a project token
- THEN the built stylesheet contains the corresponding rule

#### Scenario: Stock type scale, radii and shadows are equally zeroed

- GIVEN the built stylesheet
- WHEN a stock type-scale, radius or shadow utility is used
- THEN no rule is generated for it

### Requirement: No prefabricated component kit

`web/package.json` MUST NOT depend on Flowbite, shadcn/ui, Bootstrap, DaisyUI, Material UI, Chakra or an
equivalent component kit, directly or transitively as a runtime dependency.

#### Scenario: A component kit fails the dependency guard

- GIVEN a dependency guard test over `web/package.json` and its lockfile
- WHEN a forbidden kit is added
- THEN the guard test fails naming the package
- AND CI fails

### Requirement: Light and dark token sets are independently authored

Dark mode MUST be a second, explicitly authored token set. It MUST NOT be derived, inverted or
algorithmically generated from the light set. Every semantic token defined for light MUST have an
explicit dark counterpart.

#### Scenario: Every light token has an explicit dark counterpart

- GIVEN the theme token registry
- WHEN light and dark sets are compared
- THEN every semantic token name present in light is also present in dark
- AND no dark value is computed from its light value at build time

### Requirement: AA contrast is measured in both themes, including tooltips

Every text-on-background pairing, every tooltip pairing and every chart-series-against-plot-background
pairing MUST meet WCAG 2.1 AA by **measurement**, in both light and dark. Contrast MUST NOT be assumed
or derived from the light palette.

#### Scenario: Every pairing is measured in both themes

- GIVEN the enumerated set of foreground/background pairings used by the component library
- WHEN contrast is computed for each pairing in light and in dark
- THEN each meets at least 4.5:1 for body text, 3:1 for large text, and 3:1 for non-text graphical
  objects including chart strokes
- AND the assertion is a test, not a documented claim

#### Scenario: A token change that breaks contrast fails the build

- GIVEN a change to any theme token
- WHEN the contrast test suite runs
- THEN a pairing that falls below its threshold in either theme fails, naming the pairing and the theme

### Requirement: Colour is never the sole channel distinguishing a series

Every series encoding MUST carry a non-colour channel — line pattern or marker shape — in addition to
hue. This holds even where a chart renders a single series today.

#### Scenario: Two series remain distinguishable without colour

- GIVEN a chart rendering two series
- WHEN the colour channel is removed from the rendered output
- THEN the two series remain distinguishable by line pattern or marker shape
- AND the legend still identifies each series

### Requirement: Reserved semantics are exclusive

**Amber MUST mean pending data** and **dotted grey MUST mean provisional data**. Neither encoding MAY be
reused for any other meaning anywhere in the product, because they carry meaning rather than identity.

#### Scenario: The amber token is used only for pending data

- GIVEN the component library and the token registry
- WHEN usages of the amber token are enumerated
- THEN every usage is a pending-data indication

#### Scenario: Dotted grey is used only for provisional data

- GIVEN the chart components
- WHEN dotted-grey stroke usages are enumerated
- THEN every usage marks a provisional observation

### Requirement: Tabular figures on all numerals

Every rendered numeral — header values, variation figures, axis labels, tooltip values and data-table
cells — MUST render with tabular figures, so columns and axes align (PRD §12.1).

#### Scenario: Numeric elements compute tabular figures

- GIVEN a rendered indicator page
- WHEN the computed `font-variant-numeric` of each numeric element is inspected
- THEN every one resolves to tabular figures

### Requirement: Eight components in a documented workbench

The library MUST provide exactly these components: indicator card, interactive chart, methodology sheet,
break band, annotation chip, action bar, freshness semaphore, accessible data table. All eight MUST
render in a documented workbench. Seven MUST ship **zero runtime JavaScript**; only the chart is hydrated.

#### Scenario: All eight components render in the workbench

- GIVEN the component workbench
- WHEN it is built
- THEN all eight components render with representative props
- AND each has documented props and at least one state variant

#### Scenario: Seven components ship no runtime JavaScript

- GIVEN a page rendering the seven static components without the chart island
- WHEN its built output is inspected
- THEN it contains no client-side script

### Requirement: Interactive controls meet the 44 px touch target

Every interactive control in the library MUST present a hit area of at least 44 × 44 CSS pixels at the
mobile viewport (PRD §12.3).

#### Scenario: A control's hit area meets the minimum

- GIVEN any interactive control rendered at a 375 px-wide viewport
- WHEN its bounding hit area is measured
- THEN both dimensions are at least 44 CSS pixels
