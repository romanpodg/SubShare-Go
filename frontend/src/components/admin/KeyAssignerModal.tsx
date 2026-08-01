"use client";

import { useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { EmojiText } from "@/components/ui/EmojiText";
import { users as usersApi } from "@/lib/api";
import type { User, VLESSKey } from "@/lib/types";

interface Props {
  user: User;
  assignableKeys: VLESSKey[];
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function KeyAssignerModal({ user, assignableKeys, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);

  const initialAssigned = user.assigned_key_ids
    ? user.assigned_key_ids.split(",").map(Number).filter(Boolean)
    : [];
  const [assignedIds, setAssignedIds] = useState<Set<number>>(new Set(initialAssigned));
  const [mode, setMode] = useState<"all" | "selected">(user.key_assignment_mode || "selected");

  const toggle = (id: number) => {
    setAssignedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const addAll = () => {
    setAssignedIds(new Set(assignableKeys.map((k) => k.id)));
  };

  const removeAll = () => {
    setAssignedIds(new Set());
  };

  const handleSave = async () => {
    setLoading(true);
    try {
      await usersApi.updateKeyAssignment(user.id, mode, Array.from(assignedIds));
      toast("Ключи обновлены", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить ключи", "error");
    } finally {
      setLoading(false);
    }
  };

  const assigned = assignableKeys.filter((k) => assignedIds.has(k.id));
  const available = assignableKeys.filter((k) => !assignedIds.has(k.id));

  return (
    <Modal open onClose={onClose} title={`Ключи — ${user.name}`}>
      <div className="mb-4 grid grid-cols-2 gap-2 rounded-lg border border-border bg-surface-2 p-1">
        <button
          type="button"
          onClick={() => setMode("all")}
          className={`rounded-md px-3 py-2 text-sm ${mode === "all" ? "bg-accent text-accent-fg" : "text-zinc-400 hover:bg-surface-1"}`}
        >
          Все, включая будущие
        </button>
        <button
          type="button"
          onClick={() => setMode("selected")}
          className={`rounded-md px-3 py-2 text-sm ${mode === "selected" ? "bg-accent text-accent-fg" : "text-zinc-400 hover:bg-surface-1"}`}
        >
          Фиксированный набор
        </button>
      </div>
      {mode === "all" && (
        <div className="mb-4 rounded-lg border border-info/20 bg-info/10 px-3 py-2 text-xs text-info">
          Пользователь получит все текущие и автоматически получит новые ключи.
        </div>
      )}
      <div className={mode === "all" ? "pointer-events-none opacity-50" : ""}>
      <div className="flex gap-2 mb-4">
        <Button variant="ghost" onClick={addAll} className="text-xs">
          Добавить все
        </Button>
        <Button variant="ghost" onClick={removeAll} className="text-xs">
          Убрать все
        </Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
        <div>
          <h3 className="text-sm text-zinc-400 mb-2">Доступные</h3>
          <div className="flex flex-wrap gap-1.5 min-h-[60px] bg-surface-2 rounded-lg p-3">
            {available.map((key) => (
              <button
                key={key.id}
                onClick={() => toggle(key.id)}
                className="px-2.5 py-1 rounded-md bg-surface-1 text-xs text-zinc-300 hover:bg-accent/20 transition-colors"
              >
                <EmojiText text={key.label} />
              </button>
            ))}
            {available.length === 0 && <span className="text-xs text-zinc-500">Пусто</span>}
          </div>
        </div>
        <div>
          <h3 className="text-sm text-zinc-400 mb-2">Назначенные</h3>
          <div className="flex flex-wrap gap-1.5 min-h-[60px] bg-surface-2 rounded-lg p-3">
            {assigned.map((key) => (
              <button
                key={key.id}
                onClick={() => toggle(key.id)}
                className="px-2.5 py-1 rounded-md bg-accent/20 text-xs text-indigo-300 hover:bg-accent/30 transition-colors"
              >
                <EmojiText text={key.label} />
              </button>
            ))}
            {assigned.length === 0 && <span className="text-xs text-zinc-500">Пусто</span>}
          </div>
        </div>
      </div>
      </div>

      <Button onClick={handleSave} loading={loading} className="w-full">
        Сохранить
      </Button>
    </Modal>
  );
}
