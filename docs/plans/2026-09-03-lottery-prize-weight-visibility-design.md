# Lottery Prize Weight Visibility Design

## Goal

Hide prize probability weights from the user-facing lottery prize description cards while preserving weights for administrator configuration and server-side draw selection.

## Scope and approach

The user page currently renders the prize's `weight` beside the balance amount or product inventory. Remove only that presentation element from `frontend/src/views/user/LotteryView.vue`. Keep the public API model, admin form, translations, and draw algorithm unchanged so the weight remains available where it is needed internally.

## Verification

Add a user lottery view test with a populated prize whose weight is known, then assert the prize card still shows the reward details but does not render the weight label/value. Run the focused Vitest file, TypeScript checking, ESLint, and the production frontend build.
