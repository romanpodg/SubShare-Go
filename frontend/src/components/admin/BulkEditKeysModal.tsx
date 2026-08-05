"use client";

import { FormEvent, useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory, KeySummary } from "@/lib/types";
import { Select } from "@/components/ui/Select";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";

interface Props {
  keys: KeySummary[];
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function BulkEditKeysModal({ keys, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [categories, setCategories] = useState<KeyCategory[]>([]);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [status, setStatus] = useState<"active" | "non-active">("active");
  const [category, setCategory] = useState("");

  useEffect(() => {
    void keysApi
      .listCategories()
      .then((response) => setCategories(response.categories || []))
      .catch(() => undefined);
  }, []);

  const categoryOptions = [{ value: "", label: "Не менять категорию" }].concat(
    categories.map((item) => ({ value: item.name, label: item.name }))
  );

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (keys.length === 0) {
      return;
    }

    setLoading(true);
    try {
      const keyIDs = keys.map((key) => key.id);
      const response = await keysApi.bulkUpdateStatus(keyIDs, status, category.trim() || undefined);
      const updatedCount = response.updated ?? keyIDs.length;
      toast(updatedCount === 1 ? "Конфигурация обновлена" : `Обновлено конфигураций: ${updatedCount}`, "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось применить массовые изменения", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Изменить конфигурации (${keys.length})`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-sm text-zinc-400">
          Изменения будут применены ко всем выбранным конфигурациям.
        </p>

        <Select
          id="bulk-key-status"
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value as "active" | "non-active")}
          options={[
            { value: "active", label: "Активен" },
            { value: "non-active", label: "Неактивен" },
          ]}
        />

        <div className="grid gap-2 md:grid-cols-[minmax(0,1fr)_auto] md:items-end">
          <Select
            label="Категория"
            value={category}
            onChange={(event) => setCategory(event.target.value)}
            options={categoryOptions}
          />
          <Button type="button" variant="ghost" onClick={() => setShowCreateCategory(true)}>
            + Добавить категорию
          </Button>
        </div>

        <Button type="submit" loading={loading}>
          Сохранить
        </Button>

        <CreateKeyCategoryModal
          open={showCreateCategory}
          onClose={() => setShowCreateCategory(false)}
          onCreated={(nextCategory) => {
            setCategories((previous) => {
              const next = previous.filter((item) => item.name !== nextCategory.name);
              return [...next, nextCategory].sort((left, right) => left.name.localeCompare(right.name, "ru"));
            });
            setCategory(nextCategory.name);
          }}
        />
      </form>
    </Modal>
  );
}
