"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput, type AppleEmojiInputHandle } from "@/components/ui/AppleEmojiInput";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import {
  buildConfiguration,
  extractLabelFromConfiguration,
  getConfigurationPlaceholder,
  parseConfiguration,
  type ConfigurationDraft,
} from "@/lib/configuration";
import { keyTemplateVariables } from "./keyTemplateVariables";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  insertAtIndex?: number | null;
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

export function AddKeyModal({ open, onClose, onRefresh, insertAtIndex, onCreated }: Props) {
  const { toast } = useToast();
  const templateInputRef = useRef<AppleEmojiInputHandle>(null);
  void insertAtIndex;
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState("");
  const [labelTouched, setLabelTouched] = useState(false);
  const [status, setStatus] = useState("active");
  const [kind, setKind] = useState<"real" | "informational">("real");
  const [rawConfig, setRawConfig] = useState("");
  const [templateText, setTemplateText] = useState("");
  const [configDraft, setConfigDraft] = useState<ConfigurationDraft | null>(null);
  const [configError, setConfigError] = useState("");

  const resetForm = () => {
    setLabel("");
    setLabelTouched(false);
    setStatus("active");
    setKind("real");
    setRawConfig("");
    setTemplateText("");
    setConfigDraft(null);
    setConfigError("");
  };

  useEffect(() => {
    if (open) {
      resetForm();
    }
  }, [open]);

  const syncConfiguration = (nextRawConfig: string, allowAutofillLabel: boolean) => {
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

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.create({
        label,
        url: kind === "real" ? rawConfig.trim() : undefined,
        status,
        kind,
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

  const showConfigurationEditor = kind === "real" && (Boolean(configDraft) || Boolean(configError) || rawConfig.trim() !== "");
  const identifierLabel = configDraft?.protocol === "trojan" ? "Пароль" : "UUID / ID";
  const paramsLabel = configDraft?.protocol === "vmess" ? "JSON-параметры" : "Параметры";

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
              <label htmlFor="configuration-raw" className="text-sm text-zinc-400">
                Конфигурация
              </label>
              <textarea
                id="configuration-raw"
                value={rawConfig}
                onChange={(e) => syncConfiguration(e.target.value, !labelTouched || label.trim() === "")}
                placeholder={getConfigurationPlaceholder()}
                className="min-h-32 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                required
              />
              <p className="text-xs text-zinc-500">
                Поддерживаются `vless://`, `vmess://` и `trojan://`. Название автоматически берется из тега после `#` или из поля `ps`.
              </p>
            </div>

            <div
              className={`overflow-hidden transition-all duration-300 ease-out ${
                showConfigurationEditor ? "max-h-[720px] opacity-100" : "max-h-0 opacity-0"
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
          </>
        ) : (
          <>
            <AppleEmojiInput
              ref={templateInputRef}
              label="Шаблон текста"
              value={templateText}
              onChange={(e) => setTemplateText(e.target.value)}
              placeholder="User: @{telegram}"
            />
            <div className="-mt-2">
              <EmojiPickerButton inline onSelect={(emoji) => templateInputRef.current?.insertEmoji(emoji)} />
            </div>
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
    </Modal>
  );
}
