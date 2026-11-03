# Actions timings

Seconds; queue = workflow creation to job start (includes dependency wait).
Setup includes checkout, tool/dependency setup, services, cleanup and unaccounted overhead.
Execution includes build, tests, lint and security analysis; composite actions may include internal setup.
Medians below include successful jobs only; raw evidence retains failures/cancellations.

| Job | Samples | Queue | Setup | Execution |
|---|---:|---:|---:|---:|
| analyze-go | 1 | 3 | 21 | 40 |
| analyze-javascript-typescript | 1 | 3 | 25 | 39 |
| api-smoke | 1 | 2 | 32 | 16 |
| dependency-review | 1 | 3 | 5 | 5 |
| golangci-lint | 1 | 3 | 12 | 8 |
| govulncheck | 1 | 3 | 9 | 5 |
| monitoring-contracts | 1 | 2 | 8 | 24 |
| server-check | 1 | 3 | 8 | 9 |
| web-quality | 1 | 2 | 10 | 20 |
