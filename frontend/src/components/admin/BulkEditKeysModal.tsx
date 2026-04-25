"use client";

import { FormEvent, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { VLESSKey } from "@/lib/types";

interface Props {
  keys: VLESSKey[];
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function BulkEditKeysModal({ keys, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<"active" | "non-active">("active");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (keys.length === 0) {
      return;
    }

    setLoading(true);
    try {
      const results = await Promise.allSettled(
        keys.map((key) =>
          keysApi.update(key.id, {
            label: key.label,
            status,
            raw_url: key.kind === "real" ? key.url : undefined,
            uuid: key.edit_uuid || "",
            host: key.edit_host || "",
            port: key.edit_port || "",
            query: key.edit_query || "",
            fragment: key.edit_fragment || "",
            kind: key.kind,
            template_text: key.kind === "informational" ? (key.template_text || key.label) : undefined,
          })
        )
      );

      const failedKeys = results
        .map((result, index) => (result.status === "rejected" ? keys[index] : null))
        .filter((key): key is VLESSKey => key !== null);
      const successCount = keys.length - failedKeys.length;

      if (successCount > 0 && failedKeys.length === 0) {
        toast(successCount === 1 ? "Конфигурация обновлена" : `Обновлено конфигураций: ${successCount}`, "success");
        onClose();
      } else if (successCount > 0) {
        toast(`Обновлено ${successCount} из ${keys.length} конфигураций`, "error");
      } else {
        toast("Не удалось применить массовые изменения", "error");
      }

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

        <div className="flex flex-col gap-1.5">
          <label htmlFor="bulk-key-status" className="text-sm text-zinc-400">
            Статус
          </label>
          <select
            id="bulk-key-status"
            value={status}
            onChange={(e) => setStatus(e.target.value as "active" | "non-active")}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          >
            <option value="active">Активен</option>
            <option value="non-active">Неактивен</option>
          </select>
        </div>

        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}

