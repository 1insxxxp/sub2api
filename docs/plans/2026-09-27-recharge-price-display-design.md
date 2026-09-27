# Recharge Price Comparison Display

## Goal

Make temporary recharge promotions visibly communicate the discount by showing the regular price, the current activity price, and the saved amount in both the offer overview and quick recharge buttons.

## Design

- Keep checkout and billing behavior unchanged. The selected `amount` remains the actual activity price sent to checkout.
- Extend the view-level recharge option data with `originalAmount` and a non-negative `savings` value. Normal tiers use the same value for original and current price.
- Extend `AmountInput` with optional display metadata so the existing numeric `amounts` API and model values remain compatible. When metadata is present, a quick button shows the original amount with a strikethrough, the current amount prominently, and the savings label.
- Update the recharge offer overview to use the same comparison: original amount muted and struck through, activity price emphasized, and a localized savings label. Credited balance remains unchanged and prominent.
- Only render discount-specific UI when the activity price is lower than the regular price. Expired, invalid, equal-price, and non-promotional tiers retain the current presentation.
- Add Chinese and English strings for the savings label and the accessible original-price label.

## Testing

- Unit-test the recharge option calculation for original price, activity price, and savings.
- Extend `PaymentView` tests to assert both the overview and quick buttons expose the comparison during an active promotion, while normal tiers do not expose discount metadata.
- Run the focused frontend tests and the frontend type/build checks available in the repository.
