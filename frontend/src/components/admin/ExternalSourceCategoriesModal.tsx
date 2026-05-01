"use client";

import { useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import { externalSources } from "@/lib/api";
import type { ExternalSourceCategory } from "@/lib/types";
import { CreateExternalCategoryModal } from "./CreateExternalCategoryModal";

interface Props {
  open: boolean;
  onClose: () => void;
  categories: ExternalSourceCategory[];
  onSaved: (selectedCategory?: string) => Promise<void>;
}

export function ExternalSourceCategoriesModal({ open, onClose, categories, onSaved }: Props) {
  const { toast } = useToast();
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [savingFor, setSavingFor] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    if (!open) {
      return;
    }
    const nextDrafts: Record<string, string> = {};
    for (const category of categories) {
      nextDrafts[category.name] = category.name;
    }
    setDrafts(nextDrafts);
  }, [open, categories]);

  const handleRename = async (oldName: string) => {
    const nextName = (drafts[oldName] || "").trim();
    if (!nextName) {
      toast("Введите новое название категории", "error");
      return;
    }
    if (nextName === oldName) {
      toast("Название категории не изменилось", "error");
      return;
    }

    setSavingFor(oldName);
    try {
      await externalSources.renameCategory(oldName, nextName);
      toast(`Категория "${oldName}" переименована`, "success");
      await onSaved(nextName);
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось переименовать категорию", "error");
    } finally {
      setSavingFor(null);
    }
  };

  return (
    <>
      <Modal open={open} onClose={() => (savingFor ? undefined : onClose())} title="Категории источников" className="max-w-2xl">
        <div className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-sm text-zinc-400">Переименование категории применится ко всем источникам этой категории.</p>
            <Button variant="ghost" className="text-xs" onClick={() => setShowCreate(true)} disabled={Boolean(savingFor)}>
              + Создать категорию
            </Button>
          </div>

          {categories.length === 0 ? (
            <div className="rounded-lg border border-border bg-surface-2/50 px-4 py-6 text-sm text-zinc-500">
              Категорий пока нет.
            </div>
          ) : (
            <div className="space-y-2">
              {categories.map((category) => {
                const value = drafts[category.name] ?? category.name;
                const rowBusy = savingFor === category.name;
                return (
                  <div key={category.name} className="rounded-lg border border-border bg-surface-2/40 p-3">
                    <div className="grid grid-cols-1 gap-2 md:grid-cols-[1fr_auto] md:items-end">
                      <Input
                        label={`Категория · источников: ${category.sources_count}`}
                        value={value}
                        onChange={(event) =>
                          setDrafts((previous) => ({
                            ...previous,
                            [category.name]: event.target.value,
                          }))
                        }
                        maxLength={24}
                        disabled={Boolean(savingFor)}
                      />
                      <Button
                        variant="ghost"
                        onClick={() => void handleRename(category.name)}
                        loading={rowBusy}
                        disabled={Boolean(savingFor) || value.trim() === category.name}
                      >
                        Сохранить
                      </Button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </Modal>

      <CreateExternalCategoryModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onCreated={async (categoryName) => {
          await onSaved(categoryName);
        }}
      />
    </>
  );
}
