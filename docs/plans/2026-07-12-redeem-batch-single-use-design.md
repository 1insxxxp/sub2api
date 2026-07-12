# Redeem Batch Single-use Design

## Goal

Allow administrators to mark one generated batch of balance, concurrency, or subscription redeem codes as an activity where each account may redeem at most one code from that batch.

## Batch Semantics

- Every click on "Generate" creates an independent batch when the option is enabled.
- All codes created by that operation share a random batch identifier.
- A user who successfully redeems any code in the batch cannot redeem another code from the same batch.
- A later generation operation is a different batch and is unaffected.
- Invitation codes do not support the option because they are consumed during registration before the normal logged-in redemption flow.
- Existing codes and newly generated codes without the option remain unrestricted.
- The batch restriction is immutable after generation.

## Persistence And Concurrency

Add an optional batch identifier to redeem codes. A non-empty identifier means the code belongs to a single-use activity batch.

Add a durable batch claim table containing batch identifier, user identifier, redeemed code identifier, and creation time. Enforce a unique database index on `(batch_id, user_id)`.

During redemption, insert the batch claim inside the same database transaction before consuming the code or granting its benefit. A duplicate unique-key error becomes `REDEEM_BATCH_USER_LIMIT`. If any later redemption step fails, the transaction rolls back both the claim and code consumption.

The claim record intentionally remains independent from redeem-code deletion. Deleting an already used code must not let the same user claim another code from that activity batch.

## API And Service Behavior

The administrator generation request adds `single_use_per_user`. When true, the service creates one random batch identifier and assigns it to every generated code. The backend rejects the option for invitation codes.

Administrator redeem-code responses expose `batch_id` and `single_use_per_user` so the management interface can identify activity codes. User redemption history does not need to expose the internal batch identifier.

When a user has already claimed the batch, the attempted code remains unused and no balance, concurrency, or subscription benefit is granted. The API returns HTTP 409 with code `REDEEM_BATCH_USER_LIMIT`.

## Frontend Behavior

The generation dialog adds a checkbox labeled "活动兑换码，一人限用一次" for balance, concurrency, and subscription codes. It is hidden and reset for invitation codes.

The administrator list shows a compact "一人限用一次" badge beside codes from restricted batches.

The user redemption page handles `REDEEM_BATCH_USER_LIMIT` explicitly. It shows the existing red error toast and inline error text with the localized message "活动兑换码一人限用一次" instead of the generic redemption failure message.

## Compatibility

The new request field defaults to false and existing redeem-code rows have a null batch identifier. Existing API callers, fixed-code administrator integrations, invitation flows, and unrestricted redemption behavior remain unchanged.

## Testing And Verification

- Service tests cover first claim success, second code rejection, different batch success, unrestricted compatibility, transaction rollback, and concurrent attempts.
- Repository/schema tests cover the unique batch-user constraint and persistence after code deletion.
- Administrator handler/API tests cover generation request propagation and invitation validation.
- Frontend tests cover the generation checkbox, request payload, invitation hiding/reset, administrator badge, and localized user error toast.
- Run Ent generation, backend unit tests, frontend tests, type checking, and production build.
- Verify Vite HMR through `http://127.0.0.1:3000` and restart only `sub2api-dev`; PostgreSQL and Redis remain running.
