# Before / after

Baseline: successful jobs among the most recent 50 workflow runs before this change.
After: the second live PR iteration (one warm-cache sample per job). Times exclude queue.
These are observations, not a controlled benchmark; runner/network variance remains.

| Job | Before median (s) | After (s) | Change |
|---|---:|---:|---:|
| api-smoke | 82 | 48 | -41% |
| govulncheck | 43 | 14 | -67% |
| monitoring-contracts | 54 | 32 | -41% |
| server-check | 26 | 17 | -35% |
| golangci-lint | 25 | 20 | -20% |
| web-quality | 31.5 | 30 | -5% |
| analyze-go | 76.5 | 61 | -20% |
| analyze-javascript-typescript | 72 | 64 | -11% |
| dependency-review | 7 | 10 | +43% |

All nine jobs passed in both live iterations. Cold smoke cache fill took 114 s; its second run took 48 s.
The audit threshold and all checks were retained. Release parallelization was statically verified, not benchmarked (no existing candidate tags).

Evidence: `before/raw.json`, `iteration-1/raw.json`, `iteration-2/raw.json` contain GitHub URLs and timestamps.
`iteration-2/api-smoke.log` is the uploaded real CI E2E evidence; rerun with `make smoke-api`.
Per-job queue/setup/execution splits appear in each directory’s `summary.md` and `jobs.json`.

Cache usage after iteration 2: 338,503,321 bytes across 11 caches (~339 MB of 10 GB).
