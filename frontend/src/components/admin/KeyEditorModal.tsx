"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Copy,
  Eye,
  FolderPlus,
  Info,
  Lock,
} from "lucide-react";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { ApiError, keys as keysApi } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import {
  buildXrayJSONConfiguration,
  createConfigurationFromXrayJSON,
  parseEditableConfiguration,
  parseXrayJSONConfiguration,
  patchEditableConfiguration,
  type XrayJSONDraft,
  type XrayJSONPatch,
} from "@/lib/configuration";
import { formatXrayJSONWithinLimit, inspectXrayJSONDocument } from "@/lib/xray-json-document";
import type {
  CreateKeyProfileInput,
  ExternalProfileProtocol,
  KeyCategory,
  KeyEditorSchemaResponse,
  KeyProfileDetailResponse,
  KeyRawSecretResponse,
  KeyStructuredSecretsResponse,
  StructuredProfilePatch,
  UpdateKeyProfileInput,
} from "@/lib/types";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { KeyEditorConflictDialog } from "./KeyEditorConflictDialog";
import { informationalTemplatePreviewParts, keyTemplateVariables } from "./keyTemplateVariables";
import { OutputCapabilitiesMatrix } from "./OutputCapabilitiesMatrix";
import {
  isLegacyEditorProtocol,
  PROTOCOL_EDITOR_CAPABILITIES,
  protocolEditorCapability,
} from "./protocol-editor-capabilities";
import { LegacyXrayFields } from "./protocol-editors/LegacyXrayFields";
import {
  buildHysteria2Patch,
  Hysteria2Fields,
} from "./protocol-editors/Hysteria2Fields";
import {
  buildShadowsocksPatch,
  ShadowsocksFields,
} from "./protocol-editors/ShadowsocksFields";
import { TuicV4ReadOnlyBanner } from "./protocol-editors/TuicV4ReadOnlyBanner";
import {
  buildTUICPatch,
  TuicV5Fields,
} from "./protocol-editors/TuicV5Fields";

const LABEL_LIMIT = 255;

function emptyLegacyDraft(protocol: "vless" | "vmess" | "trojan" = "vless"): XrayJSONDraft {
  return {
    protocol,
    server: "",
    port: "443",
    identifier: "",
    network: "tcp",
    security: "none",
    path: "",
    host: "",
    sni: "",
    alpn: "",
    remark: "",
  };
}

function isXrayJSONRaw(protocol: ExternalProfileProtocol, raw: string) {
  return protocol === "xray-json" || raw.trim().startsWith("{");
}

export interface KeyEditorModalProps {
  open: boolean;
  keyId?: number; // If provided, edit mode via GET /api/v1/keys/{id}
  initialLabel?: string;
  initialCategory?: string;
  initialKind?: "real" | "informational";
  initialProtocol?: ExternalProfileProtocol;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function KeyEditorModal({
  open,
  keyId,
  initialLabel = "",
  initialCategory = "",
  initialKind = "real",
  initialProtocol,
  onClose,
  onRefresh,
}: KeyEditorModalProps) {
  const { toast } = useToast();

  const defaultMode = protocolEditorCapability(initialProtocol) ? "structured" : "raw";

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revealing, setRevealing] = useState(false);
  const [cloning, setCloning] = useState(false);
  const [detail, setDetail] = useState<KeyProfileDetailResponse | null>(null);
  const [schemaResponse, setSchemaResponse] = useState<KeyEditorSchemaResponse | null>(null);

  // Form State
  const [label, setLabel] = useState(initialLabel);
  const [status, setStatus] = useState<"active" | "non-active">("active");
  const [kind, setKind] = useState<"real" | "informational">(initialKind);
  const [category, setCategory] = useState(initialCategory);
  const [templateText, setTemplateText] = useState("");
  const templateTextRef = useRef<HTMLTextAreaElement | null>(null);
  const [mode, setMode] = useState<"structured" | "raw">(defaultMode);
  const [protocol, setProtocol] = useState<ExternalProfileProtocol>(initialProtocol || "shadowsocks");

  // Raw URI / JSON state
  const [rawUri, setRawUri] = useState("");
  const [authoritativeRaw, setAuthoritativeRaw] = useState("");
  const [revealedRaw, setRevealedRaw] = useState(false);
  const [rawEdited, setRawEdited] = useState(false);
  const [structuredEdited, setStructuredEdited] = useState(false);
  const [rawValidationError, setRawValidationError] = useState("");
  const [rawFormattingWarning, setRawFormattingWarning] = useState("");
  const [legacyDraft, setLegacyDraft] = useState<XrayJSONDraft>(() =>
    emptyLegacyDraft(isLegacyEditorProtocol(initialProtocol || "shadowsocks") ? initialProtocol as "vless" | "vmess" | "trojan" : "vless")
  );
  const [legacySecretsRevealed, setLegacySecretsRevealed] = useState(!keyId);

  // Structured fields state
  const [server, setServer] = useState("");
  const [port, setPort] = useState("");
  const [displayName, setDisplayName] = useState("");

  // Shadowsocks fields
  const [ssMethod, setSsMethod] = useState("2022-blake3-aes-128-gcm");
  const [ssPassword, setSsPassword] = useState<string | undefined>(undefined);
  const [ssPluginName, setSsPluginName] = useState<string | undefined>(undefined);
  const [ssPluginOptions, setSsPluginOptions] = useState<string | undefined>(undefined);

  // Hysteria 2 fields
  const [hy2Auth, setHy2Auth] = useState<string | undefined>(undefined);
  const [hy2Sni, setHy2Sni] = useState("");
  const [hy2Insecure, setHy2Insecure] = useState(false);
  const [hy2CertSha, setHy2CertSha] = useState("");
  const [hy2ObfsType, setHy2ObfsType] = useState("");
  const [hy2ObfsPassword, setHy2ObfsPassword] = useState<string | undefined>(undefined);

  // TUIC fields
  const [tuicUuid, setTuicUuid] = useState<string | undefined>(undefined);
  const [tuicPassword, setTuicPassword] = useState<string | undefined>(undefined);
  const [tuicSni, setTuicSni] = useState("");
  const [tuicAlpn, setTuicAlpn] = useState("h3");
  const [tuicSkipCert, setTuicSkipCert] = useState(false);
  const [tuicCc, setTuicCc] = useState("bbr");
  const [tuicUdpRelay, setTuicUdpRelay] = useState("native");
  const [tuicUdpOverStream, setTuicUdpOverStream] = useState(false);
  const [tuicZeroRtt, setTuicZeroRtt] = useState(false);
  const [tuicHeartbeat, setTuicHeartbeat] = useState("10s");

  // Modal dialog states
  const [availableCategories, setAvailableCategories] = useState<KeyCategory[]>([]);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showConflictDialog, setShowConflictDialog] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  const [isDirty, setIsDirty] = useState(false);

  const templatePreviewParts = useMemo(
    () => informationalTemplatePreviewParts(templateText),
    [templateText]
  );

  const insertTemplateVariable = useCallback((token: string) => {
    const editor = templateTextRef.current;
    const start = editor?.selectionStart ?? templateText.length;
    const end = editor?.selectionEnd ?? start;
    const next = `${templateText.slice(0, start)}${token}${templateText.slice(end)}`;
    const caret = start + token.length;
    setTemplateText(next);
    setIsDirty(true);
    window.requestAnimationFrame(() => {
      templateTextRef.current?.focus();
      templateTextRef.current?.setSelectionRange(caret, caret);
    });
  }, [templateText]);

  const handleCloseAttempt = () => {
    if (isDirty) {
      setShowDiscardConfirm(true);
    } else {
      onClose();
    }
  };

  // Clear state on unmount/close
  const clearSensitiveState = useCallback(() => {
    setIsDirty(false);
    setShowDiscardConfirm(false);
    setDetail(null);
    setLabel(initialLabel);
    setStatus("active");
    setKind(initialKind);
    setCategory(initialCategory);
    setTemplateText("");
    setMode(defaultMode);
    setProtocol(initialProtocol || "shadowsocks");
    setRawUri("");
    setAuthoritativeRaw("");
    setRevealedRaw(false);
    setRawEdited(false);
    setStructuredEdited(false);
    setRawValidationError("");
    setRawFormattingWarning("");
    setLegacyDraft(emptyLegacyDraft(isLegacyEditorProtocol(initialProtocol || "shadowsocks") ? initialProtocol as "vless" | "vmess" | "trojan" : "vless"));
    setLegacySecretsRevealed(!keyId);
    setServer("");
    setPort("");
    setDisplayName("");
    setSsMethod("2022-blake3-aes-128-gcm");
    setSsPassword(undefined);
    setSsPluginName(undefined);
    setSsPluginOptions(undefined);
    setHy2Auth(undefined);
    setHy2Sni("");
    setHy2Insecure(false);
    setHy2CertSha("");
    setHy2ObfsType("");
    setHy2ObfsPassword(undefined);
    setTuicUuid(undefined);
    setTuicPassword(undefined);
    setTuicSni("");
    setTuicAlpn("h3");
    setTuicSkipCert(false);
    setTuicCc("bbr");
    setTuicUdpRelay("native");
    setTuicUdpOverStream(false);
    setTuicZeroRtt(false);
    setTuicHeartbeat("10s");
  }, [defaultMode, initialCategory, initialKind, initialLabel, initialProtocol, keyId]);

  // Load detail & schemas
  const loadDetailAndSchema = useCallback(async () => {
    setLoading(true);
    try {
      const [schemaRes, catRes] = await Promise.all([
        keysApi.editorSchema(),
        keysApi.listCategories(),
      ]);
      setSchemaResponse(schemaRes.data);
      setAvailableCategories(catRes.categories || []);

      if (keyId) {
        const detailRes = await keysApi.get(keyId);
        const k = detailRes.data;
        setDetail(k);
        setLabel(k.label);
        setStatus(k.status);
        setKind(k.kind);
        setCategory(k.category || "");
        setTemplateText(k.template_text || "");
        setProtocol(k.protocol);

        if (!k.safe_structured || !protocolEditorCapability(k.protocol)) {
          setMode("raw");
        } else {
          setMode("structured");
        }

        if (k.safe_structured) {
          setServer(k.safe_structured.server || "");
          setPort(k.safe_structured.port || "");
          setDisplayName(
            k.ownership === "external_source" && !k.client_display_name_overridden
              ? ""
              : (k.client_display_name || k.safe_structured.display_name || k.label)
          );

          if (isLegacyEditorProtocol(k.protocol)) {
            setLegacyDraft({
              ...emptyLegacyDraft(k.protocol),
              server: k.safe_structured.server || "",
              port: k.safe_structured.port || "443",
              remark: k.safe_structured.display_name || k.label,
            });
            setLegacySecretsRevealed(false);
          }

          if (k.safe_structured.shadowsocks) {
            setSsMethod(k.safe_structured.shadowsocks.method || "2022-blake3-aes-128-gcm");
            setSsPluginName(k.safe_structured.shadowsocks.plugin_name || "");
          }
          if (k.safe_structured.hysteria2) {
            setHy2Sni(k.safe_structured.hysteria2.sni || "");
            setHy2Insecure(k.safe_structured.hysteria2.insecure || false);
            setHy2CertSha(k.safe_structured.hysteria2.certificate_sha256 || "");
            setHy2ObfsType(k.safe_structured.hysteria2.obfuscation_type || "");
          }
          if (k.safe_structured.tuic) {
            setTuicSni(k.safe_structured.tuic.sni || "");
            setTuicAlpn((k.safe_structured.tuic.alpn || []).join(","));
            setTuicSkipCert(k.safe_structured.tuic.skip_cert_verify || false);
            setTuicCc(k.safe_structured.tuic.congestion_controller || "bbr");
            setTuicUdpRelay(k.safe_structured.tuic.udp_relay_mode || "native");
            setTuicUdpOverStream(k.safe_structured.tuic.udp_over_stream || false);
            setTuicZeroRtt(k.safe_structured.tuic.zero_rtt || false);
            setTuicHeartbeat(k.safe_structured.tuic.heartbeat || "10s");
          }
        }
      }
    } catch (error) {
      toast(error instanceof Error ? error.message : "Ошибка загрузки", "error");
    } finally {
      setLoading(false);
    }
  }, [keyId, toast]);

  useEffect(() => {
    if (open) {
      clearSensitiveState();
      void loadDetailAndSchema();
    } else {
      clearSensitiveState();
    }
  }, [open, loadDetailAndSchema, clearSensitiveState]);

  // Reveal secret call
  const handleReveal = async (target: "raw" | "structured-secrets") => {
    if (!keyId || !detail) return;
    setRevealing(true);
    try {
      const res = await keysApi.reveal(keyId, {
        profile_revision: detail.profile_revision,
        target,
      });
      if (target === "raw") {
        const rawRes = res.data as KeyRawSecretResponse;
        setAuthoritativeRaw(rawRes.raw_uri);
        let displayRaw = rawRes.raw_uri;
        let formattingWarning = "";
        if (isXrayJSONRaw(detail.protocol, rawRes.raw_uri)) {
          try {
            const inspection = inspectXrayJSONDocument(rawRes.raw_uri);
            if (inspection.duplicateKeys.length > 0) {
              formattingWarning = "JSON оставлен без форматирования: обнаружены повторяющиеся ключи.";
            } else {
              const formatted = formatXrayJSONWithinLimit(rawRes.raw_uri);
              displayRaw = formatted.value;
              if (formatted.exceededLimit) {
                formattingWarning = "JSON оставлен без форматирования: форматированная версия превышает допустимый размер.";
              }
            }
          } catch {
            displayRaw = rawRes.raw_uri;
          }
        }
        setRawUri(displayRaw);
        setRawFormattingWarning(formattingWarning);
        setRevealedRaw(true);
        setRawEdited(false);
        setRawValidationError("");
        if (isLegacyEditorProtocol(detail.protocol)) {
          try {
            setLegacyDraft(parseEditableConfiguration(rawRes.raw_uri));
            setLegacySecretsRevealed(true);
          } catch (error) {
            setRawValidationError(error instanceof Error ? error.message : "Не удалось разобрать конфигурацию");
          }
        }
        toast("Сырая ссылка раскрыта", "info");
      } else {
        const secRes = res.data as KeyStructuredSecretsResponse;
        if (secRes.secrets.password !== undefined) setSsPassword(secRes.secrets.password);
        if (secRes.secrets.plugin_options !== undefined) setSsPluginOptions(secRes.secrets.plugin_options);
        if (secRes.secrets.authentication !== undefined) setHy2Auth(secRes.secrets.authentication);
        if (secRes.secrets.obfuscation_password !== undefined) setHy2ObfsPassword(secRes.secrets.obfuscation_password);
        if (secRes.secrets.uuid !== undefined) setTuicUuid(secRes.secrets.uuid);
        if (secRes.secrets.password !== undefined) setTuicPassword(secRes.secrets.password);
        toast("Секретные поля раскрыты", "info");
      }
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setShowConflictDialog(true);
      } else {
        toast(error instanceof Error ? error.message : "Ошибка раскрытия секретов", "error");
      }
    } finally {
      setRevealing(false);
    }
  };

  // Clone call
  const handleClone = async () => {
    if (!keyId || !detail) return;
    setCloning(true);
    try {
      await keysApi.clone(keyId, {
        expected_profile_revision: detail.profile_revision,
      });
      toast("Локальный клон успешно создан", "success");
      await onRefresh();
      onClose();
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setShowConflictDialog(true);
      } else {
        toast(error instanceof Error ? error.message : "Ошибка клонирования", "error");
      }
    } finally {
      setCloning(false);
    }
  };

  const isSourceOwned = detail?.ownership === "external_source";
  const sourceDefaultClientName = detail?.label || "";
  const schemaForProtocol = useMemo(() => {
    return schemaResponse?.protocols.find((p) => p.protocol === protocol);
  }, [schemaResponse, protocol]);

  const changeLegacyDraft = (patch: XrayJSONPatch) => {
    setLegacyDraft((current) => ({ ...current, ...patch }));
    if (patch.server !== undefined) setServer(patch.server);
    if (patch.port !== undefined) setPort(patch.port);
    setStructuredEdited(true);
    setIsDirty(true);
    if (revealedRaw && rawUri) {
      try {
        const nextRaw = patchEditableConfiguration(rawUri, patch);
        setRawUri(nextRaw);
        setRawValidationError("");
      } catch (error) {
        setRawValidationError(error instanceof Error ? error.message : "Не удалось обновить raw-представление");
      }
    }
  };

  const markStructuredChange = () => {
    setStructuredEdited(true);
    setIsDirty(true);
  };

  const changeRaw = (value: string) => {
    setRawUri(value);
    setRawEdited(true);
    setIsDirty(true);
    setRawValidationError("");
    setRawFormattingWarning("");
    try {
      if (isXrayJSONRaw(protocol, value)) {
        parseXrayJSONConfiguration(value);
      } else if (isLegacyEditorProtocol(protocol)) {
        const parsed = parseEditableConfiguration(value);
        setLegacyDraft(parsed);
        setLegacySecretsRevealed(true);
      }
    } catch (error) {
      setRawValidationError(error instanceof Error ? error.message : "Некорректная конфигурация");
    }
  };

  const buildLegacyCreateRaw = () =>
    createConfigurationFromXrayJSON(buildXrayJSONConfiguration({
      ...legacyDraft,
      remark: displayName.trim() || label.trim(),
    }));

  const validateRawBeforeSubmit = (value: string) => {
    if (!value.trim()) throw new Error("Сначала заполните raw-конфигурацию");
    if (isXrayJSONRaw(protocol, value)) {
      const inspection = inspectXrayJSONDocument(value);
      if (inspection.duplicateKeys.length > 0) {
        throw new Error("XRAY-JSON содержит повторяющиеся ключи. Устраните неоднозначность перед сохранением.");
      }
      parseXrayJSONConfiguration(value);
    }
    else if (isLegacyEditorProtocol(protocol)) parseEditableConfiguration(value);
  };

  // Submit Handler
  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!label.trim()) return;

    setSaving(true);
    try {
      if (keyId && detail) {
        // Update existing key
        if (isSourceOwned) {
          await keysApi.updateProfile(keyId, {
            label: detail.label,
            client_display_name: displayName.trim(),
            category: detail.category,
            status: detail.status,
            kind: detail.kind,
            template_text: detail.template_text,
            profile_revision: detail.profile_revision,
            patch_mode: "structured",
          });
          toast("Название в клиенте обновлено", "success");
          await onRefresh();
          onClose();
          return;
        }

        let patch: StructuredProfilePatch | undefined;

        if (mode === "structured" && kind === "real") {
          patch = {};
          if (server !== (detail.safe_structured?.server || "")) {
            patch.server = { operation: "set", value: server };
          }
          if (port !== (detail.safe_structured?.port || "")) {
            patch.port = { operation: "set", value: port };
          }
          if (protocol === "shadowsocks") {
            patch.shadowsocks = buildShadowsocksPatch(
              detail.safe_structured?.shadowsocks?.method || "2022-blake3-aes-128-gcm",
              ssMethod,
              ssPassword,
              detail.safe_structured?.shadowsocks?.plugin_name || "",
              ssPluginName,
              "",
              ssPluginOptions
            );
          } else if (protocol === "hysteria2") {
            patch.hysteria2 = buildHysteria2Patch(
              detail.safe_structured?.hysteria2?.sni || "", hy2Sni,
              detail.safe_structured?.hysteria2?.insecure || false, hy2Insecure,
              detail.safe_structured?.hysteria2?.certificate_sha256 || "", hy2CertSha,
              detail.safe_structured?.hysteria2?.obfuscation_type || "", hy2ObfsType,
              hy2Auth,
              hy2ObfsPassword
            );
          } else if (protocol === "tuic") {
            patch.tuic = buildTUICPatch(
              detail.safe_structured?.tuic?.sni || "", tuicSni,
              (detail.safe_structured?.tuic?.alpn || []).join(","), tuicAlpn,
              detail.safe_structured?.tuic?.skip_cert_verify || false, tuicSkipCert,
              detail.safe_structured?.tuic?.congestion_controller || "bbr", tuicCc,
              detail.safe_structured?.tuic?.udp_relay_mode || "native", tuicUdpRelay,
              detail.safe_structured?.tuic?.udp_over_stream || false, tuicUdpOverStream,
              detail.safe_structured?.tuic?.zero_rtt || false, tuicZeroRtt,
              detail.safe_structured?.tuic?.heartbeat || "10s", tuicHeartbeat,
              tuicUuid,
              tuicPassword
            );
          }
        }

        let patchMode: "raw" | "structured" = mode;
        let rawForUpdate: string | undefined;
        if (kind === "real" && isLegacyEditorProtocol(protocol)) {
          if (revealedRaw) {
            validateRawBeforeSubmit(rawUri);
            patchMode = "raw";
            rawForUpdate = structuredEdited || rawEdited ? rawUri : authoritativeRaw;
          } else if (mode === "raw") {
            throw new Error("Сначала раскройте raw-конфигурацию");
          } else {
            patchMode = "structured";
            patch ||= {};
          }
        } else if (kind === "real" && protocol === "xray-json") {
          if (rawEdited) {
            validateRawBeforeSubmit(rawUri);
            patchMode = "raw";
            rawForUpdate = rawUri;
          } else {
            patchMode = "structured";
            patch = {};
          }
        } else if (kind === "real" && mode === "raw") {
          if (!revealedRaw) throw new Error("Сначала раскройте raw-конфигурацию");
          if (structuredEdited) throw new Error("Structured-версия уже изменена. Вернитесь в structured режим для сохранения или откройте профиль заново.");
          patchMode = "raw";
          rawForUpdate = rawEdited ? rawUri : authoritativeRaw;
        } else if (kind === "real" && rawEdited) {
          throw new Error("Raw-версия уже изменена. Вернитесь в raw режим для сохранения или откройте профиль заново.");
        }

        const updateInput: UpdateKeyProfileInput = {
          label: label.trim(),
          client_display_name: displayName.trim() !== detail.client_display_name
            ? (displayName.trim() || label.trim())
            : undefined,
          category,
          status,
          kind,
          template_text: templateText,
          profile_revision: detail.profile_revision,
          patch_mode: patchMode,
          raw_uri: patchMode === "raw" ? rawForUpdate : undefined,
          structured_patch: patchMode === "structured" ? (patch || {}) : undefined,
        };

        await keysApi.updateProfile(keyId, updateInput);
        toast("Профиль успешно обновлён", "success");
      } else {
        // Create new local key
        let patch: StructuredProfilePatch | undefined;

        if (mode === "structured" && kind === "real") {
          patch = {
            server: { operation: "set", value: server },
            port: { operation: "set", value: port },
            display_name: { operation: "set", value: displayName || label },
          };
          if (protocol === "shadowsocks") {
            patch.shadowsocks = {
              method: { operation: "set", value: ssMethod },
              password: { operation: "set", value: ssPassword || "" },
              plugin_name: ssPluginName ? { operation: "set", value: ssPluginName } : undefined,
              plugin_options: ssPluginOptions ? { operation: "set", value: ssPluginOptions } : undefined,
            };
          } else if (protocol === "hysteria2") {
            patch.hysteria2 = {
              authentication: { operation: "set", value: hy2Auth || "" },
              sni: hy2Sni ? { operation: "set", value: hy2Sni } : undefined,
              insecure: { operation: "set", value: hy2Insecure },
              certificate_sha256: hy2CertSha ? { operation: "set", value: hy2CertSha } : undefined,
              obfuscation_type: hy2ObfsType ? { operation: "set", value: hy2ObfsType } : undefined,
              obfuscation_password: hy2ObfsPassword ? { operation: "set", value: hy2ObfsPassword } : undefined,
            };
          } else if (protocol === "tuic") {
            patch.tuic = {
              uuid: { operation: "set", value: tuicUuid || "" },
              password: { operation: "set", value: tuicPassword || "" },
              sni: tuicSni ? { operation: "set", value: tuicSni } : undefined,
              alpn: tuicAlpn ? { operation: "set", value: tuicAlpn.split(",").map((s) => s.trim()).filter(Boolean) } : undefined,
              skip_cert_verify: { operation: "set", value: tuicSkipCert },
              congestion_controller: { operation: "set", value: tuicCc },
              udp_relay_mode: { operation: "set", value: tuicUdpRelay },
              udp_over_stream: { operation: "set", value: tuicUdpOverStream },
              zero_rtt: { operation: "set", value: tuicZeroRtt },
              heartbeat: { operation: "set", value: tuicHeartbeat },
            };
          }
        }

        let creationMode: "raw" | "structured" = mode;
        let rawForCreate = mode === "raw" ? rawUri : undefined;
        if (kind === "real" && isLegacyEditorProtocol(protocol) && mode === "structured") {
          rawForCreate = buildLegacyCreateRaw();
          validateRawBeforeSubmit(rawForCreate);
          creationMode = "raw";
          patch = undefined;
        } else if (kind === "real" && mode === "raw") {
          validateRawBeforeSubmit(rawUri);
        }

        const createInput: CreateKeyProfileInput = {
          label: label.trim(),
          client_display_name: displayName.trim() || label.trim(),
          category,
          status,
          kind,
          template_text: templateText,
          creation_mode: creationMode,
          raw_uri: creationMode === "raw" ? rawForCreate : undefined,
          protocol,
          structured: creationMode === "structured" ? patch : undefined,
        };

        await keysApi.createProfile(createInput);
        toast("Профиль успешно создан", "success");
      }

      await onRefresh();
      onClose();
    } catch (error) {
      if (error instanceof ApiError && error.status === 409) {
        setShowConflictDialog(true);
      } else {
        toast(error instanceof Error ? error.message : "Ошибка сохранения", "error");
      }
    } finally {
      setSaving(false);
    }
  };

  const copyRaw = async () => {
    const value = rawEdited ? rawUri : authoritativeRaw || rawUri;
    if (!value) return;
    const ok = await copyToClipboard(value);
    if (ok) toast("Сырая ссылка скопирована", "success");
    else toast("Не удалось скопировать", "error");
  };

  const titleText = keyId
    ? `${kind === "informational" ? "Изменить информационный ключ" : "Изменить конфигурацию"} — ${detail?.label || label || initialLabel || ""}`
    : kind === "informational" || initialKind === "informational"
    ? "Добавить информационный ключ"
    : "Добавить конфигурацию";

  return (
    <>
      <Modal
        open={open}
        onClose={handleCloseAttempt}
        title={titleText}
        className="flex h-[92dvh] max-h-[56rem] w-[96vw] max-w-6xl flex-col overflow-hidden"
        contentClassName="flex min-h-0 flex-1 flex-col overflow-hidden"
      >
        {loading ? (
          <div className="flex min-h-64 items-center justify-center text-sm text-zinc-400">
            Загрузка профиля и схем...
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
            <div className="ui-key-editor-scroll-region min-h-0 flex-1 overflow-y-auto space-y-4 pr-1">
              {/* Header Info & Source Banner */}
              {isSourceOwned && (
                <div className="flex items-center justify-between gap-3 rounded-sm border border-sky-500/30 bg-sky-500/10 p-3.5 text-xs text-sky-200">
                  <div className="flex items-center gap-2">
                    <Info className="h-4 w-4 shrink-0 text-sky-300" aria-hidden="true" />
                    <span>
                      Управляется источником <strong>{detail?.external_source_name}</strong>. Изменение параметров профиля заблокировано.
                    </span>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    loading={cloning}
                    onClick={handleClone}
                    className="border-sky-500/40 text-sky-200 hover:bg-sky-500/20"
                  >
                    Клонировать как локальный
                  </Button>
                </div>
              )}

              <div
                data-testid="key-editor-workspace"
                className={`ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 rounded-sm ${kind === "real" ? "lg:grid-cols-2" : ""}`}
              >
                {/* Main Parameters */}
                <section className="rounded-sm border border-border bg-surface-2/25 p-4">
                  <div className="system-label mb-3">ОСНОВНЫЕ ПАРАМЕТРЫ</div>
                  <div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
                    <div className="min-w-0 md:col-span-2">
                      <Input
                        label="Название"
                        value={label}
                        maxLength={LABEL_LIMIT}
                        onChange={(e) => { setLabel(e.target.value); setIsDirty(true); }}
                        disabled={Boolean(isSourceOwned)}
                        required
                      />
                      <p className="mt-1 text-xs text-zinc-600">Используется только в панели управления.</p>
                    </div>
                    {kind === "real" && (
                      <div className="min-w-0 md:col-span-2">
                        <Input
                          label="Название в клиенте"
                          value={displayName}
                          maxLength={LABEL_LIMIT}
                          placeholder={isSourceOwned ? sourceDefaultClientName : (label || "Совпадает с названием в панели")}
                          onChange={(event) => { setDisplayName(event.target.value); setIsDirty(true); }}
                        />
                        {isSourceOwned ? (
                          <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-600">
                            <span>Локальное название для подписки. Не изменяется при синхронизации источника. По умолчанию: {sourceDefaultClientName}</span>
                            <button
                              type="button"
                              className="text-sky-300 transition-colors hover:text-sky-200 disabled:opacity-50"
                              disabled={!displayName && !detail?.client_display_name_overridden}
                              onClick={() => {
                                setDisplayName("");
                                setIsDirty(Boolean(detail?.client_display_name_overridden));
                              }}
                            >
                              Использовать название источника
                            </button>
                          </div>
                        ) : (
                          <p className="mt-1 text-xs text-zinc-600">Отображается пользователю в приложении после добавления подписки.</p>
                        )}
                      </div>
                    )}
                    <Select
                      label="Статус"
                      value={status}
                      onChange={(e) => { setStatus(e.target.value as "active" | "non-active"); setIsDirty(true); }}
                      options={[
                        { value: "active", label: "Активен" },
                        { value: "non-active", label: "Неактивен" },
                      ]}
                      disabled={Boolean(isSourceOwned)}
                    />
                    <Select
                      label="Тип"
                      value={kind}
                      onChange={(e) => { setKind(e.target.value as "real" | "informational"); setIsDirty(true); }}
                      options={[
                        { value: "real", label: "Конфигурация" },
                        { value: "informational", label: "Информационный ключ" },
                      ]}
                      disabled={Boolean(isSourceOwned)}
                    />
                    <div className="min-w-0 md:col-span-2 xl:col-span-4">
                      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_44px] items-end gap-2">
                        <Select
                          label="Категория"
                          value={category}
                          onChange={(e) => { setCategory(e.target.value); setIsDirty(true); }}
                          options={[{ value: "", label: "Без категории" }].concat(
                            availableCategories.map((item) => ({ value: item.name, label: item.name }))
                          )}
                          disabled={Boolean(isSourceOwned)}
                        />
                        <button
                          type="button"
                          className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-zinc-400 transition-colors hover:bg-surface-2 hover:text-zinc-100"
                          onClick={() => setShowCreateCategory(true)}
                          disabled={Boolean(isSourceOwned)}
                          title="Добавить категорию"
                        >
                          <FolderPlus className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    </div>
                  </div>
                </section>

                {kind === "informational" && (
                  <section className="space-y-4 rounded-sm border border-border bg-surface-1 p-4">
                    <div className="system-label">ИНФОРМАЦИОННЫЙ КЛЮЧ</div>
                    <label className="block space-y-2">
                      <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
                        Текст информационного ключа
                      </span>
                      <textarea
                        ref={templateTextRef}
                        aria-label="Текст информационного ключа"
                        value={templateText}
                        maxLength={8192}
                        onChange={(event) => {
                          setTemplateText(event.target.value);
                          setIsDirty(true);
                        }}
                        className="min-h-32 w-full resize-y rounded-sm border border-border bg-surface-2 p-3 text-sm leading-6 text-zinc-100 focus:border-accent focus:outline-none"
                        placeholder="Например: Подписка {user_name} действует до {expires_date}"
                      />
                    </label>

                    <div>
                      <div className="mb-2 text-sm font-semibold text-zinc-200">Доступные переменные</div>
                      <div className="flex flex-wrap gap-2">
                        {keyTemplateVariables.map((variable) => (
                          <button
                            key={variable.token}
                            type="button"
                            onClick={() => insertTemplateVariable(variable.token)}
                            className="rounded-sm border border-border bg-surface-2 px-2.5 py-2 text-left transition-colors hover:border-accent"
                            title={`Вставить ${variable.token}`}
                          >
                            <code className="block text-xs text-accent">{variable.token}</code>
                            <span className="mt-0.5 block text-[11px] text-zinc-500">{variable.description}</span>
                          </button>
                        ))}
                      </div>
                    </div>

                    <div>
                      <div className="mb-2 text-sm font-semibold text-zinc-200">Предпросмотр</div>
                      <div className="rounded-sm border border-border bg-surface-2 p-3" data-testid="informational-preview">
                        <div className="text-[11px] text-zinc-500">Пример для подписчика</div>
                        <div className="mt-2 whitespace-pre-wrap break-words text-sm text-zinc-100">
                          {templatePreviewParts.length === 0 ? (
                            <span className="text-zinc-600">Введите текст информационного ключа</span>
                          ) : templatePreviewParts.map((part, index) => part.unknown ? (
                            <span
                              key={`${part.text}-${index}`}
                              data-unknown-placeholder
                              className="rounded-sm bg-amber-500/15 px-1 text-amber-300 ring-1 ring-amber-500/30"
                              title="Неизвестная переменная"
                            >
                              {part.text}
                            </span>
                          ) : <span key={`${part.text}-${index}`}>{part.text}</span>)}
                        </div>
                        <div className="mt-3 border-t border-border pt-2 text-xs text-zinc-500">
                          Название в панели: <span className="text-zinc-300">{label || "Без названия"}</span>
                        </div>
                      </div>
                    </div>
                  </section>
                )}

                {/* Real Profile Workspace */}
                {kind === "real" && (
                  <section className="space-y-4 rounded-sm border border-border bg-surface-1 p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="system-label">РЕДАКТОР ПРОФИЛЯ</div>
                    <div className="flex rounded-sm border border-border bg-surface-2 p-1">
                      <button
                        type="button"
                        onClick={() => setMode("structured")}
                        className={`px-3 py-1 text-xs font-medium rounded-sm ${
                          mode === "structured" ? "bg-accent text-accent-fg" : "text-zinc-400"
                        }`}
                      >
                        Структурированный
                      </button>
                      <button
                        type="button"
                        aria-label="XRAY-JSON"
                        onClick={() => setMode("raw")}
                        className={`px-3 py-1 text-xs font-medium rounded-sm ${
                          mode === "raw" ? "bg-accent text-accent-fg" : "text-zinc-400"
                        }`}
                      >
                        Raw / XRAY-JSON
                      </button>
                    </div>
                  </div>

                  {!keyId && (
                    <Select
                      label="Протокол"
                      value={protocol}
                      onChange={(e) => {
                        const nextProtocol = e.target.value as ExternalProfileProtocol;
                        setProtocol(nextProtocol);
                        setRawUri("");
                        setAuthoritativeRaw("");
                        setRawEdited(false);
                        setStructuredEdited(false);
                        setRawValidationError("");
                        if (isLegacyEditorProtocol(nextProtocol)) {
                          setLegacyDraft(emptyLegacyDraft(nextProtocol));
                          setLegacySecretsRevealed(true);
                          setServer("");
                          setPort("443");
                        }
                        setIsDirty(true);
                      }}
                      options={PROTOCOL_EDITOR_CAPABILITIES.map((item) => ({ value: item.protocol, label: item.label }))}
                    />
                  )}

                  <div className={mode === "structured" ? "space-y-4" : "hidden space-y-4"}>
                    {detail?.safe_structured?.tuic?.generation === 4 && (
                      <TuicV4ReadOnlyBanner tokenPresent={detail.safe_structured.tuic.token_present} />
                    )}

                    {isLegacyEditorProtocol(protocol) && (
                      <LegacyXrayFields
                        draft={legacyDraft}
                        onChange={changeLegacyDraft}
                        readOnly={Boolean(isSourceOwned)}
                        secretsRevealed={legacySecretsRevealed}
                        onReveal={() => void handleReveal("raw")}
                      />
                    )}

                    {/* Connection Fields */}
                    {!isLegacyEditorProtocol(protocol) && <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <Input
                        label="Сервер (Host)"
                        value={server}
                        onChange={(e) => { setServer(e.target.value); markStructuredChange(); }}
                        disabled={isSourceOwned}
                      />
                      <Input
                        label="Порт / Выражение портов"
                        value={port}
                        placeholder="443 или 20000-50000"
                        onChange={(e) => { setPort(e.target.value); markStructuredChange(); }}
                        disabled={isSourceOwned}
                      />
                    </div>}

                    {/* Secret Reveal Trigger Banner */}
                    {keyId && !isSourceOwned && !isLegacyEditorProtocol(protocol) && (
                      <div className="flex items-center justify-between rounded-sm border border-border bg-surface-2/40 p-3 text-xs text-zinc-300">
                        <div className="flex items-center gap-2">
                          <Lock className="h-4 w-4 text-zinc-400" aria-hidden="true" />
                          <span>Секретные поля скрыты по умолчанию.</span>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          loading={revealing}
                          onClick={() => handleReveal("structured-secrets")}
                        >
                          <Eye className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                          Раскрыть секреты
                        </Button>
                      </div>
                    )}

                    {/* Protocol Specific Editors */}
                    {protocol === "shadowsocks" && (
                      <ShadowsocksFields
                        schema={schemaForProtocol}
                        method={ssMethod}
                        onMethodChange={(v) => { setSsMethod(v); markStructuredChange(); }}
                        password={ssPassword}
                        onPasswordChange={(v) => { setSsPassword(v); markStructuredChange(); }}
                        pluginName={ssPluginName}
                        onPluginNameChange={(v) => { setSsPluginName(v); markStructuredChange(); }}
                        pluginOptions={ssPluginOptions}
                        onPluginOptionsChange={(v) => { setSsPluginOptions(v); markStructuredChange(); }}
                        readOnly={isSourceOwned}
                      />
                    )}

                    {protocol === "hysteria2" && (
                      <Hysteria2Fields
                        schema={schemaForProtocol}
                        authentication={hy2Auth}
                        onAuthChange={(v) => { setHy2Auth(v); markStructuredChange(); }}
                        sni={hy2Sni}
                        onSniChange={(v) => { setHy2Sni(v); markStructuredChange(); }}
                        insecure={hy2Insecure}
                        onInsecureChange={(v) => { setHy2Insecure(v); markStructuredChange(); }}
                        certSha256={hy2CertSha}
                        onCertSha256Change={(v) => { setHy2CertSha(v); markStructuredChange(); }}
                        obfsType={hy2ObfsType}
                        onObfsTypeChange={(v) => { setHy2ObfsType(v); markStructuredChange(); }}
                        obfsPassword={hy2ObfsPassword}
                        onObfsPasswordChange={(v) => { setHy2ObfsPassword(v); markStructuredChange(); }}
                        readOnly={isSourceOwned}
                      />
                    )}

                    {protocol === "tuic" && detail?.safe_structured?.tuic?.generation !== 4 && (
                      <TuicV5Fields
                        schema={schemaForProtocol}
                        uuid={tuicUuid}
                        onUuidChange={(v) => { setTuicUuid(v); markStructuredChange(); }}
                        password={tuicPassword}
                        onPasswordChange={(v) => { setTuicPassword(v); markStructuredChange(); }}
                        sni={tuicSni}
                        onSniChange={(v) => { setTuicSni(v); markStructuredChange(); }}
                        alpn={tuicAlpn}
                        onAlpnChange={(v) => { setTuicAlpn(v); markStructuredChange(); }}
                        skipCertVerify={tuicSkipCert}
                        onSkipCertVerifyChange={(v) => { setTuicSkipCert(v); markStructuredChange(); }}
                        congestionControl={tuicCc}
                        onCongestionControlChange={(v) => { setTuicCc(v); markStructuredChange(); }}
                        udpRelayMode={tuicUdpRelay}
                        onUdpRelayModeChange={(v) => { setTuicUdpRelay(v); markStructuredChange(); }}
                        udpOverStream={tuicUdpOverStream}
                        onUdpOverStreamChange={(v) => { setTuicUdpOverStream(v); markStructuredChange(); }}
                        zeroRtt={tuicZeroRtt}
                        onZeroRttChange={(v) => { setTuicZeroRtt(v); markStructuredChange(); }}
                        heartbeat={tuicHeartbeat}
                        onHeartbeatChange={(v) => { setTuicHeartbeat(v); markStructuredChange(); }}
                        readOnly={isSourceOwned}
                      />
                    )}
                  </div>

                  {/* Raw Mode */}
                  <div className={mode === "raw" ? "space-y-3" : "hidden space-y-3"}>
                    {keyId && !revealedRaw && (
                      <div className="flex items-center justify-between rounded-sm border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs text-amber-200">
                        <div className="flex items-center gap-2">
                          <Lock className="h-4 w-4 shrink-0 text-amber-400" aria-hidden="true" />
                          <span>Сырой URI содержит расшифрованные учетные данные.</span>
                        </div>
                        <Button
                          type="button"
                          variant="ghost"
                          loading={revealing}
                          onClick={() => handleReveal("raw")}
                        >
                          <Eye className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                          Раскрыть сырую ссылку
                        </Button>
                      </div>
                    )}

                    <textarea
                      id="key-editor-raw"
                      value={rawUri}
                      aria-label="Raw-конфигурация"
                      onChange={(e) => changeRaw(e.target.value)}
                      placeholder="vless://... / vmess://... / trojan://... / ss://... / hysteria2://... / tuic://... / { XRAY-JSON }"
                      rows={16}
                      disabled={isSourceOwned || (keyId ? !revealedRaw : false)}
                      spellCheck={false}
                      className="min-h-72 w-full resize-y overflow-auto whitespace-pre rounded-sm border border-border bg-surface-2 p-3 font-mono text-xs leading-5 text-zinc-200"
                    />

                    {rawValidationError && (
                      <div role="alert" className="rounded-sm border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">
                        {rawValidationError}
                      </div>
                    )}

                    {rawFormattingWarning && (
                      <div role="status" className="rounded-sm border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-200">
                        {rawFormattingWarning}
                      </div>
                    )}

                    {revealedRaw && isXrayJSONRaw(protocol, authoritativeRaw) && !rawEdited && authoritativeRaw !== rawUri && (
                      <div className="text-[11px] text-zinc-500">
                        JSON отформатирован только для отображения. Сохранение и «Копировать raw» используют исходное представление, пока текст не изменён вручную.
                      </div>
                    )}

                    {rawUri && (
                      <div className="flex justify-end">
                        <Button type="button" variant="ghost" onClick={copyRaw}>
                          <Copy className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                          Копировать raw
                        </Button>
                      </div>
                    )}
                  </div>

                  {/* Capabilities Matrix */}
                  <OutputCapabilitiesMatrix
                    capabilities={detail?.capabilities}
                    exclusionReasonsCatalog={schemaResponse?.exclusion_reason_codes}
                  />
                </section>
              )}
              </div>
            </div>

            {/* Modal Actions */}
            <div className="-mx-5 -mb-5 mt-4 flex shrink-0 justify-end gap-2 border-t border-border bg-surface-1 px-5 py-4">
              <Button type="button" variant="ghost" onClick={handleCloseAttempt} disabled={saving}>
                Отмена
              </Button>
              <Button type="submit" loading={saving} disabled={Boolean(isSourceOwned && !isDirty)}>
                {keyId ? "Сохранить" : "Добавить профиль"}
              </Button>
            </div>
          </form>
        )}

        <CreateKeyCategoryModal
          open={showCreateCategory}
          onClose={() => setShowCreateCategory(false)}
          onCreated={(nextCategory) => {
            setAvailableCategories((prev) => [...prev, nextCategory]);
            setCategory(nextCategory.name);
            setShowCreateCategory(false);
          }}
        />
      </Modal>

      <ConfirmDialog
        open={showDiscardConfirm}
        title="Закрыть без сохранения?"
        message="Внесённые изменения будут потеряны."
        confirmLabel="Закрыть"
        onCancel={() => setShowDiscardConfirm(false)}
        onConfirm={() => {
          setShowDiscardConfirm(false);
          onClose();
        }}
      />

      <KeyEditorConflictDialog
        open={showConflictDialog}
        onRefreshLatest={async () => {
          setShowConflictDialog(false);
          await loadDetailAndSchema();
        }}
        onClose={() => setShowConflictDialog(false)}
      />
    </>
  );
}
