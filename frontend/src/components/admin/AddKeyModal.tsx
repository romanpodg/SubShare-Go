"use client";

import { useState, useEffect, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput } from "@/components/ui/AppleEmojiInput";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import { keyTemplateVariables } from "./keyTemplateVariables";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  /** When set, the newly created key should be inserted at this row index. */
  insertAtIndex?: number | null;
  /** Called after key is successfully created so the parent can reorder. */
  onCreated?: () => void;
}

export function AddKeyModal({ open, onClose, onRefresh, insertAtIndex, onCreated }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState("");
  const [url, setUrl] = useState("");
  const [status, setStatus] = useState("active");
  const [kind, setKind] = useState<"real" | "informational">("real");
  const [templateText, setTemplateText] = useState("");

  const resetForm = () => {
    setLabel("");
    setUrl("");
    setStatus("active");
    setKind("real");
    setTemplateText("");
  };

  useEffect(() => {
    if (open) {
      resetForm();
    }
  }, [open]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.create({
        label,
        url: kind === "real" ? url : undefined,
        status,
        kind,
        template_text: kind === "informational" ? templateText : undefined,
      });
      toast("Ключ добавлен", "success");
      resetForm();
      onClose();
      await onRefresh();
      if (onCreated) onCreated();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось добавить ключ", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить ключ">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input
          label="Название"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          required
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
          <Input
            label="VLESS URL"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="vless://..."
            required
          />
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
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          options={[
            { value: "active", label: "Активен" },
            { value: "non-active", label: "Неактивен" },
          ]}
        />
        <Button type="submit" loading={loading}>
          Добавить
        </Button>
      </form>
    </Modal>
  );
}
