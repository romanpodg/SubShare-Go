import type { StructuredProfilePatch } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { validateEditorRaw } from "./key-editor-raw-policy";

function revealedLegacyContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined) {
  validateEditorRaw(state.protocol, state.rawUri);
  const raw = state.structuredEdited || state.rawEdited ? state.rawUri : state.authoritativeRaw;
  return { mode: "raw" as const, raw, patch };
}

function unrevealedLegacyContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined) {
  if (state.rawEdited || state.mode === "raw") throw new Error("Сначала раскройте raw-конфигурацию");
  return { mode: "structured" as const, patch: patch || {} };
}

export function buildLegacyUpdateContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined) {
  if (state.revealedRaw) return revealedLegacyContent(state, patch);
  return unrevealedLegacyContent(state, patch);
}
