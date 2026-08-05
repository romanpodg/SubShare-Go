"use client";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import type { KeyCategory, KeySummary } from "@/lib/types";
import { keys as keysApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Select } from "@/components/ui/Select";
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
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Eye,
  EyeOff,
  FolderPlus,
  KeyRound,
  Pencil,
  Plus,
  RadioTower,
  RefreshCw,
  Search,
  Trash2,
  X,
} from "lucide-react";

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

// eslint-disable-next-line @typescript-eslint/no-unused-vars
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

const DRAG_AUTO_SCROLL_EDGE_PX = 120;
const DRAG_AUTO_SCROLL_MAX_PX = 28;

export function dragAutoScrollVelocity(
  pointerY: number,
  top: number,
  bottom: number,
  edgeSize = DRAG_AUTO_SCROLL_EDGE_PX,
  maxVelocity = DRAG_AUTO_SCROLL_MAX_PX
): number {
  const height = bottom - top;
  if (!Number.isFinite(pointerY) || height <= 0 || edgeSize <= 0 || maxVelocity <= 0) {
    return 0;
  }

  const edge = Math.min(edgeSize, height / 2);
  const upperBoundary = top + edge;
  const lowerBoundary = bottom - edge;

  if (pointerY < upperBoundary) {
    const proximity = Math.min(1, Math.max(0, (upperBoundary - pointerY) / edge));
    return proximity === 0 ? 0 : -Math.max(2, Math.round(maxVelocity * proximity * proximity));
  }
  if (pointerY > lowerBoundary) {
    const proximity = Math.min(1, Math.max(0, (pointerY - lowerBoundary) / edge));
    return proximity === 0 ? 0 : Math.max(2, Math.round(maxVelocity * proximity * proximity));
  }
  return 0;
}

function dragScrollTargetAt(clientX: number, clientY: number): { target: HTMLElement; velocity: number } | null {
  const candidates: HTMLElement[] = [];
  let element = document.elementFromPoint(clientX, clientY);

  while (element instanceof HTMLElement) {
    const style = window.getComputedStyle(element);
    if (
      /(auto|scroll)/.test(style.overflowY) &&
      element.scrollHeight > element.clientHeight + 1
    ) {
      candidates.push(element);
    }
    element = element.parentElement;
  }

  const root = document.scrollingElement;
  if (root instanceof HTMLElement && !candidates.includes(root)) {
    candidates.push(root);
  }

  for (const target of candidates) {
    const isRoot = target === document.scrollingElement;
    const rect = isRoot
      ? { top: 0, bottom: window.innerHeight }
      : target.getBoundingClientRect();
    const velocity = dragAutoScrollVelocity(clientY, rect.top, rect.bottom);
    const canScrollUp = target.scrollTop > 0;
    const canScrollDown = target.scrollTop + target.clientHeight < target.scrollHeight - 1;

    if ((velocity < 0 && canScrollUp) || (velocity > 0 && canScrollDown)) {
      return { target, velocity };
    }
  }

  return null;
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
  keys: KeySummary[];
  subscriptionFormat: "links" | "xray-json";
  onRefresh: () => Promise<void>;
}

interface CategoryGroup {
  category: string;
  keys: KeySummary[];
}

export interface AlignedKeyRow {
  key: KeySummary;
  informational: KeySummary | null;
  real: KeySummary | null;
}

export function alignKeysBySubscriptionOrder(keys: KeySummary[]): AlignedKeyRow[] {
  return keys.map((key) => ({
    key,
    informational: key.kind === "informational" ? key : null,
    real: key.kind === "informational" ? null : key,
  }));
}

export function KeysSection({ keys, subscriptionFormat, onRefresh }: Props) {
  const { toast } = useToast();

  const [collapsed, setCollapsed] = useState(false);
  const [hiddenPane, setHiddenPane] = useState<"informational" | "real" | null>(null);
  const [hiddenCategories, setHiddenCategories] = useState<Record<string, boolean>>({});

  const [orderedKeys, setOrderedKeys] = useState<KeySummary[]>(keys);
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);

  // Search and status filter states
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">("all");

  const [showAddKey, setShowAddKey] = useState(false);
  const [showBulkEdit, setShowBulkEdit] = useState(false);
  const [editKey, setEditKey] = useState<KeySummary | null>(null);
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
  const [addKeyKind, setAddKeyKind] = useState<"real" | "informational">("real");

  const selectAllRef = useRef<HTMLInputElement | null>(null);
  const uncategorizedBlockRef = useRef<HTMLDivElement | null>(null);
  const categoryBlockRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const dragAutoScrollRef = useRef<{
    frame: number | null;
    target: HTMLElement | null;
    velocity: number;
  }>({ frame: null, target: null, velocity: 0 });

  const stopDragAutoScroll = useCallback(() => {
    const state = dragAutoScrollRef.current;
    if (state.frame !== null) {
      window.cancelAnimationFrame(state.frame);
    }
    state.frame = null;
    state.target = null;
    state.velocity = 0;
  }, []);

  const updateDragAutoScroll = useCallback(
    (clientX: number, clientY: number) => {
      const decision = dragScrollTargetAt(clientX, clientY);
      if (!decision) {
        stopDragAutoScroll();
        return;
      }

      const state = dragAutoScrollRef.current;
      state.target = decision.target;
      state.velocity = decision.velocity;
      if (state.frame !== null) {
        return;
      }

      const scrollFrame = () => {
        const current = dragAutoScrollRef.current;
        if (!current.target || current.velocity === 0) {
          current.frame = null;
          return;
        }

        const before = current.target.scrollTop;
        current.target.scrollTop += current.velocity;
        if (current.target.scrollTop === before) {
          current.frame = null;
          current.target = null;
          current.velocity = 0;
          return;
        }
        current.frame = window.requestAnimationFrame(scrollFrame);
      };

      state.frame = window.requestAnimationFrame(scrollFrame);
    },
    [stopDragAutoScroll]
  );

  // Filter keys based on search query and status filter
  const filteredOrderedKeys = useMemo(() => {
    return orderedKeys.filter((key) => {
      const query = searchQuery.toLowerCase().trim();
      if (query) {
        const labelMatch = key.label?.toLowerCase().includes(query);
        const categoryMatch = key.category?.toLowerCase().includes(query);
        const sourceMatch = key.external_source_name?.toLowerCase().includes(query);
        const protocolMatch = key.protocol?.toLowerCase().includes(query);
        const idMatch = String(key.id).includes(query);
        if (!labelMatch && !categoryMatch && !sourceMatch && !protocolMatch && !idMatch) {
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

  useEffect(() => {
    if (!dragging) {
      stopDragAutoScroll();
      return;
    }

    const handleDragOver = (event: DragEvent) => {
      updateDragAutoScroll(event.clientX, event.clientY);
    };
    const handleDragFinished = () => {
      stopDragAutoScroll();
    };

    document.addEventListener("dragover", handleDragOver, true);
    document.addEventListener("drop", handleDragFinished, true);
    document.addEventListener("dragend", handleDragFinished, true);

    return () => {
      document.removeEventListener("dragover", handleDragOver, true);
      document.removeEventListener("drop", handleDragFinished, true);
      document.removeEventListener("dragend", handleDragFinished, true);
      stopDragAutoScroll();
    };
  }, [dragging, stopDragAutoScroll, updateDragAutoScroll]);

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

  const scrollToCategory = useCallback((categoryName: string | null) => {
    const target = categoryName
      ? categoryBlockRefs.current.get(categoryName)
      : uncategorizedBlockRef.current;
    if (!target) {
      return;
    }

    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    target.scrollIntoView({
      behavior: reduceMotion ? "auto" : "smooth",
      block: "start",
    });
    window.requestAnimationFrame(() => target.focus({ preventScroll: true }));
  }, []);

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

  const startAddConfiguration = (
    index: number,
    categoryHint?: string,
    kind: "real" | "informational" = "real"
  ) => {
    setInsertAtIndex(index);
    setAddKeyCategoryHint(categoryHint ? normalizeCategory(categoryHint) : undefined);
    setAddKeyKind(kind);
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

  const moveKeyToCategory = async (key: KeySummary, nextCategory: string) => {
    await keysApi.update(key.id, {
      label: key.label,
      status: key.status,
      kind: key.kind,
      category: nextCategory,
      uuid: "",
      host: "",
      port: "",
      query: "",
      fragment: "",
      template_text: key.kind === "informational" ? key.template_text : undefined,
    });
    await refreshKeysData();
  };

  function checkStatusLabel(status: string): string {
    switch (status) {
      case "up":
        return "Доступен";
      case "down":
        return "Недоступен";
      default:
        return "Не проверен";
    }
  }

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

  const renderKeyCard = (key: KeySummary) => {
    const isReal = key.kind !== "informational";
    const isInactiveJSON =
      isReal && subscriptionFormat === "links" && (key.protocol === "vless" || key.protocol === "vmess" || key.protocol === "trojan" || key.protocol === "legacy");
    const isDragging = dragging?.id === key.id;
    const isSelected = selectedKeyIDs.includes(key.id);
    const lastChecked = formatDateTime(key.last_checked_at);
    const statusParts = [checkStatusLabel(key.check_status)];
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
          event.stopPropagation();
          if (!dragging || dragging.id === key.id) {
            return;
          }
          moveKeyToIndex(dragging.id, globalIndex);
          setDragging(null);
        }}
        className={`ui-key-card ${isDragging ? "opacity-40" : "opacity-100"} ${
          isSelected
            ? "!border-accent/60 !bg-accent/5"
            : isInactiveJSON
              ? "!border-zinc-700 !bg-zinc-900/25"
              : ""
        }`}
      >
        <div className="mb-2 flex min-w-0 items-center gap-2">
          <label className="flex h-9 w-9 shrink-0 cursor-pointer items-center justify-center rounded-sm border border-border bg-surface-1 transition-colors hover:border-border/70 hover:bg-surface-2/60">
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
            className="shrink-0 cursor-grab text-zinc-500 transition-colors hover:text-zinc-300 active:cursor-grabbing"
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
          <span className="shrink-0 font-mono text-[12px] text-zinc-500">ID {key.id}</span>
          <EmojiText
            text={key.label}
            truncate
            className={`ui-key-card-title min-w-0 flex-1 text-[15px] font-semibold leading-5 ${
              isInactiveJSON ? "text-zinc-300" : "text-zinc-100"
            }`}
          />
          <div className="flex shrink-0 items-center">
            {isInactiveJSON ? (
              <span className="border border-border bg-surface-1 px-2.5 py-1 text-[12px] text-zinc-300">
                <EmojiText text="Неактивен JSON" />
              </span>
            ) : (
              <StatusBadge status={key.status} />
            )}
          </div>
        </div>

        {isExternalKey ? (
          <div className="mb-2 min-w-0">
            <span className="inline-flex max-w-full min-w-0 border border-border bg-surface-1 px-2.5 py-1 text-[12px] text-zinc-300">
              <EmojiText
                text={key.external_source_name ? `Источник: ${key.external_source_name}` : "Сторонняя подписка"}
                truncate
                className="ui-key-card-source min-w-0"
              />
            </span>
          </div>
        ) : null}

        {isReal ? (
          <>
            <div className={`mb-2 flex min-w-0 items-baseline gap-1 text-sm leading-5 ${isInactiveJSON ? "text-zinc-500" : "text-zinc-400"}`}>
              <span className="shrink-0">Название в клиенте:</span>
              <EmojiText
                text={key.label}
                truncate
                className="ui-key-card-client-name min-w-0 flex-1 text-zinc-300"
              />
            </div>
            <div className="mb-2 flex items-center gap-2">
              {healthDot(key.check_status, checkStatusLabel(key.check_status))}
              <span className={`text-[13px] leading-5 ${isInactiveJSON ? "text-zinc-500" : "text-zinc-400"}`}>
                {isInactiveJSON
                  ? "JSON-конфиг отключен для ссылочного формата подписки"
                  : statusParts.join(" · ")}
              </span>
            </div>
            <div className="flex flex-nowrap gap-1.5 overflow-x-auto">
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
            <div className="mb-2 text-sm leading-5 text-zinc-400">
              <EmojiText text={key.template_text || key.label} />
            </div>
            <div className="mb-2 text-[13px] text-zinc-500">
              Категория: <span className="text-zinc-300">{categoryDisplayName(normalizeCategory(key.category))}</span>
            </div>
            <div className="flex flex-nowrap gap-1.5 overflow-x-auto">
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

  const renderKeyRows = (sectionKeys: KeySummary[], categoryHint?: string) => {
    const alignedRows = alignKeysBySubscriptionOrder(sectionKeys);
    const infoKeys = alignedRows.map((row) => row.informational);
    const realKeys = alignedRows.map((row) => row.real);
    const infoKeysCount = infoKeys.filter((key) => key !== null).length;
    const realKeysCount = realKeys.filter((key) => key !== null).length;

    if (sectionKeys.length === 0) {
      return null;
    }

    return (
      <div
        className={`ui-key-order-grid ${
          informationalHidden ? "ui-key-order-grid-info-hidden" : realHidden ? "ui-key-order-grid-real-hidden" : ""
        }`}
      >
        {/* Информационная колонка */}
        <div 
          className={`ui-key-order-column ui-key-order-column-info ${informationalHidden ? "hidden" : ""}`}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault();
            if (dragging) {
              const lastKey = sectionKeys[sectionKeys.length - 1];
              const targetIndex = lastKey ? (keyIndexByID.get(lastKey.id) ?? 0) + 1 : orderedKeys.length;
              moveKeyToIndex(dragging.id, targetIndex);
              setDragging(null);
            }
          }}
        >
          <div className="ui-key-column-title mb-2">
            <span>Информационные сообщения ({infoKeysCount})</span>
            <button
              onClick={() => startAddConfiguration(orderedKeys.length, categoryHint, "informational")}
              className="inline-flex h-9 min-h-9 -translate-y-0.5 shrink-0 items-center gap-1.5 border border-border bg-surface-1 px-3 font-mono text-[11px] font-semibold uppercase tracking-[0.08em] text-accent transition-colors hover:border-[var(--border-strong)] hover:bg-surface-2"
            >
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              Добавить
            </button>
          </div>
          {alignedRows.map((row, rowIndex) => (
            <div
              key={`informational-slot-${row.key.id}`}
              className={`ui-key-order-slot ${row.informational ? "" : "ui-key-order-placeholder-slot"}`}
              style={{ gridRow: rowIndex + 2 }}
              data-order-position={(keyIndexByID.get(row.key.id) ?? rowIndex) + 1}
            >
              {row.informational ? (
                renderKeyCard(row.informational)
              ) : (
                <div
                  className="ui-key-order-gap"
                  aria-hidden="true"
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    if (!dragging || dragging.id === row.key.id) {
                      return;
                    }
                    moveKeyToIndex(dragging.id, keyIndexByID.get(row.key.id) ?? rowIndex);
                    setDragging(null);
                  }}
                />
              )}
            </div>
          ))}
        </div>

        {/* Колонка конфигураций */}
        <div 
          className={`ui-key-order-column ui-key-order-column-real ${realHidden ? "hidden" : ""}`}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => {
            event.preventDefault();
            if (dragging) {
              const lastKey = sectionKeys[sectionKeys.length - 1];
              const targetIndex = lastKey ? (keyIndexByID.get(lastKey.id) ?? 0) + 1 : orderedKeys.length;
              moveKeyToIndex(dragging.id, targetIndex);
              setDragging(null);
            }
          }}
        >
          <div className="ui-key-column-title mb-2">
            <span>Рабочие конфигурации ({realKeysCount})</span>
            <button
              onClick={() => startAddConfiguration(orderedKeys.length, categoryHint, "real")}
              className="inline-flex h-9 min-h-9 -translate-y-0.5 shrink-0 items-center gap-1.5 border border-border bg-surface-1 px-3 font-mono text-[11px] font-semibold uppercase tracking-[0.08em] text-accent transition-colors hover:border-[var(--border-strong)] hover:bg-surface-2"
            >
              <Plus className="h-3.5 w-3.5" aria-hidden="true" />
              Добавить
            </button>
          </div>
          {alignedRows.map((row, rowIndex) => (
            <div
              key={`real-slot-${row.key.id}`}
              className={`ui-key-order-slot ${row.real ? "" : "ui-key-order-placeholder-slot"}`}
              style={{ gridRow: rowIndex + 2 }}
              data-order-position={(keyIndexByID.get(row.key.id) ?? rowIndex) + 1}
            >
              {row.real ? (
                renderKeyCard(row.real)
              ) : (
                <div
                  className="ui-key-order-gap"
                  aria-hidden="true"
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    if (!dragging || dragging.id === row.key.id) {
                      return;
                    }
                    moveKeyToIndex(dragging.id, keyIndexByID.get(row.key.id) ?? rowIndex);
                    setDragging(null);
                  }}
                />
              )}
            </div>
          ))}
        </div>
      </div>
    );
  };

  const renderUncategorizedSection = () => {
    if (uncategorizedCount === 0) {
      return null;
    }

    return (
      <div
        ref={uncategorizedBlockRef}
        tabIndex={-1}
        data-category-block="uncategorized"
        className="scroll-mt-24 rounded-2xl border border-border bg-surface-2/20 p-3 focus:outline-none"
      >
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

  const renderCategoryBlock = (categoryName: string, categoryKeys: KeySummary[], empty?: boolean) => {
    const hidden = Boolean(hiddenCategories[categoryName]);
    const count = categoryCounts.get(categoryName) || 0;
    const categoryColor = getCategoryColor(categoryName);
    const categoryIndex = allCategoryNames.findIndex((item) => item === categoryName);

    return (
      <div
        key={`${categoryName}-${empty ? "empty" : categoryKeys[0]?.id ?? "run"}`}
        ref={(node) => {
          if (node) {
            categoryBlockRefs.current.set(categoryName, node);
          } else {
            categoryBlockRefs.current.delete(categoryName);
          }
        }}
        tabIndex={-1}
        data-category-block={categoryName}
        className="ui-key-category-block relative scroll-mt-24 focus:outline-none"
      >
        <div
          className="ui-key-category-panel min-w-0 rounded-2xl border p-3"
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

            <div className="ui-key-category-controls flex flex-wrap items-center gap-1">
              <Button
                variant="ghost"
                className="ui-key-category-icon w-11 px-0"
                onClick={() => void moveCategoryBlock(categoryName, -1)}
                disabled={categoryIndex <= 0}
                aria-label={`Переместить категорию «${categoryName}» вверх`}
                title="Переместить вверх"
              >
                <ArrowUp className="h-4 w-4" aria-hidden="true" />
              </Button>
              <Button
                variant="ghost"
                className="ui-key-category-icon w-11 px-0"
                onClick={() => void moveCategoryBlock(categoryName, 1)}
                disabled={categoryIndex === -1 || categoryIndex >= allCategoryNames.length - 1}
                aria-label={`Переместить категорию «${categoryName}» вниз`}
                title="Переместить вниз"
              >
                <ArrowDown className="h-4 w-4" aria-hidden="true" />
              </Button>
              <Button
                variant="ghost"
                className="ui-key-category-icon w-11 px-0"
                onClick={() => openCategoryEditor(categoryName)}
                aria-label={`Редактировать категорию «${categoryName}»`}
                title="Редактировать категорию"
              >
                <Pencil className="h-4 w-4" aria-hidden="true" />
              </Button>
              <Button
                variant="ghost"
                className="ui-key-category-icon w-11 px-0"
                onClick={() =>
                  setHiddenCategories((previous) => ({
                    ...previous,
                    [categoryName]: !previous[categoryName],
                  }))
                }
                aria-label={`${hidden ? "Показать" : "Скрыть"} категорию «${categoryName}»`}
                title={hidden ? "Показать категорию" : "Скрыть категорию"}
              >
                {hidden ? (
                  <Eye className="h-4 w-4" aria-hidden="true" />
                ) : (
                  <EyeOff className="h-4 w-4" aria-hidden="true" />
                )}
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

        <div
          className="ui-key-category-rail"
          style={{
            "--ui-key-category-color": categoryColor,
            "--ui-key-category-rail-color": alphaHexColor(categoryColor, "73"),
          } as React.CSSProperties}
          aria-hidden="true"
        >
          <div className="ui-key-category-rail-line" />
          <div className="ui-key-category-rail-label flex justify-center pt-3">
            <span
              className="ui-key-category-rail-badge inline-flex max-h-52 overflow-hidden rounded-xl px-2 py-3 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-900 shadow-[0_8px_24px_rgba(0,0,0,0.35)]"
              style={{ writingMode: "vertical-rl", textOrientation: "mixed" }}
              title={categoryName}
            >
              {categoryName}
            </span>
          </div>
        </div>
      </div>
    );
  };

  return (
    <Card className="ui-key-module-card !p-0">
      <div className="ui-section-header ui-section-header--responsive ui-key-module-header">
        <div className="ui-key-module-title">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="ui-icon-button"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Развернуть" : "Свернуть"}
          >
            <ChevronDown className={`h-4 w-4 text-zinc-400 transition-transform ${collapsed ? "-rotate-90" : ""}`} />
          </button>
          <label className="ui-icon-button cursor-pointer">
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
            <span className="flex items-center gap-2">
              <KeyRound className="h-4 w-4 text-accent" aria-hidden="true" />
              <span>Ключи ({orderedKeys.length})</span>
              {selectedCount > 0 ? (
                <span className="ui-key-selection-count" role="status">
                  Выбрано: {selectedCount}
                </span>
              ) : null}
            </span>
          </h2>
        </div>

        <div
          className={`ui-toolbar-actions ui-key-module-actions ${
            selectedCount > 0 ? "ui-key-module-actions--selection" : ""
          }`}
        >
          {selectedCount === 0 ? (
            <Link
              href="/admin/sources"
              role="button"
              className="inline-flex h-11 min-h-11 shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-sm border border-border bg-transparent px-4 font-mono text-xs font-semibold uppercase tracking-[0.08em] text-zinc-300 transition-colors hover:border-[var(--border-strong)] hover:bg-surface-2"
            >
              <RadioTower className="h-4 w-4" aria-hidden="true" />
              Внешние источники
            </Link>
          ) : null}
          {selectedCount > 0 ? (
            <>
              <Button
                variant="ghost"
                className="ui-key-bulk-icon w-11 px-0"
                onClick={() => setShowBulkEdit(true)}
                aria-label={`Изменить выбранные ключи (${selectedCount})`}
                title={`Изменить выбранные ключи (${selectedCount})`}
              >
                <Pencil className="h-4 w-4" aria-hidden="true" />
              </Button>
              <Button
                variant="danger"
                className="ui-key-bulk-icon w-11 px-0"
                onClick={openDeleteSelectedDialog}
                aria-label={`Удалить выбранные ключи (${selectedCount})`}
                title={`Удалить выбранные ключи (${selectedCount})`}
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </Button>
            </>
          ) : null}
          <Button variant="ghost" onClick={handleCheckAll} loading={checkingAll} className="text-xs">
            <RefreshCw className="h-4 w-4" aria-hidden="true" />
            Проверить все
          </Button>
          <Button variant="ghost" className="text-xs" onClick={() => openCreateCategory()}>
            <FolderPlus className="h-4 w-4" aria-hidden="true" />
            Добавить категорию
          </Button>
          <Button
            onClick={() => startAddConfiguration(orderedKeys.length)}
            aria-label="Добавить конфигурацию"
            title="Добавить конфигурацию"
            className="ui-key-add-icon w-11 shrink-0 px-0"
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>
      </div>

      {/* Панель поиска и фильтров */}
      {!collapsed && (
        <div className="ui-list-filters">
          <div className="flex-1 relative flex items-center">
            <Search className="pointer-events-none absolute left-3 h-4 w-4 text-zinc-500" aria-hidden="true" />
            <input
              type="text"
              placeholder="Поиск по названию, категории, URL, ID..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="ui-control w-full pl-9 pr-10 text-sm placeholder:text-zinc-600"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                aria-label="Очистить поиск"
                className="absolute right-0 top-0 flex h-11 min-h-11 w-11 items-center justify-center border-l border-border text-zinc-500 hover:bg-surface-1 hover:text-zinc-200"
              >
                <X className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            )}
          </div>
          <div className="flex gap-2">
            <Select
              ariaLabel="Фильтр по статусу"
              value={statusFilter}
              onChange={(e) =>
                setStatusFilter(e.target.value as "all" | "active" | "inactive")
              }
              className="w-52 min-w-52"
              options={[
                { value: "all", label: "Все статусы" },
                { value: "active", label: "Активен" },
                { value: "inactive", label: "Неактивен" },
              ]}
            />
          </div>
        </div>
      )}

      {!collapsed ? (
        <div className="space-y-4 p-4">
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

          {uncategorizedCount > 0 || allCategoryNames.length > 0 ? (
            <section className="ui-key-category-nav" aria-labelledby="key-category-nav-title">
              <div className="ui-key-category-nav-header">
                <div>
                  <div id="key-category-nav-title" className="font-medium text-zinc-300">
                    Категории
                  </div>
                  <div className="mt-0.5 text-xs text-zinc-600">
                    Перейдите к блоку или измените его видимость
                  </div>
                </div>
                <span className="font-mono text-[10px] uppercase tracking-[0.12em] text-zinc-600">
                  {allCategoryNames.length + (uncategorizedCount > 0 ? 1 : 0)} блоков
                </span>
              </div>

              <div className="ui-key-category-nav-grid">
                {uncategorizedCount > 0 ? (
                  <div className="ui-key-category-nav-item ui-key-category-nav-item--static">
                    <button
                      type="button"
                      className="ui-key-category-nav-main"
                      onClick={() => scrollToCategory(null)}
                      aria-label={`Перейти к категории «${UNCATEGORIZED_LABEL}»`}
                      title={UNCATEGORIZED_LABEL}
                    >
                      <span className="ui-key-category-color bg-zinc-500" aria-hidden="true" />
                      <span className="min-w-0">
                        <span className="block truncate font-medium text-zinc-300">{UNCATEGORIZED_LABEL}</span>
                        <span className="mt-1 block truncate text-[11px] text-zinc-600">
                          Инфо: {uncategorizedKeys.filter((key) => key.kind === "informational").length} · Конфигурации:{" "}
                          {uncategorizedKeys.filter((key) => key.kind !== "informational").length}
                        </span>
                      </span>
                      <span className="ui-key-category-count">{uncategorizedCount}</span>
                    </button>
                  </div>
                ) : null}

                {categoryGroups.map((group) => {
                  const categoryName = group.category;
                  const hidden = Boolean(hiddenCategories[categoryName]);
                  const count = categoryCounts.get(categoryName) || 0;
                  const categoryColor = getCategoryColor(categoryName);
                  const informationalCount = group.keys.filter((key) => key.kind === "informational").length;
                  const configurationCount = group.keys.length - informationalCount;

                  return (
                    <div
                      key={categoryName}
                      className={`ui-key-category-nav-item ${hidden ? "ui-key-category-nav-item--hidden" : ""}`}
                    >
                      <button
                        type="button"
                        className="ui-key-category-nav-main"
                        onClick={() => scrollToCategory(categoryName)}
                        aria-label={`Перейти к категории «${categoryName}»`}
                        title={categoryName}
                      >
                        <span
                          className="ui-key-category-color"
                          style={{ backgroundColor: categoryColor }}
                          aria-hidden="true"
                        />
                        <span className="min-w-0">
                          <span className="block truncate font-medium text-zinc-300">{categoryName}</span>
                          <span className="mt-1 block truncate text-[11px] text-zinc-600">
                            Инфо: {informationalCount} · Конфигурации: {configurationCount}
                          </span>
                        </span>
                        <span className="ui-key-category-count">{count}</span>
                      </button>
                      <button
                        type="button"
                        className="ui-key-category-visibility"
                        onClick={() =>
                          setHiddenCategories((previous) => ({
                            ...previous,
                            [categoryName]: !previous[categoryName],
                          }))
                        }
                        aria-pressed={hidden}
                        aria-label={`${hidden ? "Показать" : "Скрыть"} категорию «${categoryName}»`}
                        title={hidden ? "Показать категорию" : "Скрыть категорию"}
                      >
                        {hidden ? (
                          <Eye className="h-4 w-4" aria-hidden="true" />
                        ) : (
                          <EyeOff className="h-4 w-4" aria-hidden="true" />
                        )}
                      </button>
                    </div>
                  );
                })}
              </div>
            </section>
          ) : null}

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
          setAddKeyKind("real");
        }}
        onRefresh={refreshKeysData}
        insertAtIndex={insertAtIndex}
        initialCategory={addKeyCategoryHint}
        initialKind={addKeyKind}
        onCreated={() => {
          if (insertAtIndex !== null) {
            setTimeout(async () => {
              try {
                const { data: freshKeys } = await keysApi.listSummaries({ page_size: 200 });
                if (!freshKeys || freshKeys.length === 0) {
                  return;
                }
                const newKey = freshKeys[freshKeys.length - 1];
                const withoutNew = freshKeys.filter((item: KeySummary) => item.id !== newKey.id);
                const targetIndex = Math.min(insertAtIndex, withoutNew.length);
                withoutNew.splice(targetIndex, 0, newKey);
                await keysApi.reorder(withoutNew.map((item: KeySummary) => item.id));
                await refreshKeysData();
              } catch {
                // The key is still created even if reorder fails.
              }
              setInsertAtIndex(null);
              setAddKeyCategoryHint(undefined);
              setAddKeyKind("real");
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
