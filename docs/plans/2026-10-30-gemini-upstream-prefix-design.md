# Gemini upstream model prefix compatibility

The user approved repairing and deploying support for this upstream, then importing its `[量]` models into group 121. Requests currently fail before reaching the upstream because the shared path validator rejects `[量]gemini-4-argon`; removing the prefix returns upstream `model_not_found`.

Use a dedicated AI Studio final-model validator rather than weakening the shared Responses/path guard or public native routing validation. Accept existing ASCII names, or one leading `[label]` followed by an existing valid ASCII model name. Labels contain only Unicode letters/numbers and `._-`; the complete value remains limited to 128 bytes. Reject empty, repeated/nested labels and all path/encoding metacharacters. Escape the validated model as one URL segment with `url.PathEscape`.

Native aliases mapped to tagged targets use this validator only on AI Studio branches. Keep Vertex, Code Assist, metadata GET and cached-content public validation unchanged. OpenAI/Messages bodies routed through an empty account mapping may also use safe tagged model names; the imported account instead uses an explicit seven-model mapping.

Alternatives: removing the prefix is not functional (verified 503); relaxing the shared validator would broaden unrelated endpoints; a separate production proxy adds an unnecessary service. The scoped validator and encoder are the smallest repair.

Verify failing then passing URL/HTTP/native-alias/countTokens recovery tests, existing path security and Gemini handler/service regressions, independent review and CI. Keep version 0.2.19, build exact source SHA in Actions, deploy two new app instances without rebuilding production or restarting databases, preserve runtime and Nginx protections, verify before switching and retain stopped rollback instances. Create/test the independent account and bind only group 121 after release validation. Preserve account 6010 and the existing group pricing; custom tools remain unsupported by the upstream.
