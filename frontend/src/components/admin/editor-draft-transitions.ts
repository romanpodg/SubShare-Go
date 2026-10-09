import { parseEditableConfiguration, patchEditableConfiguration, type XrayJSONPatch } from "@/lib/configuration";
import type { ExternalProfileProtocol, KeyStructuredSecretsResponse } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import type { EditorDraftState } from "./use-editor-draft";
import { projectEditorRawDisplay } from "./key-editor-raw-policy";
import { isLegacyEditorProtocol } from "./protocol-editor-capabilities";

export const nativeSecretFields = ["ssPassword", "ssPluginOptions", "hy2Auth", "hy2ObfsPassword", "tuicUuid", "tuicPassword"] as const;
export type NativeSecretField = typeof nativeSecretFields[number];
const secretBindings = [
  ["ssPassword", "password"], ["ssPluginOptions", "plugin_options"],
  ["hy2Auth", "authentication"], ["hy2ObfsPassword", "obfuscation_password"],
  ["tuicUuid", "uuid"], ["tuicPassword", "password"],
] as const;

export function isNativeSecretField(field: keyof EditorDraftState): field is NativeSecretField {
  return nativeSecretFields.includes(field as NativeSecretField);
}

export function editDraftField<Field extends keyof EditorDraftState>(draft: EditorDraftState, field: Field, value: EditorDraftState[Field]) {
  const next = { ...draft, [field]: value };
  if (isNativeSecretField(field)) next.nativeTouched = { ...draft.nativeTouched, [field]: true };
  return next;
}

export function editorCommandState(draft: EditorDraftState): EditorCommandState {
  return {
    ...draft,
    ssPassword: draft.nativeTouched.ssPassword ? draft.ssPassword : undefined,
    ssPluginOptions: draft.nativeTouched.ssPluginOptions ? draft.ssPluginOptions : undefined,
    hy2Auth: draft.nativeTouched.hy2Auth ? draft.hy2Auth : undefined,
    hy2ObfsPassword: draft.nativeTouched.hy2ObfsPassword ? draft.hy2ObfsPassword : undefined,
    tuicUuid: draft.nativeTouched.tuicUuid ? draft.tuicUuid : undefined,
    tuicPassword: draft.nativeTouched.tuicPassword ? draft.tuicPassword : undefined,
  };
}

export function applyNativeReveal(draft: EditorDraftState, secrets: KeyStructuredSecretsResponse["secrets"]) {
  const next = { ...draft };
  for (const [field, key] of secretBindings) {
    const value = secrets[key];
    if (value !== undefined && !draft.nativeTouched[field]) next[field] = value;
  }
  return next;
}

export function loadDraftRevision(draft: EditorDraftState, revision: number) {
  if (draft.profileRevision === revision) return draft;
  const next: EditorDraftState = {
    ...draft, profileRevision: revision, authoritativeRaw: "", revealedRaw: false,
    rawUri: draft.rawEdited ? draft.rawUri : "", legacySecretsRevealed: false, legacyEdits: {},
    structuredEdited: nativeSecretFields.some((field) => draft.nativeTouched[field]),
  };
  for (const field of nativeSecretFields) {
    if (!draft.nativeTouched[field]) next[field] = undefined;
  }
  return next;
}

export function recordLegacyEdit(draft: EditorDraftState, patch: XrayJSONPatch) {
  return { ...draft, legacyEdits: { ...draft.legacyEdits, ...patch } };
}

function rawRevealBase(draft: EditorDraftState, protocol: ExternalProfileProtocol, raw: string) {
  const display = projectEditorRawDisplay(protocol, raw);
  return {
    ...draft, authoritativeRaw: raw, revealedRaw: true,
    rawUri: draft.rawEdited ? draft.rawUri : display.value,
    rawFormattingWarning: draft.rawEdited ? draft.rawFormattingWarning : display.warning,
    rawValidationError: draft.rawEdited ? draft.rawValidationError : "",
  };
}

function patchedLegacyRaw(draft: EditorDraftState) {
  if (draft.rawEdited || Object.keys(draft.legacyEdits).length === 0) return draft.rawUri;
  return patchEditableConfiguration(draft.rawUri, draft.legacyEdits);
}

function legacyReveal(draft: EditorDraftState, raw: string) {
  try {
    const parsed = parseEditableConfiguration(draft.rawEdited ? draft.rawUri : raw);
    return {
      ...draft, rawUri: patchedLegacyRaw(draft), legacySecretsRevealed: true,
      legacyDraft: draft.rawEdited ? parsed : { ...parsed, ...draft.legacyEdits },
    };
  } catch (error) {
    return { ...draft, rawValidationError: error instanceof Error ? error.message : "Не удалось разобрать конфигурацию" };
  }
}

export function applyRawReveal(draft: EditorDraftState, protocol: ExternalProfileProtocol, raw: string) {
  const next = rawRevealBase(draft, protocol, raw);
  return isLegacyEditorProtocol(protocol) ? legacyReveal(next, raw) : next;
}
