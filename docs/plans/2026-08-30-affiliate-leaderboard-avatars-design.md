# Affiliate Leaderboard Avatars Design

## Goal

Show each inviter's configured user avatar in the admin workbench invite leaderboard while preserving a clear fallback for users without an avatar or with an image that cannot be loaded.

## Data Flow

The existing aggregate leaderboard query remains the source of ranking data. It will left join `user_avatars` by inviter user ID and project the stored avatar URL into `AffiliateInviterSummary`. The workbench handler will expose that value as `inviter_avatar_url`, and the frontend API type will carry it into the leaderboard component.

The join is optional so users without an avatar remain eligible for the leaderboard. Ranking, filtering, pagination, and rebate calculations are unchanged.

## Interface

Desktop and mobile leaderboard rows will place a fixed-size circular avatar beside the inviter identity. A non-empty URL renders as an image with `object-cover`. Empty URLs and image load failures render a deterministic initial derived from the username, then email, then user ID.

The layout keeps the existing rank badge and statistics. Avatar dimensions remain fixed so images and fallback text cannot shift the table or mobile cards.

## Error Handling

An invalid or unreachable avatar URL is handled entirely in the component. The failed image is hidden and replaced with the initial fallback without failing the leaderboard request or showing a broken-image icon.

## Verification

- Repository tests verify the summary query joins and projects `user_avatars.url`.
- Handler tests verify `inviter_avatar_url` is included in the workbench response.
- Component tests verify both configured avatars and fallback initials on desktop and mobile.
- Existing affiliate tests, frontend type checking, and the production frontend build remain green.
