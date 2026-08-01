"use client";

import {
  ClipboardEvent as ReactClipboardEvent,
  FormEvent,
  ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  AlertTriangle,
  ArrowRightLeft,
  Braces,
  CheckCircle2,
  Copy,
  FolderPlus,
} from "lucide-react";
import { AppleEmojiInput, type AppleEmojiInputHandle } from "@/components/ui/AppleEmojiInput";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { EmojiText } from "@/components/ui/EmojiText";
import { Input } from "@/components/ui/Input";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import {
  buildConfiguration,
  createConfigurationFromXrayJSON,
  createXrayJSONFromConfiguration,
  extractLabelFromConfiguration,
  getConfigurationPlaceholder,
  getXrayJSONPlaceholder,
  isXrayJSONConfiguration,
  patchXrayJSONConfiguration,
  parseConfiguration,
  parseXrayJSONConfiguration,
  type ConfigurationDraft,
  type RealConfigurationMode,
  type XrayJSONDraft,
} from "@/lib/configuration";
import {
  CONFIG_LIMIT,
  configurationByteLength,
  DuplicateJSONKeyError,
  formatXrayJSON,
  formatXrayJSONWithinLimit,
  inspectXrayJSONDocument,
} from "@/lib/xray-json-document";
import type { KeyCategory } from "@/lib/types";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { keyTemplateVariables } from "./keyTemplateVariables";

const LABEL_LIMIT = 255;
const TEMPLATE_LIMIT = 8_192;
const DUPLICATE_JSON_WARNING =
  "JSON содержит повторяющиеся ключи. Форматирование, преобразование и структурированное редактирование отключены; копирование и сохранение используют исходный текст.";
const FORMAT_LIMIT_WARNING =
  "Форматированная версия превышает лимит 65 535 байт. Исходный компактный JSON сохранён без изменений.";

const TEMPLATE_PREVIEW_VALUES: Record<string, string> = {
  "{user_name}": "Иван",
  "{telegram}": "ivanov",
  "{subscription_id}": "0123456789abcdef",
  "{expires_date}": "2026-12-31",
  "{expires_at}": "2026-12-31 23:59:59",
  "{real_keys_count}": "3",
};

export type KeyEditorKind = "real" | "informational";
export type KeyEditorStatus = "active" | "non-active";

export interface KeyEditorInitialValue {
  label: string;
  status: KeyEditorStatus;
  category: string;
  kind: KeyEditorKind;
  templateText: string;
  rawConfig: string;
}

export interface KeyEditorSubmitValue {
  label: string;
  status: KeyEditorStatus;
  category: string;
  kind: KeyEditorKind;
  templateText: string;
  rawConfig: string;
}

interface KeyEditorModalProps {
  open: boolean;
  initialValue: KeyEditorInitialValue;
  title: string | ((kind: KeyEditorKind) => string);
  submitLabel: string;
  onClose: () => void;
  onSubmit: (value: KeyEditorSubmitValue) => Promise<void>;
  autofillLabelFromConfig?: boolean;
}

interface ParsedValue<T> {
  value: T | null;
  error: string;
}

type PendingConversion = {
  target: RealConfigurationMode;
} | null;

const fieldLabelClass =
  "font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500";
const textareaClass =
  "w-full min-w-0 resize-y rounded-sm border border-border bg-surface-2 px-3 py-2 font-mono text-xs leading-5 text-zinc-200 placeholder:text-zinc-600 transition-colors hover:border-[var(--border-strong)] focus:border-accent focus-visible:border-accent";
const toolButtonClass =
  "inline-flex h-9 items-center justify-center gap-2 rounded-sm border border-border bg-transparent px-3 font-mono text-[10px] font-semibold uppercase tracking-[0.08em] text-zinc-400 transition-colors duration-200 hover:border-[var(--border-strong)] hover:bg-surface-2 hover:text-zinc-100 disabled:cursor-not-allowed disabled:opacity-40";

function parseLink(raw: string): ParsedValue<ConfigurationDraft> {
  if (!raw.trim()) return { value: null, error: "" };
  try {
    return { value: parseConfiguration(raw.trim()), error: "" };
  } catch (error: unknown) {
    return {
      value: null,
      error: error instanceof Error ? error.message : "Не удалось разобрать конфигурацию",
    };
  }
}

function parseXray(raw: string): ParsedValue<ReturnType<typeof parseXrayJSONConfiguration>> {
  if (!raw.trim()) return { value: null, error: "" };
  try {
    return { value: parseXrayJSONConfiguration(raw.trim()), error: "" };
  } catch (error: unknown) {
    return {
      value: null,
      error: error instanceof Error ? error.message : "Не удалось разобрать XRAY-JSON",
    };
  }
}

function getRawXrayPort(parsed: NonNullable<ReturnType<typeof parseXrayJSONConfiguration>>) {
  const outbounds = Array.isArray(parsed.config.outbounds) ? parsed.config.outbounds : [];
  const outbound = outbounds[parsed.outboundIndex];
  if (!outbound || typeof outbound !== "object" || Array.isArray(outbound)) return "";
  const settings = (outbound as Record<string, unknown>).settings;
  if (!settings || typeof settings !== "object" || Array.isArray(settings)) return "";
  const settingsRecord = settings as Record<string, unknown>;
  const nodes =
    parsed.draft.protocol === "trojan" ? settingsRecord.servers : settingsRecord.vnext;
  if (!Array.isArray(nodes) || !nodes[0] || typeof nodes[0] !== "object" || Array.isArray(nodes[0])) {
    return "";
  }
  const port = (nodes[0] as Record<string, unknown>).port;
  return typeof port === "number" || typeof port === "string" ? String(port) : "";
}

export function isValidKeyPort(port: string) {
  if (!/^\d+$/.test(port.trim())) return false;
  const numeric = Number(port);
  return Number.isInteger(numeric) && numeric >= 1 && numeric <= 65_535;
}

export function renderKeyTemplatePreview(template: string, fallbackLabel: string) {
  let result = template.trim() || fallbackLabel.trim();
  Object.entries(TEMPLATE_PREVIEW_VALUES).forEach(([token, value]) => {
    result = result.replaceAll(token, value.trim());
  });
  return result.trim();
}

export function findUnknownKeyTemplateTokens(template: string) {
  const supported = new Set(keyTemplateVariables.map((item) => item.token));
  return Array.from(new Set(template.match(/\{[a-z_][a-z0-9_]*\}/gi) ?? [])).filter(
    (token) => !supported.has(token)
  );
}

export { formatXrayJSON };

function stateSnapshot(value: {
  label: string;
  status: KeyEditorStatus;
  category: string;
  kind: KeyEditorKind;
  templateText: string;
  configMode: RealConfigurationMode;
  linkRaw: string;
  xrayRaw: string;
}) {
  return JSON.stringify(value);
}

function normalizeInitialXrayJSON(raw: string) {
  if (!raw.trim()) return raw;
  try {
    return formatXrayJSONWithinLimit(raw).value;
  } catch {
    return raw;
  }
}

function initialEditorState(initialValue: KeyEditorInitialValue) {
  const configMode: RealConfigurationMode =
    initialValue.kind === "real" && isXrayJSONConfiguration(initialValue.rawConfig)
      ? "xray-json"
      : "link";
  return {
    label: initialValue.label,
    status: initialValue.status,
    category: initialValue.category || "",
    kind: initialValue.kind,
    templateText: initialValue.templateText || "",
    configMode,
    linkRaw: configMode === "link" ? initialValue.rawConfig : "",
    xrayRaw:
      configMode === "xray-json" ? normalizeInitialXrayJSON(initialValue.rawConfig) : "",
  };
}

function EditorSection({
  title,
  children,
  open = true,
}: {
  title: string;
  children: ReactNode;
  open?: boolean;
}) {
  const [expanded, setExpanded] = useState(open);

  return (
    <details
      open={expanded}
      onToggle={(event) => setExpanded(event.currentTarget.open)}
      className="group rounded-sm border border-border bg-surface-2/45"
    >
      <summary className="cursor-pointer select-none px-4 py-3 font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-zinc-300 marker:text-zinc-600">
        {title}
      </summary>
      <div className="grid min-w-0 grid-cols-1 gap-3 border-t border-border p-4 sm:grid-cols-2">
        {children}
      </div>
    </details>
  );
}

function ValidationState({
  raw,
  error,
  valid,
  warning,
  description,
}: {
  raw: string;
  error: string;
  valid: boolean;
  warning?: string;
  description: string;
}) {
  if (!raw.trim()) {
    return (
      <div className="flex items-start gap-2 rounded-sm border border-border bg-surface-2/50 px-3 py-2 text-xs text-zinc-500">
        <Braces className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        Вставьте конфигурацию, чтобы открыть структурированный редактор.
      </div>
    );
  }
  if (!valid) {
    return (
      <div
        role="alert"
        className="flex items-start gap-2 rounded-sm border border-danger/35 bg-danger/8 px-3 py-2 text-xs text-red-200"
      >
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span className="min-w-0 break-words">{error || "Конфигурация заполнена некорректно"}</span>
      </div>
    );
  }
  if (warning) {
    return (
      <div
        role="status"
        className="flex items-start gap-2 rounded-sm border border-amber-500/30 bg-amber-500/8 px-3 py-2 text-xs text-amber-200"
      >
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
        <span className="min-w-0 break-words">{warning}</span>
      </div>
    );
  }
  return (
    <div className="flex items-start gap-2 rounded-sm border border-success/25 bg-success/8 px-3 py-2 text-xs text-success">
      <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span>{description}</span>
    </div>
  );
}

export function KeyEditorModal({
  open,
  initialValue,
  title,
  submitLabel,
  onClose,
  onSubmit,
  autofillLabelFromConfig = false,
}: KeyEditorModalProps) {
  const { toast } = useToast();
  const templateInputRef = useRef<AppleEmojiInputHandle>(null);
  const firstState = initialEditorState(initialValue);
  const baselineRef = useRef(stateSnapshot(firstState));
  const bypassCloseGuardRef = useRef(false);

  const [label, setLabel] = useState(firstState.label);
  const [labelTouched, setLabelTouched] = useState(!autofillLabelFromConfig);
  const [status, setStatus] = useState<KeyEditorStatus>(firstState.status);
  const [category, setCategory] = useState(firstState.category);
  const [kind, setKind] = useState<KeyEditorKind>(firstState.kind);
  const [templateText, setTemplateText] = useState(firstState.templateText);
  const [configMode, setConfigMode] = useState<RealConfigurationMode>(firstState.configMode);
  const [linkRaw, setLinkRaw] = useState(firstState.linkRaw);
  const [xrayRaw, setXrayRaw] = useState(firstState.xrayRaw);
  const [availableCategories, setAvailableCategories] = useState<KeyCategory[]>([]);
  const [loading, setLoading] = useState(false);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);
  const [pendingConversion, setPendingConversion] = useState<PendingConversion>(null);

  const currentSnapshot = stateSnapshot({
    label,
    status,
    category,
    kind,
    templateText,
    configMode,
    linkRaw,
    xrayRaw,
  });
  const dirty = currentSnapshot !== baselineRef.current;

  const resetEditor = useCallback(() => {
    const nextState = initialEditorState({
      label: initialValue.label,
      status: initialValue.status,
      category: initialValue.category,
      kind: initialValue.kind,
      templateText: initialValue.templateText,
      rawConfig: initialValue.rawConfig,
    });
    setLabel(nextState.label);
    setLabelTouched(!autofillLabelFromConfig);
    setStatus(nextState.status);
    setCategory(nextState.category);
    setKind(nextState.kind);
    setTemplateText(nextState.templateText);
    setConfigMode(nextState.configMode);
    setLinkRaw(nextState.linkRaw);
    setXrayRaw(nextState.xrayRaw);
    setShowCreateCategory(false);
    setShowDiscardConfirm(false);
    setPendingConversion(null);
    bypassCloseGuardRef.current = false;
    baselineRef.current = stateSnapshot(nextState);
  }, [
    autofillLabelFromConfig,
    initialValue.category,
    initialValue.kind,
    initialValue.label,
    initialValue.rawConfig,
    initialValue.status,
    initialValue.templateText,
  ]);

  useEffect(() => {
    if (!open) return;
    resetEditor();
    void keysApi
      .listCategories()
      .then((response) => setAvailableCategories(response.categories || []))
      .catch(() => undefined);
  }, [open, resetEditor]);

  const linkParsed = useMemo(() => parseLink(linkRaw), [linkRaw]);
  const xrayParsed = useMemo(() => parseXray(xrayRaw), [xrayRaw]);
  const xrayDocument = useMemo(() => {
    if (!xrayRaw.trim()) {
      return {
        duplicateKeys: [],
        formatted: "",
        formattedExceedsLimit: false,
      };
    }
    try {
      const { duplicateKeys } = inspectXrayJSONDocument(xrayRaw);
      if (duplicateKeys.length > 0) {
        return {
          duplicateKeys,
          formatted: "",
          formattedExceedsLimit: false,
        };
      }
      const formatted = formatXrayJSON(xrayRaw);
      return {
        duplicateKeys,
        formatted,
        formattedExceedsLimit: configurationByteLength(formatted) > CONFIG_LIMIT,
      };
    } catch {
      return {
        duplicateKeys: [],
        formatted: "",
        formattedExceedsLimit: false,
      };
    }
  }, [xrayRaw]);
  const activeRaw = configMode === "link" ? linkRaw : xrayRaw;
  const activeByteLength = configurationByteLength(activeRaw);
  const activeError = configMode === "link" ? linkParsed.error : xrayParsed.error;
  const activeValid =
    configMode === "link" ? Boolean(linkParsed.value) : Boolean(xrayParsed.value);
  const xrayHasDuplicateKeys = xrayDocument.duplicateKeys.length > 0;
  const xrayTransformable = Boolean(xrayParsed.value) && !xrayHasDuplicateKeys;
  const activeWarning =
    configMode === "xray-json"
      ? xrayHasDuplicateKeys
        ? DUPLICATE_JSON_WARNING
        : xrayDocument.formattedExceedsLimit
          ? FORMAT_LIMIT_WARNING
          : ""
      : "";
  const activePort =
    configMode === "link"
      ? linkParsed.value?.port
      : xrayParsed.value
        ? getRawXrayPort(xrayParsed.value) || xrayParsed.value.draft.port
        : undefined;
  const activePortValid = activePort ? isValidKeyPort(activePort) : false;
  const unknownTokens = useMemo(() => findUnknownKeyTemplateTokens(templateText), [templateText]);
  const templatePreview = useMemo(
    () => renderKeyTemplatePreview(templateText, label),
    [label, templateText]
  );
  const categoryOptions = useMemo(() => {
    const next = [...availableCategories];
    if (category && !next.some((item) => item.name === category)) {
      next.push({ name: category, color: "#d8b33d", keys_count: 0 });
    }
    return next.sort((left, right) => left.name.localeCompare(right.name, "ru"));
  }, [availableCategories, category]);

  const resolvedTitle = typeof title === "function" ? title(kind) : title;
  const nestedDialogOpen =
    showCreateCategory || showDiscardConfirm || pendingConversion !== null;
  const labelValid = label.trim().length > 0 && label.length <= LABEL_LIMIT;
  const canSubmit =
    !loading &&
    labelValid &&
      (kind === "informational" ||
      (activeRaw.trim().length > 0 &&
        activeByteLength <= CONFIG_LIMIT &&
        activeValid &&
        activePortValid));

  const maybeAutofillLabel = (raw: string, mode: RealConfigurationMode) => {
    if (!autofillLabelFromConfig || labelTouched || !raw.trim()) return;
    try {
      const detected =
        mode === "link"
          ? extractLabelFromConfiguration(raw)
          : parseXrayJSONConfiguration(raw)?.draft.remark.trim() || "";
      if (detected) setLabel(detected.slice(0, LABEL_LIMIT));
    } catch {
      // Parsing feedback is rendered next to the editor.
    }
  };

  const changeRaw = (value: string) => {
    if (configMode === "link") setLinkRaw(value);
    else setXrayRaw(value);
    maybeAutofillLabel(value, configMode);
  };

  const updateLinkDraft = (patch: Partial<ConfigurationDraft>) => {
    if (!linkParsed.value) return;
    const nextDraft = { ...linkParsed.value, ...patch };
    try {
      const rebuilt = buildConfiguration(nextDraft);
      setLinkRaw(rebuilt);
      maybeAutofillLabel(rebuilt, "link");
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось собрать конфигурацию", "error");
    }
  };

  const updateXrayDraft = (patch: Partial<XrayJSONDraft>) => {
    if (!xrayTransformable) return;
    try {
      const rebuilt = patchXrayJSONConfiguration(xrayRaw, patch);
      if (configurationByteLength(rebuilt) > CONFIG_LIMIT) {
        toast(FORMAT_LIMIT_WARNING, "info");
        return;
      }
      setXrayRaw(rebuilt);
      maybeAutofillLabel(rebuilt, "xray-json");
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось собрать XRAY-JSON", "error");
    }
  };

  const executeConversion = (target: RealConfigurationMode) => {
    try {
      if (target === "xray-json") {
        const converted = createXrayJSONFromConfiguration(linkRaw);
        setXrayRaw(converted);
        setConfigMode("xray-json");
        maybeAutofillLabel(converted, "xray-json");
        toast("Создан XRAY-JSON черновик", "success");
      } else {
        const converted = createConfigurationFromXrayJSON(xrayRaw);
        setLinkRaw(converted);
        setConfigMode("link");
        maybeAutofillLabel(converted, "link");
        toast("Извлечён первый поддерживаемый outbound", "success");
      }
      setPendingConversion(null);
    } catch (error: unknown) {
      setPendingConversion(null);
      toast(error instanceof Error ? error.message : "Не удалось преобразовать конфигурацию", "error");
    }
  };

  const requestConversion = (target: RealConfigurationMode) => {
    if (configMode === "xray-json" && xrayHasDuplicateKeys) {
      toast(DUPLICATE_JSON_WARNING, "info");
      return;
    }
    const sourceValid = target === "xray-json" ? Boolean(linkParsed.value) : Boolean(xrayParsed.value);
    if (!sourceValid) {
      toast("Сначала исправьте исходную конфигурацию", "error");
      return;
    }
    const targetRaw = target === "xray-json" ? xrayRaw : linkRaw;
    if (targetRaw.trim()) {
      setPendingConversion({ target });
      return;
    }
    executeConversion(target);
  };

  const copyActiveConfiguration = async () => {
    const valueToCopy =
      configMode === "xray-json" && xrayDocument.formatted && !xrayHasDuplicateKeys
        ? xrayDocument.formatted
        : activeRaw;
    const copied = await copyToClipboard(valueToCopy);
    if (!copied) {
      toast("Не удалось скопировать конфигурацию", "error");
    } else if (configMode === "xray-json" && xrayHasDuplicateKeys) {
      toast("JSON с повторяющимися ключами скопирован без форматирования", "info");
    } else {
      toast("Конфигурация скопирована", "success");
    }
  };

  const formatActiveJSON = () => {
    try {
      const result = formatXrayJSONWithinLimit(xrayRaw);
      if (result.exceededLimit) {
        toast(FORMAT_LIMIT_WARNING, "info");
        return;
      }
      setXrayRaw(result.value);
      toast("JSON отформатирован", "success");
    } catch (error: unknown) {
      toast(
        error instanceof DuplicateJSONKeyError
          ? DUPLICATE_JSON_WARNING
          : "Сначала исправьте синтаксис JSON",
        error instanceof DuplicateJSONKeyError ? "info" : "error"
      );
    }
  };

  const pasteXrayJSON = (event: ReactClipboardEvent<HTMLTextAreaElement>) => {
    if (configMode !== "xray-json") return;
    const pasted = event.clipboardData.getData("text/plain");
    if (!pasted) return;

    event.preventDefault();
    const textarea = event.currentTarget;
    const selectionStart = textarea.selectionStart ?? xrayRaw.length;
    const selectionEnd = textarea.selectionEnd ?? selectionStart;
    const pastedRaw = `${xrayRaw.slice(0, selectionStart)}${pasted}${xrayRaw.slice(selectionEnd)}`;
    let nextValue = pastedRaw;
    try {
      const result = formatXrayJSONWithinLimit(pastedRaw);
      if (result.exceededLimit) {
        toast(FORMAT_LIMIT_WARNING, "info");
      } else {
        nextValue = result.value;
      }
    } catch {
      // Invalid, partial, and duplicate-key JSON must remain exactly as pasted.
    }
    setXrayRaw(nextValue);
    maybeAutofillLabel(nextValue, "xray-json");
  };

  const requestClose = () => {
    if (bypassCloseGuardRef.current || !dirty) {
      onClose();
      return;
    }
    setShowDiscardConfirm(true);
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    setLoading(true);
    try {
      await onSubmit({
        label: label.trim(),
        status,
        category,
        kind,
        templateText,
        rawConfig:
          configMode === "xray-json" && !xrayHasDuplicateKeys
            ? formatXrayJSONWithinLimit(xrayRaw).value
            : activeRaw.trim(),
      });
      bypassCloseGuardRef.current = true;
      onClose();
    } catch {
      // The wrapper reports the API error and the editor remains open.
    } finally {
      setLoading(false);
    }
  };

  const linkDraft = linkParsed.value;
  const xrayDraft = xrayParsed.value?.draft ?? null;

  return (
    <>
      <Modal
        open={open}
        onClose={requestClose}
        title={resolvedTitle}
        closeOnEscape={!nestedDialogOpen}
        closeOnBackdrop={!nestedDialogOpen}
        className="flex h-[92dvh] max-h-[56rem] w-[96vw] max-w-6xl flex-col overflow-hidden"
        contentClassName="flex min-h-0 flex-1 flex-col overflow-hidden"
      >
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <div className="ui-key-editor-scroll-region min-h-0 flex-1 overflow-y-auto overflow-x-hidden pr-1">
            <section className="mb-4 rounded-sm border border-border bg-surface-2/25 p-4">
              <div className="mb-3 flex items-center justify-between gap-3">
                <div className="system-label">ОСНОВНЫЕ ПАРАМЕТРЫ</div>
                <span className="font-mono text-[10px] text-zinc-600">
                  {dirty ? "ЕСТЬ ИЗМЕНЕНИЯ" : "БЕЗ ИЗМЕНЕНИЙ"}
                </span>
              </div>
              <div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
                <div className="min-w-0 md:col-span-2">
                  <Input
                    label="Название"
                    value={label}
                    maxLength={LABEL_LIMIT}
                    onChange={(event) => {
                      setLabelTouched(true);
                      setLabel(event.target.value);
                    }}
                    error={!label.trim() && labelTouched ? "Укажите название" : undefined}
                    required
                  />
                  <div className="mt-1 text-right font-mono text-[10px] text-zinc-600">
                    {label.length}/{LABEL_LIMIT}
                  </div>
                </div>
                <Select
                  label="Статус"
                  value={status}
                  onChange={(event) => setStatus(event.target.value as KeyEditorStatus)}
                  options={[
                    { value: "active", label: "Активен" },
                    { value: "non-active", label: "Неактивен" },
                  ]}
                />
                <Select
                  label="Тип"
                  value={kind}
                  onChange={(event) => setKind(event.target.value as KeyEditorKind)}
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
                      onChange={(event) => setCategory(event.target.value)}
                      options={[{ value: "", label: "Без категории" }].concat(
                        categoryOptions.map((item) => ({ value: item.name, label: item.name }))
                      )}
                    />
                    <button
                      type="button"
                      className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-zinc-400 transition-colors hover:border-[var(--border-strong)] hover:bg-surface-2 hover:text-zinc-100"
                      onClick={() => setShowCreateCategory(true)}
                      aria-label="Добавить категорию"
                      title="Добавить категорию"
                    >
                      <FolderPlus className="h-4 w-4" aria-hidden="true" />
                    </button>
                  </div>
                </div>
              </div>
            </section>

            {kind === "real" ? (
              <div className="ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 lg:grid-cols-2">
                <section className="min-w-0 rounded-sm border border-border bg-surface-1 p-4">
                  <div className="mb-3">
                    <div className="system-label mb-2">ИСХОДНАЯ КОНФИГУРАЦИЯ</div>
                    <div className="grid grid-cols-2 gap-1 rounded-sm border border-border bg-surface-2 p-1">
                      {([
                        ["link", "Ключ-ссылка"],
                        ["xray-json", "XRAY-JSON"],
                      ] as const).map(([mode, modeLabel]) => (
                        <button
                          key={mode}
                          type="button"
                          onClick={() => setConfigMode(mode)}
                          className={`min-w-0 rounded-sm px-3 py-2 text-sm font-medium transition-colors duration-200 motion-reduce:transition-none ${
                            configMode === mode
                              ? "bg-accent text-accent-fg"
                              : "text-zinc-400 hover:bg-surface-1 hover:text-zinc-100"
                          }`}
                        >
                          {modeLabel}
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <label htmlFor="key-editor-raw" className={fieldLabelClass}>
                      {configMode === "xray-json" ? "XRAY-JSON" : "Конфигурация"}
                    </label>
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        className={toolButtonClass}
                        onClick={copyActiveConfiguration}
                        disabled={!activeRaw}
                      >
                        <Copy className="h-3.5 w-3.5" aria-hidden="true" />
                        Копировать
                      </button>
                      {configMode === "xray-json" && (
                        <button
                          type="button"
                          className={toolButtonClass}
                          onClick={formatActiveJSON}
                          disabled={!xrayRaw || xrayHasDuplicateKeys}
                          title={xrayHasDuplicateKeys ? DUPLICATE_JSON_WARNING : undefined}
                        >
                          <Braces className="h-3.5 w-3.5" aria-hidden="true" />
                          Форматировать
                        </button>
                      )}
                    </div>
                  </div>

                  <textarea
                    id="key-editor-raw"
                    value={activeRaw}
                    onChange={(event) => changeRaw(event.target.value)}
                    onPaste={pasteXrayJSON}
                    placeholder={
                      configMode === "xray-json"
                        ? getXrayJSONPlaceholder()
                        : getConfigurationPlaceholder()
                    }
                    maxLength={CONFIG_LIMIT}
                    spellCheck={false}
                    className={`${textareaClass} min-h-72`}
                    required
                  />
                  <div className="mt-1 flex items-center justify-between gap-3 font-mono text-[10px] text-zinc-600">
                    <span>{configMode === "xray-json" ? "JSON OBJECT" : "VLESS / VMESS / TROJAN"}</span>
                    <span>{activeByteLength}/{CONFIG_LIMIT} B</span>
                  </div>

                  <div className="mt-3">
                    <ValidationState
                      raw={activeRaw}
                      error={activeError || (!activePortValid && activeValid ? "Порт должен быть от 1 до 65535" : "")}
                      valid={activeValid && activePortValid}
                      warning={activeWarning}
                      description={
                        configMode === "link" && linkDraft
                          ? `${linkDraft.protocol.toUpperCase()} · ${linkDraft.server}:${linkDraft.port}`
                          : xrayDraft
                            ? `${xrayDraft.protocol.toUpperCase()} · ${xrayDraft.network.toUpperCase()} · ${xrayDraft.security.toUpperCase()}`
                            : "Конфигурация распознана"
                      }
                    />
                  </div>

                  <div className="mt-4 rounded-sm border border-border bg-surface-2/35 p-3">
                    <div className={`${fieldLabelClass} mb-2`}>Преобразование формата</div>
                    <p className="mb-3 text-xs leading-5 text-zinc-500">
                      Черновики хранятся раздельно. Преобразование не удаляет исходную версию.
                    </p>
                    <button
                      type="button"
                      className={toolButtonClass}
                      onClick={() =>
                        requestConversion(configMode === "link" ? "xray-json" : "link")
                      }
                      disabled={configMode === "xray-json" && xrayHasDuplicateKeys}
                    >
                      <ArrowRightLeft className="h-3.5 w-3.5" aria-hidden="true" />
                      {configMode === "link" ? "Создать XRAY-JSON" : "Извлечь ключ-ссылку"}
                    </button>
                    {configMode === "xray-json" && (
                      <p className="mt-2 text-[11px] leading-4 text-amber-300/75">
                        В ссылку переносится только первый поддерживаемый outbound; остальные секции JSON остаются в исходном черновике.
                      </p>
                    )}
                  </div>
                </section>

                <section className="min-w-0 space-y-3 rounded-sm border border-border bg-surface-1 p-4">
                  <div className="system-label">СТРУКТУРИРОВАННЫЙ РЕДАКТОР</div>
                  {!activeValid ? (
                    <div className="rounded-sm border border-dashed border-border p-5 text-center text-sm leading-6 text-zinc-500">
                      Структурированные поля появятся после успешного разбора конфигурации.
                    </div>
                  ) : configMode === "link" && linkDraft ? (
                    <>
                      <EditorSection title="Подключение">
                        <Input label="Протокол" value={linkDraft.protocol.toUpperCase()} readOnly />
                        <Input
                          label="Сервер"
                          value={linkDraft.server}
                          onChange={(event) => updateLinkDraft({ server: event.target.value })}
                        />
                        <Input
                          label="Порт"
                          value={linkDraft.port}
                          inputMode="numeric"
                          onChange={(event) => updateLinkDraft({ port: event.target.value })}
                          error={!isValidKeyPort(linkDraft.port) ? "Допустимо 1–65535" : undefined}
                        />
                        <Input
                          label={linkDraft.protocol === "trojan" ? "Пароль" : "UUID / ID"}
                          value={linkDraft.identifier}
                          onChange={(event) => updateLinkDraft({ identifier: event.target.value })}
                        />
                        <div className="sm:col-span-2">
                          <Input
                            label="Название в клиенте"
                            value={linkDraft.remark}
                            onChange={(event) => updateLinkDraft({ remark: event.target.value })}
                          />
                        </div>
                      </EditorSection>
                      <EditorSection title="Дополнительные параметры">
                        <div className="min-w-0 sm:col-span-2">
                          <label htmlFor="key-editor-params" className={fieldLabelClass}>
                            {linkDraft.protocol === "vmess" ? "JSON-параметры" : "Query-параметры"}
                          </label>
                          <textarea
                            id="key-editor-params"
                            value={linkDraft.params}
                            onChange={(event) => updateLinkDraft({ params: event.target.value })}
                            rows={linkDraft.protocol === "vmess" ? 8 : 5}
                            className={`${textareaClass} mt-2`}
                          />
                        </div>
                      </EditorSection>
                    </>
                  ) : xrayHasDuplicateKeys ? (
                    <div className="rounded-sm border border-amber-500/30 bg-amber-500/8 p-5 text-sm leading-6 text-amber-200">
                      {DUPLICATE_JSON_WARNING}
                    </div>
                  ) : xrayDraft ? (
                    <>
                      <EditorSection title="Подключение">
                        <Select
                          label="Протокол"
                          value={xrayDraft.protocol}
                          onChange={(event) =>
                            updateXrayDraft({ protocol: event.target.value as XrayJSONDraft["protocol"] })
                          }
                          options={[
                            { value: "vless", label: "VLESS" },
                            { value: "vmess", label: "VMESS" },
                            { value: "trojan", label: "TROJAN" },
                          ]}
                        />
                        <Input
                          label="Сервер"
                          value={xrayDraft.server}
                          onChange={(event) => updateXrayDraft({ server: event.target.value })}
                        />
                        <Input
                          label="Порт"
                          value={xrayDraft.port}
                          inputMode="numeric"
                          onChange={(event) => updateXrayDraft({ port: event.target.value })}
                          error={!isValidKeyPort(xrayDraft.port) ? "Допустимо 1–65535" : undefined}
                        />
                        <Input
                          label={xrayDraft.protocol === "trojan" ? "Пароль" : "UUID / ID"}
                          value={xrayDraft.identifier}
                          onChange={(event) => updateXrayDraft({ identifier: event.target.value })}
                        />
                        <div className="sm:col-span-2">
                          <Input
                            label="Название (tag)"
                            value={xrayDraft.remark}
                            onChange={(event) => updateXrayDraft({ remark: event.target.value })}
                          />
                        </div>
                      </EditorSection>

                      <EditorSection title="Транспорт">
                        <Select
                          label="Network"
                          value={xrayDraft.network}
                          onChange={(event) =>
                            updateXrayDraft({ network: event.target.value as XrayJSONDraft["network"] })
                          }
                          options={[
                            { value: "tcp", label: "TCP" },
                            { value: "ws", label: "WebSocket" },
                            { value: "grpc", label: "gRPC" },
                            { value: "httpupgrade", label: "HTTPUpgrade" },
                            { value: "xhttp", label: "XHTTP" },
                            { value: "h2", label: "HTTP/2" },
                            { value: "quic", label: "QUIC" },
                          ]}
                        />
                        <Input
                          label={xrayDraft.network === "grpc" ? "Service Name" : "Путь"}
                          value={xrayDraft.path}
                          onChange={(event) => updateXrayDraft({ path: event.target.value })}
                        />
                        {xrayDraft.network !== "quic" && xrayDraft.network !== "h2" && (
                          <Input
                            label="Host"
                            value={xrayDraft.host}
                            onChange={(event) => updateXrayDraft({ host: event.target.value })}
                          />
                        )}
                        {xrayDraft.network === "grpc" && (
                          <Input
                            label="gRPC authority"
                            value={xrayDraft.grpcAuthority || ""}
                            onChange={(event) => updateXrayDraft({ grpcAuthority: event.target.value })}
                          />
                        )}
                        {xrayDraft.network === "tcp" && (
                          <Select
                            label="TCP header"
                            value={xrayDraft.headerType || ""}
                            onChange={(event) => updateXrayDraft({ headerType: event.target.value })}
                            options={[
                              { value: "", label: "Не задан" },
                              { value: "none", label: "none" },
                              { value: "http", label: "http" },
                            ]}
                          />
                        )}
                      </EditorSection>

                      <EditorSection title="Безопасность">
                        <Select
                          label="Security"
                          value={xrayDraft.security}
                          onChange={(event) =>
                            updateXrayDraft({ security: event.target.value as XrayJSONDraft["security"] })
                          }
                          options={[
                            { value: "none", label: "none" },
                            { value: "tls", label: "tls" },
                            { value: "reality", label: "reality" },
                          ]}
                        />
                        {xrayDraft.security !== "none" && (
                          <Input
                            label="SNI / ServerName"
                            value={xrayDraft.sni}
                            onChange={(event) => updateXrayDraft({ sni: event.target.value })}
                          />
                        )}
                        {xrayDraft.security === "tls" && (
                          <>
                            <Input
                              label="ALPN"
                              value={xrayDraft.alpn}
                              placeholder="h2,http/1.1"
                              onChange={(event) => updateXrayDraft({ alpn: event.target.value })}
                            />
                            <Input
                              label="Fingerprint"
                              value={xrayDraft.fingerprint || ""}
                              onChange={(event) => updateXrayDraft({ fingerprint: event.target.value })}
                            />
                            <label className="flex min-h-11 items-center gap-3 rounded-sm border border-border bg-surface-2 px-3 text-sm text-zinc-300">
                              <input
                                type="checkbox"
                                checked={Boolean(xrayDraft.allowInsecure)}
                                onChange={(event) => updateXrayDraft({ allowInsecure: event.target.checked })}
                                className="h-4 w-4 accent-[var(--accent)]"
                              />
                              Allow insecure
                            </label>
                          </>
                        )}
                        {xrayDraft.security === "reality" && (
                          <>
                            <Input
                              label="Fingerprint"
                              value={xrayDraft.fingerprint || ""}
                              onChange={(event) => updateXrayDraft({ fingerprint: event.target.value })}
                            />
                            <Input
                              label="Public key"
                              value={xrayDraft.publicKey || ""}
                              onChange={(event) => updateXrayDraft({ publicKey: event.target.value })}
                            />
                            <Input
                              label="Short ID"
                              value={xrayDraft.shortId || ""}
                              onChange={(event) => updateXrayDraft({ shortId: event.target.value })}
                            />
                            <Input
                              label="Spider X"
                              value={xrayDraft.spiderX || ""}
                              onChange={(event) => updateXrayDraft({ spiderX: event.target.value })}
                            />
                          </>
                        )}
                      </EditorSection>

                      <EditorSection title="Дополнительные параметры" open={false}>
                        {xrayDraft.protocol === "vless" && (
                          <>
                            <Input
                              label="Flow"
                              value={xrayDraft.flow || ""}
                              onChange={(event) => updateXrayDraft({ flow: event.target.value })}
                            />
                            <Input
                              label="Encryption"
                              value={xrayDraft.encryption || ""}
                              onChange={(event) => updateXrayDraft({ encryption: event.target.value })}
                            />
                          </>
                        )}
                        {xrayDraft.protocol === "vmess" && (
                          <>
                            <Input
                              label="VMess security"
                              value={xrayDraft.vmessSecurity || ""}
                              onChange={(event) => updateXrayDraft({ vmessSecurity: event.target.value })}
                            />
                            <Input
                              label="Alter ID"
                              value={xrayDraft.vmessAlterId || ""}
                              inputMode="numeric"
                              onChange={(event) => updateXrayDraft({ vmessAlterId: event.target.value })}
                            />
                          </>
                        )}
                        {xrayDraft.protocol === "trojan" && (
                          <p className="text-xs text-zinc-500 sm:col-span-2">
                            Для TROJAN дополнительные протокольные параметры не требуются.
                          </p>
                        )}
                      </EditorSection>
                    </>
                  ) : null}
                </section>
              </div>
            ) : (
              <div className="ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 lg:grid-cols-2">
                <section className="min-w-0 rounded-sm border border-border bg-surface-1 p-4">
                  <div className="system-label mb-3">ШАБЛОН СООБЩЕНИЯ</div>
                  <AppleEmojiInput
                    ref={templateInputRef}
                    label="Текст"
                    value={templateText}
                    onChange={(event) => setTemplateText(event.target.value)}
                    placeholder="Привет, {user_name}! 👋"
                    multiline
                    maxLength={TEMPLATE_LIMIT}
                    showCounter
                    rightSlot={
                      <EmojiPickerButton
                        iconOnly
                        onSelect={(emoji) => templateInputRef.current?.insertEmoji(emoji)}
                      />
                    }
                  />
                  {!templateText.trim() && (
                    <div className="mt-3 rounded-sm border border-amber-500/25 bg-amber-500/8 px-3 py-2 text-xs leading-5 text-amber-200">
                      Шаблон пуст: сервер использует название ключа.
                    </div>
                  )}
                  {unknownTokens.length > 0 && (
                    <div role="status" className="mt-3 rounded-sm border border-amber-500/25 bg-amber-500/8 px-3 py-2 text-xs leading-5 text-amber-200">
                      Неизвестные переменные не будут заменены: {unknownTokens.join(", ")}
                    </div>
                  )}
                </section>

                <section className="min-w-0 space-y-4 rounded-sm border border-border bg-surface-1 p-4">
                  <div>
                    <div className="system-label mb-3">ПЕРЕМЕННЫЕ</div>
                    <div className="flex flex-wrap gap-2">
                      {keyTemplateVariables.map((item) => (
                        <button
                          key={item.token}
                          type="button"
                          className="rounded-sm border border-border bg-surface-2 px-2.5 py-2 text-left font-mono text-[11px] text-zinc-300 transition-colors hover:border-accent/60 hover:text-accent"
                          onClick={() => templateInputRef.current?.insertText(item.token)}
                          title={item.description}
                        >
                          {item.token}
                        </button>
                      ))}
                    </div>
                    <div className="mt-3 space-y-1 text-xs text-zinc-500">
                      {keyTemplateVariables.map((item) => (
                        <div key={item.token} className="grid grid-cols-[minmax(0,auto)_1fr] gap-2">
                          <span className="font-mono text-zinc-400">{item.token}</span>
                          <span>— {item.description}</span>
                        </div>
                      ))}
                    </div>
                  </div>

                  <div>
                    <div className="system-label mb-3">ПРЕДПРОСМОТР</div>
                    <div className="min-h-32 whitespace-pre-wrap break-words rounded-sm border border-border bg-surface-2 p-4 text-sm leading-6 text-zinc-200">
                      {templatePreview ? (
                        <EmojiText text={templatePreview} />
                      ) : (
                        <span className="text-zinc-600">Укажите название или текст шаблона</span>
                      )}
                    </div>
                    <p className="mt-2 text-[11px] leading-4 text-zinc-600">
                      Используются демонстрационные данные; при выдаче подписки сервер подставит значения пользователя.
                    </p>
                  </div>
                </section>
              </div>
            )}
          </div>

          <div className="-mx-5 -mb-5 mt-4 flex shrink-0 flex-col-reverse gap-2 border-t border-border bg-surface-1 px-5 pb-5 pt-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="font-mono text-[10px] uppercase tracking-[0.08em] text-zinc-600">
              {dirty ? "Несохранённые изменения" : "Изменений нет"}
            </div>
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              <Button type="button" variant="ghost" onClick={requestClose} disabled={loading}>
                Отмена
              </Button>
              <Button type="submit" loading={loading} disabled={!canSubmit}>
                {submitLabel}
              </Button>
            </div>
          </div>
        </form>

        <CreateKeyCategoryModal
          open={showCreateCategory}
          onClose={() => setShowCreateCategory(false)}
          onCreated={(nextCategory) => {
            setAvailableCategories((previous) => {
              const next = previous.filter((item) => item.name !== nextCategory.name);
              return [...next, nextCategory];
            });
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
          bypassCloseGuardRef.current = true;
          setShowDiscardConfirm(false);
          onClose();
        }}
      />

      <ConfirmDialog
        open={pendingConversion !== null}
        title="Перезаписать черновик?"
        message={
          pendingConversion?.target === "link"
            ? "Существующий черновик ссылки будет заменён. Исходный XRAY-JSON останется без изменений."
            : "Существующий черновик XRAY-JSON будет заменён. Исходная ссылка останется без изменений."
        }
        confirmLabel="Преобразовать"
        onCancel={() => setPendingConversion(null)}
        onConfirm={() => {
          if (pendingConversion) executeConversion(pendingConversion.target);
        }}
      />
    </>
  );
}
