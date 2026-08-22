# Design Guidelines

This project uses the adidas Design Language (aDL) Storybook as a reference for UI structure, component discipline, theming, iconography, grid behavior, and accessibility.

Source reference: https://designlanguage.adidas.com/?path=/docs/welcome--documentation

The aDL source describes itself as documentation and development guidance for building adidas `.com` digital experiences. Treat it as inspiration for clear commerce/product tooling, not as permission to copy proprietary branding, assets, or package code.

## Product Feel

Clever Consumer should feel like a practical commerce utility: direct, structured, trustworthy, and fast to scan. Favor functional layouts over marketing pages. The first screen should expose the working product experience: watchlists, product previews, price history, alerts, and account controls.

Use a restrained aDL-aligned direction:

- Clear hierarchy.
- Confident typography.
- Minimal decoration.
- Strong alignment.
- Crisp component states.
- Responsive grid behavior.
- Accessibility built into every interaction.

Avoid ornamental gradients, decorative blobs, oversized marketing hero layouts, and new visual treatments that do not help product tracking workflows.

## Local Token Mapping

The public aDL Storybook currently defines black and white as its primary light-surface colors, neutral gray surfaces and dividers, blue for focus/interactive emphasis, green for success, and red for errors. Map those roles to the local tokens in `web/clever-consumer/src/index.css`:

```css
--bg: #ffffff;
--panel: #ffffff;
--panel-strong: #000000;
--text-inverse: #ffffff;
--surface-subtle: #f5f5f5;
--surface-medium: #eceff1;
--surface-error: #fff0f0;
--text: #000000;
--muted: #767677;
--line: #d9dbdd;
--line-strong: #929396;
--line-inverse: #3b3b3c;
--accent: #000000;
--accent-hover: #1e1e1e;
--focus: #007bc6;
--success: #00aa55;
--warning: #e32b2b;
--overlay: rgba(0, 0, 0, 0.6);
```

The proprietary aDL typeface is not distributed with this project. Use `Arial, Helvetica, sans-serif`, which follows aDL's own fallback stack, and rely on weight and spacing for hierarchy. Do not download or copy Adidas fonts or brand assets.

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

- Desktop app shell: `280px` navigation sidebar, collapsible to a `72px` icon rail, plus a flexible workspace.
- Mobile breakpoint: `860px`, collapsing to one column.
- Card grid: `repeat(auto-fill, minmax(300px, 1fr))`.
- Standard page/workspace padding: `24px` on compact screens and `32px` on desktop.
- Standard compact popup padding: `16px`.
- Standard aDL-aligned spacing steps: `4px`, `8px`, `16px`, `24px`, `32px`, `40px`, `48px`, `64px`, `80px`.

Use CSS grid for page structure and repeated cards. Use flex only for inline groups that wrap naturally, such as action rows.

Keep the sidebar navigational. Account and scraper configuration belongs in dedicated workspace pages reached from sidebar items, not in forms embedded directly in the sidebar. Keep the sidebar collapse control beside the active workspace title and available in both expanded and minimized states.

When adding responsive behavior:

- Start with the smallest usable layout.
- Keep forms to one column on narrow screens.
- Ensure text wraps without overlapping buttons, prices, images, or history rows.
- Preserve stable card dimensions so loading, hover, or state changes do not shift the grid.
- Use existing breakpoints before adding new ones.

## Components

Reuse existing component patterns before creating new ones.

Buttons:

- Primary buttons use a black fill, white text, square corners, and a `48px` minimum height. High-emphasis primary actions may use the aDL offset outer border treatment.
- Secondary buttons use a white fill, black text, a black `1px` border, and square corners.
- Compact and icon buttons may use `40px`; avoid making primary workflow buttons smaller than `48px`.
- Pair a short action label with a trailing icon when the icon clarifies direction or intent. Do not add an icon only as decoration.
- Always provide disabled styling with reduced opacity and `not-allowed` cursor.
- Loading labels should preserve button width where possible.

Forms:

- Labels are stacked above controls.
- Label text is muted, small, and strongly weighted.
- Inputs and selects use full width, a neutral `1px` border, square corners, and a stable `48px` minimum height.
- Hover and active input borders become black. Focus uses the shared blue focus token without changing layout.
- Required, error, hint, and success states must be visually and semantically distinct.

Cards and panels:

- Use cards for individual repeated items, previews, history panels, profile panels, and modal-like content.
- Use square corners for operational product cards and modal panels. A `4px` or `8px` radius is reserved for components whose meaning benefits from it, such as tags or status badges.
- Do not nest cards inside cards.
- Keep operational screens dense but breathable.

Status and metadata:

- Eyebrows and statuses use uppercase text, strong weight, and the primary or semantic status color.
- Prices are prominent but not hero-sized outside a true product detail context.
- Metadata should use muted color and line-height around `1.45`.

## Icons

aDL recommends standalone icon components over sprite-based icon rendering because standalone components support better code splitting and tree shaking.

For this repo:

- Use `lucide-react` as the public standalone React icon source. The official `@adl/iconography` package referenced by aDL is not publicly installable for this project.
- Prefer component-based icons over hand-authored SVGs for controls.
- Use a consistent `20px` control icon size and `1.75px` stroke; use `24px` only where the larger icon has a clear hierarchy role.
- Navigation uses familiar icons with visible text. Icon-only menu and close controls use tooltips and accessible names.
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

## Source Notes

The mapping above was checked against the current public aDL Storybook metadata and styles on 2026-08-22. Relevant source sections are `Theming/Overview`, `Core Components/Button`, `Core Components/Button Icon`, `Core Components/Input Text`, `Core Components/Tag`, `Iconography/Overview`, and `Grid`.

Do not copy Adidas trademarks, product imagery, private packages, or proprietary fonts. Reuse the system's interaction and token principles while keeping Clever Consumer's own identity and content.
