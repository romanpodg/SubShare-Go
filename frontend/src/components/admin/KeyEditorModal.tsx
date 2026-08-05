"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
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
import { OutputCapabilitiesMatrix } from "./OutputCapabilitiesMatrix";
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

  const defaultMode = initialProtocol && initialProtocol !== "shadowsocks" && initialProtocol !== "hysteria2" && initialProtocol !== "tuic" ? "raw" : "structured";

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
  const [mode, setMode] = useState<"structured" | "raw">(defaultMode);
  const [protocol, setProtocol] = useState<ExternalProfileProtocol>(initialProtocol || "shadowsocks");

  // Raw URI / JSON state
  const [rawUri, setRawUri] = useState("");
  const [revealedRaw, setRevealedRaw] = useState(false);

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
    setRevealedRaw(false);
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
  }, [defaultMode, initialCategory, initialKind, initialLabel, initialProtocol]);

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

        if (!k.safe_structured || (k.protocol !== "shadowsocks" && k.protocol !== "hysteria2" && k.protocol !== "tuic")) {
          setMode("raw");
        } else {
          setMode("structured");
        }

        if (k.safe_structured) {
          setServer(k.safe_structured.server || "");
          setPort(k.safe_structured.port || "");
          setDisplayName(k.safe_structured.display_name || "");

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
        setRawUri(rawRes.raw_uri);
        setRevealedRaw(true);
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
  const schemaForProtocol = useMemo(() => {
    return schemaResponse?.protocols.find((p) => p.protocol === protocol);
  }, [schemaResponse, protocol]);

  // Submit Handler
  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!label.trim()) return;

    setSaving(true);
    try {
      if (keyId && detail) {
        // Update existing key
        let patch: StructuredProfilePatch | undefined;

        if (mode === "structured" && kind === "real") {
          patch = {};
          if (server !== (detail.safe_structured?.server || "")) {
            patch.server = { operation: "set", value: server };
          }
          if (port !== (detail.safe_structured?.port || "")) {
            patch.port = { operation: "set", value: port };
          }
          if (displayName !== (detail.safe_structured?.display_name || "")) {
            patch.display_name = { operation: "set", value: displayName };
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

        const updateInput: UpdateKeyProfileInput = {
          label: label.trim(),
          category,
          status,
          kind,
          template_text: templateText,
          profile_revision: detail.profile_revision,
          patch_mode: mode,
          raw_uri: mode === "raw" ? rawUri : undefined,
          structured_patch: mode === "structured" ? patch : undefined,
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

        const createInput: CreateKeyProfileInput = {
          label: label.trim(),
          category,
          status,
          kind,
          template_text: templateText,
          creation_mode: mode,
          raw_uri: mode === "raw" ? rawUri : undefined,
          protocol,
          structured: mode === "structured" ? patch : undefined,
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
    if (!rawUri) return;
    const ok = await copyToClipboard(rawUri);
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

              <div className="ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 lg:grid-cols-2 rounded-sm">
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
                        required
                      />
                    </div>
                    <Select
                      label="Статус"
                      value={status}
                      onChange={(e) => { setStatus(e.target.value as "active" | "non-active"); setIsDirty(true); }}
                      options={[
                        { value: "active", label: "Активен" },
                        { value: "non-active", label: "Неактивен" },
                      ]}
                    />
                    <Select
                      label="Тип"
                      value={kind}
                      onChange={(e) => { setKind(e.target.value as "real" | "informational"); setIsDirty(true); }}
                      options={[
                        { value: "real", label: "Конфигурация" },
                        { value: "informational", label: "Информационный ключ" },
                      ]}
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
                        />
                        <button
                          type="button"
                          className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-zinc-400 transition-colors hover:bg-surface-2 hover:text-zinc-100"
                          onClick={() => setShowCreateCategory(true)}
                          title="Добавить категорию"
                        >
                          <FolderPlus className="h-4 w-4" aria-hidden="true" />
                        </button>
                      </div>
                    </div>
                  </div>
                </section>

                {/* Real Profile Workspace */}
                {kind === "real" && (
                  <section className="space-y-4 rounded-sm border border-border bg-surface-1 p-4">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <div className="system-label">РЕДАКТОР ПРОФИЛЯ</div>
                    <div className="flex rounded-sm border border-border bg-surface-2 p-1">
                      <button
                        type="button"
                        onClick={() => { setMode("structured"); setIsDirty(true); }}
                        className={`px-3 py-1 text-xs font-medium rounded-sm ${
                          mode === "structured" ? "bg-accent text-accent-fg" : "text-zinc-400"
                        }`}
                      >
                        Структурированный
                      </button>
                      <button
                        type="button"
                        onClick={() => { setMode("raw"); setIsDirty(true); }}
                        className={`px-3 py-1 text-xs font-medium rounded-sm ${
                          mode === "raw" ? "bg-accent text-accent-fg" : "text-zinc-400"
                        }`}
                      >
                        XRAY-JSON
                      </button>
                    </div>
                  </div>

                  {!keyId && (
                    <Select
                      label="Протокол"
                      value={protocol}
                      onChange={(e) => { setProtocol(e.target.value as ExternalProfileProtocol); setIsDirty(true); }}
                      options={[
                        { value: "shadowsocks", label: "Shadowsocks" },
                        { value: "hysteria2", label: "Hysteria 2" },
                        { value: "tuic", label: "TUIC v5" },
                      ]}
                    />
                  )}

                  <div className={mode === "structured" ? "space-y-4" : "hidden space-y-4"}>
                    {detail?.safe_structured?.tuic?.generation === 4 && (
                      <TuicV4ReadOnlyBanner tokenPresent={detail.safe_structured.tuic.token_present} />
                    )}

                    {/* Connection Fields */}
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
                      <Input
                        label="Сервер (Host)"
                        value={server}
                        onChange={(e) => { setServer(e.target.value); setIsDirty(true); }}
                        disabled={isSourceOwned}
                      />
                      <Input
                        label="Порт / Выражение портов"
                        value={port}
                        placeholder="443 или 20000-50000"
                        onChange={(e) => { setPort(e.target.value); setIsDirty(true); }}
                        disabled={isSourceOwned}
                      />
                    </div>

                    {/* Secret Reveal Trigger Banner */}
                    {keyId && !isSourceOwned && (
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
                        onMethodChange={(v) => { setSsMethod(v); setIsDirty(true); }}
                        password={ssPassword}
                        onPasswordChange={(v) => { setSsPassword(v); setIsDirty(true); }}
                        pluginName={ssPluginName}
                        onPluginNameChange={(v) => { setSsPluginName(v); setIsDirty(true); }}
                        pluginOptions={ssPluginOptions}
                        onPluginOptionsChange={(v) => { setSsPluginOptions(v); setIsDirty(true); }}
                        readOnly={isSourceOwned}
                      />
                    )}

                    {protocol === "hysteria2" && (
                      <Hysteria2Fields
                        schema={schemaForProtocol}
                        authentication={hy2Auth}
                        onAuthChange={(v) => { setHy2Auth(v); setIsDirty(true); }}
                        sni={hy2Sni}
                        onSniChange={(v) => { setHy2Sni(v); setIsDirty(true); }}
                        insecure={hy2Insecure}
                        onInsecureChange={(v) => { setHy2Insecure(v); setIsDirty(true); }}
                        certSha256={hy2CertSha}
                        onCertSha256Change={(v) => { setHy2CertSha(v); setIsDirty(true); }}
                        obfsType={hy2ObfsType}
                        onObfsTypeChange={(v) => { setHy2ObfsType(v); setIsDirty(true); }}
                        obfsPassword={hy2ObfsPassword}
                        onObfsPasswordChange={(v) => { setHy2ObfsPassword(v); setIsDirty(true); }}
                        readOnly={isSourceOwned}
                      />
                    )}

                    {protocol === "tuic" && detail?.safe_structured?.tuic?.generation !== 4 && (
                      <TuicV5Fields
                        schema={schemaForProtocol}
                        uuid={tuicUuid}
                        onUuidChange={(v) => { setTuicUuid(v); setIsDirty(true); }}
                        password={tuicPassword}
                        onPasswordChange={(v) => { setTuicPassword(v); setIsDirty(true); }}
                        sni={tuicSni}
                        onSniChange={(v) => { setTuicSni(v); setIsDirty(true); }}
                        alpn={tuicAlpn}
                        onAlpnChange={(v) => { setTuicAlpn(v); setIsDirty(true); }}
                        skipCertVerify={tuicSkipCert}
                        onSkipCertVerifyChange={(v) => { setTuicSkipCert(v); setIsDirty(true); }}
                        congestionControl={tuicCc}
                        onCongestionControlChange={(v) => { setTuicCc(v); setIsDirty(true); }}
                        udpRelayMode={tuicUdpRelay}
                        onUdpRelayModeChange={(v) => { setTuicUdpRelay(v); setIsDirty(true); }}
                        udpOverStream={tuicUdpOverStream}
                        onUdpOverStreamChange={(v) => { setTuicUdpOverStream(v); setIsDirty(true); }}
                        zeroRtt={tuicZeroRtt}
                        onZeroRttChange={(v) => { setTuicZeroRtt(v); setIsDirty(true); }}
                        heartbeat={tuicHeartbeat}
                        onHeartbeatChange={(v) => { setTuicHeartbeat(v); setIsDirty(true); }}
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
                      onChange={(e) => { setRawUri(e.target.value); setIsDirty(true); }}
                      placeholder="vless://... / ss://... / hysteria2://... / tuic://..."
                      rows={6}
                      disabled={isSourceOwned || (keyId ? !revealedRaw : false)}
                      className="w-full resize-y rounded-sm border border-border bg-surface-2 p-3 font-mono text-xs text-zinc-200"
                    />

                    {rawUri && (
                      <div className="flex justify-end">
                        <Button type="button" variant="ghost" onClick={copyRaw}>
                          <Copy className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
                          Копировать сырую ссылку
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
              <Button type="submit" loading={saving} disabled={isSourceOwned}>
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
