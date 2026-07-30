# ADR-8 — The party-colour prohibition is lifted; palette choice is a design decision

- **Status**: Accepted
- **Date**: 2026-07-29
- **Supersedes**: the colour restriction in PRD §12.1 as written through v2.3
- **Decided by**: product owner
- **Required by**: PRD §14, "Cambios posteriores requieren ADR"

## Context

PRD §12.1 required an "accessible and politically neutral palette" and forbade red, blue,
green and purple in any context where they could be read as party colours, requiring
series to use a documented bespoke scale instead.

The rationale was that a portal whose thesis is "do not trust the framing, look at the
data" undermines itself if its own charts appear partisan.

Two things made the restriction more expensive than it first appears:

1. **The loaded set in Spain is larger than the four named.** PSOE red, PP blue, Vox
   green and Podemos purple are the obvious ones, but Ciudadanos is orange and Sumar is
   magenta/coral. Amber and dotted grey were already spent as reserved semantics (pending
   data, provisional data). What remains is a narrow band of the colour space.
2. **It made hue unusable as the primary categorical channel**, which pushed the design
   toward direct labelling, line patterns and marker shapes carrying the full
   distinguishing load — a defensible design, but a significant constraint to accept
   before a single chart exists.

The product owner raised the question during Fase 1 exploration and, after the trade-off
was set out, directed twice that the restriction be dropped: colour choice is a design
decision with no vetoed list.

## Decision

**The prohibition on red, blue, green, purple and any other hue is lifted.** Palette
choice is a design decision made on design grounds. There is no list of forbidden colours
and no requirement to research or justify avoidance of party-associated hues.

## What is NOT changed by this decision

These are accessibility requirements from §12.5, not colour preferences, and they were
never in scope of the owner's instruction:

1. **Contrast must meet WCAG 2.1 AA, measured in both light and dark themes**, including
   in tooltips. §12.5 requires this to be verified by audit, not derived automatically
   from a light-mode palette.
2. **Colour is never the sole channel distinguishing one series from another.** Line
   pattern or marker shape must always carry the distinction as well, so the chart
   remains readable to a reader who cannot separate two hues.

The reserved semantics in §12.1 also stand, because they carry meaning rather than
identity: **amber = pending data**, **dotted grey = provisional**.

## Consequences

- Milestone 1.1's design system defines the palette on ordinary design criteria. No
  bespoke neutrality analysis is produced.
- Hue becomes available as a categorical channel again, alongside pattern and shape.
- Milestone 1.2 renders one series per page, so the categorical question barely binds in
  this change; it becomes real in the international comparator (§6.3, Fase 2), where
  multiple countries appear in one chart.
- The residual risk is a presentational one the owner has accepted: a chart shared on
  social media may be read as carrying a political tint it does not intend. The
  countermeasures that actually address that risk are unaffected by this decision — every
  figure remains traceable to its primary source (P2), the pipeline stays public (P5),
  and the symmetry counter (P3) is a published metric rather than a visual signal.

## Scope

Fase 1 onward. Nothing shipped in Fase 0 is affected: it contains no colour and no
frontend beyond the milestone-0.1 hello-world.
