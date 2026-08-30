# Admin List Actions Without Refresh Design

## Goal

Keep the current page, filters, scroll position, and visible rows stable after an administrator refreshes an account's upstream billing rate or saves an edited group.

## Current Behavior

- `GroupsView` saves an edit and then unconditionally calls `loadGroups()`. That request sets the table's shared `loading` state, so the existing list disappears or flashes while the full page is fetched again.
- `AccountsView` patches a successful upstream billing probe into the account row, but then asks the compact sorted-list synchronization path to run. When the upstream-rate sort order changes, that path can fall back to the full account-list loader.

## Chosen Design

Use the successful mutation responses as the source for immediate UI state:

- Merge the `AdminGroup` returned by `adminAPI.groups.update` into the matching row. Preserve list-only fields from the existing row if the update response omits them.
- Patch the returned upstream billing snapshot into the matching account row and stop there. Do not re-fetch or reorder the paginated account list after a manual single or batch probe.
- Keep the edited or probed row in its current position. A later explicit list refresh, filter change, sort change, page change, or automatic account-list synchronization can restore server ordering.

This avoids a second network request and therefore preserves page and scroll context. The accepted trade-off is that a row whose edited value affects the active sort or filter may remain in its previous visible position until the next list synchronization.

## Alternatives Considered

1. Re-fetch the list with a silent loading flag. This removes the loading flash but can still reorder rows, change pages, and move the user's scroll target.
2. Re-sort only the current page locally. This looks fresh but is incorrect for server-side pagination because records on other pages are unavailable.
3. Patch the affected row and defer reconciliation. This is the smallest and most predictable behavior, so it is selected.

## Error Handling

Mutation failures leave existing rows untouched and continue using the current translated error notifications. No optimistic write occurs before the API succeeds.

## Testing

- Mount `GroupsView`, edit a group, and assert the returned group data appears without a second `groups.list` call.
- Verify list-only account counts survive a partial update response.
- Assert both single and batch upstream probes patch their snapshots without invoking the sorted-list synchronization helper.
- Run the focused Vitest suites, TypeScript checking, and the production frontend build.
