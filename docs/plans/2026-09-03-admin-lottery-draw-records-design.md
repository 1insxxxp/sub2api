# Admin Lottery Draw Records Design

**Goal:** Make the admin lottery page show who drew, when they drew, and what each draw produced, with an optional winners-only filter.

**Scope:** Extend the existing admin lottery draw endpoint and add a paginated records section to the existing admin lottery page. Keep the current lottery configuration and attempt-management flows intact.

## Design

- Reuse `GET /admin/lottery/draws` as the single source for audit records.
- Return the existing draw fields plus the already-supported user identity and attempt source fields: user email/name, prize name/type, balance amount or product content, activity versus reward-wallet source, and creation time.
- Add optional server-side filters for `user_id`, `prize_type`, `attempt_source`, and `winners_only`. Pagination remains server-side using the existing `page` and `page_size` convention.
- Define a winner as a draw whose prize type is not the configured no-win type. The API should apply this consistently so the UI does not infer business rules from labels.
- Add a records section below the configuration area with a compact responsive table on desktop and stacked rows on narrow screens. Show all records by default, with a segmented filter for all draws versus winners only and a refresh action.
- Keep empty states explicit: distinguish “no records yet” from “no winners match the filter.” Preserve deleted-user indication using the existing `user_deleted` field.

## Data Flow

1. The admin page requests the current lottery configuration and the first draw page independently.
2. Filter changes reset the page to 1 and request the filtered draw page.
3. Pagination requests only the selected page; no full-page refresh is needed.
4. The handler passes query filters to the service, which delegates filtering and ordering to the repository.

## Error Handling

- Invalid enum filters return the project’s normal bad-request response.
- Failed list requests show the existing admin error toast and leave the previous rows visible when possible.
- Missing optional user or prize data is rendered with a localized placeholder rather than breaking the row.

## Testing

- Backend: service/repository tests cover default all-record listing, winners-only filtering, source/type filters, stable newest-first ordering, and pagination.
- API: handler test verifies query parameters are parsed and passed through, including invalid values.
- Frontend: API test verifies query serialization; page test verifies loading, filtering, pagination, empty state, and display of user/prize/source data.

