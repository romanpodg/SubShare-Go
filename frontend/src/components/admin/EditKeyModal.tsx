"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput } from "@/components/ui/AppleEmojiInput";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { VLESSKey } from "@/lib/types";
import { keyTemplateVariables } from "./keyTemplateVariables";

interface Props {
  keyData: VLESSKey;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditKeyModal({ keyData, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState(keyData.label);
  const [status, setStatus] = useState<"active" | "non-active">(keyData.status);
  const [kind, setKind] = useState<"real" | "informational">(keyData.kind);
  const [templateText, setTemplateText] = useState(keyData.template_text ?? "");
  const [uuid, setUuid] = useState(keyData.edit_uuid);
  const [host, setHost] = useState(keyData.edit_host);
  const [port, setPort] = useState(keyData.edit_port);
  const [query, setQuery] = useState(keyData.edit_query);
  const [fragment, setFragment] = useState(keyData.edit_fragment);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.update(keyData.id, {
        label,
        status,
        uuid,
        host,
        port,
        query,
        fragment,
        kind,
        template_text: kind === "informational" ? templateText : undefined,
      });
      toast("Ключ обновлен", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить ключ", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Редактировать — ${keyData.label}`}>
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
            { value: "real", label: "Настоящий ключ" },
            { value: "informational", label: "Информационный ключ" },
          ]}
        />
        {kind === "real" ? (
          <>
            <Input label="UUID" value={uuid} onChange={(e) => setUuid(e.target.value)} />
            <Input label="Host" value={host} onChange={(e) => setHost(e.target.value)} />
            <Input label="Port" value={port} onChange={(e) => setPort(e.target.value)} />
            <Input label="Query" value={query} onChange={(e) => setQuery(e.target.value)} />
            <Input label="Fragment" value={fragment} onChange={(e) => setFragment(e.target.value)} />
          </>
        ) : (
          <>
            <AppleEmojiInput
              label="Шаблон текста"
              value={templateText}
              onChange={(e) => setTemplateText(e.target.value)}
              placeholder="User: @{telegram}"
            />
            <div className="-mt-2">
              <EmojiPickerButton inline onSelect={(emoji) => setTemplateText((prev) => `${prev}${emoji}`)} />
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
