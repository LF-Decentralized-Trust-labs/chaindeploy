# ChainLaunch Dashboard — Page Layout Design System

Authoritative rules for spacing, sizing, and structure of dashboard pages in
`chaindeploy/web/src/pages/`. Every new page **must** follow these rules.
Every existing page should converge on them over time.

> If you're reaching for a different padding/margin/max-width than what's
> listed here, revisit the page structure first — you probably don't need it.

---

## 1. Shell structure (two-element rule)

Every dashboard page is composed of **two concentric elements**:

```tsx
// 1. Outer — handles vertical padding. Full-bleed to the sidebar edge.
// 2. Inner — handles max-width, centering, horizontal padding.
<div className="flex-1 py-8">
  <div className="mx-auto w-full max-w-6xl px-4 sm:px-6 lg:px-8">
    {/* page content */}
  </div>
</div>
```

**Rules:**

- Never combine `p-8` (all-sides padding) on the outer wrapper — split vertical
  (outer) and horizontal (inner) so max-width can apply correctly.
- Never use `container mx-auto py-6` on dashboard pages — it's inconsistent
  with the two-element pattern. Use the shell above.
- Never nest a second `max-w-*` inside the inner — child grids/lists fill the
  inner's width. The only exception is self-contained units intended to feel
  bounded (centered forms, pricing cards, comparison tables, modals).

### Use the `<PageShell>` component

For any new page, import the shared shell instead of hand-writing the wrapper:

```tsx
import { PageShell, PageHeader } from '@/components/layout/page-shell'

<PageShell>
  <PageHeader title="Nodes" description="Manage your blockchain nodes">
    <Button>Create Node</Button>
  </PageHeader>
  {/* sections */}
</PageShell>
```

---

## 2. Max-width tokens

Pick one based on content density. Do not invent new widths.

| Token                | Class       | Use for                                                                 |
| -------------------- | ----------- | ----------------------------------------------------------------------- |
| `form` (default form pages)   | `max-w-2xl` | Single-column forms (node create, key create, login).          |
| `detail`             | `max-w-4xl` | Detail/settings pages (org detail, settings, import wizards).           |
| `dashboard` (default)| `max-w-6xl` | List/index dashboard pages (nodes, networks, keys, organizations).     |
| `wide`               | `max-w-7xl` | Multi-column grids, metrics overview, platform analytics.               |
| `full`               | `max-w-none`| Full-bleed tables, editors, explorers.                                  |

---

## 3. Vertical padding (outer wrapper)

Single token: `py-8`.

- No `pt-6` asymmetries. No `p-4 md:p-8` mixing. No responsive bumps.
- The horizontal padding belongs to the inner (see §1); `py-8` is the single
  source of truth for top/bottom breathing room.

## 4. Horizontal padding (inner wrapper)

Single token: `px-4 sm:px-6 lg:px-8`.

- Covers mobile (16px), small-tablet (24px), and desktop (32px) in one class.
- No arbitrary px values. No `px-8` across all breakpoints (too tight on mobile).

---

## 5. Page header

Every page has one and only one `<h1>`. Use `PageHeader` or match this markup:

```tsx
<div className="mb-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
  <div>
    <h1 className="text-2xl font-semibold tracking-tight">Nodes</h1>
    <p className="mt-1 text-sm text-muted-foreground">
      Manage your blockchain nodes
    </p>
  </div>
  <div className="flex gap-2">{/* actions */}</div>
</div>
```

**Rules:**

- Title: `text-2xl font-semibold tracking-tight`. Never `text-3xl font-bold`.
  Never responsive scaling (`text-2xl md:text-3xl`) — the sidebar already makes
  the content zone narrow enough that 2xl reads well at every breakpoint.
- Description: `text-sm text-muted-foreground mt-1`.
- Spacing below header: `mb-8` (section-level break).
- Actions align right on `sm+`, stack below title on mobile.
- No icons next to the title. Icons go on action buttons, not headings.

---

## 6. Vertical rhythm inside a page

Use one of three spacing scales consistently within a page — don't mix:

| Scale           | Class        | Use for                                                        |
| --------------- | ------------ | -------------------------------------------------------------- |
| **Dense**       | `space-y-4`  | Rows in a list, fields in a form.                              |
| **Section**     | `space-y-6`  | **Default.** Stacked sections inside a page.                   |
| **Chapter**     | `space-y-8`  | Very distinct blocks (stats strip → tabs → list).              |

- Prefer `space-y-*` on the parent over `mb-*` on children (flex/grid siblings
  should never have ad-hoc margins; use `gap-*`).
- Within a section, use `space-y-4` between title and body.

### Section headings (h2)

```tsx
<section className="space-y-4">
  <h2 className="text-lg font-semibold">Networks</h2>
  {/* body */}
</section>
```

- `text-lg font-semibold` — never `text-xl`, never `font-bold`.
- No responsive scaling on section headings.

---

## 7. Grids

- Card grids: `grid gap-4 sm:grid-cols-2 lg:grid-cols-3`.
- Stats strips: `grid grid-cols-2 sm:grid-cols-4 gap-4` (or borderless `dl`
  pattern used on the Nodes page — keep that pattern if already there).
- Gap scale: `gap-4` for dense cells, `gap-6` for airy cards. Never `gap-3`,
  `gap-5`, `gap-7`.
- Never add a `max-w-*` to a grid/list — it should fill the inner wrapper.

---

## 8. Cards

- Padding: use the `Card` component's default (`p-6` via shadcn) — don't
  override unless the card hosts a table or editor.
- Inside a card, use `space-y-4` between blocks.
- Card grids use `gap-4` (dense) or `gap-6` (featured/marketing).

---

## 9. Empty states

Empty states share the same shell but re-center content:

```tsx
<PageShell maxWidth="detail">
  <div className="py-12 text-center">
    <Icon className="mx-auto mb-4 h-12 w-12 text-muted-foreground" />
    <h1 className="text-2xl font-semibold tracking-tight">Create your first node</h1>
    <p className="mt-2 text-sm text-muted-foreground">
      Get started by creating a blockchain node.
    </p>
  </div>
  <div className="space-y-4">{/* option cards */}</div>
</PageShell>
```

- Icon: `h-12 w-12`, centered, `mb-4`.
- Use `detail` (max-w-4xl) max-width for empty states — `dashboard` feels too
  wide for a CTA-first layout.

---

## 10. Tabs

- `<TabsList>`: default (no overrides to width or padding).
- Gap between tab row and body: handled by `<Tabs className="space-y-6">`.
- Never mix responsive text classes on `<TabsTrigger>` (`text-xs sm:text-sm`) —
  keep one size. Truncate or shorten labels if they're too long on mobile.

---

## 11. What NOT to do

- ❌ `container mx-auto py-6` — inconsistent with two-element shell.
- ❌ `flex-1 space-y-4 p-8 pt-6` — mixes padding and rhythm; breaks the shell.
- ❌ `max-w-4xl mx-auto` on a list page — list pages use `dashboard`/`wide`.
- ❌ Arbitrary `px-[…]`, `py-[…]`, `mt-7`, `gap-5`.
- ❌ Responsive heading scaling on pages already constrained by the sidebar.
- ❌ Icons inside stat/metric card titles (see `design-guidelines/dashboards`).
- ❌ Multiple `<h1>` per page.
- ❌ Margins between flex/grid siblings (`mb-*`) — use `gap-*` on parent.

---

## 12. Quick reference

```tsx
import { PageShell, PageHeader } from '@/components/layout/page-shell'

export default function MyDashboardPage() {
  return (
    <PageShell>
      <PageHeader
        title="My Page"
        description="Short description of what this page does"
      >
        <Button>Primary action</Button>
      </PageHeader>

      <div className="space-y-8">
        <section className="space-y-4">
          <h2 className="text-lg font-semibold">Section title</h2>
          {/* content */}
        </section>

        <section className="space-y-4">
          <h2 className="text-lg font-semibold">Another section</h2>
          {/* content */}
        </section>
      </div>
    </PageShell>
  )
}
```

---

## 13. File checklist for any new page

- [ ] Wrapped in `<PageShell>` (or two-element shell with `py-8` outer +
      `max-w-* mx-auto px-4 sm:px-6 lg:px-8` inner).
- [ ] Exactly one `<h1>` rendered via `<PageHeader>`.
- [ ] Title is `text-2xl font-semibold tracking-tight` (no responsive bumps).
- [ ] Max-width picked from §2 table (default: `dashboard` / `max-w-6xl`).
- [ ] Sections separated with `space-y-6` (default) or `space-y-8` (chapters).
- [ ] Grids use `gap-4` or `gap-6` — no other gaps.
- [ ] No `p-4 md:p-8`, no `pt-6`, no `container mx-auto py-6`.
- [ ] No margin-based spacing between flex/grid siblings.
- [ ] `usePageTitle('…')` called at the top of the component.
