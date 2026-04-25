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
  parseConfiguration,
  type ConfigurationDraft,
} from "@/lib/configuration";
import type { VLESSKey } from "@/lib/types";
import { keyTemplateVariables } from "./keyTemplateVariables";

interface Props {
  keyData: VLESSKey;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

const emptyDraft: ConfigurationDraft = {
  protocol: "vless",
  server: "",
  port: "443",
  identifier: "",
  params: "",
  remark: "",
};

export function EditKeyModal({ keyData, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const templateInputRef = useRef<AppleEmojiInputHandle>(null);
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState(keyData.label);
  const [status, setStatus] = useState<"active" | "non-active">(keyData.status);
  const [kind, setKind] = useState<"real" | "informational">(keyData.kind);
  const [templateText, setTemplateText] = useState(keyData.template_text ?? "");
  const [rawConfig, setRawConfig] = useState(keyData.url);
  const [configDraft, setConfigDraft] = useState<ConfigurationDraft | null>(null);
  const [configError, setConfigError] = useState("");

  useEffect(() => {
    if (keyData.kind !== "real") {
      setConfigDraft(null);
      setConfigError("");
      return;
    }

    try {
      setConfigDraft(parseConfiguration(keyData.url));
      setConfigError("");
    } catch (error: unknown) {
      setConfigDraft(null);
      setConfigError(error instanceof Error ? error.message : "Не удалось разобрать конфигурацию");
    }
  }, [keyData.kind, keyData.url]);

  const syncConfiguration = (nextRawConfig: string) => {
    setRawConfig(nextRawConfig);

    const trimmed = nextRawConfig.trim();
    if (!trimmed) {
      setConfigDraft(null);
      setConfigError("");
      return;
    }

    try {
      setConfigDraft(parseConfiguration(trimmed));
      setConfigError("");
    } catch (error: unknown) {
      setConfigDraft(null);
      setConfigError(error instanceof Error ? error.message : "Не удалось разобрать конфигурацию");
    }
  };

  const updateConfigurationDraft = (patch: Partial<ConfigurationDraft>) => {
    const nextDraft = { ...(configDraft ?? emptyDraft), ...patch };
    setConfigDraft(nextDraft);

    try {
      setRawConfig(buildConfiguration(nextDraft));
      setConfigError("");
    } catch (error: unknown) {
      setConfigError(error instanceof Error ? error.message : "Не удалось собрать конфигурацию");
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.update(keyData.id, {
        label,
        status,
        raw_url: kind === "real" ? rawConfig.trim() : undefined,
        uuid: "",
        host: "",
        port: "",
        query: "",
        fragment: "",
        kind,
        template_text: kind === "informational" ? templateText : undefined,
      });
      toast("Конфигурация обновлена", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить конфигурацию", "error");
    } finally {
      setLoading(false);
    }
  };

  const showConfigurationEditor = kind === "real" && (Boolean(configDraft) || Boolean(configError) || rawConfig.trim() !== "");
  const identifierLabel = configDraft?.protocol === "trojan" ? "Пароль" : "UUID / ID";
  const paramsLabel = configDraft?.protocol === "vmess" ? "JSON-параметры" : "Параметры";

  return (
    <Modal open onClose={onClose} title={`Изменить конфигурацию — ${keyData.label}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input
          label="Название"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          required
        />
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value as "active" | "non-active")}
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
              <label htmlFor="edit-configuration-raw" className="text-sm text-zinc-400">
                Конфигурация
              </label>
              <textarea
                id="edit-configuration-raw"
                value={rawConfig}
                onChange={(e) => syncConfiguration(e.target.value)}
                className="min-h-32 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                required
              />
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
                      <label htmlFor="edit-configuration-params" className="text-sm text-zinc-400">
                        {paramsLabel}
                      </label>
                      <textarea
                        id="edit-configuration-params"
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
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
