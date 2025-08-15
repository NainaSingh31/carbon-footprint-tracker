# Carbon Footprint Tracker

A minimal full‑stack project to log activities and estimate your carbon footprint.

- **Backend**: Go (Gin + GORM + SQLite)
- **Frontend**: Vanilla HTML/CSS/JS + Chart.js (CDN)

## Prerequisites
- Go 1.22+ installed

## Run (Backend)
```bash
cd backend
go mod tidy
go run main.go
```
The API will start at `http://localhost:8080`.

## Run (Frontend)
Open `frontend/index.html` in your browser (double‑click) **or** serve it with a simple static server on port 5500/5173/any.
The frontend calls the backend at `http://localhost:8080`.

## API Quick Test
```bash
# Add an activity (20 km by car on 2025-08-10)
curl -X POST http://localhost:8080/api/activities \
  -H "Content-Type: application/json" \
  -d '{"category":"transport","type":"car","quantity":20,"unit":"km","date":"2025-08-10"}'
```

## Activity Types & Units
- `transport`:
  - `car|bus|train|bike|walk|flight` with `quantity = distance_km`, `unit = km`
- `energy`:
  - `electricity` with `quantity = kWh`, `unit = kwh`
  - `lpg` with `quantity = kg`, `unit = kg`
- `food` (per day entries):
  - `meat_heavy_day`, `vegetarian_day`, `vegan_day` (unit can be `day`)
- `shopping`:
  - `quantity = amount_spent`, any currency (unit optional)
- `other`:
  - Provide direct `quantity` measured in `kgco2e`

> Emission factors are illustrative and simplified for a student project. Replace with official factors as needed.

## Project Structure
```
carbon-footprint-tracker/
  backend/
    main.go
    go.mod
  frontend/
    index.html
    app.js
    styles.css
  README.md
```

## Notes
- This demo skips authentication for simplicity.
- SQLite database file `app.db` is created in `/backend` at first run.
- You can adjust factors in `computeEmission` inside `main.go`.
