# Recharge Promotion Price Tiers Design

## Goal

Allow a time-limited recharge promotion to change the cash price of a fixed
credited balance without changing the credited amount. For example, keep the
normal tier `10 CNY -> 50 balance`, while the active promotion can configure
`50 balance -> 8 CNY`.

## Scope

- Reuse the existing balance recharge promotion and its time window,
  eligibility, and blacklist behavior.
- Add optional per-tier promotional prices keyed by credited balance.
- Keep the existing base recharge tiers unchanged and preserve the existing
  multiplier-based promotion format for backward compatibility.
- Promotional price tiers take precedence over the legacy promotion multiplier
  for matching amounts.
- Expired, inactive, unmatched, or blacklisted requests continue using the
  existing base recharge pricing and credit calculation.

## Data Model

Extend the stored promotion JSON with an optional list:

```json
{
  "enabled": true,
  "start_at": "2026-10-01T00:00:00+08:00",
  "end_at": "2026-10-08T00:00:00+08:00",
  "price_tiers": [
    { "credited_amount": 50, "price": 8 }
  ]
}
```

`credited_amount` identifies the existing base tier's credited balance and
`price` is the amount the customer pays during the promotion. The price must
be positive. Each credited amount must be unique and must correspond to a base
recharge tier, so a promotion cannot introduce an unapproved balance amount.

The legacy `multiplier` field remains readable and writable for compatibility.
An enabled promotion is valid when it has either a positive multiplier or at
least one valid price tier. When a price tier matches, the order uses its price
as the request amount and credits the tier's fixed credited balance. Otherwise
the existing multiplier behavior remains in effect.

## Data Flow

1. The admin settings page loads base recharge tiers and the promotion.
2. The promotion editor renders one optional activity-price input for each
   base tier, showing the credited balance derived from the base tier.
3. Checkout returns effective promotional quick amounts for the current user;
   the customer sees the promotional price while the selected credited amount
   remains attached to that option.
4. Order creation resolves the selected promotional price server-side,
   validates payment limits against the price, and stores the original order
   amount plus the fixed credited amount needed for fulfillment.
5. Ineligible users, expired activities, and unmatched prices use the current
   base-tier path without changing existing orders.

## Compatibility and Safety

- Existing promotion JSON without `price_tiers` behaves exactly as before.
- Existing base tier JSON remains readable; no database migration is needed.
- The backend is authoritative for both price eligibility and credited amount;
  frontend values are previews only.
- Payment provider fees and limits apply to the actual promotional price.
- Refund calculation continues to use the order's stored payment and credited
  amounts, so historical orders are unaffected by later settings changes.

## Verification

- Validate promotion price tiers, uniqueness, positivity, and base-tier
  references.
- Test active, expired, unmatched, and blacklisted users.
- Test checkout display and order creation for `50 -> 8` while crediting 50.
- Test legacy multiplier promotions and base pricing remain unchanged.
- Test admin settings round-trip and frontend editor behavior.
