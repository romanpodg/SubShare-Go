"use client";

import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput, type AppleEmojiInputHandle } from "@/components/ui/AppleEmojiInput";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory } from "@/lib/types";
import {
  buildConfiguration,
  buildXrayJSONConfiguration,
  createConfigurationFromXrayJSON,
  createXrayJSONFromConfiguration,
  extractLabelFromConfiguration,
  getConfigurationPlaceholder,
  getXrayJSONPlaceholder,
  isXrayJSONConfiguration,
  parseConfiguration,
  parseXrayJSONConfiguration,
  type ConfigurationDraft,
  type RealConfigurationMode,
  type XrayJSONDraft,
} from "@/lib/configuration";
import { keyTemplateVariables } from "./keyTemplateVariables";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  insertAtIndex?: number | null;
  initialCategory?: string;
  onCreated?: () => void;
}

const emptyDraft: ConfigurationDraft = {
  protocol: "vless",
  server: "",
  port: "443",
  identifier: "",
  params: "",
  remark: "",
};

const emptyXrayDraft: XrayJSONDraft = {
  protocol: "vless",
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

export function AddKeyModal({ open, onClose, onRefresh, insertAtIndex, initialCategory, onCreated }: Props) {
  const { toast } = useToast();
  const templateInputRef = useRef<AppleEmojiInputHandle>(null);
  void insertAtIndex;
  const [loading, setLoading] = useState(false);
  const [availableCategories, setAvailableCategories] = useState<KeyCategory[]>([]);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [label, setLabel] = useState("");
  const [labelTouched, setLabelTouched] = useState(false);
  const [status, setStatus] = useState("active");
  const [category, setCategory] = useState("");
  const [kind, setKind] = useState<"real" | "informational">("real");
  const [configMode, setConfigMode] = useState<RealConfigurationMode>("link");
  const [rawConfig, setRawConfig] = useState("");
  const [templateText, setTemplateText] = useState("");
  const [configDraft, setConfigDraft] = useState<ConfigurationDraft | null>(null);
  const [configError, setConfigError] = useState("");
  const [xrayDraft, setXrayDraft] = useState<XrayJSONDraft | null>(null);
  const [xrayConfigDocument, setXrayConfigDocument] = useState<Record<string, unknown> | null>(null);
  const [xrayOutboundIndex, setXrayOutboundIndex] = useState<number | null>(null);
  const [xrayError, setXrayError] = useState("");

  const resetForm = useCallback(() => {
    setLabel("");
    setLabelTouched(false);
    setStatus("active");
    setCategory(initialCategory?.trim() ? initialCategory.trim() : "");
    setKind("real");
    setConfigMode("link");
    setRawConfig("");
    setTemplateText("");
    setConfigDraft(null);
    setConfigError("");
    setXrayDraft(null);
    setXrayConfigDocument(null);
    setXrayOutboundIndex(null);
    setXrayError("");
  }, [initialCategory]);

  useEffect(() => {
    if (open) {
      resetForm();
      void keysApi
        .listCategories()
        .then((response) => setAvailableCategories(response.categories || []))
        .catch(() => undefined);
    }
  }, [open, initialCategory, resetForm]);

  const categoryOptions = [...availableCategories];
  if (category && !categoryOptions.some((item) => item.name === category)) {
    categoryOptions.push({ name: category, color: "#d8b33d", keys_count: 0 });
  }

  const canAutofillLabel = !labelTouched || label.trim() === "";

  const syncLinkConfiguration = (nextRawConfig: string, allowAutofillLabel: boolean) => {
    setRawConfig(nextRawConfig);

    const trimmed = nextRawConfig.trim();
    if (!trimmed) {
      setConfigDraft(null);
      setConfigError("");
      return;
    }

    try {
      const parsed = parseConfiguration(trimmed);
      setConfigDraft(parsed);
      setConfigError("");

      const detectedLabel = extractLabelFromConfiguration(trimmed);
      if (allowAutofillLabel && detectedLabel) {
        setLabel(detectedLabel);
      }
    } catch (error: unknown) {
      setConfigDraft(null);
      setConfigError(error instanceof Error ? error.message : "Не удалось разобрать конфигурацию");
    }
  };

  const syncXrayConfiguration = (nextRawConfig: string, allowAutofillLabel: boolean) => {
    setRawConfig(nextRawConfig);

    const trimmed = nextRawConfig.trim();
    if (!trimmed) {
      setXrayDraft(null);
      setXrayConfigDocument(null);
      setXrayOutboundIndex(null);
      setXrayError("");
      return;
    }

    try {
      const parsed = parseXrayJSONConfiguration(trimmed);
      setXrayDraft(parsed?.draft ?? null);
      setXrayConfigDocument(parsed?.config ?? null);
      setXrayOutboundIndex(parsed?.outboundIndex ?? null);
      setXrayError("");

      if (allowAutofillLabel && parsed?.draft.remark.trim()) {
        setLabel(parsed.draft.remark.trim());
      }
    } catch (error: unknown) {
      setXrayDraft(null);
      setXrayConfigDocument(null);
      setXrayOutboundIndex(null);
      setXrayError(error instanceof Error ? error.message : "Не удалось разобрать XRAY-JSON");
    }
  };

  const syncConfiguration = (nextRawConfig: string, allowAutofillLabel: boolean) => {
    if (configMode === "xray-json") {
      syncXrayConfiguration(nextRawConfig, allowAutofillLabel);
      return;
    }
    syncLinkConfiguration(nextRawConfig, allowAutofillLabel);
  };

  const switchConfigurationMode = (nextMode: RealConfigurationMode) => {
    if (nextMode === configMode) {
      return;
    }

    let nextRaw = rawConfig;
    if (nextMode === "xray-json") {
      if (!isXrayJSONConfiguration(nextRaw)) {
        try {
          nextRaw = createXrayJSONFromConfiguration(nextRaw);
        } catch {
          nextRaw = getXrayJSONPlaceholder();
        }
      }
      setConfigMode(nextMode);
      syncXrayConfiguration(nextRaw, canAutofillLabel);
      return;
    }

    if (isXrayJSONConfiguration(nextRaw)) {
      try {
        nextRaw = createConfigurationFromXrayJSON(nextRaw);
      } catch {
        nextRaw = "";
      }
    }
    setConfigMode(nextMode);
    syncLinkConfiguration(nextRaw, canAutofillLabel);
  };

  const updateConfigurationDraft = (patch: Partial<ConfigurationDraft>) => {
    const nextDraft = { ...(configDraft ?? emptyDraft), ...patch };
    setConfigDraft(nextDraft);

    try {
      const rebuilt = buildConfiguration(nextDraft);
      setRawConfig(rebuilt);
      setConfigError("");
      if (!labelTouched && nextDraft.remark.trim()) {
        setLabel(nextDraft.remark.trim());
      }
    } catch (error: unknown) {
      setConfigError(error instanceof Error ? error.message : "Не удалось собрать конфигурацию");
    }
  };

  const updateXrayDraft = (patch: Partial<XrayJSONDraft>) => {
    const nextDraft = { ...(xrayDraft ?? emptyXrayDraft), ...patch };
    setXrayDraft(nextDraft);

    try {
      const rebuilt = buildXrayJSONConfiguration(nextDraft, xrayConfigDocument ?? undefined, xrayOutboundIndex ?? undefined);
      setRawConfig(rebuilt);
      setXrayError("");

      const reparsed = parseXrayJSONConfiguration(rebuilt);
      if (reparsed) {
        setXrayDraft(reparsed.draft);
        setXrayConfigDocument(reparsed.config);
        setXrayOutboundIndex(reparsed.outboundIndex);
      }

      if (!labelTouched && nextDraft.remark.trim()) {
        setLabel(nextDraft.remark.trim());
      }
    } catch (error: unknown) {
      setXrayError(error instanceof Error ? error.message : "Не удалось собрать XRAY-JSON");
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.create({
        label,
        url: kind === "real" ? rawConfig.trim() : undefined,
        status,
        kind,
        category,
        template_text: kind === "informational" ? templateText : undefined,
      });
      toast("Конфигурация добавлена", "success");
      resetForm();
      onClose();
      await onRefresh();
      if (onCreated) {
        onCreated();
      }
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось добавить конфигурацию", "error");
    } finally {
      setLoading(false);
    }
  };

  const showLinkEditor = kind === "real" && configMode === "link" && (Boolean(configDraft) || Boolean(configError) || rawConfig.trim() !== "");
  const showXrayEditor = kind === "real" && configMode === "xray-json" && (Boolean(xrayDraft) || Boolean(xrayError) || rawConfig.trim() !== "");
  const identifierLabel = configDraft?.protocol === "trojan" ? "Пароль" : "UUID / ID";
  const paramsLabel = configDraft?.protocol === "vmess" ? "JSON-параметры" : "Параметры";
  const xrayIdentifierLabel = xrayDraft?.protocol === "trojan" ? "Пароль" : "UUID / ID";
  const xrayPathLabel = xrayDraft?.network === "grpc" ? "Service Name" : "Путь";
  const rawPlaceholder = configMode === "xray-json" ? getXrayJSONPlaceholder() : getConfigurationPlaceholder();

  return (
    <Modal open={open} onClose={onClose} title="Добавить конфигурацию">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input
          label="Название"
          value={label}
          onChange={(e) => {
            setLabelTouched(true);
            setLabel(e.target.value);
          }}
          required
        />
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          options={[
            { value: "active", label: "Активен" },
            { value: "non-active", label: "Неактивен" },
          ]}
        />
        <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <Select
            label="Категория"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            options={
              [{ value: "", label: "Без категории" }].concat(
                categoryOptions.map((item) => ({ value: item.name, label: item.name }))
              )
            }
          />
          <Button type="button" variant="ghost" onClick={() => setShowCreateCategory(true)}>
            + Добавить категорию
          </Button>
        </div>
        <Select
          label="Тип"
          value={kind}
          onChange={(e) => setKind(e.target.value as "real" | "informational")}
          options={[
            { value: "real", label: "Конфигурация" },
            { value: "informational", label: "Информационный ключ" },
          ]}
        />
        {kind === "real" ? (
          <>
            <div className="flex flex-col gap-1.5">
              <span className="text-sm text-zinc-400">Формат конфигурации</span>
              <div className="grid grid-cols-2 gap-1 rounded-xl border border-border bg-surface-2 p-1">
                <button
                  type="button"
                  onClick={() => switchConfigurationMode("link")}
                  className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                    configMode === "link"
                      ? "bg-accent text-accent-fg shadow-sm"
                      : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                  }`}
                >
                  Ключ-ссылка
                </button>
                <button
                  type="button"
                  onClick={() => switchConfigurationMode("xray-json")}
                  className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                    configMode === "xray-json"
                      ? "bg-accent text-accent-fg shadow-sm"
                      : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                  }`}
                >
                  XRAY-JSON
                </button>
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <label htmlFor="configuration-raw" className="text-sm text-zinc-400">
                {configMode === "xray-json" ? "XRAY-JSON конфигурация" : "Конфигурация"}
              </label>
              <textarea
                id="configuration-raw"
                value={rawConfig}
                onChange={(e) => syncConfiguration(e.target.value, canAutofillLabel)}
                placeholder={rawPlaceholder}
                className="min-h-36 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                required
              />
              {configMode === "xray-json" ? (
                <p className="text-xs text-zinc-500">
                  Укажите полноценный XRAY JSON с секцией `outbounds`. Быстрое редактирование ниже изменяет этот JSON автоматически.
                </p>
              ) : (
                <p className="text-xs text-zinc-500">
                  Поддерживаются `vless://`, `vmess://` и `trojan://`. Название автоматически берется из тега после `#` или из поля `ps`.
                </p>
              )}
            </div>

            {configMode === "link" ? (
              <div
                className={`overflow-hidden transition-all duration-300 ease-out ${
                  showLinkEditor ? "max-h-[720px] opacity-100" : "max-h-0 opacity-0"
                }`}
              >
                <div className="rounded-xl border border-border bg-surface-2/60 p-4">
                  <div className="mb-3 text-sm font-medium text-zinc-200">Состав конфигурации</div>

                  {configError && (
                    <div className="mb-3 rounded-lg border border-red-500/25 bg-red-500/10 px-3 py-2 text-sm text-red-200">
                      {configError}
                    </div>
                  )}

                  {configDraft && (
                    <div className="grid grid-cols-1 gap-3">
                      <Input label="Протокол" value={configDraft.protocol.toUpperCase()} readOnly />
                      <Input
                        label="Сервер"
                        value={configDraft.server}
                        onChange={(e) => updateConfigurationDraft({ server: e.target.value })}
                      />
                      <Input
                        label="Порт"
                        value={configDraft.port}
                        onChange={(e) => updateConfigurationDraft({ port: e.target.value })}
                      />
                      <Input
                        label={identifierLabel}
                        value={configDraft.identifier}
                        onChange={(e) => updateConfigurationDraft({ identifier: e.target.value })}
                      />
                      <Input
                        label="Название в клиенте"
                        value={configDraft.remark}
                        onChange={(e) => updateConfigurationDraft({ remark: e.target.value })}
                      />
                      <div className="flex flex-col gap-1.5">
                        <label htmlFor="configuration-params" className="text-sm text-zinc-400">
                          {paramsLabel}
                        </label>
                        <textarea
                          id="configuration-params"
                          value={configDraft.params}
                          onChange={(e) => updateConfigurationDraft({ params: e.target.value })}
                          rows={configDraft.protocol === "vmess" ? 7 : 4}
                          className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                        />
                      </div>
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div
                className={`overflow-hidden transition-all duration-300 ease-out ${
                  showXrayEditor ? "max-h-[900px] opacity-100" : "max-h-0 opacity-0"
                }`}
              >
                <div className="rounded-xl border border-border bg-surface-2/60 p-4">
                  <div className="mb-3 text-sm font-medium text-zinc-200">Быстрое редактирование XRAY-JSON</div>

                  {xrayError && (
                    <div className="mb-3 rounded-lg border border-red-500/25 bg-red-500/10 px-3 py-2 text-sm text-red-200">
                      {xrayError}
                    </div>
                  )}

                  {xrayDraft && (
                    <div className="grid grid-cols-1 gap-3">
                      <Select
                        label="Протокол"
                        value={xrayDraft.protocol}
                        onChange={(e) => updateXrayDraft({ protocol: e.target.value as XrayJSONDraft["protocol"] })}
                        options={[
                          { value: "vless", label: "VLESS" },
                          { value: "vmess", label: "VMESS" },
                          { value: "trojan", label: "TROJAN" },
                        ]}
                      />
                      <Input
                        label="Сервер"
                        value={xrayDraft.server}
                        onChange={(e) => updateXrayDraft({ server: e.target.value })}
                      />
                      <Input
                        label="Порт"
                        value={xrayDraft.port}
                        onChange={(e) => updateXrayDraft({ port: e.target.value })}
                      />
                      <Input
                        label={xrayIdentifierLabel}
                        value={xrayDraft.identifier}
                        onChange={(e) => updateXrayDraft({ identifier: e.target.value })}
                      />
                      <Input
                        label="Название (tag)"
                        value={xrayDraft.remark}
                        onChange={(e) => updateXrayDraft({ remark: e.target.value })}
                      />
                      <Select
                        label="Транспорт (network)"
                        value={xrayDraft.network}
                        onChange={(e) => updateXrayDraft({ network: e.target.value as XrayJSONDraft["network"] })}
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
                      <Select
                        label="Шифрование канала (security)"
                        value={xrayDraft.security}
                        onChange={(e) => updateXrayDraft({ security: e.target.value as XrayJSONDraft["security"] })}
                        options={[
                          { value: "none", label: "none" },
                          { value: "tls", label: "tls" },
                          { value: "reality", label: "reality" },
                        ]}
                      />
                      <Input
                        label={xrayPathLabel}
                        value={xrayDraft.path}
                        onChange={(e) => updateXrayDraft({ path: e.target.value })}
                        placeholder={xrayDraft.network === "grpc" ? "grpc-service" : "/"}
                      />
                      <Input
                        label="Host"
                        value={xrayDraft.host}
                        onChange={(e) => updateXrayDraft({ host: e.target.value })}
                        placeholder="example.com"
                      />
                      <Input
                        label="SNI / ServerName"
                        value={xrayDraft.sni}
                        onChange={(e) => updateXrayDraft({ sni: e.target.value })}
                        placeholder="example.com"
                      />
                      <Input
                        label="ALPN (через запятую)"
                        value={xrayDraft.alpn}
                        onChange={(e) => updateXrayDraft({ alpn: e.target.value })}
                        placeholder="h2,http/1.1"
                      />
                    </div>
                  )}
                </div>
              </div>
            )}
          </>
        ) : (
          <>
            <AppleEmojiInput
              ref={templateInputRef}
              label="Шаблон текста"
              value={templateText}
              onChange={(e) => setTemplateText(e.target.value)}
              placeholder="User: @{telegram}"
              rightSlot={<EmojiPickerButton iconOnly onSelect={(emoji) => templateInputRef.current?.insertEmoji(emoji)} />}
            />
            <div className="rounded-lg bg-surface-2 p-3 text-xs text-zinc-400">
              <div className="mb-1 font-medium text-zinc-300">Переменные:</div>
              <div className="grid grid-cols-1 gap-1">
                {keyTemplateVariables.map((item) => (
                  <div key={item.token}>
                    {item.token} — {item.description}
                  </div>
                ))}
              </div>
            </div>
          </>
        )}
        <Button type="submit" loading={loading}>
          Добавить
        </Button>
      </form>

      <CreateKeyCategoryModal
        open={showCreateCategory}
        onClose={() => setShowCreateCategory(false)}
        onCreated={(nextCategory) => {
          setAvailableCategories((previous) => {
            const next = previous.filter((item) => item.name !== nextCategory.name);
            return [...next, nextCategory].sort((left, right) => left.name.localeCompare(right.name, "ru"));
          });
          setCategory(nextCategory.name);
        }}
      />
    </Modal>
  );
}
