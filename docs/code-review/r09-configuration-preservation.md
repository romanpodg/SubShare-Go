# R09 — Frontend configuration preservation corpus

R09 is tests and fixtures only. The production configuration editor, token document editor, protocol parsers, defaults and public APIs are unchanged. It establishes a deterministic prerequisite for R10's patch planning/application and security phase extraction.

## Corpus and invariants

The reusable fixture corpus covers **63 combinations**: VLESS/VMess/Trojan × TCP/WS/gRPC/HTTP Upgrade/XHTTP/H2/QUIC × none/TLS/Reality. Each document contains unrelated top-level data, inbound/routing/DNS settings, unsupported and extra supported outbounds, extra nodes/users/servers, unknown nested fields, all dormant transport/security branches, a huge exponent, an integer beyond JavaScript precision and an escaped string token. The combinations characterize editor preservation, not runtime connectivity certification.

**754 new cases** protect normalized no-op/idempotent bytes, whole-document goldens for connection/discriminator/SNI edits, absent stream settings, required-field clear rejection, TLS false removal, Reality alias shapes, protocol conversions with present and absent target branches, active transport edits/clears, URI parameter order/duplicates, VMess zero/false values and duplicate unknown JSON key rejection. Each protocol-conversion case also checks both other target protocols. The existing 128 frontend tests remain intact.

The baseline exposed two existing rules, retained explicitly: a stored none discriminator with dormant Reality settings projects as Reality while no-op editing keeps the stored none; VMess false clears the encoded insecure flag, and explicit zero remains represented. These are characterization, not silent behavior corrections. Formatting normalization is intentional; exact-byte comparisons use the existing normalized document and preserve token lexemes within it.

## Validation and next batch

The review-expanded full frontend suite passes **882 tests in 33 files**, with zero failures. TypeScript (`--noEmit --incremental false`), lint and exported build pass. All three corpus files score **10.00** in direct CodeScene reviews. Evidence is saved under `.cache/r09`. Hosted checks and automated review must pass at the final PR head before merge. Frontend coverage remains unavailable; case counts do not establish a percentage.

R10a separates pure patch planning from ordered document application using the existing token editor. R10b extracts a security phase against the retained combination/clear/alias corpus. Existing parser selection, mode inference, formatting and unsupported-field behavior remain contracts.

## PR #15 review follow-up

The initial corpus at `8c9b267` passed all six hosted checks, but Codex review identified five actionable coverage gaps. The follow-up adds source-only connection fixtures with exact goldens for both conversion directions, set/clear transport expectations across every network, raw-TCP branch precedence, all explicit security transitions (including none and its subsequent Reality projection), and nonempty Reality edits for modern/legacy/both/neither alias shapes. Conversion assertions also compare the full primary node/user/server after accounting only for represented changes, protecting their unknown fields.

Expected documents are built independently from fixture data; their generation does not call production patch planning or mutation functions. The baseline patcher ignores transport hints for H2/QUIC; those no-op bytes are characterized rather than silently extended. The 754-case targeted suite and full 882-test suite pass against unchanged production code. An account usage limit temporarily prevented automatic approval of final checks; after the reported reset passed and usage became available, the same normal approval path resumed validation successfully. No approval or check was bypassed.
