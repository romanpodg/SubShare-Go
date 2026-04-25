"use client";

import React, { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import type { VLESSKey } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { AddKeyModal } from "./AddKeyModal";
import { BulkEditKeysModal } from "./BulkEditKeysModal";
import { EditKeyModal } from "./EditKeyModal";
import { copyToClipboard } from "@/lib/clipboard";
import { EmojiText } from "@/components/ui/EmojiText";

const formatDateTime = (value: string | null) => {
  if (!value) {
    return null;
  }

  const isoMatch = value.match(/^(\d{4}-\d{2}-\d{2})(?:[T\s](\d{2}:\d{2})(?::\d{2})?)?/);
  if (isoMatch) {
    const parsed = new Date(value.replace(/-/g, "/"));
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
        .toLocaleString("en-GB", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          hour: isoMatch[2] ? "2-digit" : undefined,
          minute: isoMatch[2] ? "2-digit" : undefined,
        })
        .replace(",", "");
    }
  }

  const slashMatch = value.match(/^(\d{2})\/(\d{2})\/(\d{4})\s+(\d{2}):(\d{2})$/);
  if (slashMatch) {
    return `${slashMatch[1]}/${slashMatch[2]}/${slashMatch[3]} ${slashMatch[4]}:${slashMatch[5]}`;
  }

  return value;
};

interface Props {
  keys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

/** Semi-transparent "+ Добавить" button shown in the gap between key rows on hover. */
function InsertGapButton({ index, onInsert }: { index: number; onInsert: (index: number) => void }) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      className="relative flex items-center justify-center"
      style={{ height: hovered ? "32px" : "6px", transition: "height 150ms ease" }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      <button
        type="button"
        onClick={() => onInsert(index)}
        className="absolute inset-0 flex items-center justify-center rounded-md text-xs font-medium transition-opacity duration-150"
        style={{ opacity: hovered ? 0.7 : 0, pointerEvents: hovered ? "auto" : "none" }}
      >
        <span className="flex items-center gap-1 rounded-md bg-zinc-700 px-3 py-1 text-zinc-300 shadow-sm">
          + Добавить
        </span>
      </button>
      {/* Thin line hint visible on hover */}
      {!hovered && (
        <div className="absolute inset-x-4 top-1/2 -translate-y-1/2 h-px bg-transparent group-hover:bg-zinc-700 transition-colors" />
      )}
    </div>
  );
}

export function KeysSection({ keys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [hiddenCategory, setHiddenCategory] = useState<"informational" | "real" | null>(null);
  const [orderedKeys, setOrderedKeys] = useState<VLESSKey[]>(keys);
  const [showAddKey, setShowAddKey] = useState(false);
  const [showBulkEdit, setShowBulkEdit] = useState(false);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [checkingAll, setCheckingAll] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{id: number, label: string} | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [bulkDeleteTargetIDs, setBulkDeleteTargetIDs] = useState<number[] | null>(null);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [selectedKeyIDs, setSelectedKeyIDs] = useState<number[]>([]);
  const [dragging, setDragging] = useState<{ id: number; rowIndex: number } | null>(null);
  const [reordering, setReordering] = useState(false);
  const [hasUnsavedOrder, setHasUnsavedOrder] = useState(false);
  const [insertAtIndex, setInsertAtIndex] = useState<number | null>(null);
  const cardHeightsRef = useRef<Record<string, HTMLDivElement | null>>({});
  const selectAllRef = useRef<HTMLInputElement | null>(null);
  const [rowHeights, setRowHeights] = useState<Record<number, { informational: number; real: number }>>({});
  const selectedCount = selectedKeyIDs.length;
  const allSelected = orderedKeys.length > 0 && selectedCount === orderedKeys.length;
  const partiallySelected = selectedCount > 0 && !allSelected;
  const selectedKeys = orderedKeys.filter((key) => selectedKeyIDs.includes(key.id));

  useEffect(() => {
    if (!hasUnsavedOrder) {
      setOrderedKeys(keys);
    }
  }, [keys, hasUnsavedOrder]);

  useEffect(() => {
    setSelectedKeyIDs((prev) => prev.filter((id) => orderedKeys.some((key) => key.id === id)));
  }, [orderedKeys]);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = selectedCount > 0 && !allSelected;
  }, [selectedCount, allSelected]);

  // Auto-scroll the page while dragging near edges
  useEffect(() => {
    if (!dragging) return;
    let rafId = 0;
    const EDGE = 80; // px from viewport edge to start scrolling
    const SPEED = 18; // px per frame at the very edge
    let lastY = 0;

    const onDragOver = (e: DragEvent) => {
      lastY = e.clientY;
    };

    const tick = () => {
      const vh = window.innerHeight;
      if (lastY > 0 && lastY < EDGE) {
        window.scrollBy(0, -SPEED * (1 - lastY / EDGE));
      } else if (lastY > vh - EDGE) {
        window.scrollBy(0, SPEED * (1 - (vh - lastY) / EDGE));
      }
      rafId = requestAnimationFrame(tick);
    };

    window.addEventListener("dragover", onDragOver);
    rafId = requestAnimationFrame(tick);

    return () => {
      window.removeEventListener("dragover", onDragOver);
      cancelAnimationFrame(rafId);
    };
  }, [dragging]);

  const handleDelete = (id: number, label: string) => {
    setDeleteTarget({ id, label });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await keysApi.delete(deleteTarget.id);
      toast("Конфигурация удалена", "success");
      await onRefresh();
    } catch {
      toast("Не удалось удалить конфигурацию", "error");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const toggleKeySelection = (id: number) => {
    setSelectedKeyIDs((prev) => (prev.includes(id) ? prev.filter((value) => value !== id) : [...prev, id]));
  };

  const toggleSelectAllKeys = () => {
    setSelectedKeyIDs(allSelected ? [] : orderedKeys.map((key) => key.id));
  };

  const openDeleteSelectedDialog = () => {
    if (selectedKeyIDs.length === 0) {
      return;
    }
    setBulkDeleteTargetIDs([...selectedKeyIDs]);
  };

  const confirmBulkDelete = async () => {
    if (!bulkDeleteTargetIDs || bulkDeleteTargetIDs.length === 0) {
      return;
    }

    setBulkDeleting(true);
    try {
      const results = await Promise.allSettled(bulkDeleteTargetIDs.map((id) => keysApi.delete(id)));
      const failedIDs = results
        .map((result, index) => (result.status === "rejected" ? bulkDeleteTargetIDs[index] : null))
        .filter((id): id is number => id !== null);
      const successCount = bulkDeleteTargetIDs.length - failedIDs.length;

      if (successCount > 0 && failedIDs.length === 0) {
        toast(successCount === 1 ? "Конфигурация удалена" : `Удалено конфигураций: ${successCount}`, "success");
      } else if (successCount > 0) {
        toast(`Удалено ${successCount} из ${bulkDeleteTargetIDs.length}. Проверьте оставшиеся конфигурации.`, "error");
      } else {
        toast("Не удалось удалить выбранные конфигурации", "error");
      }

      setSelectedKeyIDs(failedIDs);
      await onRefresh();
    } catch {
      toast("Не удалось удалить выбранные конфигурации", "error");
    } finally {
      setBulkDeleting(false);
      setBulkDeleteTargetIDs(null);
    }
  };

  const handleCheck = async (id: number) => {
    try {
      await keysApi.check(id);
      toast("Конфигурация проверена", "success");
      await onRefresh();
    } catch {
      toast("Не удалось проверить конфигурацию", "error");
    }
  };

  const handleCheckAll = async () => {
    setCheckingAll(true);
    try {
      const result = await keysApi.checkAll();
      toast(`Проверено конфигураций: ${result.checked}`, "success");
      await onRefresh();
    } catch {
      toast("Не удалось проверить конфигурации", "error");
    } finally {
      setCheckingAll(false);
    }
  };

  const handleCopy = async (url: string) => {
    try {
      const copied = await copyToClipboard(url);
      if (!copied) {
        if (typeof window !== "undefined") {
          window.prompt("Скопируйте ссылку вручную:", url);
          toast("Буфер обмена недоступен: ссылка показана для ручного копирования", "info");
          return;
        }
        toast("Не удалось скопировать", "error");
        return;
      }
      toast("URL скопирован", "info");
    } catch {
      if (typeof window !== "undefined") {
        window.prompt("Скопируйте ссылку вручную:", url);
        toast("Буфер обмена недоступен: ссылка показана для ручного копирования", "info");
        return;
      }
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
  const informationalHidden = hiddenCategory === "informational";
  const realHidden = hiddenCategory === "real";

  const gridLayoutStyle = {
    gridTemplateColumns: informationalHidden
      ? "minmax(0, 0fr) minmax(0, 1fr)"
      : realHidden
        ? "minmax(0, 1fr) minmax(0, 0fr)"
        : "minmax(0, 0.75fr) minmax(0, 1.25fr)",
    columnGap: hiddenCategory ? "0px" : "0.75rem",
    transition: "grid-template-columns 320ms ease, column-gap 320ms ease",
  } as const;

  const buildRows = () =>
    orderedKeys.map((item) =>
      item.kind === "informational"
        ? { informational: item }
        : { real: item }
    );

  const rows = buildRows();

  const recalculateRowHeights = useCallback(() => {
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

  useLayoutEffect(() => {
    recalculateRowHeights();
  }, [recalculateRowHeights, hiddenCategory]);

  useEffect(() => {
    const transitionTimer = window.setTimeout(() => {
      recalculateRowHeights();
    }, 360);

    return () => {
      window.clearTimeout(transitionTimer);
    };
  }, [hiddenCategory, recalculateRowHeights]);

  useEffect(() => {
    const onResize = () => {
      recalculateRowHeights();
    };

    window.addEventListener("resize", onResize);
    return () => {
      window.removeEventListener("resize", onResize);
    };
  }, [recalculateRowHeights]);

  const saveOrder = async () => {
    const ids = orderedKeys.map((item) => item.id);
    setReordering(true);
    try {
      await keysApi.reorder(ids);
      await onRefresh();
      setHasUnsavedOrder(false);
      toast("Порядок конфигураций сохранен", "success");
    } catch {
      toast("Не удалось изменить порядок конфигураций", "error");
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
    const isSelected = selectedKeyIDs.includes(key.id);
    const lastChecked = formatDateTime(key.last_checked_at);
    const statusParts = [key.check_status_label];

    if (key.last_latency_ms > 0) {
      statusParts.push(`${key.last_latency_ms}ms`);
    }

    if (lastChecked) {
      statusParts.push(`последняя проверка ${lastChecked}`);
    }
    return (
      <div
        draggable={!reordering}
        onDragStart={(event) => {
          setDragging({ id: key.id, rowIndex });
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", String(key.id));
        }}
        onDragEnd={() => setDragging(null)}
        className={`group rounded-lg border px-3 py-2 transition-opacity ${isDragging ? "opacity-40" : "opacity-100"} ${
          isSelected ? "border-accent/60 bg-accent/5" : "border-border bg-surface-2"
        }`}
      >
        <div className="mb-1 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <label className="flex h-7 w-7 cursor-pointer items-center justify-center rounded-xl border border-transparent transition-colors hover:border-border/70 hover:bg-surface-2/60">
              <input
                type="checkbox"
                checked={isSelected}
                onChange={() => toggleKeySelection(key.id)}
                aria-label={`Выбрать конфигурацию ${key.label}`}
                className="peer sr-only"
              />
              <span
                className={`flex h-5 w-5 items-center justify-center rounded-md border shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] transition-all peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface-1 ${
                  isSelected
                    ? "border-accent/80 bg-accent/20 text-accent"
                    : "border-border bg-surface-2 text-transparent"
                }`}
              >
                <svg
                  viewBox="0 0 16 16"
                  aria-hidden="true"
                  className={`h-3.5 w-3.5 transition-opacity ${isSelected ? "opacity-100" : "opacity-0"}`}
                >
                  <path
                    d="M4 8.25 6.5 10.75 12 5.25"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                  />
                </svg>
              </span>
            </label>
            <span
              className="text-zinc-500 group-hover:text-zinc-300 cursor-grab active:cursor-grabbing"
              aria-label="Перетащить конфигурацию"
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
            <span className="text-sm font-medium text-zinc-100 truncate"><EmojiText text={key.label} /></span>
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
                {statusParts.join(" · ")}
              </span>
            </div>
            <div className="flex gap-1.5">
              <Button variant="ghost" className="text-xs" onClick={() => handleCheck(key.id)}>
                Проверить
              </Button>
              <Button variant="ghost" className="text-xs" onClick={() => setEditKey(key)}>
                Изменить
              </Button>
              <Button variant="danger" className="text-xs" onClick={() => handleDelete(key.id, key.label)}>
                Удалить
              </Button>
            </div>
          </>
        ) : (
          <>
            <div className="mb-2 text-xs text-zinc-400 truncate"><EmojiText text={key.template_text || key.label} /></div>
            <div className="flex gap-1.5">
              <Button variant="ghost" className="text-xs" onClick={() => setEditKey(key)}>
                Изменить
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
        <div className="flex items-center gap-2">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="h-8 w-8 flex items-center justify-center rounded-lg border border-border bg-surface-2 transition-all hover:bg-surface-1"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Развернуть" : "Свернуть"}
          >
            <span className={`text-zinc-400 transition-transform ${collapsed ? "-rotate-90" : ""}`}>▼</span>
          </button>
          <label className="flex h-9 w-9 cursor-pointer items-center justify-center rounded-xl border border-transparent transition-colors hover:border-border/70 hover:bg-surface-2/60">
            <input
              ref={selectAllRef}
              type="checkbox"
              checked={allSelected}
              onChange={toggleSelectAllKeys}
              aria-label={allSelected ? "Снять выбор со всех конфигураций" : "Выбрать все конфигурации"}
              className="peer sr-only"
            />
            <span
              className={`flex h-5 w-5 items-center justify-center rounded-md border shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] transition-all ${
                allSelected || partiallySelected
                  ? "border-accent/80 bg-accent/20 text-accent"
                  : "border-border bg-surface-2 text-transparent"
              } peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface-1`}
            >
              {partiallySelected ? (
                <span className="h-0.5 w-2.5 rounded-full bg-current" />
              ) : (
                <svg
                  viewBox="0 0 16 16"
                  aria-hidden="true"
                  className={`h-3.5 w-3.5 transition-opacity ${allSelected ? "opacity-100" : "opacity-0"}`}
                >
                  <path
                    d="M4 8.25 6.5 10.75 12 5.25"
                    fill="none"
                    stroke="currentColor"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                  />
                </svg>
              )}
            </span>
          </label>
          <h2 className="text-lg font-semibold"><EmojiText text={`🔐 Ключи (${orderedKeys.length})`} /></h2>
        </div>
        <div className="flex gap-2">
          <Button variant="ghost" className="text-xs" onClick={() => setShowBulkEdit(true)} disabled={selectedCount === 0}>
            Изменить
          </Button>
          {selectedCount > 0 && (
            <Button variant="danger" className="text-xs" onClick={openDeleteSelectedDialog}>
              Удалить ({selectedCount})
            </Button>
          )}
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
          <div className="mb-3 grid text-sm font-medium text-zinc-300" style={gridLayoutStyle}>
            <div
              className={`flex items-center justify-between gap-2 overflow-hidden transition-all duration-300 ${informationalHidden ? "-translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"}`}
            >
              <div>Информационные ключи ({informationalKeys.length})</div>
              {hiddenCategory === null && (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenCategory("informational")}>
                  Скрыть категорию
                </Button>
              )}
              {realHidden && (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenCategory(null)}>
                  Раскрыть категорию
                </Button>
              )}
            </div>
            <div
              className={`flex items-center justify-between gap-2 overflow-hidden transition-all duration-300 ${realHidden ? "translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"}`}
            >
              <div>Конфигурации ({realKeys.length})</div>
              {hiddenCategory === null && (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenCategory("real")}>
                  Скрыть категорию
                </Button>
              )}
              {informationalHidden && (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenCategory(null)}>
                  Раскрыть категорию
                </Button>
              )}
            </div>
          </div>

          <div className="space-y-2">
            {rows.map((row, index) => (
              <React.Fragment key={`row-${index}`}>
                {/* Hover gap button before this row */}
                <InsertGapButton
                  index={index}
                  onInsert={(idx) => {
                    setInsertAtIndex(idx);
                    setShowAddKey(true);
                  }}
                />
              <div className="grid" style={gridLayoutStyle}>
                  <div
                    className={`flex h-full flex-col gap-2 overflow-hidden transition-all duration-300 ${informationalHidden ? "-translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"}`}
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
                        style={
                          hiddenCategory !== null && rowHeights[index]?.informational > 0
                            ? { height: `${Math.max(rowHeights[index].informational, rowHeights[index].real)}px` }
                            : undefined
                        }
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
                    className={`flex h-full flex-col gap-2 overflow-hidden transition-all duration-300 ${realHidden ? "translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"}`}
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
                        style={
                          hiddenCategory !== null && rowHeights[index]?.real > 0
                            ? { height: `${Math.max(rowHeights[index].informational, rowHeights[index].real)}px` }
                            : undefined
                        }
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
              </React.Fragment>
            ))}

            {/* Trailing gap button after last row */}
            {rows.length > 0 && (
              <InsertGapButton
                index={rows.length}
                onInsert={(idx) => {
                  setInsertAtIndex(idx);
                  setShowAddKey(true);
                }}
              />
            )}

            {rows.length === 0 && (
              <div className="py-8 text-center text-zinc-500">Нет конфигураций</div>
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
        title="Удаление конфигурации"
        message={`Вы уверены, что хотите удалить конфигурацию "${deleteTarget?.label}"?`}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
        loading={deleting}
      />

      <ConfirmDialog
        open={!!bulkDeleteTargetIDs && bulkDeleteTargetIDs.length > 0}
        title="Удаление конфигураций"
        message={
          bulkDeleteTargetIDs && bulkDeleteTargetIDs.length > 0
            ? bulkDeleteTargetIDs.length === 1
              ? "Вы уверены, что хотите удалить выбранную конфигурацию?"
              : `Вы уверены, что хотите удалить ${bulkDeleteTargetIDs.length} выбранных конфигураций?`
            : ""
        }
        onConfirm={confirmBulkDelete}
        onCancel={() => setBulkDeleteTargetIDs(null)}
        loading={bulkDeleting}
      />

      <AddKeyModal
        open={showAddKey}
        onClose={() => { setShowAddKey(false); setInsertAtIndex(null); }}
        onRefresh={onRefresh}
        insertAtIndex={insertAtIndex}
        onCreated={() => {
          if (insertAtIndex !== null) {
            // After refresh, reorder to place new key at desired position.
            setTimeout(async () => {
              try {
                const { keys: freshKeys } = await keysApi.list();
                if (freshKeys.length === 0) return;
                // Keys are returned sorted by sort_order (ascending).
                // The newest key is the last one (appended at end by backend).
                const newKey = freshKeys[freshKeys.length - 1];
                // Build the desired order: remove the new key, then splice it in.
                const withoutNew = freshKeys.filter((k: VLESSKey) => k.id !== newKey.id);
                const targetIdx = Math.min(insertAtIndex, withoutNew.length);
                withoutNew.splice(targetIdx, 0, newKey);
                await keysApi.reorder(withoutNew.map((k: VLESSKey) => k.id));
                await onRefresh();
              } catch {
                // Reorder failed — key is still created, just at end.
              }
              setInsertAtIndex(null);
            }, 100);
          }
        }}
      />
      {editKey && (
        <EditKeyModal keyData={editKey} onClose={() => setEditKey(null)} onRefresh={onRefresh} />
      )}
      {showBulkEdit && selectedKeys.length > 0 && (
        <BulkEditKeysModal
          keys={selectedKeys}
          onClose={() => setShowBulkEdit(false)}
          onRefresh={onRefresh}
        />
      )}
    </Card>
  );
}
