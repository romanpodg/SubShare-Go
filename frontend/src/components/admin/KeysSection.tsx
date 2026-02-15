"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
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
  const [orderedKeys, setOrderedKeys] = useState<VLESSKey[]>(keys);
  const [showAddKey, setShowAddKey] = useState(false);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [checkingAll, setCheckingAll] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{id: number, label: string} | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [dragging, setDragging] = useState<{ id: number; rowIndex: number } | null>(null);
  const [reordering, setReordering] = useState(false);
  const [hasUnsavedOrder, setHasUnsavedOrder] = useState(false);
  const cardHeightsRef = useRef<Record<string, HTMLDivElement | null>>({});
  const [rowHeights, setRowHeights] = useState<Record<number, { informational: number; real: number }>>({});

  useEffect(() => {
    if (!hasUnsavedOrder) {
      setOrderedKeys(keys);
    }
  }, [keys, hasUnsavedOrder]);

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
    const pulseClass = status === "up" ? "status-pulse" : "";
    return (
      <span
        className={`inline-block w-2 h-2 rounded-full ${colors[status] || colors.unknown} ${pulseClass}`}
        title={label}
      />
    );
  };

  const realKeys = orderedKeys.filter((item) => item.kind !== "informational");
  const informationalKeys = orderedKeys.filter((item) => item.kind === "informational");

  const buildRows = () =>
    orderedKeys.map((item) =>
      item.kind === "informational"
        ? { informational: item }
        : { real: item }
    );

  const rows = buildRows();

  useLayoutEffect(() => {
    setRowHeights((previous) => {
      const next: Record<number, { informational: number; real: number }> = {};

      rows.forEach((_, index) => {
        const informational = cardHeightsRef.current[`informational-${index}`]?.offsetHeight ?? 0;
        const real = cardHeightsRef.current[`real-${index}`]?.offsetHeight ?? 0;
        next[index] = { informational, real };
      });

      const previousKeys = Object.keys(previous);
      const nextKeys = Object.keys(next);
      if (previousKeys.length !== nextKeys.length) {
        return next;
      }

      for (const rowIndexText of nextKeys) {
        const rowIndex = Number(rowIndexText);
        if (
          previous[rowIndex]?.informational !== next[rowIndex].informational ||
          previous[rowIndex]?.real !== next[rowIndex].real
        ) {
          return next;
        }
      }

      return previous;
    });
  }, [rows]);

  const saveOrder = async () => {
    const ids = orderedKeys.map((item) => item.id);
    setReordering(true);
    try {
      await keysApi.reorder(ids);
      await onRefresh();
      setHasUnsavedOrder(false);
      toast("Порядок ключей сохранен", "success");
    } catch {
      toast("Не удалось изменить порядок ключей", "error");
    } finally {
      setReordering(false);
    }
  };

  const flattenRows = (nextRows: Array<{ informational?: VLESSKey; real?: VLESSKey }>) =>
    nextRows
      .map((row) => row.informational ?? row.real)
      .filter((item): item is VLESSKey => Boolean(item));

  const onDropRow = async (targetRowIndex: number) => {
    if (!dragging) return;
    const sourceRowIndex = dragging.rowIndex;
    if (sourceRowIndex < 0 || targetRowIndex < 0 || sourceRowIndex === targetRowIndex) {
      setDragging(null);
      return;
    }

    const nextRows = [...rows];
    [nextRows[sourceRowIndex], nextRows[targetRowIndex]] = [nextRows[targetRowIndex], nextRows[sourceRowIndex]];
    const next = flattenRows(nextRows);

    setOrderedKeys(next);
    setHasUnsavedOrder(true);
    setDragging(null);
  };

  const keyCard = (key: VLESSKey, rowIndex: number) => {
    const isReal = key.kind !== "informational";
    const isDragging = dragging?.id === key.id;
    return (
      <div
        draggable={!reordering}
        onDragStart={(event) => {
          setDragging({ id: key.id, rowIndex });
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", String(key.id));
        }}
        onDragEnd={() => setDragging(null)}
        className={`group rounded-lg border border-border bg-surface-2 px-3 py-2 transition-opacity ${isDragging ? "opacity-40" : "opacity-100"}`}
      >
        <div className="mb-1 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <span
              className="text-zinc-500 group-hover:text-zinc-300 cursor-grab active:cursor-grabbing"
              aria-label="Перетащить ключ"
              title="Перетащите для изменения порядка"
            >
              <svg width="14" height="14" viewBox="0 0 14 14" fill="currentColor" aria-hidden>
                <circle cx="4" cy="3" r="1" />
                <circle cx="10" cy="3" r="1" />
                <circle cx="4" cy="7" r="1" />
                <circle cx="10" cy="7" r="1" />
                <circle cx="4" cy="11" r="1" />
                <circle cx="10" cy="11" r="1" />
              </svg>
            </span>
            <span className="font-mono text-xs text-zinc-400">ID {key.id}</span>
            <span className="text-sm font-medium text-zinc-100 truncate">{key.label}</span>
          </div>
          <StatusBadge status={key.status} />
        </div>

        {isReal ? (
          <>
            <div className="mb-2 flex items-center gap-2">
              <span className="font-mono text-xs text-zinc-400 truncate">{key.url_short}</span>
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
            <div className="mb-2 flex items-center gap-2">
              {healthDot(key.check_status, key.check_status_label)}
              <span className="text-xs text-zinc-400">
                {key.check_status_label}
                {key.last_latency_ms > 0 && ` (${key.last_latency_ms}ms)`}
              </span>
            </div>
            <div className="flex gap-1.5">
              <Button variant="ghost" className="text-xs" onClick={() => handleCheck(key.id)}>
                Проверить
              </Button>
              <Button variant="ghost" className="text-xs" onClick={() => setEditKey(key)}>
                Изм.
              </Button>
              <Button variant="danger" className="text-xs" onClick={() => handleDelete(key.id, key.label)}>
                Удалить
              </Button>
            </div>
          </>
        ) : (
          <>
            <div className="mb-2 text-xs text-zinc-400 truncate">{key.template_text || key.label}</div>
            <div className="flex gap-1.5">
              <Button variant="ghost" className="text-xs" onClick={() => setEditKey(key)}>
                Изм.
              </Button>
              <Button variant="danger" className="text-xs" onClick={() => handleDelete(key.id, key.label)}>
                Удалить
              </Button>
            </div>
          </>
        )}
      </div>
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
          Ключи ({orderedKeys.length})
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
        <div>
          <div className="mb-3 grid grid-cols-[minmax(0,0.75fr)_minmax(0,1.25fr)] gap-3 text-sm font-medium text-zinc-300">
            <div>Информационные ключи ({informationalKeys.length})</div>
            <div>VLESS ключи ({realKeys.length})</div>
          </div>

          <div className="space-y-2">
            {rows.map((row, index) => (
              <div key={`row-${index}`} className="grid grid-cols-[minmax(0,0.75fr)_minmax(0,1.25fr)] gap-3">
                  <div
                    className="flex h-full flex-col gap-2"
                    onDragOver={(event) => {
                      event.preventDefault();
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      void onDropRow(index);
                    }}
                  >
                  {row.informational ? (
                    <>
                      <div
                        ref={(element) => {
                          cardHeightsRef.current[`informational-${index}`] = element;
                        }}
                      >
                        {keyCard(row.informational, index)}
                      </div>
                    </>
                  ) : (
                    <div
                      className="h-full min-h-[72px] rounded-lg border border-border bg-surface-2/40"
                      style={
                        rowHeights[index]?.real > 0
                          ? { height: `${rowHeights[index].real}px` }
                          : undefined
                      }
                    />
                  )}
                </div>
                  <div
                    className="flex h-full flex-col gap-2"
                    onDragOver={(event) => {
                      event.preventDefault();
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      void onDropRow(index);
                    }}
                  >
                  {row.real ? (
                    <>
                      <div
                        ref={(element) => {
                          cardHeightsRef.current[`real-${index}`] = element;
                        }}
                      >
                        {keyCard(row.real, index)}
                      </div>
                    </>
                  ) : (
                    <div
                      className="h-full min-h-[72px] rounded-lg border border-border bg-surface-2/40"
                      style={
                        rowHeights[index]?.informational > 0
                          ? { height: `${rowHeights[index].informational}px` }
                          : undefined
                      }
                    />
                  )}
                </div>
              </div>
            ))}

            {rows.length === 0 && (
              <div className="py-8 text-center text-zinc-500">Нет ключей</div>
            )}

            {rows.length > 0 && (
              <div className="pt-2 flex justify-end">
                <Button
                  onClick={() => void saveOrder()}
                  loading={reordering}
                  disabled={!hasUnsavedOrder || reordering}
                  className="text-xs"
                >
                  Сохранить порядок
                </Button>
              </div>
            )}
          </div>
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
