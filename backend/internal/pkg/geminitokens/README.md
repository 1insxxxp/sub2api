# Embedded text tokenizer

`gemma3.model` is Google's public Gemma3 SentencePiece vocabulary, as pinned by
the Google GenAI SDK. It is a local counting input, not a published official
`gemini-4-argon` mapping or a measurement of provider consumption.

- Source: https://raw.githubusercontent.com/google/gemma_pytorch/014acb7ac4563a5f77c76d7ff98f31b568c16508/tokenizer/gemma3_cleaned_262144_v2.spiece.model
- SHA-256: `1299c11d7cf632ef3b4e11937501358ada021bbdf7c47638d13c0ee982f2e79c`
- Bytes: 4,689,074
- Official loader: https://github.com/googleapis/go-genai/blob/main/tokenizer/tokenizer.go
- Source repository license: `LICENSE.google-gemma` (Apache 2.0), copied from
  that source commit. Copyright Google LLC.

The vocabulary is embedded and validated in memory once. Runtime tokenization
has no network or filesystem dependency. `testdata/python_counts.json` contains
only synthetic text and expected raw counts produced by SentencePiece 0.2.1
with this exact vocabulary. Request-wrapper estimates belong to the versioned
service policy, not this text-only tokenizer.

The service's `vertex_like_discount_v1` profile sums these raw text counts,
adds 31 wrapper tokens and 2 per additional conversation content. Those
constants were fixed before six independent synthetic holdouts on 2026-11-01:

| Holdout | Profile candidate | Vertex response input | Difference |
| --- | ---: | ---: | ---: |
| Short | 40 | 40 | 0.00% |
| Chinese | 166 | 160 | +3.75% |
| English | 105 | 105 | 0.00% |
| JSON text | 287 | 286 | +0.35% |
| Long Chinese | 5,350 | 5,070 | +5.52% |
| Conversation | 201 | 203 | -0.99% |

These are candidate-profile comparisons before the customer ceiling, not an
accuracy guarantee. The service only decreases measured ent input; an ent count
already below the candidate stays unchanged, even when it differs from vertex.
Tools, media, cache and unknown request shapes are outside this profile.
