// Package profiles parses, validates, canonicalizes, safely describes, and
// fingerprints share-link protocol profiles.
//
// Parsing is intentionally independent from importer persistence and delivery
// rendering. A Profile keeps the exact original URI in a SensitiveValue while
// exposing only explicitly safe display metadata. Canonical serialization is
// semantic normalization, not a claim that the source bytes can be reproduced;
// callers that need byte-for-byte output must request OriginalSerialization.
//
// Duplicate query policy is first-value-wins for typed fields. Every occurrence
// remains in QueryParameters in source order and in exact original output.
// Canonical output groups parameters deterministically while retaining the
// relative order of duplicate values for the same semantic key. Conflicting
// values produce an ambiguity warning. Unknown extensions participate in
// fingerprints, including duplicate occurrence order.
//
// Fingerprint keys are mandatory caller-owned runtime configuration. This
// package deliberately provides no default or fallback key; importer wiring is
// responsible for supplying one in a later stage.
package profiles
