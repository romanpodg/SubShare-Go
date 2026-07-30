"use client";

import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import {
  KeyEditorModal,
  type KeyEditorKind,
  type KeyEditorSubmitValue,
} from "./KeyEditorModal";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  insertAtIndex?: number | null;
  initialCategory?: string;
  initialKind?: KeyEditorKind;
  onCreated?: () => void;
}

export function AddKeyModal({
  open,
  onClose,
  onRefresh,
  insertAtIndex,
  initialCategory,
  initialKind = "real",
  onCreated,
}: Props) {
  const { toast } = useToast();
  void insertAtIndex;

  const handleSubmit = async (value: KeyEditorSubmitValue) => {
    try {
      await keysApi.create({
        label: value.label,
        url: value.kind === "real" ? value.rawConfig : undefined,
        status: value.status,
        kind: value.kind,
        category: value.category,
        template_text: value.kind === "informational" ? value.templateText : undefined,
      });
      toast(
        value.kind === "informational"
          ? "Информационный ключ добавлен"
          : "Конфигурация добавлена",
        "success"
      );
      await onRefresh();
      onCreated?.();
    } catch (error: unknown) {
      toast(
        error instanceof Error ? error.message : "Не удалось добавить ключ",
        "error"
      );
      throw error;
    }
  };

  return (
    <KeyEditorModal
      open={open}
      onClose={onClose}
      title={(kind) =>
        kind === "informational"
          ? "Добавить информационный ключ"
          : "Добавить конфигурацию"
      }
      submitLabel="Добавить"
      initialValue={{
        label: "",
        status: "active",
        category: initialCategory?.trim() || "",
        kind: initialKind,
        templateText: "",
        rawConfig: "",
      }}
      autofillLabelFromConfig
      onSubmit={handleSubmit}
    />
  );
}
