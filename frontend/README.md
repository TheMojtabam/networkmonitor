# PortSleuth frontend

React + Vite + Tailwind frontend for the PortSleuth monitoring daemon.

## Scripts

```bash
npm install
npm run dev        # vite dev server on :5173 (proxies /api and /ws to :1234)
npm run build      # outputs to ../backend/cmd/portsleuthd/web for Go embedding
npm run typecheck  # tsc --noEmit
```

## Architecture

```
src/
  main.tsx          # entry, providers, toaster
  App.tsx           # router + WebSocket subscription
  index.css         # Tailwind + global resets

  pages/            # one file per page
  components/
    layout/         # AppShell, PageHeader, StatCard
    ui/             # shadcn-style primitives (Button, Card, Badge, …)
    charts/         # Recharts wrappers (BandwidthChart, BandwidthBars)

  hooks/
    useWebSocket.ts # auto-reconnecting WS client

  store/
    app.ts          # Zustand store: snapshot, alerts, ws status

  lib/
    api.ts          # typed REST client
    utils.ts        # cn(), formatBytes, formatBps, …

  types/
    index.ts        # mirrors backend Snapshot/Port/Connection JSON
```

## Live data flow

- `useWebSocket` subscribes to `/ws` and pushes each Snapshot into the
  Zustand store via `setSnapshot`.
- Pages read from `useApp((s) => s.snapshot)` and re-render when the
  store updates.
- Heavier requests (full connection list, history series, alert events)
  go through TanStack Query with a short polling interval as a safety
  net in case WebSocket drops.

## Adding a page

1. Create `src/pages/MyPage.tsx`.
2. Add the route to `src/App.tsx`.
3. Add a nav entry in `src/components/layout/AppShell.tsx` if it's
   user-facing.

Use the existing pages as templates — they all follow the same shape:
`PageHeader` → stat row → main `Card`s.

## Theming

All colors live in `tailwind.config.ts` under `theme.extend.colors`.
Edit there; the rest of the app uses those tokens via Tailwind classes
(`bg-bg-surface`, `text-fg-primary`, `border-accent-border`, etc.).

If you want a light theme later: add a `light` variant to the same
tokens and toggle the `class="dark"` on `<html>` (currently hard-coded
in `index.html`).
