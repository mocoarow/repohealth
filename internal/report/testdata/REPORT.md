# Repository health

Snapshot of abc123 committed at 2026-03-03T10:11:12+05:45, taken at 2026-03-04T05:06:07+05:45 (repohealth 1.2.3, Biome 2.3.4).

## Areas

| Area | Go lines | Go test lines | Go functions | Go cyclo max | Go cognit sum | Go cognit max | Go over limit | TS lines | TS test lines | TS functions (cognit ≥ 2) | TS cognit sum | TS cognit max | TS over limit | Coverage |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| api | 400 | 200 | 40 | 5 | 28 | 6 | 4 | 120 | 80 | 12 | 20 | 7 | 8 | 80.0% |
| web | 300 | 150 | 30 | 4 | 21 | 5 | 3 | 90 | 60 | 9 | 15 | 6 | 7 | - |

## Trends

### Coverage by area

| Committed | Rev | api | web |
| --- | --- | --- | --- |
| 2026-03-01T09:00:00+05:45 | r1 | 50.0% | - |
| 2026-03-02T09:00:00+05:45 | r2 | 66.7% | - |
| 2026-03-03T10:11:12+05:45 | abc123 | 80.0% | - |

### Go cognitive complexity (sum)

| Committed | Rev | api | web |
| --- | --- | --- | --- |
| 2026-03-01T09:00:00+05:45 | r1 | 7 | - |
| 2026-03-02T09:00:00+05:45 | r2 | 14 | - |
| 2026-03-03T10:11:12+05:45 | abc123 | 28 | 21 |

### TS cognitive complexity (sum)

| Committed | Rev | api | web |
| --- | --- | --- | --- |
| 2026-03-01T09:00:00+05:45 | r1 | 5 | - |
| 2026-03-02T09:00:00+05:45 | r2 | 10 | - |
| 2026-03-03T10:11:12+05:45 | abc123 | 20 | 15 |

## Cross-area changes

### Months

| Month | PRs | Cross-area | Cross-area (backend) | Cross-area (web) |
| --- | --- | --- | --- | --- |
| 2026-02 | 8 | 3 | 2 | 1 |
| 2026-03 | 5 | 4 | 3 | 4 |

### Pairs

| Areas | PRs |
| --- | --- |
| api + core | 4 |

## Dependencies

### Area imports

| From | To | Imports |
| --- | --- | --- |
| api | core | 6 |

### Stability

| Package | Afferent | Efferent | Instability |
| --- | --- | --- | --- |
| `internal/api` | 2 | 3 | 0.600 |

## Hotspots

| File | Score | Churn | Recent churn | Cognit sum | Cognit max |
| --- | --- | --- | --- | --- | --- |
| `internal/api/handler.go` | 153 | 9 | 5 | 17 | 12 |

## Functions

| Lang | Function | File | Line | Cognit | Cyclo |
| --- | --- | --- | --- | --- | --- |
| go | `Handle` | `internal/api/handler.go` | 42 | 12 | 6 |
| ts | `render` | `web/app.ts` | 7 | 4 | - |

## Coverage

### Layers

| Area | Layer | Covered | Lines | Coverage |
| --- | --- | --- | --- | --- |
| api | `internal` | 8 | 10 | 80.0% |

### Reports

| File | Modified | Skipped |
| --- | --- | --- |
| `coverage/lcov.info` | 2026-03-02T01:02:03+05:45 | 7 |

## Largest files

| File | Area | Lang | Lines |
| --- | --- | --- | --- |
| `web/app.ts` | web | ts | 321 |
