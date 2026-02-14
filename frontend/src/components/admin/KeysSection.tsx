"use client";

import { useState } from "react";
import type { VLESSKey } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
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

  const handleDelete = async (id: number, label: string) => {
    if (!confirm(`Удалить ключ "${label}"?`)) return;
    try {
      await keysApi.delete(id);
      toast("Key deleted", "success");
      await onRefresh();
    } catch {
      toast("Failed to delete key", "error");
    }
  };

  const handleCheck = async (id: number) => {
    try {
      await keysApi.check(id);
      toast("Key checked", "success");
      await onRefresh();
    } catch {
      toast("Failed to check key", "error");
    }
  };

  const handleCheckAll = async () => {
    setCheckingAll(true);
    try {
      const result = await keysApi.checkAll();
      toast(`Checked ${result.checked} keys`, "success");
      await onRefresh();
    } catch {
      toast("Failed to check keys", "error");
    } finally {
      setCheckingAll(false);
    }
  };

  const handleCopy = async (url: string) => {
    await navigator.clipboard.writeText(url);
    toast("URL copied", "info");
  };

  const healthDot = (status: string) => {
    const colors: Record<string, string> = {
      up: "bg-green-500",
      down: "bg-red-500",
      unknown: "bg-zinc-500",
    };
    return (
      <span
        className={`inline-block w-2 h-2 rounded-full ${colors[status] || colors.unknown}`}
      />
    );
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-center gap-2 text-lg font-semibold"
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
            <thead>
              <tr className="text-left text-zinc-500 border-b border-border">
                <th className="pb-2 pr-4">Label</th>
                <th className="pb-2 pr-4">URL</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2 pr-4">Health</th>
                <th className="pb-2">Действия</th>
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
                        className="text-zinc-500 hover:text-zinc-300 text-xs"
                      >
                        copy
                      </button>
                    </div>
                  </td>
                  <td className="py-3 pr-4">
                    <span
                      className={`text-xs px-2 py-0.5 rounded-full ${key.status === "active" ? "bg-green-500/10 text-green-400" : "bg-zinc-500/10 text-zinc-400"}`}
                    >
                      {key.status_label}
                    </span>
                  </td>
                  <td className="py-3 pr-4">
                    <div className="flex items-center gap-2">
                      {healthDot(key.check_status)}
                      <span className="text-xs text-zinc-400">
                        {key.check_status_label}
                        {key.last_latency_ms > 0 && ` (${key.last_latency_ms}ms)`}
                      </span>
                    </div>
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => handleCheck(key.id)}
                      >
                        Check
                      </Button>
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => setEditKey(key)}
                      >
                        Edit
                      </Button>
                      <Button
                        variant="danger"
                        className="text-xs"
                        onClick={() => handleDelete(key.id, key.label)}
                      >
                        Del
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {keys.length === 0 && (
                <tr>
                  <td colSpan={5} className="py-8 text-center text-zinc-500">
                    Нет ключей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <AddKeyModal open={showAddKey} onClose={() => setShowAddKey(false)} onRefresh={onRefresh} />
      {editKey && (
        <EditKeyModal keyData={editKey} onClose={() => setEditKey(null)} onRefresh={onRefresh} />
      )}
    </Card>
  );
}
