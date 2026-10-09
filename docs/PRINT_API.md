# Print job reads and synchronization

All routes require `Authorization: Bearer <accessToken>` and restrict records
to the authenticated account. Read endpoints use stored state and never call
the printer provider. Deploy the updated frontend and API together: list
responses are now bounded summaries rather than full-content history.

## List summaries

`GET /api/v1/print-jobs?status=all&limit=20&cursor=<nextCursor>`

- `status`: `all`, `active` (pending or queued), or `history` (completed,
  failed, or cancelled).
- `limit`: defaults to 20, maximum 100.
- `cursor`: opaque cursor from the previous response. Keep the same filter
  when continuing a page.

```json
{
  "printJobs": [{
    "id": "print-example",
    "title": "Reminder",
    "source": "Manual",
    "deviceId": "device-example",
    "status": "queued",
    "createdAt": "2026-10-01T00:00:00Z",
    "updatedAt": "2026-10-01T00:00:00Z"
  }],
  "nextCursor": null
}
```

Summaries omit `content`. Ordering is descending creation time, then ID;
status checks do not move the pagination key. Status transitions can change
filter membership, so refresh the first page and restart the affected cursor
when global counts change. Merge refreshed pages by job ID.

## Read content for preview

`GET /api/v1/print-jobs/{jobId}` returns `{ "printJob": ... }` with the full
job, including `content`. Another account's job returns not found.

## Poll stored status

`GET /api/v1/print-jobs/status?ids=id1,id2&since=2026-10-01T00%3A00%3A00%2B08%3A00`

The optional `ids` parameter accepts at most 100 job IDs. Each returned entry
contains `id`, `status`, `deviceId`, and `updatedAt`, with no content. Omit
`ids` when only counts and new-job discovery are needed. `since` is an RFC3339
timestamp used for `todayCompleted`; clients pass the start of their local
day, with its timezone offset. If omitted, the server uses the start of the
current UTC day.

Use `metadata=false` for additional ID chunks in the same poll to omit `counts`
and avoid repeating the account-wide aggregate. `latestJobId` is null in these
responses and must be ignored. The default includes metadata.

```json
{
  "printJobs": [],
  "counts": {
    "pending": 0, "queued": 1, "completed": 500,
    "failed": 0, "cancelled": 0, "todayCompleted": 2
  },
  "latestJobId": "print-example"
}
```

Counts cover the whole account, independently of loaded pages. A changed
`latestJobId` lets the browser discover schedule-created jobs. Polling with
no queued jobs still discovers changes; hidden pages pause polling. Clients
must discard responses from a previous account and preserve a newer job
version when overlapping requests finish out of order.

## Background completion

After a print submission is accepted, the API returns `queued`. The status
worker checks due jobs in bounded serial batches, persists completion, and
backs off on provider failures. Completion is eventually visible through
the status endpoint even when no browser is open. Failed status requests do
not mark an accepted print job as a failed print or resubmit its content.

The worker updates only the same observed queued provider job/version.
Cancellation or resubmission while a provider call is outstanding prevents
the old result from overwriting the newer state. Persisted next-check times
survive API restarts. Due rows are atomically leased with `SKIP LOCKED` before
provider calls, so multiple API processes do not poll the same claim. An expired
lease can be reclaimed after a worker exits; late results cannot replace a newer
claim. Queued rows written by an older API without a next-check time are also
eligible for reconciliation.

If a process exits before persisting the submission result, a queued job without
a provider ID becomes failed after three minutes. Its summary and detail expose
an optional `errorMessage` instructing the user to check the device first.
Automatic delivery retries never resubmit these uncertain jobs. Other internal
provider errors remain private. Existing content and history are preserved.

In the print view, active jobs and history load independently in pages of 20.
Each section offers **Load more** when another page exists; **Preview** retrieves
one full job and then renders its PNG. Guest printing keeps its local content
and existing preview flow. See [self-hosting settings](SELF_HOSTING.md) for
worker intervals and timeouts.

`GET` and `PUT /api/v1/workspace` return `printJobs: []`; workspace saves ignore
that deprecated duplicate field. Print history and content remain in the
printer repository and are accessed through the endpoints above. Workspace
saves require the current revision; see [Workspace synchronization](WORKSPACE_API.md).

## Content limits

Preview and physical printing share the same validation and PNG renderer.
Titles allow at most 512 UTF-8 bytes; bodies allow at most 64 KiB; title and
body together allow at most 8,000 Unicode code points. Wrapped output is
limited to 384 × 8,192 pixels. Excessive runs of zero-width characters are
also rejected to bound layout computation. These checks happen before image
allocation, and content is never silently truncated.

Print-content limit violations return HTTP 413, `print_content_too_large`. This applies to
plain text, flattened plugin blocks, new jobs and previously stored pending
jobs when submitted. Existing jobs rejected before submission remain pending,
so they can still be cancelled or assigned to another device.

JSON request bodies have a separate 1 MiB limit; exceeding that limit returns
HTTP 413, `request_too_large` before print-content validation. Feedback printing
also uses the print-content limit response.
