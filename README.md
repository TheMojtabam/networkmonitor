# PortSleuth

Modern network + system monitoring panel.

- Backend: Go (REST API + metrics collectors)
- Frontend: React + Tailwind + charts

## Dev

### Backend

```bash
cd backend
go run ./cmd/portsleuthd --listen :1234
```

### Frontend

```bash
cd frontend
npm install
npm run dev
```

## Deploy

See `install/` for one-line installer (install/update/uninstall) and systemd unit.
