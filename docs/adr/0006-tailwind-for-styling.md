# ADR-6 — Tailwind CSS with a hand-written theme, no component kits

- **Status**: Accepted
- **Date**: 2026-07-28
- **Supersedes**: the styling restriction introduced in PRD v2.1 (§12.1, §14.2)
- **Decided by**: product owner
- **Required by**: PRD §14, "Cambios posteriores requieren ADR"

## Context

PRD v2.1 forbade Tailwind CSS, every utility framework and every prefabricated
component kit, and mandated hand-written CSS with custom properties as design
tokens. The stated reason was a product one rather than a technical one: those
tools produce the homogeneous aesthetic that dominates the current web, and this
portal needs a visual identity recognisable at a glance, because §12.4 makes the
shared chart image a first-order distribution channel.

The product owner has since decided to lift the Tailwind prohibition, citing
development experience: hand-writing a design system from scratch is work they do
not want to take on for this project.

Three options were considered against the decided architecture.

## Options considered

**1. Keep hand-written CSS (PRD v2.1 as written).** Maximum identity control and
zero new dependencies; Astro and Svelte already scope `<style>` natively, so no
CSS Modules or vanilla-extract would have been needed. Rejected by the product
owner on effort grounds.

**2. Tailwind plus a component kit (Flowbite was proposed).** Rejected on two
concrete technical grounds, not aesthetic ones:

- Flowbite's interactive components (dropdowns, modals, tooltips, datepickers)
  require runtime JavaScript. PRD §14.2's central rule is that Astro emits zero
  JavaScript by default and the chart is the only interactive island, and §12.3
  caps an indicator page at 300 KB transferred *including its data*. Every kit
  component spends part of that budget. `flowbite-svelte` fits the island model
  better but turns each component into an island, which is not free either.
- Seven of the eight components §12.1 enumerates — indicator card, methodology
  sheet, break band, annotation chip, action bar, verification card, symmetry
  counter — are static presentation. Only the chart and the action bar need
  interactivity. The kit would be carried at full weight and used at roughly a
  tenth of it.

**3. Tailwind with a hand-written theme, no component kit. CHOSEN.**

## Decision

Use **Tailwind CSS** as the styling engine, with a **theme written from scratch**.
Do not adopt Flowbite, shadcn/ui, Bootstrap, DaisyUI, Material UI, Chakra or any
equivalent component kit.

Tailwind is a build-time tool: it generates CSS in CI and adds no production
runtime, so PRD §14.2's "no Node in production" rule is unaffected. This is the
reason the change does not disturb the rest of the decided architecture.

## Constraints that survive this decision

These are not softened, and they are the reason the theme must be hand-written:

1. **Tailwind's default design values are forbidden.** Its stock palette,
   type scale, border radii and shadows are exactly the homogeneity §12.1 warned
   about. The Tailwind `theme` configuration IS this project's design-token file:
   it is authored from scratch and documented in the repository.

2. **The colour restriction is untouched and outranks any Tailwind default.**
   §12.1 forbids red, blue, green and purple wherever they could be read as party
   colours. In Spain those four map to PSOE, PP, Vox and Podemos respectively —
   the entire default semantic palette of most frameworks. This is a political
   neutrality requirement, not a matter of taste, and it is load-bearing for
   principle P3 (audited symmetry).

3. **Visual identity remains a product requirement.** §12.4 makes the social card
   the distribution vehicle: a chart that looks like every other chart breaks the
   distribution mechanism. Recognition is a feature, not decoration.

4. **The page budget still applies.** §12.3: under 300 KB transferred per
   indicator page excluding the typeface, data included. Tailwind purges unused
   utilities, so its CSS output is small; the budget is preserved as long as no
   runtime component JavaScript is added.

5. **Accessibility is unchanged.** §12.5 requires WCAG 2.1 AA verified by audit,
   including screen-reader testing, contrast in dark mode and never using colour
   alone to distinguish series. Utility classes make contrast regressions easy to
   introduce, so this must be audited rather than assumed.

## Consequences

- No CSS Modules and no vanilla-extract: Astro and Svelte scope `<style>`
  natively, so component scoping needs no additional tooling.
- Tailwind and its build step join the CI-only Node ecosystem already accepted in
  §14.2. Production still runs only the Go binary and PostgreSQL.
- Milestone 1.1 (design system and component library) now begins from the
  Tailwind theme rather than from a custom-property token layer. The set of
  components to build is unchanged.
- The `tailwind-4` skill, previously recorded as forbidden for this project in
  `.atl/skill-registry.md` and `openspec/config.yaml`, becomes applicable.

## Scope note

This decision affects Fase 1 onward. Fase 0 (`phase-0-data-foundations`) contains
no styling work beyond the milestone-0.1 hello-world Astro artifact, which has no
components and no CSS framework, and is therefore unaffected.
