import type { KeyProfileDetailResponse, StructuredProfilePatch, UpdateKeyProfileInput } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { validateEditorRaw } from "./key-editor-raw-policy";
import { buildEditorUpdatePatch } from "./key-editor-update-patch";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";

interface UpdateContent { mode: "raw" | "structured"; raw?: string; patch?: StructuredProfilePatch; }

function legacyContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined): UpdateContent {
  if (state.revealedRaw) {
    validateEditorRaw(state.protocol, state.rawUri);
    return { mode: "raw", raw: state.structuredEdited || state.rawEdited ? state.rawUri : state.authoritativeRaw, patch };
  }
  if (state.mode === "raw") throw new Error("Сначала раскройте raw-конфигурацию");
  return { mode: "structured", patch: patch || {} };
}

function xrayContent(state: EditorCommandState): UpdateContent {
  if (state.rawEdited) {
    validateEditorRaw(state.protocol, state.rawUri);
    return { mode: "raw", raw: state.rawUri };
  }
  return { mode: "structured", patch: {} };
}

function nativeRawContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined): UpdateContent {
  if (!state.revealedRaw) throw new Error("Сначала раскройте raw-конфигурацию");
  if (state.structuredEdited) throw new Error("Structured-версия уже изменена. Вернитесь в structured режим для сохранения или откройте профиль заново.");
  return { mode: "raw", raw: state.rawEdited ? state.rawUri : state.authoritativeRaw, patch };
}

function updateContent(state: EditorCommandState, patch: StructuredProfilePatch | undefined): UpdateContent {
  if (state.kind !== "real") return { mode: state.mode, patch };
  if (isLegacyEditorProtocol(state.protocol)) return legacyContent(state, patch);
  if (state.protocol === "xray-json") return xrayContent(state);
  if (state.mode === "raw") return nativeRawContent(state, patch);
  if (state.rawEdited) throw new Error("Raw-версия уже изменена. Вернитесь в raw режим для сохранения или откройте профиль заново.");
  return { mode: state.mode, patch };
}

function sourceUpdate(state: EditorCommandState, detail: KeyProfileDetailResponse): UpdateKeyProfileInput {
  const currentClientDisplayNameOverride = detail.client_display_name_overridden ? detail.client_display_name.trim() : "";
  return {
    label: detail.label,
    client_display_name: state.displayName.trim() !== currentClientDisplayNameOverride ? state.displayName.trim() : undefined,
    category: detail.category,
    status: state.status,
    kind: detail.kind,
    template_text: detail.template_text,
    profile_revision: detail.profile_revision,
    patch_mode: "structured",
  };
}

export function buildEditorUpdateCommand(state: EditorCommandState, detail: KeyProfileDetailResponse): UpdateKeyProfileInput {
  if (detail.ownership === "external_source") return sourceUpdate(state, detail);
  const content = updateContent(state, buildEditorUpdatePatch(state, detail));
  return {
    label: state.label.trim(),
    client_display_name: state.displayName.trim() !== detail.client_display_name ? (state.displayName.trim() || state.label.trim()) : undefined,
    category: state.category,
    status: state.status,
    kind: state.kind,
    template_text: state.templateText,
    profile_revision: detail.profile_revision,
    patch_mode: content.mode,
    raw_uri: content.mode === "raw" ? content.raw : undefined,
    structured_patch: content.mode === "structured" ? (content.patch || {}) : undefined,
  };
}
