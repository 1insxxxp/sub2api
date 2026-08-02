# Check-in Trigger Interaction Design

## Goal

Restore one-click check-in while keeping reward details available and preventing a deliberately opened popover from closing on pointer movement.

## Interaction Rules

- When check-in is available, clicking the header button immediately submits the check-in.
- After a successful submission, the reward-details popover remains open.
- When the user is already checked in or is ineligible, clicking the header button opens details without submitting.
- Clicking the trigger pins the popover open. A pinned popover closes only on outside click or Escape.
- Desktop pointer hover may temporarily preview details. Leaving the component closes only an unpinned hover preview.
- Focus may preview details, but focus movement must not override a pinned open state.

## State Model

Track explicit/pinned open state separately from transient pointer/focus preview state. Render the popover whenever either state is true. Submission success pins the popover. Outside click, Escape, and user changes clear both states.

## Error Handling

If check-in submission fails, keep the details popover open so the user can see the current eligibility/reward context and the existing error notification. Loading/submitting continues to disable repeat submissions.

## Testing

- An eligible trigger click submits exactly once and opens the popover.
- A checked-in or ineligible trigger click opens without submitting.
- Mouse leave closes hover-only preview.
- Mouse leave does not close a click-pinned popover.
- Outside click and Escape close a pinned popover.

## Scope

Local frontend interaction change only. No API, database, server deployment, or production configuration change.
