# Print data flow performance baseline

`server/scripts/print_performance.py` measures real HTTP handlers, authentication,
PostgreSQL queries, and PNG generation. It builds an archived baseline and a
frozen copy of the candidate source, creates its own disposable PostgreSQL 16
containers, and serves a local Memobird status stub. It does not use the development
database, `.env`, account credentials, or a physical printer. Containers and API
processes are cleaned up on success and failure.

From the repository root, with Docker running, Python 3 and the repository's Go
toolchain installed:

```sh
python3 server/scripts/print_performance.py \
  --baseline-ref 39026e74aa7a0f50f908684006f335345d278bea \
  --assert-improvement --verify-store-tests \
  --output /tmp/ink-print-performance.json

python3 server/scripts/print_performance.py \
  --baseline-ref 39026e74aa7a0f50f908684006f335345d278bea --provider-delay-ms 0 --core-only \
  --assert-improvement --output /tmp/ink-print-performance-zero-delay.json
```

Fresh reruns use the stable pre-optimization commit on `main` above. The historical
2026-10-01 reports retain their original PR-internal baseline `4399d13` and candidate
`f08952b`; those exact reruns require a checkout retaining those Git objects.

Each route has 3 warmup requests and 30 measured sequential requests by default.
p95 uses nearest rank, including reading the complete response body. The fixture
contains 1,000 print jobs, 20 queued jobs with provider IDs, and 4,096 ASCII bytes
of content per job. The workspace fixture deliberately duplicates this history,
matching the old snapshot behavior; other workspace collections remain small.
Record the machine, container image ID, Go version, source SHA-256, and raw request
samples when comparing subsequent changes. Run measurements without competing
builds, tests, or workloads. `--verify-store-tests` runs PostgreSQL integration tests
with the race detector against the same disposable container, outside the latency
sampling phase.

The default stub adds 20 ms per status call. This is a controlled experiment, not
a measurement of production Memobird latency. The zero-delay experiment isolates
the local data-flow costs. The old API returns all jobs and their content even
when `limit=20` is sent; the new API returns the first 20 summaries. Thus the list
comparison measures the first-screen workflow rather than equal full-history
responses. The script also traverses every new summary page, verifies complete
and unique coverage, and retrieves a full-content detail to check that pagination
has not discarded data.

Foreground samples disable the new status worker to attribute provider calls to
read requests. List and status reads must trigger zero provider calls. The
completion experiment enables the worker with its production defaults: 2 s
scan interval, batch size 20, and a 10 s healthy per-job recheck. After at least
one "not yet printed" response, the stub changes to completed while no browser reads
are made. Database polling verifies eventual completion. This moves provider
traffic to the background; it does not imply zero total provider traffic.

The due-schedule experiment uses a 2 s scheduler interval for both versions and
confirmation mode, so it measures due time to local job creation without physical
printing. Its delay comes from PostgreSQL timestamps, independent of the observer's
Docker/psql overhead. It is a single timing sample, not a latency distribution;
the production schedule cadence remains 30 s.

Rendering uses a fresh API process for each input size, one warmup, and three
measured PNG requests. RSS is sampled with `ps` at a target 20 ms cadence; process
scheduling and `ps` execution can widen that interval. The observed maximum is a
sampled process RSS, not an exact peak, renderer-only allocation, or `allocs/op`.
The report includes pre-render RSS, observed peak, and their difference. PNG
signatures and SHA-256 hashes must match across versions. The renderer is unchanged,
so these figures establish a baseline and do not demonstrate a rendering optimization.

Current reruns render 486-byte and 999-byte inputs within the image-height limit,
and separately require an oversized preview to return `print_content_too_large`.
The historical results below used the earlier 4,077-byte fixture before render
limits were introduced; they are preserved as measurements of that revision.

`--assert-improvement` requires the candidate list p95 to be less than half the
baseline and both list and workspace responses to contain less than one tenth
as many bytes. Those gates are intended for the documented fixture; very small
custom fixtures may not meet them even when the API contract is correct. The
script also fails when pagination includes content, omits or repeats jobs, a
detail loses its content, a read contacts the provider, or background completion
does not occur.

## Results — 2026-10-01

Both experiments passed their foreground improvement gates and data-contract checks.
The full run additionally verified rendering, schedule delivery, and background
completion. The `--core-only` zero-delay run did not run those additional checks. The machine
was macOS 27 on arm64, with API binaries built by Go 1.26.6 and PostgreSQL 16.
The host Go launcher was 1.26.5; the module automatically selected 1.26.6. Raw
samples, the Docker image digest, and frozen source hashes are preserved in
[the complete run](performance/print-2026-10-01.json) and
[the zero-delay run](performance/print-2026-10-01-zero-delay.json).
The candidate implementation is committed as `f08952b`; measurements preceded
the commit, so the raw metadata retains its original dirty-tree HEAD. Its server
source fingerprint matches the committed implementation.

| Metric | Baseline `4399d13` | Candidate | Change |
| --- | ---: | ---: | ---: |
| First-page list p95, 20 ms stub | 650.84 ms | 11.05 ms | 98.30% lower |
| First-page list p95, zero-delay stub | 25.62 ms | 3.35 ms | 86.94% lower |
| List response body | 4,294,849 bytes | 4,057 bytes | 99.91% smaller |
| Workspace response with duplicated history | 4,319,218 bytes | 386 bytes | 99.991% smaller |
| Provider calls triggered by 30 list reads | 600 | 0 | Read traffic no longer amplifies status calls |

The new lightweight status response for 20 IDs contained 2,324 bytes and had a
4.26 ms p95 in the zero-delay experiment, with zero provider calls. All 1,000
summary rows were retrieved in 10 pages of 100 with no missing or duplicate IDs,
no content fields, and zero provider calls. Full detail still returned the
original 4,096-byte body. PostgreSQL migration, pagination, status compare-and-swap,
workspace projection, and delivery integration tests passed with `-race`.

With no browser reads, the old version made no provider calls and left all 20
jobs queued throughout its 2.63 s observation window. The candidate first
received 20 "not printed" responses, then completed all 20 jobs 11.96 s after
the stub changed to "printed". It made 40 background calls in total: 20 initial
checks and 20 completion checks. This verifies synchronization independently of
browser polling; the 10 s recheck delay plus the 2 s scan cadence remains visible.

The single due-to-local-job measurement was 962.43 ms before and 1,373.42 ms after
with the same accelerated 2 s scheduler cadence. This is a timing baseline and
does not establish a scheduling improvement. The production scheduler interval
has not changed. Physical printer completion was simulated, so real printer
latency, printer availability, and production multi-user throughput remain unmeasured.

| Render input | Version | Pre-render RSS | Observed maximum RSS | Observed increase |
| --- | --- | ---: | ---: | ---: |
| 486 bytes | Baseline | 24.33 MiB | 31.89 MiB | 7.56 MiB |
| 486 bytes | Candidate | 24.23 MiB | 31.48 MiB | 7.25 MiB |
| 4,077 bytes | Baseline | 50.19 MiB | 98.44 MiB | 48.25 MiB |
| 4,077 bytes | Candidate | 49.81 MiB | 97.39 MiB | 47.58 MiB |

PNG bytes matched across versions for every measured request. These sampled
memory figures are consistent with an unchanged renderer and justify retaining
rendering as a candidate for profiling. They do not measure exact peak memory
or prove a rendering improvement. Rendering latency varied substantially between
the runs, so it is not used as an improvement claim.

## Work identified at measurement time

This batch establishes paging, lightweight local status reads, background status
synchronization, and removal of duplicated print history from workspace reads.
It also exposed and corrected a PostgreSQL `CASE` parameter-type error that unit
test fakes did not catch; completion and real-database checks were rerun after
the correction.

Broader workspace convergence and snapshot version conflicts, job claims and
leases, bounded execution concurrency, frontend domain-store separation, plugin
batch inserts, rendering changes, and additional SQL index tuning remain separate
work. This experiment does not rank those candidates as production bottlenecks.
The later PR #86 review implements workspace revision conflicts and recoverable
status/submission claims; the original measurements do not validate those changes.
Before changing them, collect production list p95 and body sizes, total provider
calls from worker telemetry, schedule due-to-created delays, and render profiles
on representative inputs and concurrency.
