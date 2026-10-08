# Workspace synchronization

`GET` and `PUT /api/v1/workspace` require a valid bearer session and use the
authenticated account's snapshot. Printer history remains in `print_jobs`;
the workspace's deprecated `printJobs` field stays empty.

GET returns the workspace fields plus a positive integer `revision`. PUT
sends the same fields with the revision last received from GET or a successful
PUT. The database compares that revision atomically before replacing the
snapshot, then returns the saved state with the incremented revision.

Example: two tabs load revision 7. The first PUT succeeds and returns revision
8. The second PUT using revision 7 receives HTTP 409, code `workspace_conflict`;
it cannot overwrite the first tab's saved state. Concurrent first reads create
the empty snapshot once and never replace another request's saved data.

- Missing or nonpositive revision: HTTP 428, `workspace_revision_required`.
- Outdated revision: HTTP 409, `workspace_conflict`.
- Failed saves do not advance the revision.

The web client keeps the current page's draft on conflict and pauses automatic
saves. Its error area offers **Reload workspace**, with confirmation that this
discards unsaved changes. **Download local draft JSON** exports the current
workspace content without authentication credentials or a network save. Users
can download or copy their draft before reloading. Page editing is disabled
while the confirmed reload is in progress. Normal
network errors offer **Retry**, using the existing revision. There is no
automatic content merge or forced overwrite.

Deployment requires migration `010_workspace_revision.sql` and the frontend
that carries revision. Apply migrations before starting the updated API, then
deploy the frontend with it. Older clients without revision receive 428 rather
than silently overwriting a newer snapshot; they must reload the updated web
app. Existing snapshot content is preserved by the migration.
