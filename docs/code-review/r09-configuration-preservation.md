# R09 — Frontend configuration preservation corpus

R09 is tests and fixtures only. The production configuration editor, token document editor, protocol parsers, defaults and public APIs are unchanged. It establishes a deterministic prerequisite for R10's patch planning/application and security phase extraction.

## Corpus and invariants

The reusable fixture corpus covers **63 combinations**: VLESS/VMess/Trojan × TCP/WS/gRPC/HTTP Upgrade/XHTTP/H2/QUIC × none/TLS/Reality. Each document contains unrelated top-level data, inbound/routing/DNS settings, unsupported and extra supported outbounds, extra nodes/users/servers, unknown nested fields, all dormant transport/security branches, a huge exponent, an integer beyond JavaScript precision and an escaped string token. The combinations characterize editor preservation, not runtime connectivity certification.

**388 new cases** protect normalized no-op/idempotent bytes, whole-document goldens for connection/discriminator/SNI edits, absent stream settings, required-field clear rejection, TLS false removal, both Reality aliases on explicit clear, protocol conversions, URI parameter order/duplicates, VMess zero/false values and duplicate unknown JSON key rejection. Each protocol-conversion case also checks both other target protocols. The existing 128 frontend tests remain intact.

The baseline exposed two existing rules, retained explicitly: a stored none discriminator with dormant Reality settings projects as Reality while no-op editing keeps the stored none; VMess false clears the encoded insecure flag, and explicit zero remains represented. These are characterization, not silent behavior corrections. Formatting normalization is intentional; exact-byte comparisons use the existing normalized document and preserve token lexemes within it.

## Validation and next batch

The full frontend suite passes **516 tests in 33 files**. TypeScript (`--noEmit --incremental false`), lint and exported build pass. Both new corpus files score **10.00** in direct CodeScene reviews; staged additions pass the secret scan. Evidence is saved under `.cache/r09`. Hosted checks and automated review must pass at the final PR head before merge. Frontend coverage remains unavailable; case counts do not establish a percentage.

R10a separates pure patch planning from ordered document application using the existing token editor. R10b extracts a security phase against the retained combination/clear/alias corpus. Existing parser selection, mode inference, formatting and unsupported-field behavior remain contracts.
