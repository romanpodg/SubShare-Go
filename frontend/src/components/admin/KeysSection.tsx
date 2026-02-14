"use client";

import { useState } from "react";
import type { VLESSKey } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { AddKeyModal } from "./AddKeyModal";
import { EditKeyModal } from "./EditKeyModal";

interface Props {
  keys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

export function KeysSection({ keys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [showAddKey, setShowAddKey] = useState(false);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [checkingAll, setCheckingAll] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{id: number, label: string} | null>(null);
  const [deleting, setDeleting] = useState(false);

  const handleDelete = (id: number, label: string) => {
    setDeleteTarget({ id, label });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await keysApi.delete(deleteTarget.id);
      toast("Ключ удален", "success");
      await onRefresh();
    } catch {
      toast("Не удалось удалить ключ", "error");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const handleCheck = async (id: number) => {
    try {
      await keysApi.check(id);
      toast("Ключ проверен", "success");
      await onRefresh();
    } catch {
      toast("Не удалось проверить ключ", "error");
    }
  };

  const handleCheckAll = async () => {
    setCheckingAll(true);
    try {
      const result = await keysApi.checkAll();
      toast(`Проверено ключей: ${result.checked}`, "success");
      await onRefresh();
    } catch {
      toast("Не удалось проверить ключи", "error");
    } finally {
      setCheckingAll(false);
    }
  };

  const handleCopy = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url);
      toast("URL скопирован", "info");
    } catch {
      toast("Не удалось скопировать", "error");
    }
  };

  const healthDot = (status: string, label: string) => {
    const colors: Record<string, string> = {
      up: "bg-green-500",
      down: "bg-red-500",
      unknown: "bg-zinc-500",
    };
    return (
      <span
        className={`inline-block w-2 h-2 rounded-full ${colors[status] || colors.unknown}`}
        title={label}
      />
    );
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-center gap-2 text-lg font-semibold"
          aria-expanded={!collapsed}
        >
          <span className={`transition-transform ${collapsed ? "" : "rotate-90"}`}>&#9654;</span>
          Ключи ({keys.length})
        </button>
        <div className="flex gap-2">
          <Button variant="ghost" onClick={handleCheckAll} loading={checkingAll} className="text-xs">
            Проверить все
          </Button>
          <Button onClick={() => setShowAddKey(true)} className="text-xs">
            + Добавить
          </Button>
        </div>
      </div>

      {!collapsed && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <caption className="sr-only">Список ключей</caption>
            <thead>
              <tr className="text-left text-zinc-400 border-b border-border">
                <th scope="col" className="pb-2 pr-4">Название</th>
                <th scope="col" className="pb-2 pr-4">URL</th>
                <th scope="col" className="pb-2 pr-4">Статус</th>
                <th scope="col" className="pb-2 pr-4">Состояние</th>
                <th scope="col" className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id} className="border-b border-border last:border-0">
                  <td className="py-3 pr-4">{key.label}</td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs text-zinc-400 max-w-[300px] truncate">
                        {key.url_short}
                      </span>
                      <button
                        onClick={() => handleCopy(key.url)}
                        className="text-zinc-500 hover:text-zinc-300"
                        aria-label="Скопировать URL"
                      >
                        <svg
                          xmlns="http://www.w3.org/2000/svg"
                          width="14"
                          height="14"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth={2}
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                          <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                        </svg>
                      </button>
                    </div>
                  </td>
                  <td className="py-3 pr-4">
                    <StatusBadge status={key.status} />
                  </td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      {healthDot(key.check_status, key.check_status_label)}
                      <span className="text-xs text-zinc-400">
                        {key.check_status_label}
                        {key.last_latency_ms > 0 && ` (${key.last_latency_ms}ms)`}
                      </span>
                    </div>
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1.5">
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => handleCheck(key.id)}
                      >
                        Проверить
                      </Button>
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => setEditKey(key)}
                      >
                        Изм.
                      </Button>
                      <Button
                        variant="danger"
                        className="text-xs"
                        onClick={() => handleDelete(key.id, key.label)}
                      >
                        Удалить
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {keys.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-zinc-400">
                    Нет ключей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Удаление ключа"
        message={`Вы уверены, что хотите удалить ключ "${deleteTarget?.label}"?`}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
        loading={deleting}
      />

      <AddKeyModal open={showAddKey} onClose={() => setShowAddKey(false)} onRefresh={onRefresh} />
      {editKey && (
        <EditKeyModal keyData={editKey} onClose={() => setEditKey(null)} onRefresh={onRefresh} />
      )}
    </Card>
  );
}
