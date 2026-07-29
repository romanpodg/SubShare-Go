"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import type { KeyCategory, VLESSKey } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { AddKeyModal } from "./AddKeyModal";
import { BulkEditKeysModal } from "./BulkEditKeysModal";
import { EditKeyModal } from "./EditKeyModal";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { KeyCategoryEditorModal } from "./KeyCategoryEditorModal";
import { EmojiText } from "@/components/ui/EmojiText";
import {
  DEFAULT_KEY_CATEGORY_COLOR,
  alphaHexColor,
  normalizeKeyCategoryColor,
} from "./keyCategoryColors";

const UNCATEGORIZED_LABEL = "Без категории";

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

function normalizeCategory(value: string | null | undefined): string {
  return (value || "").trim();
}

function categoryDisplayName(value: string) {
  return value || UNCATEGORIZED_LABEL;
}

function detectConfigScheme(raw: string) {
  const trimmed = raw.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("{")) return "xray-json";
  if (trimmed.toLowerCase().startsWith("vmess://")) return "vmess";
  try {
    return new URL(trimmed).protocol.replace(":", "").toLowerCase();
  } catch {
    return "";
  }
}

function moveArrayItem<T>(items: T[], fromIndex: number, toIndex: number): T[] {
  if (fromIndex < 0 || fromIndex >= items.length) {
    return items;
  }

  const next = [...items];
  const [item] = next.splice(fromIndex, 1);

  let targetIndex = toIndex;
  if (fromIndex < targetIndex) {
    targetIndex -= 1;
  }
  if (targetIndex < 0) {
    targetIndex = 0;
  }
  if (targetIndex > next.length) {
    targetIndex = next.length;
  }

  next.splice(targetIndex, 0, item);
  return next;
}

interface InsertGapActionsProps {
  index: number;
  categoryHint?: string;
  onAddConfiguration: (index: number, categoryHint?: string) => void;
  onAddCategory: (initialName?: string) => void;
  onDrop: (index: number) => void;
}

// Retained as a local drag-and-drop primitive while category reordering is
// migrated to the dedicated keys page.
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function InsertGapActions({
  index,
  categoryHint,
  onAddConfiguration,
  onAddCategory,
  onDrop,
}: InsertGapActionsProps) {
  const [hovered, setHovered] = useState(false);

  return (
    <div
      className="relative flex items-center justify-center"
      style={{ height: hovered ? "42px" : "8px", transition: "height 160ms ease" }}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      onDragOver={(event) => {
        event.preventDefault();
      }}
      onDrop={(event) => {
        event.preventDefault();
        onDrop(index);
      }}
    >
      {!hovered ? <div className="pointer-events-none h-px w-full bg-zinc-800/70" /> : null}

      <div
        className="absolute inset-0 flex items-center justify-center gap-2 transition-opacity duration-150"
        style={{ opacity: hovered ? 1 : 0, pointerEvents: hovered ? "auto" : "none" }}
      >
        <button
          type="button"
          onClick={() => onAddConfiguration(index, categoryHint)}
          className="rounded-md border border-border bg-zinc-800/80 px-3 py-1 text-[11px] font-medium text-zinc-200 transition hover:bg-zinc-700"
        >
          Добавить конфигурацию
        </button>
        <button
          type="button"
          onClick={() => onAddCategory(categoryHint)}
          className="rounded-md border border-amber-400/40 bg-amber-500/10 px-3 py-1 text-[11px] font-medium text-amber-200 transition hover:bg-amber-500/20"
        >
          Добавить категорию
        </button>
      </div>
    </div>
  );
}

interface Props {
  keys: VLESSKey[];
  subscriptionFormat: "links" | "xray-json";
  onRefresh: () => Promise<void>;
}

interface CategoryGroup {
  category: string;
  keys: VLESSKey[];
}

export function KeysSection({ keys, subscriptionFormat, onRefresh }: Props) {
  const { toast } = useToast();

  const [collapsed, setCollapsed] = useState(false);
  const [hiddenPane, setHiddenPane] = useState<"informational" | "real" | null>(null);
  const [hiddenCategories, setHiddenCategories] = useState<Record<string, boolean>>({});

  const [orderedKeys, setOrderedKeys] = useState<VLESSKey[]>(keys);
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);

  // Search and status filter states
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">("all");

  const [showAddKey, setShowAddKey] = useState(false);
  const [showBulkEdit, setShowBulkEdit] = useState(false);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showCategoryEditor, setShowCategoryEditor] = useState(false);
  const [activeCategoryName, setActiveCategoryName] = useState("");
  const [createCategoryInitial, setCreateCategoryInitial] = useState("");

  const [checkingAll, setCheckingAll] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{ id: number; label: string } | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [bulkDeleteTargetIDs, setBulkDeleteTargetIDs] = useState<number[] | null>(null);
  const [bulkDeleting, setBulkDeleting] = useState(false);

  const [selectedKeyIDs, setSelectedKeyIDs] = useState<number[]>([]);
  const [dragging, setDragging] = useState<{ id: number } | null>(null);
  const [reordering, setReordering] = useState(false);
  const [hasUnsavedOrder, setHasUnsavedOrder] = useState(false);
  const [insertAtIndex, setInsertAtIndex] = useState<number | null>(null);
  const [addKeyCategoryHint, setAddKeyCategoryHint] = useState<string | undefined>(undefined);

  const selectAllRef = useRef<HTMLInputElement | null>(null);

  // Filter keys based on search query and status filter
  const filteredOrderedKeys = useMemo(() => {
    return orderedKeys.filter((key) => {
      const query = searchQuery.toLowerCase().trim();
      if (query) {
        const labelMatch = key.label?.toLowerCase().includes(query);
        const clientNameMatch = key.client_display_name?.toLowerCase().includes(query);
        const urlMatch = key.url?.toLowerCase().includes(query);
        const categoryMatch = key.category?.toLowerCase().includes(query);
        const idMatch = String(key.id).includes(query);
        if (!labelMatch && !clientNameMatch && !urlMatch && !categoryMatch && !idMatch) {
          return false;
        }
      }
      if (statusFilter !== "all") {
        if (key.status !== statusFilter) {
          return false;
        }
      }
      return true;
    });
  }, [orderedKeys, searchQuery, statusFilter]);

  const selectedCount = selectedKeyIDs.length;
  const allSelected = filteredOrderedKeys.length > 0 && filteredOrderedKeys.every((key) => selectedKeyIDs.includes(key.id));
  const partiallySelected = selectedCount > 0 && !allSelected;
  const selectedKeys = filteredOrderedKeys.filter((key) => selectedKeyIDs.includes(key.id));
  const informationalKeysCount = filteredOrderedKeys.filter((key) => key.kind === "informational").length;
  const realKeysCount = filteredOrderedKeys.length - informationalKeysCount;

  useEffect(() => {
    if (!hasUnsavedOrder) {
      setOrderedKeys(keys);
    }
  }, [keys, hasUnsavedOrder]);

  useEffect(() => {
    setSelectedKeyIDs((previous) => previous.filter((id) => orderedKeys.some((key) => key.id === id)));
  }, [orderedKeys]);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = selectedCount > 0 && !allSelected;
  }, [selectedCount, allSelected]);

  const loadKeyCategories = useCallback(async () => {
    try {
      const response = await keysApi.listCategories();
      setKeyCategories(response.categories || []);
    } catch {
      // Categories are auxiliary UI data.
    }
  }, []);

  useEffect(() => {
    void loadKeyCategories();
  }, [loadKeyCategories]);

  const keyIndexByID = useMemo(() => {
    return new Map<number, number>(orderedKeys.map((key, index) => [key.id, index]));
  }, [orderedKeys]);

  const allCategoryNames = useMemo(() => {
    const orderedNames: string[] = [];
    const seen = new Set<string>();

    for (const category of keyCategories) {
      const name = normalizeCategory(category.name);
      if (!name || seen.has(name)) {
        continue;
      }
      seen.add(name);
      orderedNames.push(name);
    }

    for (const key of orderedKeys) {
      const category = normalizeCategory(key.category);
      if (!category || seen.has(category)) {
        continue;
      }
      seen.add(category);
      orderedNames.push(category);
    }

    return orderedNames;
  }, [orderedKeys, keyCategories]);

  useEffect(() => {
    setHiddenCategories((previous) => {
      const next: Record<string, boolean> = {};
      for (const categoryName of allCategoryNames) {
        if (previous[categoryName]) {
          next[categoryName] = true;
        }
      }
      return next;
    });
  }, [allCategoryNames]);

  const categoryCounts = useMemo(() => {
    const counts = new Map<string, number>();
    for (const key of filteredOrderedKeys) {
      const category = normalizeCategory(key.category);
      if (!category) {
        continue;
      }
      counts.set(category, (counts.get(category) || 0) + 1);
    }
    return counts;
  }, [filteredOrderedKeys]);

  const uncategorizedKeys = useMemo(() => {
    return filteredOrderedKeys.filter((key) => !normalizeCategory(key.category));
  }, [filteredOrderedKeys]);

  const uncategorizedCount = uncategorizedKeys.length;

  const categoryMeta = useMemo(() => {
    const map = new Map<string, KeyCategory>();
    for (const category of keyCategories) {
      map.set(normalizeCategory(category.name), category);
    }
    return map;
  }, [keyCategories]);

  const getCategoryColor = useCallback(
    (categoryName: string) => normalizeKeyCategoryColor(categoryMeta.get(categoryName)?.color || DEFAULT_KEY_CATEGORY_COLOR),
    [categoryMeta]
  );

  const categoryGroups = useMemo<CategoryGroup[]>(() => {
    return allCategoryNames.map((categoryName) => ({
      category: categoryName,
      keys: filteredOrderedKeys.filter((key) => normalizeCategory(key.category) === categoryName),
    }));
  }, [allCategoryNames, filteredOrderedKeys]);

  const activeCategoryKeys = useMemo(() => {
    return filteredOrderedKeys.filter((key) => normalizeCategory(key.category) === activeCategoryName);
  }, [filteredOrderedKeys, activeCategoryName]);

  const gridLayoutStyle = {
    gridTemplateColumns:
      hiddenPane === "informational"
        ? "minmax(0, 0fr) minmax(0, 1fr)"
        : hiddenPane === "real"
          ? "minmax(0, 1fr) minmax(0, 0fr)"
          : "minmax(0, 1fr) minmax(0, 1fr)",
    columnGap: hiddenPane ? "0px" : "1.25rem",
    transition: "grid-template-columns 320ms ease, column-gap 320ms ease",
  } as const;

  const informationalHidden = hiddenPane === "informational";
  const realHidden = hiddenPane === "real";

  const moveKeyToIndex = useCallback((keyID: number, targetIndex: number) => {
    setOrderedKeys((previous) => {
      const fromIndex = previous.findIndex((item) => item.id === keyID);
      if (fromIndex === -1) {
        return previous;
      }
      const next = moveArrayItem(previous, fromIndex, targetIndex);
      if (next === previous) {
        return previous;
      }
      return next;
    });
    setHasUnsavedOrder(true);
  }, []);

  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  const dropDraggedAtIndex = useCallback(
    (targetIndex: number) => {
      if (!dragging) {
        return;
      }
      moveKeyToIndex(dragging.id, targetIndex);
      setDragging(null);
    },
    [dragging, moveKeyToIndex]
  );

  const refreshKeysData = useCallback(async () => {
    await onRefresh();
    await loadKeyCategories();
  }, [onRefresh, loadKeyCategories]);

  const handleDelete = (id: number, label: string) => {
    setDeleteTarget({ id, label });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) {
      return;
    }

    setDeleting(true);
    try {
      await keysApi.delete(deleteTarget.id);
      toast("Конфигурация удалена", "success");
      await refreshKeysData();
    } catch {
      toast("Не удалось удалить конфигурацию", "error");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  const toggleKeySelection = (id: number) => {
    setSelectedKeyIDs((previous) =>
      previous.includes(id) ? previous.filter((value) => value !== id) : [...previous, id]
    );
  };

  const toggleSelectAllKeys = () => {
    if (allSelected) {
      setSelectedKeyIDs((previous) => previous.filter((id) => !filteredOrderedKeys.some((key) => key.id === id)));
    } else {
      setSelectedKeyIDs((previous) => {
        const next = [...previous];
        filteredOrderedKeys.forEach((key) => {
          if (!next.includes(key.id)) {
            next.push(key.id);
          }
        });
        return next;
      });
    }
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
      const targetIDs = [...bulkDeleteTargetIDs];
      const response = await keysApi.bulkDelete(targetIDs);
      const deletedCount = response.deleted ?? targetIDs.length;
      toast(
        deletedCount === 1 ? "Конфигурация удалена" : `Удалено конфигураций: ${deletedCount}`,
        "success"
      );
      setSelectedKeyIDs([]);
      await refreshKeysData();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось удалить выбранные конфигурации", "error");
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

  const moveCategoryBlock = async (categoryName: string, direction: -1 | 1) => {
    const currentIndex = allCategoryNames.findIndex((item) => item === categoryName);
    const targetIndex = currentIndex + direction;
    if (currentIndex < 0 || targetIndex < 0 || targetIndex >= allCategoryNames.length) {
      return;
    }

    const nextOrder = [...allCategoryNames];
    const [item] = nextOrder.splice(currentIndex, 1);
    nextOrder.splice(targetIndex, 0, item);

    try {
      await keysApi.reorderCategories(nextOrder);
      toast(`Категория "${categoryName}" перемещена`, "success");
      await refreshKeysData();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось изменить порядок категорий", "error");
    }
  };

  const startAddConfiguration = (index: number, categoryHint?: string) => {
    setInsertAtIndex(index);
    setAddKeyCategoryHint(categoryHint ? normalizeCategory(categoryHint) : undefined);
    setShowAddKey(true);
  };

  const openCreateCategory = (initialName?: string) => {
    setCreateCategoryInitial(initialName?.trim() || "");
    setShowCreateCategory(true);
  };

  const openCategoryEditor = (categoryName: string) => {
    setActiveCategoryName(categoryName);
    setShowCategoryEditor(true);
  };

  const openAddConfigurationForCategory = (categoryName: string) => {
    setShowCategoryEditor(false);
    setInsertAtIndex(null);
    setAddKeyCategoryHint(categoryName);
    setShowAddKey(true);
  };

  const moveKeyToCategory = async (key: VLESSKey, nextCategory: string) => {
    await keysApi.update(key.id, {
      label: key.label,
      status: key.status,
      kind: key.kind,
      category: nextCategory,
      raw_url: key.kind === "real" ? key.url : undefined,
      uuid: "",
      host: "",
      port: "",
      query: "",
      fragment: "",
      template_text: key.kind === "informational" ? key.template_text : undefined,
    });
    await refreshKeysData();
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
        className={`inline-block h-2 w-2 rounded-full ${colors[status] || colors.unknown} ${pulseClass}`}
        title={label}
      />
    );
  };

  const renderKeyCard = (key: VLESSKey) => {
    const isReal = key.kind !== "informational";
    const isInactiveJSON =
      isReal && subscriptionFormat === "links" && detectConfigScheme(key.url) === "xray-json";
    const isDragging = dragging?.id === key.id;
    const isSelected = selectedKeyIDs.includes(key.id);
    const lastChecked = formatDateTime(key.last_checked_at);
    const statusParts = [key.check_status_label];
    const isExternalKey = key.external_source_id > 0;
    const globalIndex = keyIndexByID.get(key.id) ?? 0;

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
          setDragging({ id: key.id });
          event.dataTransfer.effectAllowed = "move";
          event.dataTransfer.setData("text/plain", String(key.id));
        }}
        onDragEnd={() => setDragging(null)}
        onDragOver={(event) => {
          event.preventDefault();
        }}
        onDrop={(event) => {
          event.preventDefault();
          if (!dragging || dragging.id === key.id) {
            return;
          }
          moveKeyToIndex(dragging.id, globalIndex);
          setDragging(null);
        }}
        className={`h-full rounded-lg border px-3 py-2 transition-opacity ${isDragging ? "opacity-40" : "opacity-100"} ${
          isSelected
            ? "border-accent/60 bg-accent/5"
            : isInactiveJSON
              ? "border-zinc-700 bg-zinc-900/25"
              : "border-border bg-surface-2"
        }`}
      >
        <div className="mb-1 flex items-center justify-between gap-2">
          <div className="flex min-w-0 items-center gap-2">
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
              className="cursor-grab text-zinc-500 transition-colors hover:text-zinc-300 active:cursor-grabbing"
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
            <span className={`truncate text-sm font-medium ${isInactiveJSON ? "text-zinc-300" : "text-zinc-100"}`}>
              <EmojiText text={key.label} />
            </span>
          </div>

          <div className="flex items-center gap-2">
            {isExternalKey ? (
              <span className="rounded-full bg-blue-500/15 px-2 py-0.5 text-xs text-blue-300">
                <EmojiText
                  text={key.external_source_name ? `Источник: ${key.external_source_name}` : "Сторонняя подписка"}
                />
              </span>
            ) : null}
            {isInactiveJSON ? (
              <span className="rounded-full bg-zinc-500/20 px-2 py-0.5 text-xs text-zinc-300">
                <EmojiText text="Неактивен JSON" />
              </span>
            ) : (
              <StatusBadge status={key.status} />
            )}
          </div>
        </div>

        {isReal ? (
          <>
            <div className={`mb-2 text-xs ${isInactiveJSON ? "text-zinc-500" : "text-zinc-400"}`}>
              Название в клиенте:{" "}
              <span className="text-zinc-300">
                <EmojiText text={key.client_display_name || key.label} />
              </span>
            </div>
            <div className="mb-2 flex items-center gap-2">
              {healthDot(key.check_status, key.check_status_label)}
              <span className={`text-xs ${isInactiveJSON ? "text-zinc-500" : "text-zinc-400"}`}>
                {isInactiveJSON
                  ? "JSON-конфиг отключен для ссылочного формата подписки"
                  : statusParts.join(" · ")}
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
            <div className="mb-2 text-xs text-zinc-400">
              <EmojiText text={key.template_text || key.label} />
            </div>
            <div className="mb-2 text-[11px] text-zinc-500">
              Категория: <span className="text-zinc-300">{categoryDisplayName(normalizeCategory(key.category))}</span>
            </div>
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

  const renderKeyRows = (sectionKeys: VLESSKey[], categoryHint?: string) => {
    const infoKeys = sectionKeys.filter((key) => key.kind === "informational");
    const realKeys = sectionKeys.filter((key) => key.kind !== "informational");

    if (sectionKeys.length === 0) {
      return null;
    }

    return (
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Информационная колонка */}
        <div 
          className={`space-y-2 transition-all duration-300 ${informationalHidden ? "hidden" : "block"}`}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault();
            if (dragging) {
              const lastInfo = infoKeys[infoKeys.length - 1];
              const targetIndex = lastInfo ? (keyIndexByID.get(lastInfo.id) ?? 0) + 1 : 0;
              moveKeyToIndex(dragging.id, targetIndex);
              setDragging(null);
            }
          }}
        >
          <div className="flex items-center justify-between border-b border-border/40 pb-1 mb-2">
            <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Информационные сообщения ({infoKeys.length})</span>
            <button
              onClick={() => startAddConfiguration(orderedKeys.length, categoryHint)}
              className="text-[10px] text-accent hover:underline cursor-pointer"
            >
              + Добавить
            </button>
          </div>
          {infoKeys.map((key) => renderKeyCard(key))}
          {infoKeys.length === 0 && (
            <div className="border border-dashed border-border/50 rounded-lg p-4 text-center text-xs text-zinc-500">
              Нет сообщений
            </div>
          )}
        </div>

        {/* Колонка конфигураций */}
        <div 
          className={`space-y-2 transition-all duration-300 ${realHidden ? "hidden" : "block"}`}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault();
            if (dragging) {
              const lastReal = realKeys[realKeys.length - 1];
              const targetIndex = lastReal ? (keyIndexByID.get(lastReal.id) ?? 0) + 1 : orderedKeys.length;
              moveKeyToIndex(dragging.id, targetIndex);
              setDragging(null);
            }
          }}
        >
          <div className="flex items-center justify-between border-b border-border/40 pb-1 mb-2">
            <span className="text-xs font-semibold text-zinc-400 uppercase tracking-wider">Рабочие конфигурации ({realKeys.length})</span>
            <button
              onClick={() => startAddConfiguration(orderedKeys.length, categoryHint)}
              className="text-[10px] text-accent hover:underline cursor-pointer"
            >
              + Добавить
            </button>
          </div>
          {realKeys.map((key) => renderKeyCard(key))}
          {realKeys.length === 0 && (
            <div className="border border-dashed border-border/50 rounded-lg p-4 text-center text-xs text-zinc-500">
              Нет конфигураций
            </div>
          )}
        </div>
      </div>
    );
  };

  const renderUncategorizedSection = () => {
    if (uncategorizedCount === 0) {
      return null;
    }

    return (
      <div className="rounded-2xl border border-border bg-surface-2/20 p-3">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <span className="rounded-full border border-zinc-700 bg-zinc-900/45 px-2.5 py-1 text-xs font-medium text-zinc-300">
              {UNCATEGORIZED_LABEL} ({uncategorizedCount})
            </span>
            <span className="text-xs text-zinc-500">
              Инфо: {uncategorizedKeys.filter((key) => key.kind === "informational").length} · Конфигурации:{" "}
              {uncategorizedKeys.filter((key) => key.kind !== "informational").length}
            </span>
          </div>
        </div>
        {renderKeyRows(uncategorizedKeys)}
      </div>
    );
  };

  const renderCategoryBlock = (categoryName: string, categoryKeys: VLESSKey[], empty?: boolean) => {
    const hidden = Boolean(hiddenCategories[categoryName]);
    const count = categoryCounts.get(categoryName) || 0;
    const categoryColor = getCategoryColor(categoryName);
    const categoryIndex = allCategoryNames.findIndex((item) => item === categoryName);

    return (
      <div
        key={`${categoryName}-${empty ? "empty" : categoryKeys[0]?.id ?? "run"}`}
        className="grid gap-4 md:grid-cols-[minmax(0,1fr)_68px]"
      >
        <div
          className="min-w-0 rounded-2xl border p-3"
          style={{
            borderColor: alphaHexColor(categoryColor, "33"),
            backgroundColor: alphaHexColor(categoryColor, "08"),
          }}
        >
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className="rounded-full border px-2.5 py-1 text-xs font-medium"
                style={{
                  borderColor: alphaHexColor(categoryColor, "66"),
                  backgroundColor: alphaHexColor(categoryColor, "18"),
                  color: categoryColor,
                }}
              >
                {categoryName} ({count})
              </span>
              <span className="text-xs text-zinc-500">
                Инфо: {categoryKeys.filter((key) => key.kind === "informational").length} · Конфигурации:{" "}
                {categoryKeys.filter((key) => key.kind !== "informational").length}
              </span>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="ghost"
                className="text-xs"
                onClick={() => void moveCategoryBlock(categoryName, -1)}
                disabled={categoryIndex <= 0}
              >
                Вверх
              </Button>
              <Button
                variant="ghost"
                className="text-xs"
                onClick={() => void moveCategoryBlock(categoryName, 1)}
                disabled={categoryIndex === -1 || categoryIndex >= allCategoryNames.length - 1}
              >
                Вниз
              </Button>
              <Button variant="ghost" className="text-xs" onClick={() => openCategoryEditor(categoryName)}>
                Редактировать категорию
              </Button>
              <Button
                variant="ghost"
                className="text-xs"
                onClick={() =>
                  setHiddenCategories((previous) => ({
                    ...previous,
                    [categoryName]: !previous[categoryName],
                  }))
                }
              >
                {hidden ? "Показать категорию" : "Скрыть категорию"}
              </Button>
            </div>
          </div>

          {hidden ? (
            <div className="rounded-lg border border-zinc-700 bg-zinc-900/30 px-3 py-3 text-sm text-zinc-400">
              Категория скрыта. Конфигураций внутри: {count}.
            </div>
          ) : empty ? (
            <div className="rounded-lg border border-dashed border-border bg-surface-2/20 px-4 py-8 text-center text-sm text-zinc-500">
              Категория создана, но в ней пока нет ключей.
            </div>
          ) : (
            renderKeyRows(categoryKeys, categoryName)
          )}
        </div>

        <div className="relative hidden md:block">
          <div
            className="absolute inset-y-0 left-1/2 w-px -translate-x-1/2 rounded-full"
            style={{ backgroundColor: alphaHexColor(categoryColor, "73") }}
          />
          <div className="sticky top-28 flex justify-center">
            <span
              className="inline-flex rounded-xl px-2 py-3 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-900 shadow-[0_8px_24px_rgba(0,0,0,0.35)]"
              style={{ backgroundColor: categoryColor, writingMode: "vertical-rl", textOrientation: "mixed" }}
            >
              {categoryName}
            </span>
          </div>
        </div>
      </div>
    );
  };

  return (
    <Card>
      <div className="mb-4 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="flex h-8 w-8 items-center justify-center rounded-lg border border-border bg-surface-2 transition-all hover:bg-surface-1"
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
          <h2 className="text-lg font-semibold">
            <EmojiText text={`🔐 Ключи (${orderedKeys.length})`} />
          </h2>
        </div>

        <div className="flex gap-2">
          <Link
            href="/admin/sources"
            className="rounded-lg border border-transparent bg-transparent px-4 py-2 text-xs font-medium text-[var(--text-main)] transition-colors hover:bg-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          >
            Внешние источники
          </Link>
          <Button
            variant="ghost"
            className="text-xs"
            onClick={() => setShowBulkEdit(true)}
            disabled={selectedCount === 0}
          >
            Изменить
          </Button>
          {selectedCount > 0 ? (
            <Button variant="danger" className="text-xs" onClick={openDeleteSelectedDialog}>
              Удалить ({selectedCount})
            </Button>
          ) : null}
          <Button variant="ghost" onClick={handleCheckAll} loading={checkingAll} className="text-xs">
            Проверить все
          </Button>
          <Button variant="ghost" className="text-xs" onClick={() => openCreateCategory()}>
            Добавить категорию
          </Button>
          <Button onClick={() => startAddConfiguration(orderedKeys.length)} className="text-xs">
            + Добавить конфигурацию
          </Button>
        </div>
      </div>

      {/* Панель поиска и фильтров */}
      {!collapsed && (
        <div className="flex flex-col md:flex-row gap-3 mb-4 p-3 bg-surface-2/30 rounded-xl border border-border">
          <div className="flex-1 relative flex items-center">
            <span className="absolute left-3 text-zinc-500">🔍</span>
            <input
              type="text"
              placeholder="Поиск по названию, категории, URL, ID..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full h-9 bg-surface-2 border border-border rounded-lg pl-9 pr-8 text-sm focus:outline-none focus:border-accent text-zinc-100 placeholder-zinc-500 transition-colors"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 text-xs w-5 h-5 flex items-center justify-center rounded-full hover:bg-surface-1"
              >
                ✕
              </button>
            )}
          </div>
          <div className="flex gap-2">
            <select
              value={statusFilter}
              onChange={(e) =>
                setStatusFilter(e.target.value as "all" | "active" | "inactive")
              }
              className="h-9 bg-surface-2 border border-border rounded-lg px-3 text-sm focus:outline-none focus:border-accent text-zinc-300 cursor-pointer"
            >
              <option value="all">Все статусы</option>
              <option value="active">Активен</option>
              <option value="inactive">Неактивен</option>
            </select>
          </div>
        </div>
      )}

      {!collapsed ? (
        <div className="space-y-4">
          <div className="grid text-sm font-medium text-zinc-300" style={gridLayoutStyle}>
            <div
              className={`flex items-center justify-between gap-2 overflow-hidden transition-all duration-300 ${
                informationalHidden ? "-translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"
              }`}
            >
              <div>Информационные ключи ({informationalKeysCount})</div>
              {hiddenPane === null ? (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenPane("informational")}>
                  Скрыть колонку
                </Button>
              ) : null}
              {realHidden ? (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenPane(null)}>
                  Показать колонку
                </Button>
              ) : null}
            </div>

            <div
              className={`flex items-center justify-between gap-2 overflow-hidden transition-all duration-300 ${
                realHidden ? "translate-x-10 opacity-0 pointer-events-none" : "translate-x-0 opacity-100"
              }`}
            >
              <div>Конфигурации ({realKeysCount})</div>
              {hiddenPane === null ? (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenPane("real")}>
                  Скрыть колонку
                </Button>
              ) : null}
              {informationalHidden ? (
                <Button variant="ghost" className="text-xs" onClick={() => setHiddenPane(null)}>
                  Показать колонку
                </Button>
              ) : null}
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            {uncategorizedCount > 0 ? (
              <span className="rounded-full border border-zinc-700 bg-zinc-900/45 px-2.5 py-1 text-xs text-zinc-400">
                {UNCATEGORIZED_LABEL} ({uncategorizedCount})
              </span>
            ) : null}
            {allCategoryNames.map((categoryName) => {
              const hidden = Boolean(hiddenCategories[categoryName]);
              const count = categoryCounts.get(categoryName) || 0;

              return (
                <button
                  key={categoryName}
                  type="button"
                  onClick={() =>
                    setHiddenCategories((previous) => ({
                      ...previous,
                      [categoryName]: !previous[categoryName],
                    }))
                  }
                  className={`rounded-full border px-2.5 py-1 text-xs transition ${
                    hidden ? "border-zinc-700 bg-zinc-900/45 text-zinc-400" : ""
                  }`}
                  style={
                    hidden
                      ? undefined
                      : {
                          borderColor: alphaHexColor(getCategoryColor(categoryName), "66"),
                          backgroundColor: alphaHexColor(getCategoryColor(categoryName), "18"),
                          color: getCategoryColor(categoryName),
                        }
                  }
                  title={hidden ? "Показать категорию" : "Скрыть категорию"}
                >
                  {categoryName} ({count})
                </button>
              );
            })}
          </div>

          {renderUncategorizedSection()}
          {categoryGroups.map((group) => renderCategoryBlock(group.category, group.keys, group.keys.length === 0))}

          {orderedKeys.length === 0 && categoryGroups.length === 0 ? (
            <div className="rounded-lg border border-border bg-surface-2/30 px-3 py-8 text-center text-zinc-500">
              Нет конфигураций
            </div>
          ) : null}

          {orderedKeys.length > 0 ? (
            <div className="flex justify-end pt-1">
              <Button
                onClick={() => void saveOrder()}
                loading={reordering}
                disabled={!hasUnsavedOrder || reordering}
                className="text-xs"
              >
                Сохранить порядок
              </Button>
            </div>
          ) : null}
        </div>
      ) : null}

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
        onClose={() => {
          setShowAddKey(false);
          setInsertAtIndex(null);
          setAddKeyCategoryHint(undefined);
        }}
        onRefresh={refreshKeysData}
        insertAtIndex={insertAtIndex}
        initialCategory={addKeyCategoryHint}
        onCreated={() => {
          if (insertAtIndex !== null) {
            setTimeout(async () => {
              try {
                const { keys: freshKeys } = await keysApi.list();
                if (freshKeys.length === 0) {
                  return;
                }
                const newKey = freshKeys[freshKeys.length - 1];
                const withoutNew = freshKeys.filter((item: VLESSKey) => item.id !== newKey.id);
                const targetIndex = Math.min(insertAtIndex, withoutNew.length);
                withoutNew.splice(targetIndex, 0, newKey);
                await keysApi.reorder(withoutNew.map((item: VLESSKey) => item.id));
                await refreshKeysData();
              } catch {
                // The key is still created even if reorder fails.
              }
              setInsertAtIndex(null);
              setAddKeyCategoryHint(undefined);
            }, 100);
          }
        }}
      />

      {editKey ? (
        <EditKeyModal
          keyData={editKey}
          onClose={() => setEditKey(null)}
          onRefresh={refreshKeysData}
        />
      ) : null}

      {showBulkEdit && selectedKeys.length > 0 ? (
        <BulkEditKeysModal keys={selectedKeys} onClose={() => setShowBulkEdit(false)} onRefresh={refreshKeysData} />
      ) : null}

      <CreateKeyCategoryModal
        open={showCreateCategory}
        onClose={() => setShowCreateCategory(false)}
        initialValue={createCategoryInitial}
        onCreated={async (category) => {
          await loadKeyCategories();
          setActiveCategoryName(category.name);
          setShowCategoryEditor(true);
          setHiddenCategories((previous) => ({
            ...previous,
            [category.name]: false,
          }));
        }}
      />

      <KeyCategoryEditorModal
        open={showCategoryEditor}
        categoryName={activeCategoryName}
        categoryKeys={activeCategoryKeys}
        categories={keyCategories}
        onClose={() => setShowCategoryEditor(false)}
        onUpdated={async (nextCategory, previousName) => {
          setActiveCategoryName(nextCategory.name);
          setHiddenCategories((previous) => {
            const next = { ...previous };
            if (previousName !== nextCategory.name && previous[previousName]) {
              next[nextCategory.name] = previous[previousName];
              delete next[previousName];
            }
            return next;
          });
          await refreshKeysData();
        }}
        onDeleted={async () => {
          setShowCategoryEditor(false);
          setActiveCategoryName("");
          await refreshKeysData();
        }}
        onOpenAddConfiguration={openAddConfigurationForCategory}
        onEditKey={(key) => {
          setShowCategoryEditor(false);
          setEditKey(key);
        }}
        onDeleteKey={(key) => {
          handleDelete(key.id, key.label);
        }}
        onMoveKeyToCategory={moveKeyToCategory}
      />
    </Card>
  );
}
