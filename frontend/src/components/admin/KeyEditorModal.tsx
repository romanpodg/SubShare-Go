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
  parseEditableConfiguration,
  parseXrayJSONConfiguration,
  patchEditableConfiguration,
  type XrayJSONPatch,
} from "@/lib/configuration";
import type {
  ExternalProfileProtocol,
  KeyCategory,
  KeyEditorSchemaResponse,
  KeyProfileDetailResponse,
  KeyRawSecretResponse,
  KeyStructuredSecretsResponse,
} from "@/lib/types";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { KeyEditorConflictDialog } from "./KeyEditorConflictDialog";
import { informationalTemplatePreviewParts, keyTemplateVariables } from "./keyTemplateVariables";
import {
  isLegacyEditorProtocol,
  PROTOCOL_EDITOR_CAPABILITIES,
} from "./protocol-editor-capabilities";
import { LegacyXrayFields } from "./protocol-editors/LegacyXrayFields";
import {
  Hysteria2Fields,
} from "./protocol-editors/Hysteria2Fields";
import {
  ShadowsocksFields,
} from "./protocol-editors/ShadowsocksFields";
import { TuicV4ReadOnlyBanner } from "./protocol-editors/TuicV4ReadOnlyBanner";
import {
  TuicV5Fields,
} from "./protocol-editors/TuicV5Fields";

import { buildEditorSaveCommand } from "./key-editor-save-command";
import { isXrayJSONRaw } from "./key-editor-raw-policy";

import { emptyLegacyDraft } from "./key-editor-command-state";
import { buildEditorDetailProjection } from "./key-editor-detail-projection";
import { applyEditorLoadedDetail } from "./apply-editor-loaded-detail";
import { useEditorRequestSession } from "./use-editor-request-session";

import { useEditorDraft } from "./use-editor-draft";

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

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revealing, setRevealing] = useState(false);
  const [cloning, setCloning] = useState(false);
  const [detail, setDetail] = useState<KeyProfileDetailResponse | null>(null);
  const [schemaResponse, setSchemaResponse] = useState<KeyEditorSchemaResponse | null>(null);

  // Form State
  const { draft, setters, resetDraft, commandState, acceptNativeReveal, acceptRawReveal, acceptRevision, rememberLegacyEdit } = useEditorDraft({ initialLabel, initialCategory, initialKind, initialProtocol, keyId });
  const {
    label, status, kind, category, templateText, mode, protocol,
    rawUri, authoritativeRaw, revealedRaw, rawEdited,
    rawValidationError, rawFormattingWarning, legacyDraft, legacySecretsRevealed,
    server, port, displayName, ssMethod, ssPassword, ssPluginName, ssPluginOptions,
    hy2Auth, hy2Sni, hy2Insecure, hy2CertSha, hy2ObfsType, hy2ObfsPassword,
    tuicUuid, tuicPassword, tuicSni, tuicAlpn, tuicSkipCert, tuicCc, tuicUdpRelay,
    tuicUdpOverStream, tuicZeroRtt, tuicHeartbeat, isDirty,
  } = draft;
  const {
    setLabel, setStatus, setKind, setCategory, setTemplateText, setMode, setProtocol,
    setRawUri, setAuthoritativeRaw, setRawEdited, setStructuredEdited,
    setRawValidationError, setRawFormattingWarning, setLegacyDraft, setLegacySecretsRevealed,
    setServer, setPort, setDisplayName, setSsMethod, setSsPassword, setSsPluginName, setSsPluginOptions,
    setHy2Auth, setHy2Sni, setHy2Insecure, setHy2CertSha, setHy2ObfsType, setHy2ObfsPassword,
    setTuicUuid, setTuicPassword, setTuicSni, setTuicAlpn, setTuicSkipCert, setTuicCc, setTuicUdpRelay,
    setTuicUdpOverStream, setTuicZeroRtt, setTuicHeartbeat, setIsDirty,
  } = setters;

  const templateTextRef = useRef<HTMLTextAreaElement | null>(null);

  // Modal dialog states
  const [availableCategories, setAvailableCategories] = useState<KeyCategory[]>([]);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showConflictDialog, setShowConflictDialog] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  const beginSessionRequest = useEditorRequestSession(
    { open, keyId, profileRevision: detail?.profile_revision },
    { setLoading, setRevealing, setShowConflictDialog, setSaving, setCloning }
  );

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
  }, [templateText, setTemplateText, setIsDirty]);

  const handleCloseAttempt = () => {
    if (isDirty) {
      setShowDiscardConfirm(true);
    } else {
      onClose();
    }
  };

  // Clear state on unmount/close
  const clearSensitiveState = useCallback(() => {
    setShowDiscardConfirm(false);
    setDetail(null);
    resetDraft({ initialLabel, initialCategory, initialKind, initialProtocol, keyId });
  }, [initialCategory, initialKind, initialLabel, initialProtocol, keyId, resetDraft]);

  const applyDetailProjection = useCallback((projection: NonNullable<ReturnType<typeof buildEditorDetailProjection>>) => {
    setServer(projection.server);
    setPort(projection.port);
    setDisplayName(projection.displayName);
    if (projection.legacy) {
      setLegacyDraft(projection.legacy);
      setLegacySecretsRevealed(false);
    }
    if (projection.shadowsocks) {
      setSsMethod(projection.shadowsocks.method);
      setSsPluginName(projection.shadowsocks.pluginName);
    }
    if (projection.hysteria2) {
      setHy2Sni(projection.hysteria2.sni);
      setHy2Insecure(projection.hysteria2.insecure);
      setHy2CertSha(projection.hysteria2.certificate);
      setHy2ObfsType(projection.hysteria2.obfuscation);
    }
    if (projection.tuic) {
      setTuicSni(projection.tuic.sni);
      setTuicAlpn(projection.tuic.alpn);
      setTuicSkipCert(projection.tuic.skipCert);
      setTuicCc(projection.tuic.cc);
      setTuicUdpRelay(projection.tuic.relay);
      setTuicUdpOverStream(projection.tuic.udpOverStream);
      setTuicZeroRtt(projection.tuic.zeroRtt);
      setTuicHeartbeat(projection.tuic.heartbeat);
    }
  }, [setServer, setPort, setDisplayName, setLegacyDraft, setLegacySecretsRevealed,
    setSsMethod, setSsPluginName, setHy2Sni, setHy2Insecure, setHy2CertSha, setHy2ObfsType,
    setTuicSni, setTuicAlpn, setTuicSkipCert, setTuicCc, setTuicUdpRelay,
    setTuicUdpOverStream, setTuicZeroRtt, setTuicHeartbeat]);

  // Load detail & schemas
  const loadDetailAndSchema = useCallback(async () => {
    const inSession = beginSessionRequest();
    setLoading(true);
    try {
      const [schemaRes, catRes] = await Promise.all([
        keysApi.editorSchema(),
        keysApi.listCategories(),
      ]);
      await inSession(async () => {
        setSchemaResponse(schemaRes.data);
        setAvailableCategories(catRes.categories || []);
        if (keyId) {
          const detailRes = await keysApi.get(keyId);
          inSession(() => {
            acceptRevision(detailRes.data.profile_revision);
            applyEditorLoadedDetail(detailRes.data, {
            detail: setDetail, label: setLabel, status: setStatus, kind: setKind,
            category: setCategory, templateText: setTemplateText, protocol: setProtocol,
            mode: setMode, projection: applyDetailProjection,
            });
          });
        }
      });
    } catch (error) {
      inSession(() => toast(error instanceof Error ? error.message : "Ошибка загрузки", "error"));
    } finally {
      inSession(() => setLoading(false));
    }
  }, [keyId, toast, applyDetailProjection, beginSessionRequest,
    setLabel, setStatus, setKind, setCategory, setTemplateText, setProtocol, setMode, acceptRevision]);

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
    if (revealing) return;
    const inSession = beginSessionRequest("profile");
    setRevealing(true);
    try {
      const res = await keysApi.reveal(keyId, {
        profile_revision: detail.profile_revision,
        target,
      });
      inSession(() => {
        if (target === "raw") {
          const rawRes = res.data as KeyRawSecretResponse;
          acceptRawReveal(detail.protocol, rawRes.raw_uri, detail.profile_revision);
          toast("Сырая ссылка раскрыта", "info");
        } else {
          const secRes = res.data as KeyStructuredSecretsResponse;
          acceptNativeReveal(secRes.secrets, detail.profile_revision);
          toast("Секретные поля раскрыты", "info");
        }
      });
    } catch (error) {
      inSession(() => {
        if (error instanceof ApiError && error.status === 409) {
          setShowConflictDialog(true);
        } else {
          toast(error instanceof Error ? error.message : "Ошибка раскрытия секретов", "error");
        }
      });
    } finally {
      inSession(() => setRevealing(false));
    }
  };

  // Clone call
  const handleClone = async () => {
    if (!keyId || !detail) return;
    const inSession = beginSessionRequest();
    setCloning(true);
    try {
      await keysApi.clone(keyId, {
        expected_profile_revision: detail.profile_revision,
      });
      inSession(() => toast("Локальный клон успешно создан", "success"));
      await onRefresh();
      inSession(onClose);
    } catch (error) {
      inSession(() => {
        if (error instanceof ApiError && error.status === 409) setShowConflictDialog(true);
        else toast(error instanceof Error ? error.message : "Ошибка клонирования", "error");
      });
    } finally {
      inSession(() => setCloning(false));
    }
  };

  const isSourceOwned = detail?.ownership === "external_source";
  const sourceDefaultClientName = detail?.label || "";
  const schemaForProtocol = useMemo(() => {
    return schemaResponse?.protocols.find((p) => p.protocol === protocol);
  }, [schemaResponse, protocol]);

  const changeLegacyDraft = (patch: XrayJSONPatch) => {
    rememberLegacyEdit(patch);
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

  // Submit Handler
  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!label.trim()) return;
    const inSession = beginSessionRequest();
    setSaving(true);
    try {
      const command = buildEditorSaveCommand(commandState, keyId, detail);
      if (command.type === "update") {
        await keysApi.updateProfile(command.keyId, command.input);
      } else {
        await keysApi.createProfile(command.input);
      }
      inSession(() => toast(command.message, "success"));
      await onRefresh();
      inSession(onClose);
    } catch (error) {
      inSession(() => {
        if (error instanceof ApiError && error.status === 409) setShowConflictDialog(true);
        else toast(error instanceof Error ? error.message : "Ошибка сохранения", "error");
      });
    } finally {
      inSession(() => setSaving(false));
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

  const renderEditor = () => {
    return (
    <>
      <Modal
        open={open}
        onClose={handleCloseAttempt}
        title={titleText}
        className="max-h-[calc(100dvh-1.5rem)] w-[96vw] max-w-6xl overflow-hidden"
        contentClassName="flex min-h-0 flex-col overflow-hidden"
      >
        {loading ? (
          <div className="flex min-h-64 items-center justify-center text-sm text-zinc-400">
            Загрузка профиля и схем...
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex min-h-0 flex-col overflow-hidden">
            {/* Source ownership is fixed metadata, independent of the scrolling editor workspace. */}
            {isSourceOwned && (
              <div className="ui-key-editor-source-banner mb-3 flex shrink-0 items-center justify-between gap-3 rounded-sm border border-sky-500/30 bg-sky-500/10 p-3 text-xs text-sky-200">
                <div className="flex items-center gap-2">
                  <Info className="h-4 w-4 shrink-0 text-sky-300" aria-hidden="true" />
                  <span>
                    Управляется источником <strong>{detail?.external_source_name}</strong>. Подключение заблокировано; статус и название в клиенте задаются локально.
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

            <div className="ui-key-editor-scroll-region min-h-0 flex-[0_1_auto] space-y-3 overflow-y-auto pr-1">

              <div
                data-testid="key-editor-workspace"
                className={`ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 rounded-sm ${kind === "real" ? "lg:grid-cols-2" : ""}`}
              >
                {/* Main Parameters */}
                <section className="rounded-sm border border-border bg-surface-2/25 p-3.5">
                  <div className="system-label mb-3">ОСНОВНЫЕ ПАРАМЕТРЫ</div>
                  <div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2">
                    <div className={`min-w-0 ${kind === "real" ? "" : "md:col-span-2"}`}>
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
                      <div className="min-w-0">
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
                    <div className="min-w-0">
                      <Select
                        label="Статус"
                        value={status}
                        onChange={(e) => { setStatus(e.target.value as "active" | "non-active"); setIsDirty(true); }}
                        options={[
                          { value: "active", label: "Активен" },
                          { value: "non-active", label: "Неактивен" },
                        ]}
                      />
                    </div>
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
                    <div className="min-w-0 md:col-span-2">
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
                  <section className="space-y-3 rounded-sm border border-border bg-surface-1 p-3.5">
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
                  <section className="space-y-3 rounded-sm border border-border bg-surface-1 p-3.5">
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
                      rows={12}
                      disabled={isSourceOwned || (keyId ? !revealedRaw : false)}
                      spellCheck={false}
                      className="min-h-64 w-full resize-y overflow-auto whitespace-pre rounded-sm border border-border bg-surface-2 p-3 font-mono text-xs leading-5 text-zinc-200"
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

                </section>
              )}
              </div>
            </div>

            {/* Modal Actions */}
            <div className="ui-key-editor-footer mt-3 flex shrink-0 items-center justify-end border-t border-border bg-surface-1 py-4">
              <div className="ui-key-editor-footer-actions grid w-full grid-cols-2 gap-3 sm:flex sm:w-auto">
                <Button className="w-full sm:w-36" type="button" variant="outline" onClick={handleCloseAttempt} disabled={saving}>
                  Отмена
                </Button>
                <Button className={`w-full ${keyId ? "sm:w-36" : "sm:w-48"}`} type="submit" loading={saving} disabled={Boolean(isSourceOwned && !isDirty)}>
                  {keyId ? "Сохранить" : "Добавить профиль"}
                </Button>
              </div>
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
  };

  return renderEditor();
}
