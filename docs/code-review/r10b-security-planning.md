# R10b — Focused security planning module

Status: discovery plan; no R10b production changes yet. R10a is accepted in PR #16 at `8952b75`, with all six checks passing at `d225402` and automated review reporting no major issues.

Extract the security planning phase from `configuration.ts` into a focused internal module. Keep ordered path/value operations and the existing token application boundary. Pass original TLS/Reality projection and selected mode explicitly; preserve alias shapes, omitted/clear/false handling, missing/non-object parents, coupled mode/field behavior and none projections. Type-only imports may reuse existing draft/patch types; avoid a runtime cycle or new public configuration API.

Share normalization helpers through one internal implementation when needed; do not duplicate `optionalText` or ALPN splitting. Keep parent initialization semantics and lazy field normalization unchanged. The new module must meet the 10.00 new-file CodeScene gate without suppressions. All R09 goldens, the full frontend suite, types/lint/exported build and strict delta remain required. Publish/review/resolve/merge as its own batch, update the backlog, then continue the original remaining scopes.
