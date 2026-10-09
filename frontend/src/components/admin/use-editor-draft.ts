"use client";

import { useCallback, useMemo, useState, type Dispatch, type SetStateAction } from "react";
import type { ExternalProfileProtocol } from "@/lib/types";
import type { KeyStructuredSecretsResponse } from "@/lib/types";
import type { XrayJSONPatch } from "@/lib/configuration";
import { emptyLegacyDraft, type EditorCommandState } from "./key-editor-command-state";
import { isLegacyEditorProtocol, protocolEditorCapability } from "./protocol-editor-capabilities";
import { applyNativeReveal, applyRawReveal, editDraftField, editorCommandState, loadDraftRevision, recordLegacyEdit, type NativeSecretField } from "./editor-draft-transitions";

interface DraftDefaults {
  initialLabel: string;
  initialCategory: string;
  initialKind: "real" | "informational";
  initialProtocol?: ExternalProfileProtocol;
  keyId?: number;
}

export interface EditorDraftState extends EditorCommandState {
  legacySecretsRevealed: boolean;
  rawValidationError: string;
  rawFormattingWarning: string;
  isDirty: boolean;
  nativeTouched: Partial<Record<NativeSecretField, true>>;
  legacyEdits: XrayJSONPatch;
  profileRevision?: number;
}

function createEditorDraft(defaults: DraftDefaults): EditorDraftState {
  const { initialLabel, initialCategory, initialKind, initialProtocol, keyId } = defaults;
  const defaultMode = protocolEditorCapability(initialProtocol) ? "structured" : "raw";
  return {
    label: initialLabel,
    status: "active",
    kind: initialKind,
    category: initialCategory,
    templateText: "",
    mode: defaultMode,
    protocol: initialProtocol || "shadowsocks",
    rawUri: "",
    authoritativeRaw: "",
    revealedRaw: false,
    rawEdited: false,
    structuredEdited: false,
    rawValidationError: "",
    rawFormattingWarning: "",
    legacyDraft: emptyLegacyDraft(isLegacyEditorProtocol(initialProtocol || "shadowsocks") ? initialProtocol as "vless" | "vmess" | "trojan" : "vless"),
    legacySecretsRevealed: !keyId,
    server: "",
    port: "",
    displayName: "",
    ssMethod: "2022-blake3-aes-128-gcm",
    ssPassword: undefined,
    ssPluginName: undefined,
    ssPluginOptions: undefined,
    hy2Auth: undefined,
    hy2Sni: "",
    hy2Insecure: false,
    hy2CertSha: "",
    hy2ObfsType: "",
    hy2ObfsPassword: undefined,
    tuicUuid: undefined,
    tuicPassword: undefined,
    tuicSni: "",
    tuicAlpn: "h3",
    tuicSkipCert: false,
    tuicCc: "bbr",
    tuicUdpRelay: "native",
    tuicUdpOverStream: false,
    tuicZeroRtt: false,
    tuicHeartbeat: "10s",
    isDirty: false,
    nativeTouched: {},
    legacyEdits: {},
    profileRevision: undefined,
  };
}

export function useEditorDraft(defaults: DraftDefaults) {
  const [draft, setDraft] = useState(() => createEditorDraft(defaults));
  const fieldSetter = useCallback(<Field extends keyof EditorDraftState>(field: Field): Dispatch<SetStateAction<EditorDraftState[Field]>> => {
    return (value) => setDraft((previous) => editDraftField(previous, field,
      typeof value === "function" ? value(previous[field]) : value));
  }, []);
  const setters = useMemo(() => ({
    setLabel: fieldSetter("label"),
    setStatus: fieldSetter("status"),
    setKind: fieldSetter("kind"),
    setCategory: fieldSetter("category"),
    setTemplateText: fieldSetter("templateText"),
    setMode: fieldSetter("mode"),
    setProtocol: fieldSetter("protocol"),
    setRawUri: fieldSetter("rawUri"),
    setAuthoritativeRaw: fieldSetter("authoritativeRaw"),
    setRevealedRaw: fieldSetter("revealedRaw"),
    setRawEdited: fieldSetter("rawEdited"),
    setStructuredEdited: fieldSetter("structuredEdited"),
    setRawValidationError: fieldSetter("rawValidationError"),
    setRawFormattingWarning: fieldSetter("rawFormattingWarning"),
    setLegacyDraft: fieldSetter("legacyDraft"),
    setLegacySecretsRevealed: fieldSetter("legacySecretsRevealed"),
    setServer: fieldSetter("server"),
    setPort: fieldSetter("port"),
    setDisplayName: fieldSetter("displayName"),
    setSsMethod: fieldSetter("ssMethod"),
    setSsPassword: fieldSetter("ssPassword"),
    setSsPluginName: fieldSetter("ssPluginName"),
    setSsPluginOptions: fieldSetter("ssPluginOptions"),
    setHy2Auth: fieldSetter("hy2Auth"),
    setHy2Sni: fieldSetter("hy2Sni"),
    setHy2Insecure: fieldSetter("hy2Insecure"),
    setHy2CertSha: fieldSetter("hy2CertSha"),
    setHy2ObfsType: fieldSetter("hy2ObfsType"),
    setHy2ObfsPassword: fieldSetter("hy2ObfsPassword"),
    setTuicUuid: fieldSetter("tuicUuid"),
    setTuicPassword: fieldSetter("tuicPassword"),
    setTuicSni: fieldSetter("tuicSni"),
    setTuicAlpn: fieldSetter("tuicAlpn"),
    setTuicSkipCert: fieldSetter("tuicSkipCert"),
    setTuicCc: fieldSetter("tuicCc"),
    setTuicUdpRelay: fieldSetter("tuicUdpRelay"),
    setTuicUdpOverStream: fieldSetter("tuicUdpOverStream"),
    setTuicZeroRtt: fieldSetter("tuicZeroRtt"),
    setTuicHeartbeat: fieldSetter("tuicHeartbeat"),
    setIsDirty: fieldSetter("isDirty"),
  }), [fieldSetter]);
  const resetDraft = useCallback((next: DraftDefaults) => setDraft(createEditorDraft(next)), []);
  const commandState = useMemo(() => editorCommandState(draft), [draft]);
  const acceptNativeReveal = useCallback((secrets: KeyStructuredSecretsResponse["secrets"], revision: number) => setDraft((previous) =>
    previous.profileRevision === revision ? applyNativeReveal(previous, secrets) : previous), []);
  const acceptRawReveal = useCallback((protocol: ExternalProfileProtocol, raw: string, revision: number) => setDraft((previous) =>
    previous.profileRevision === revision ? applyRawReveal(previous, protocol, raw) : previous), []);
  const acceptRevision = useCallback((revision: number) => setDraft((previous) => loadDraftRevision(previous, revision)), []);
  const rememberLegacyEdit = useCallback((patch: XrayJSONPatch) => setDraft((previous) => recordLegacyEdit(previous, patch)), []);
  return { draft, setters, resetDraft, commandState, acceptNativeReveal, acceptRawReveal, acceptRevision, rememberLegacyEdit };
}
