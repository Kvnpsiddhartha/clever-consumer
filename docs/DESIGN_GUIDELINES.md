# Design Guidelines

This project uses the adidas Design Language (aDL) Storybook as a reference for UI structure, component discipline, theming, iconography, grid behavior, and accessibility.

Source reference: https://designlanguage.adidas.com/?path=/docs/welcome--documentation

The aDL source describes itself as documentation and development guidance for building adidas `.com` digital experiences. Treat it as inspiration for clear commerce/product tooling, not as permission to copy proprietary branding, assets, or package code.

## Product Feel

Clever Consumer should feel like a practical commerce utility: direct, structured, trustworthy, and fast to scan. Favor functional layouts over marketing pages. The first screen should expose the working product experience: watchlists, product previews, price history, alerts, and account controls.

Use a restrained Adidas-inspired direction:

- Clear hierarchy.
- Confident typography.
- Minimal decoration.
- Strong alignment.
- Crisp component states.
- Responsive grid behavior.
- Accessibility built into every interaction.

Avoid ornamental gradients, decorative blobs, oversized marketing hero layouts, and new visual treatments that do not help product tracking workflows.

## Existing Tokens

Reuse the current CSS custom properties in `web/clever-consumer/src/index.css` before adding anything new:

```css
--bg: #f7f7f4;
--panel: #ffffff;
--panel-strong: #111827;
--text: #27303f;
--muted: #667085;
--line: #d8ddd5;
--accent: #0f766e;
--accent-strong: #134e4a;
--warning: #9a3412;
```

Current typography is `Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`. Keep it unless a deliberate typography pass updates all surfaces together.

Do not introduce new colors, font sizes, shadows, or radii for one-off components. If a new token is necessary, add it at `:root`, document why it exists, and apply it consistently across web and extension surfaces where relevant.

## Theming

aDL frames design tokens with structured names that identify component, variant, surface, type, category, anchor when relevant, and state. Follow that same thinking even if this repo keeps simple CSS variables.

When adding component styles, account for:

- Variant: primary, secondary, subtle, destructive, or another locally meaningful role.
- Surface: light and dark when a component can appear on both.
- Type: color, typography, spacing, border, radius, or state.
- Category: label, text, background, border, icon, helper text.
- State: default, hover, focus, active, disabled, loading, error, success.

Prefer semantic naming over raw visual naming. For example, add `--color-success` only when success messaging exists; do not add `--green-500` for one local element.

## Layout And Grid

The aDL grid reference uses responsive breakpoints and a column system. For this project, keep the implementation simple, but use the same principle: layout changes should happen at named, intentional thresholds.

Current web layout:

- Desktop app shell: `300px` sidebar plus flexible workspace.
- Mobile breakpoint: `860px`, collapsing to one column.
- Card grid: `repeat(auto-fill, minmax(300px, 1fr))`.
- Standard page/workspace padding: `28px`.
- Standard compact popup padding: `16px`.
- Standard gaps: `8px`, `12px`, `14px`, `18px`, `22px`, `28px`.

Use CSS grid for page structure and repeated cards. Use flex only for inline groups that wrap naturally, such as action rows.

When adding responsive behavior:

- Start with the smallest usable layout.
- Keep forms to one column on narrow screens.
- Ensure text wraps without overlapping buttons, prices, images, or history rows.
- Preserve stable card dimensions so loading, hover, or state changes do not shift the grid.
- Use existing breakpoints before adding new ones.

## Components

Reuse existing component patterns before creating new ones.

Buttons:

- Primary buttons use `--accent`, white text, `6px` radius, at least `42px` height on web and `38px` in the extension popup.
- Secondary buttons use the current pale background, accent text, and line border.
- Always provide disabled styling with reduced opacity and `not-allowed` cursor.
- Loading labels should preserve button width where possible.

Forms:

- Labels are stacked above controls.
- Label text is muted, small, and strongly weighted.
- Inputs and selects use full width, `1px` line border, `6px` radius, and stable minimum height.
- Required, error, hint, and success states must be visually and semantically distinct.

Cards and panels:

- Use cards for individual repeated items, previews, history panels, profile panels, and modal-like content.
- Keep the existing `8px` panel radius and `1px` border.
- Do not nest cards inside cards.
- Keep operational screens dense but breathable.

Status and metadata:

- Eyebrows and statuses use uppercase text, strong weight, and accent-strong color.
- Prices are prominent but not hero-sized outside a true product detail context.
- Metadata should use muted color and line-height around `1.45`.

## Icons

aDL recommends standalone icon components over sprite-based icon rendering because standalone components support better code splitting and tree shaking.

For this repo:

- Use the existing icon system first if one is already present.
- Prefer component-based icons over hand-authored SVGs for controls.
- Icon-only buttons need accessible names.
- Decorative icons must be hidden from assistive technology.
- Size icons intentionally and align them to the text baseline or button center.

## Accessibility

Every UI change must handle keyboard, screen reader, and low-vision use.

Minimum requirements:

- Native form controls where possible.
- Visible focus states.
- Real `button`, `input`, `select`, `label`, and heading elements.
- Accessible names for icon-only controls.
- `aria-live` or equivalent treatment for async status messages when needed.
- Disabled controls must be programmatically disabled, not only visually dimmed.
- Error text must be associated with its control.
- Color must not be the only state indicator.

Before shipping UI, test keyboard navigation through the changed workflow.

## Content

Use short, concrete interface copy:

- Prefer verbs on buttons: `Preview`, `Confirm tracker`, `Run now`, `Pause`, `Resume`.
- Avoid explanatory text inside the app unless it prevents user error.
- Error messages should state what failed and, when useful, what the user can do next.
- Keep product URLs, product names, prices, and timestamps readable and wrap-safe.

## Implementation Checklist

Before modifying UI:

1. Read this file.
2. Inspect the nearest existing screen/component.
3. Reuse current tokens, spacing, typography, borders, and states.
4. Add new tokens only when the existing set cannot express the new role.
5. Check mobile and desktop layouts.
6. Check keyboard navigation and focus visibility.
7. Confirm text does not overflow or overlap.
8. Keep changes scoped to the requested workflow.

