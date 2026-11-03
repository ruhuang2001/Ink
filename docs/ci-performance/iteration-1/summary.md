# Actions timings

Seconds; queue = workflow creation to job start (includes dependency wait).
Setup includes checkout, tool/dependency setup, services, cleanup and unaccounted overhead.
Execution includes build, tests, lint and security analysis; composite actions may include internal setup.
Medians below include successful jobs only; raw evidence retains failures/cancellations.

| Job | Samples | Queue | Setup | Execution |
|---|---:|---:|---:|---:|
| analyze-go | 1 | 2 | 30 | 54 |
| analyze-javascript-typescript | 1 | 2 | 20 | 39 |
| api-smoke | 1 | 2 | 29 | 85 |
| dependency-review | 1 | 3 | 6 | 2 |
| golangci-lint | 1 | 13 | 11 | 13 |
| govulncheck | 1 | 2 | 8 | 35 |
| monitoring-contracts | 1 | 2 | 8 | 30 |
| server-check | 1 | 2 | 9 | 11 |
| web-quality | 1 | 2 | 12 | 21 |
