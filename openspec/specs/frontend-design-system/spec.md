# Frontend Design System Specification

## Purpose
Defines how the SvelteKit frontend's shadcn-svelte component library is
generated, installed, updated, and customized. It pins the library to the
current shadcn-svelte registry style, guarantees that component install/update
operations are safe and non-destructive, preserves project-specific
customizations (palette, fonts, custom variants, animations), establishes the
in-component icon convention, and requires the migrated frontend to build,
type-check, and render without regressions.

## Requirements

### Requirement: Current shadcn-svelte registry style as baseline

The frontend's UI component library SHALL be generated from the current shadcn-svelte registry style supported by the pinned CLI version, such that every component under `web/frontend/src/lib/components/ui/` matches that style's structure, class names, and component-level CSS conventions.

#### Scenario: Component matches current registry output

- **WHEN** a component already present in the project is regenerated from the registry with the same name
- **THEN** the on-disk file matches the registry's current-style output (modulo project-specific customizations and resolved import aliases) with no leftover constructs from the previous style

#### Scenario: No undefined style classes

- **WHEN** any UI component references a style utility or component class (e.g. `cn-*`)
- **THEN** that class is defined by the project's theme/CSS so the component renders as the style intends

### Requirement: Safe component install and update

After migration, adding or updating components from the registry SHALL succeed without altering unrelated components or silently changing the design.

#### Scenario: Overwrite-install an existing component is non-destructive

- **WHEN** a maintainer runs the shadcn-svelte CLI to add or update an already-installed component with overwrite enabled
- **THEN** the command completes and the regenerated component is consistent with the rest of the library, requiring only re-application of documented project-specific customizations

#### Scenario: Adding a new component does not restyle others

- **WHEN** a maintainer adds a new component that depends on existing components (e.g. a component depending on `button`)
- **THEN** the existing dependency components are not visually or structurally changed by the operation

### Requirement: Preservation of project customizations

Regenerating components SHALL preserve all project-specific customizations, including custom variants and the project palette, fonts, and custom utilities.

#### Scenario: Custom variants retained

- **WHEN** the component library is migrated to the new style
- **THEN** project-added variants (such as the `badge` `success` variant and any custom `button` variants) remain available and behave as before

#### Scenario: Theme identity preserved

- **WHEN** the theme tokens are reconciled with the new style
- **THEN** the "ahsoca" palette, DM Sans / JetBrains Mono fonts, status and attribution tokens, and custom animations (`animate-fade-up`, stagger, indicator-pulse) are retained

### Requirement: Icon convention inside components

Icons rendered inside components that document an icon-spacing convention SHALL use that convention, and consumers across the app SHALL be updated accordingly.

#### Scenario: Button icons use the documented convention

- **WHEN** an icon is placed inside a `Button`
- **THEN** it uses the new style's documented icon-spacing mechanism (e.g. `data-icon="inline-start"|"inline-end"`) rather than manual margin/size classes, and spacing renders correctly

#### Scenario: All consumers migrated

- **WHEN** the migration is complete
- **THEN** no route or shared component still relies on the old manual icon-spacing pattern for icons inside components that provide the new convention

### Requirement: No visual or functional regressions

The migrated frontend SHALL build and type-check cleanly and SHALL render without visual regressions in both light and dark themes.

#### Scenario: Build and type-check pass

- **WHEN** the frontend is built and type-checked after migration
- **THEN** `mise run build` and `npm run check` complete with no new errors attributable to the migration

#### Scenario: Visual parity in both themes

- **WHEN** key pages (dashboard, assessments list and detail, rule coverage, connectors, scenarios, packs) are reviewed in light and dark mode
- **THEN** primitives such as buttons (including `destructive`), badges, inputs, and tables render correctly with intended emphasis and no broken spacing
