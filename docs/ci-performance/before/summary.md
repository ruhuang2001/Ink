# Actions timings

Seconds; queue = workflow creation to job start (includes dependency wait).
Setup includes checkout, tool/dependency setup, services, cleanup and unaccounted overhead.
Execution includes build, tests, lint and security analysis; composite actions may include internal setup.
Medians below include successful jobs only; raw evidence retains failures/cancellations.

| Job | Samples | Queue | Setup | Execution |
|---|---:|---:|---:|---:|
| Dependabot | 3 | 4 | 4 | 148 |
| analysis | 3 | 4 | 7 | 32 |
| analyze-go | 12 | 3 | 7 | 69 |
| analyze-javascript-typescript | 12 | 3 | 7 | 63 |
| api-smoke | 7 | 3 | 30 | 50 |
| dependency-review | 7 | 2 | 5 | 1 |
| golangci-lint | 7 | 3 | 19 | 4 |
| govulncheck | 7 | 3 | 11 | 34 |
| monitoring-contracts | 3 | 2 | 8 | 35 |
| server-check | 7 | 2 | 15 | 12 |
| validate-pr | 1 | 11 | 5 | 11 |
| web-quality | 10 | 3 | 9 | 23 |
