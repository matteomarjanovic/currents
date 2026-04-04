---
name: frontend-shadcn-svelte
description: Guidelines for frontend development in the Currents SvelteKit app — visual identity, component usage, and shadcn-svelte conventions.
---

# Frontend development — Currents

## App vibe

Currents is a visual discovery app (Pinterest alternative). The UI should feel:

- **Clean and image-first** — content (images, collections) is the hero. Chrome is minimal and stays out of the way.
- **Calm, not flashy** — no heavy gradients, no aggressive animations. Subtle transitions only.
- **Neutral with purpose** — use the shadcn-svelte neutral palette as the base; color accents are rare and intentional.

## Styling rules

1. **Use shadcn-svelte exclusively for UI primitives.** Do not reach for raw HTML elements when a shadcn-svelte component exists for the job.
2. **Style everything with CSS variables from the shadcn-svelte theme.** Never hardcode colors, radii, or shadows — use `hsl(var(--background))`, `hsl(var(--foreground))`, `hsl(var(--muted))`, `hsl(var(--primary))`, etc. See the [Theming docs](https://shadcn-svelte.com/docs/theming.md) for the full token list.
3. **Tailwind utility classes are fine for layout and spacing.** Use them freely for `flex`, `grid`, `gap`, `p-*`, `m-*`, `w-*`, `h-*`, etc. For color/appearance, always prefer CSS variables over Tailwind color classes (e.g. `text-foreground` not `text-gray-900`).
4. **Dark mode is a first-class concern.** All color choices must work in both light and dark mode — this is automatic if you use CSS variables.

## Component conventions

- Add components with the CLI: `npx shadcn-svelte@latest add <component>`. Do not hand-roll what shadcn-svelte already provides.
- Components live in `src/lib/components/ui/` (managed by the CLI). Do not edit them directly — override via CSS variables or wrapper components.
- For app-specific composed components, put them in `src/lib/components/` alongside the `ui/` folder.

## Workflow when building UI

1. Check which shadcn-svelte components are available before writing any markup — consult the list below or the MCP tools.
2. Use the `svelte` MCP server to look up component APIs and Svelte patterns:
   - `list-sections` — browse the docs structure
   - `get-documentation` — fetch a specific doc page
   - `svelte-autofixer` — catch and fix Svelte 5 issues after writing a component
   - `playground-link` — share a repro if debugging
3. After finishing a component, run `svelte-autofixer` to confirm there are no rune/syntax issues.

## shadcn-svelte documentation links

Full docs are available as LLM-readable markdown. Reference these when implementing or debugging:

**Core**
- [Theming](https://shadcn-svelte.com/docs/theming.md) — CSS variables reference
- [CLI](https://shadcn-svelte.com/docs/cli.md) — adding components
- [components.json](https://shadcn-svelte.com/docs/components-json.md) — project config
- [SvelteKit installation](https://shadcn-svelte.com/docs/installation/sveltekit.md)

**Components**
- Form & Input: Button, Button Group, Calendar, Checkbox, Combobox, Date Picker, Field, Formsnap, Input, Input Group, Input OTP, Label, Native Select, Radio Group, Select, Slider, Switch, Textarea
- Layout & Navigation: Accordion, Breadcrumb, Navigation Menu, Resizable, Scroll Area, Separator, Sidebar, Tabs
- Overlays & Dialogs: Alert Dialog, Command, Context Menu, Dialog, Drawer, Dropdown Menu, Hover Card, Menubar, Popover, Sheet, Tooltip
- Feedback & Status: Alert, Badge, Empty, Progress, Skeleton, Sonner, Spinner
- Display & Media: Aspect Ratio, Avatar, Card, Carousel, Chart, Data Table, Item, Kbd, Table, Typography
- Misc: Collapsible, Pagination, Range Calendar, Toggle, Toggle Group

Component docs follow the pattern `https://shadcn-svelte.com/docs/components/<name>.md` (kebab-case).

**Migration & dark mode**
- [Svelte 5 migration](https://shadcn-svelte.com/docs/migration/svelte-5.md)
- [Tailwind v4 migration](https://shadcn-svelte.com/docs/migration/tailwind-v4.md)
- [Dark mode (Svelte)](https://shadcn-svelte.com/docs/dark-mode/svelte.md)
