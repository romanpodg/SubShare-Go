"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { externalSources } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (name: string) => void;
  initialValue?: string;
}

export function CreateExternalCategoryModal({ open, onClose, onCreated, initialValue = "" }: Props) {
  const { toast } = useToast();
  const [name, setName] = useState(initialValue);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setName(initialValue);
    }
  }, [open, initialValue]);

  const handleSave = async () => {
    const value = name.trim();
    if (!value) {
      toast("Введите название категории", "error");
      return;
    }
    setSaving(true);
    try {
      const response = await externalSources.createCategory(value);
      const categoryName = response.category?.name || value.slice(0, 24);
      toast("Категория сохранена", "success");
      onCreated(categoryName);
      onClose();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить категорию", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={() => (!saving ? onClose() : undefined)} title="Создать категорию" className="max-w-md">
      <div className="space-y-4">
        <Input
          label="Название категории"
          value={name}
          onChange={(event) => setName(event.target.value)}
          maxLength={24}
          placeholder="Например, Партнёрские"
          disabled={saving}
        />
        <div className="flex items-center gap-2">
          <Button onClick={() => void handleSave()} loading={saving}>
            Сохранить
          </Button>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            Отмена
          </Button>
        </div>
      </div>
    </Modal>
  );
}
