"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory } from "@/lib/types";
import {
  DEFAULT_KEY_CATEGORY_COLOR,
  KEY_CATEGORY_COLOR_PRESETS,
  normalizeKeyCategoryColor,
} from "./keyCategoryColors";

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: (category: KeyCategory) => void;
  initialValue?: string;
  initialColor?: string;
  title?: string;
  submitLabel?: string;
}

export function CreateKeyCategoryModal({
  open,
  onClose,
  onCreated,
  initialValue = "",
  initialColor = DEFAULT_KEY_CATEGORY_COLOR,
  title = "Добавить категорию",
  submitLabel = "Сохранить",
}: Props) {
  const { toast } = useToast();
  const [name, setName] = useState(initialValue);
  const [color, setColor] = useState(normalizeKeyCategoryColor(initialColor));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setName(initialValue);
      setColor(normalizeKeyCategoryColor(initialColor));
    }
  }, [open, initialValue, initialColor]);

  const handleSave = async () => {
    const value = name.trim();
    if (!value) {
      toast("Введите название категории", "error");
      return;
    }
    setSaving(true);
    try {
      const response = await keysApi.createCategory(value, color);
      const category = response.category || {
        name: value.slice(0, 24),
        color: normalizeKeyCategoryColor(color),
        keys_count: 0,
      };
      toast("Категория сохранена", "success");
      onCreated(category);
      onClose();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить категорию", "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={() => (!saving ? onClose() : undefined)} title={title} className="max-w-md">
      <div className="space-y-4">
        <Input
          label="Название категории"
          value={name}
          onChange={(event) => setName(event.target.value)}
          maxLength={24}
          placeholder="Например, Москва TCP"
          disabled={saving}
        />
        <div className="space-y-2">
          <label className="text-sm text-zinc-400">Цвет категории</label>
          <div className="flex items-center gap-3 rounded-lg border border-border bg-surface-2 px-3 py-2">
            <input
              type="color"
              value={color}
              onChange={(event) => setColor(normalizeKeyCategoryColor(event.target.value))}
              disabled={saving}
              className="h-10 w-12 cursor-pointer rounded border border-border bg-transparent p-0"
            />
            <input
              value={color}
              onChange={(event) => setColor(normalizeKeyCategoryColor(event.target.value))}
              maxLength={7}
              disabled={saving}
              className="h-10 flex-1 rounded-lg border border-border bg-bg px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
            />
          </div>
          <div className="flex flex-wrap gap-2">
            {KEY_CATEGORY_COLOR_PRESETS.map((preset) => (
              <button
                key={preset}
                type="button"
                onClick={() => setColor(preset)}
                disabled={saving}
                className={`h-8 w-8 rounded-full border transition ${
                  color === preset ? "border-white ring-2 ring-white/30" : "border-border"
                }`}
                style={{ backgroundColor: preset }}
                aria-label={`Выбрать цвет ${preset}`}
                title={preset}
              />
            ))}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Button onClick={() => void handleSave()} loading={saving}>
            {submitLabel}
          </Button>
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            Отмена
          </Button>
        </div>
      </div>
    </Modal>
  );
}
