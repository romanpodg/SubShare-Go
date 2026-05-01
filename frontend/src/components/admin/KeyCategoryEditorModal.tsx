"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory, VLESSKey } from "@/lib/types";
import { EmojiText } from "@/components/ui/EmojiText";
import {
  KEY_CATEGORY_COLOR_PRESETS,
  DEFAULT_KEY_CATEGORY_COLOR,
  normalizeKeyCategoryColor,
} from "./keyCategoryColors";

interface Props {
  open: boolean;
  categoryName: string;
  categoryKeys: VLESSKey[];
  categories: KeyCategory[];
  onClose: () => void;
  onUpdated: (nextCategory: KeyCategory, previousName: string) => Promise<void>;
  onDeleted: (categoryName: string, mode: "delete_with_keys" | "keep_keys") => Promise<void>;
  onOpenAddConfiguration: (categoryName: string) => void;
  onEditKey: (key: VLESSKey) => void;
  onDeleteKey: (key: VLESSKey) => void;
  onMoveKeyToCategory: (key: VLESSKey, nextCategory: string) => Promise<void>;
}

function normalizeCategory(value: string | null | undefined): string {
  return (value || "").trim();
}

export function KeyCategoryEditorModal({
  open,
  categoryName,
  categoryKeys,
  categories,
  onClose,
  onUpdated,
  onDeleted,
  onOpenAddConfiguration,
  onEditKey,
  onDeleteKey,
  onMoveKeyToCategory,
}: Props) {
  const { toast } = useToast();
  const [draftName, setDraftName] = useState(categoryName);
  const [draftColor, setDraftColor] = useState(DEFAULT_KEY_CATEGORY_COLOR);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteMode, setDeleteMode] = useState<"delete_with_keys" | "keep_keys">("keep_keys");
  const [movingKeyID, setMovingKeyID] = useState<number | null>(null);
  const [moveDrafts, setMoveDrafts] = useState<Record<number, string>>({});

  useEffect(() => {
    if (!open) {
      return;
    }

    setDraftName(categoryName);
    setDraftColor(normalizeKeyCategoryColor(categories.find((item) => normalizeCategory(item.name) === categoryName)?.color));
    const nextDrafts: Record<number, string> = {};
    for (const key of categoryKeys) {
      nextDrafts[key.id] = normalizeCategory(key.category);
    }
    setMoveDrafts(nextDrafts);
    setDeleteMode("keep_keys");
  }, [open, categoryName, categoryKeys, categories]);

  const handleSave = async () => {
    const nextName = draftName.trim();
    if (!nextName) {
      toast("Введите название категории", "error");
      return;
    }
    const nextColor = normalizeKeyCategoryColor(draftColor);
    const currentColor = normalizeKeyCategoryColor(
      categories.find((item) => normalizeCategory(item.name) === categoryName)?.color
    );
    if (nextName === categoryName && nextColor === currentColor) {
      toast("Параметры категории не изменились", "error");
      return;
    }

    setSaving(true);
    try {
      const response = await keysApi.updateCategory(categoryName, nextName, nextColor);
      toast(`Категория "${categoryName}" обновлена`, "success");
      await onUpdated(
        response.category || { name: nextName, color: nextColor, keys_count: categoryKeys.length },
        categoryName
      );
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось обновить категорию", "error");
    } finally {
      setSaving(false);
    }
  };

  const handleMoveKey = async (key: VLESSKey) => {
    const nextCategory = normalizeCategory(moveDrafts[key.id]);
    if (nextCategory === normalizeCategory(key.category)) {
      toast("Категория у ключа не изменилась", "error");
      return;
    }

    setMovingKeyID(key.id);
    try {
      await onMoveKeyToCategory(key, nextCategory);
      toast(`Ключ "${key.label}" перенесён`, "success");
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось перенести ключ", "error");
    } finally {
      setMovingKeyID(null);
    }
  };

  const handleDeleteCategory = async () => {
    setDeleting(true);
    try {
      await keysApi.deleteCategory(categoryName, deleteMode);
      await onDeleted(categoryName, deleteMode);
      toast(
        deleteMode === "delete_with_keys"
          ? `Категория "${categoryName}" удалена вместе с ключами`
          : `Категория "${categoryName}" удалена, ключи остались без категории`,
        "success"
      );
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось удалить категорию", "error");
    } finally {
      setDeleting(false);
    }
  };

  const categoryOptions = categories
    .map((category) => ({
      value: normalizeCategory(category.name),
      label: `${normalizeCategory(category.name)} (${category.keys_count})`,
    }))
    .sort((left, right) => left.label.localeCompare(right.label, "ru"));

  const currentCategory = categories.find((item) => normalizeCategory(item.name) === categoryName);
  const deletionLocked = false;

  return (
    <Modal
      open={open}
      onClose={() => (!saving && !deleting && movingKeyID === null ? onClose() : undefined)}
      title={`Категория — ${categoryName}`}
      className="max-w-4xl max-h-[88vh] overflow-y-auto"
    >
      <div className="space-y-4">
        <div className="rounded-xl border border-border bg-surface-2/40 p-4">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_220px_auto] md:items-end">
            <div className="space-y-3">
              <Input
                label="Название категории"
                value={draftName}
                onChange={(event) => setDraftName(event.target.value)}
                maxLength={24}
                disabled={saving || deleting || movingKeyID !== null}
              />
              <div className="space-y-2">
                <label className="text-sm text-zinc-400">Цвет категории</label>
                <div className="flex items-center gap-3 rounded-lg border border-border bg-bg/50 px-3 py-2">
                  <input
                    type="color"
                    value={draftColor}
                    onChange={(event) => setDraftColor(normalizeKeyCategoryColor(event.target.value))}
                    disabled={saving || deleting || movingKeyID !== null}
                    className="h-10 w-12 cursor-pointer rounded border border-border bg-transparent p-0"
                  />
                  <input
                    value={draftColor}
                    onChange={(event) => setDraftColor(normalizeKeyCategoryColor(event.target.value))}
                    disabled={saving || deleting || movingKeyID !== null}
                    className="h-10 flex-1 rounded-lg border border-border bg-bg px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                  />
                </div>
                <div className="flex flex-wrap gap-2">
                  {KEY_CATEGORY_COLOR_PRESETS.map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      onClick={() => setDraftColor(preset)}
                      disabled={saving || deleting || movingKeyID !== null}
                      className={`h-8 w-8 rounded-full border transition ${
                        draftColor === preset ? "border-white ring-2 ring-white/30" : "border-border"
                      }`}
                      style={{ backgroundColor: preset }}
                      aria-label={`Выбрать цвет ${preset}`}
                      title={preset}
                    />
                  ))}
                </div>
              </div>
            </div>
            <div className="rounded-lg border border-border bg-bg/50 px-4 py-3 text-sm text-zinc-300">
              <div>Ключей: {categoryKeys.length}</div>
              <div className="mt-1 text-xs text-zinc-500">Инфо: {categoryKeys.filter((key) => key.kind === "informational").length}</div>
              <div className="text-xs text-zinc-500">Конфигурации: {categoryKeys.filter((key) => key.kind !== "informational").length}</div>
              {currentCategory?.color ? (
                <div className="mt-2 flex items-center gap-2 text-xs text-zinc-500">
                  <span className="inline-block h-3 w-3 rounded-full border border-white/10" style={{ backgroundColor: currentCategory.color }} />
                  Текущий цвет: {currentCategory.color}
                </div>
              ) : null}
            </div>
            <div className="flex flex-col items-stretch gap-2">
              <Button variant="ghost" onClick={() => onOpenAddConfiguration(categoryName)} disabled={saving || deleting || movingKeyID !== null}>
                Добавить конфигурацию
              </Button>
              <Button
                onClick={() => void handleSave()}
                loading={saving}
                disabled={
                  movingKeyID !== null ||
                  deleting ||
                  (draftName.trim() === categoryName &&
                    normalizeKeyCategoryColor(draftColor) === normalizeKeyCategoryColor(currentCategory?.color))
                }
              >
                Сохранить
              </Button>
            </div>
          </div>
        </div>

        <div className="rounded-xl border border-red-500/20 bg-red-500/5 p-4">
          <div className="mb-3 text-sm font-medium text-zinc-100">Удаление категории</div>
          {deletionLocked ? (
            <div className="rounded-lg border border-border bg-bg/50 px-3 py-3 text-sm text-zinc-400">
              Эта категория защищена от удаления.
            </div>
          ) : (
            <div className="space-y-3">
              <div className="grid gap-2 md:grid-cols-2">
                <button
                  type="button"
                  onClick={() => setDeleteMode("keep_keys")}
                  className={`rounded-lg border px-3 py-3 text-left text-sm transition ${
                    deleteMode === "keep_keys"
                      ? "border-accent bg-accent/10 text-zinc-100"
                      : "border-border bg-bg/40 text-zinc-400 hover:bg-bg/60"
                  }`}
                >
                  Удалить категорию, оставить ключи
                  <div className="mt-1 text-xs text-zinc-500">Все ключи из категории станут без категории.</div>
                </button>
                <button
                  type="button"
                  onClick={() => setDeleteMode("delete_with_keys")}
                  className={`rounded-lg border px-3 py-3 text-left text-sm transition ${
                    deleteMode === "delete_with_keys"
                      ? "border-red-400 bg-red-500/10 text-zinc-100"
                      : "border-border bg-bg/40 text-zinc-400 hover:bg-bg/60"
                  }`}
                >
                  Удалить категорию вместе с ключами
                  <div className="mt-1 text-xs text-zinc-500">Будут удалены и сама категория, и все ключи внутри нее.</div>
                </button>
              </div>
              <div className="flex justify-end">
                <Button variant="danger" onClick={() => void handleDeleteCategory()} loading={deleting} disabled={saving || movingKeyID !== null}>
                  Удалить категорию
                </Button>
              </div>
            </div>
          )}
        </div>

        {categoryKeys.length === 0 ? (
          <div className="rounded-xl border border-border bg-surface-2/20 px-4 py-8 text-center text-sm text-zinc-500">
            В этой категории пока нет ключей. Добавьте новую конфигурацию или перенесите существующую.
          </div>
        ) : (
          <div className="space-y-3">
            {categoryKeys.map((key) => {
              const isReal = key.kind !== "informational";
              const moving = movingKeyID === key.id;

              return (
                <div key={key.id} className="rounded-xl border border-border bg-surface-2/25 p-3">
                  <div className="mb-2 flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium text-zinc-100">
                        <EmojiText text={key.label} />
                      </div>
                      <div className="text-xs text-zinc-400">
                        {isReal ? "Конфигурация" : "Информационный ключ"} · ID {key.id}
                      </div>
                    </div>
                    <div className="rounded-full border border-border bg-bg/50 px-2.5 py-1 text-xs text-zinc-300">
                      {key.status === "active" ? "Активен" : "Неактивен"}
                    </div>
                  </div>

                  <div className="mb-3 text-xs text-zinc-400">
                    {isReal ? (
                      <>
                        Название в клиенте: <span className="text-zinc-300"><EmojiText text={key.client_display_name || key.label} /></span>
                      </>
                    ) : (
                      <span className="text-zinc-300"><EmojiText text={key.template_text || key.label} /></span>
                    )}
                  </div>

                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                    <Select
                      label="Категория ключа"
                      value={moveDrafts[key.id] || normalizeCategory(key.category)}
                      onChange={(event) =>
                        setMoveDrafts((previous) => ({
                          ...previous,
                          [key.id]: event.target.value,
                        }))
                      }
                      options={[{ value: "", label: "Без категории" }, ...categoryOptions]}
                      disabled={moving || saving || deleting}
                    />
                    <div className="flex flex-wrap items-end gap-2">
                      <Button
                        variant="ghost"
                        onClick={() => void handleMoveKey(key)}
                        loading={moving}
                        disabled={
                          moving ||
                          saving ||
                          deleting ||
                          normalizeCategory(moveDrafts[key.id]) === normalizeCategory(key.category)
                        }
                      >
                        Перенести
                      </Button>
                      <Button variant="ghost" onClick={() => onEditKey(key)} disabled={moving || saving || deleting}>
                        Изменить
                      </Button>
                      <Button variant="danger" onClick={() => onDeleteKey(key)} disabled={moving || saving || deleting}>
                        Удалить
                      </Button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Modal>
  );
}
