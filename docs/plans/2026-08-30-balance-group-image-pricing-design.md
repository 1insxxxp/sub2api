# Balance Group Image Pricing Design

## Goal

Charge `gpt-image-*` generation in the three OpenAI balance groups at the same final per-image price as the balance Images2 group: `$0.375` per generated image, without applying the text token multiplier.

## Scope

- Group 2: Pro benefit balance pool
- Group 9: Pro stable balance pool
- Group 25: GPT benefit balance pool
- Models matched by the `gpt-image-*` prefix wildcard

Subscription and integration groups are intentionally excluded.

## Configuration

Each target group receives a group-level model pricing override with image billing and a default per-request price of `$0.375`. Group-level pricing already has precedence over channel token pricing. Image billing is made independent with an image multiplier of `1.0`, so the final charge remains `$0.375` regardless of the group's text multiplier.

Existing model-pricing entries must be preserved. The update must go through the administrative service path, or otherwise explicitly invalidate group authentication caches, so active API keys observe the new configuration immediately.

## Verification

1. Confirm all three groups retain their existing non-image settings.
2. Resolve `gpt-image-2` pricing for each group and verify source `group`, mode `image`, unit price `$0.375`, and multiplier `1.0`.
3. Confirm subscription and integration groups are unchanged.
4. Preserve the pre-change JSON configuration as the rollback baseline.
