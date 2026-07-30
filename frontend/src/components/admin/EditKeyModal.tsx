"use client";

import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { VLESSKey } from "@/lib/types";
import {
  KeyEditorModal,
  type KeyEditorSubmitValue,
} from "./KeyEditorModal";

interface Props {
  keyData: VLESSKey;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditKeyModal({ keyData, onClose, onRefresh }: Props) {
  const { toast } = useToast();

  const handleSubmit = async (value: KeyEditorSubmitValue) => {
    try {
      await keysApi.update(keyData.id, {
        label: value.label,
        status: value.status,
        category: value.category,
        raw_url: value.kind === "real" ? value.rawConfig : undefined,
        uuid: "",
        host: "",
        port: "",
        query: "",
        fragment: "",
        kind: value.kind,
        template_text: value.kind === "informational" ? value.templateText : undefined,
      });
      toast(
        value.kind === "informational"
          ? "Информационный ключ обновлён"
          : "Конфигурация обновлена",
        "success"
      );
      await onRefresh();
    } catch (error: unknown) {
      toast(
        error instanceof Error ? error.message : "Не удалось обновить ключ",
        "error"
      );
      throw error;
    }
  };

  return (
    <KeyEditorModal
      open
      onClose={onClose}
      title={(kind) =>
        `${kind === "informational" ? "Изменить информационный ключ" : "Изменить конфигурацию"} — ${keyData.label}`
      }
      submitLabel="Сохранить"
      initialValue={{
        label: keyData.label,
        status: keyData.status,
        category: keyData.category || "",
        kind: keyData.kind,
        templateText: keyData.template_text || "",
        rawConfig: keyData.url,
      }}
      onSubmit={handleSubmit}
    />
  );
}
