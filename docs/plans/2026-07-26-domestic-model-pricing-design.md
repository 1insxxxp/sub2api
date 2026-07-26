# Domestic Model Pricing Design

## Goal

Ensure every domestic model currently routed through the `国模测试` group resolves to a non-zero official list price, while preserving the project's USD balance accounting and existing multiplier controls.

## Pricing Policy

- Keep the billing engine's canonical unit as USD per token.
- Use official vendor list prices, not temporary promotional discounts.
- Use official USD prices directly when the vendor publishes them.
- Convert official CNY prices at the existing project convention of `1 USD = 7.14 CNY`.
- Keep group and account multipliers unchanged. The charged amount remains `base cost * group multiplier * account multiplier`.
- Preserve cache-read prices when the vendor publishes them. Leave cache creation at the existing input-price behavior unless a distinct official write price exists.

## Model Coverage

Add exact fallback pricing and matching for:

- `kimi-k3`
- `kimi-k2.7-code`
- `qwen3.7-max`
- `qwen3.7-plus`
- `mimo-v2.5-pro`
- `mimo-v2.5`
- `glm-5.2`

Existing DeepSeek V4 and MiniMax M3 entries remain in place. Exact matches must run before broad family matches so `glm-5.2` does not fall through to `glm-5` and `kimi-k2.7-code` does not fall through to `kimi-k2`.

Provider-prefixed model names such as `cline-pass/kimi-k3` must resolve through the same exact model markers because production usage logs retain the upstream routing prefix.

## Tiered Pricing

- `qwen3.7-plus` uses the official list tier for contexts up to 256K tokens and a 3x input/output multiplier above 256K.
- `qwen3.7-max` uses its published list price through its supported context window.
- Existing MiniMax M3 long-context behavior is outside this change unless required by a regression test.

## Data Flow

1. Dynamic remote pricing remains the first source.
2. The new exact fallback entries are used when the remote price registry has no matching model.
3. The billing service calculates input, cached-input, and output cost in USD.
4. Group and account multipliers produce `actual_cost`.
5. Existing zero-cost behavior remains only for genuinely unknown models.

## Testing

- Add table-driven tests for every new model and its `cline-pass/` alias.
- Assert input, output, and cache-read rates.
- Assert GLM-5.2 and Kimi K2.7 Code no longer inherit older family prices.
- Assert Qwen3.7 Plus applies the 256K long-context multiplier.
- Assert a representative cost calculation is non-zero and group multiplier scales only `ActualCost`.
- Run the focused billing tests, the full service package tests, formatting, and the repository's relevant validation command before deployment.

## Deployment And Verification

Deploy only after local verification succeeds. After deployment, issue a minimal request through the `国模测试` group for a newly priced model and verify that the resulting usage row has non-zero `total_cost`, non-zero `actual_cost`, and the expected multiplier. Historical zero-cost rows are not retroactively charged.
