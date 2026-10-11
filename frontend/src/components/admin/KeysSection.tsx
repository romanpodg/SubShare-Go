"use client";

import { filterKeyList, normalizeCategory, keyInactiveInLinkFormat } from "./key-list-state";
import { useKeyListOrder } from "./use-key-list-order";
import { useKeyListSelection } from "./use-key-list-selection";
import { useKeyListSingleDeletion, useKeyListBulkDeletion } from "./use-key-list-deletion";
import { useKeyListHealth } from "./use-key-list-health";
import { useKeyListCreation } from "./use-key-list-creation";
import { useKeyListCategoryData } from "./use-key-list-category-data";
import { keyListDragVelocity } from "./key-list-drag";
import { useKeyListDrag } from "./use-key-list-drag";
import { useKeyListHiddenCategories } from "./use-key-list-hidden-categories";
import { keyListCategoryNames, keyListCategoryCounts, keyListCategoryMetadata, keyListCategoryGroups, moveKeyListCategoryBlock, moveKeyListKeyCategory } from "./key-list-categories";
import { useKeyListCategoryNavigation } from "./use-key-list-category-navigation";
import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import type { KeySummary } from "@/lib/types";
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
import { KeyHealthBadge } from "./KeyHealthBadge";
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

export function dragAutoScrollVelocity(
  pointerY: number,
  top: number,
  bottom: number,
  edgeSize = 120,
  maxVelocity = 28
): number { return keyListDragVelocity({ pointerY, top, bottom, edgeSize, maxVelocity }); }

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

export function keyCardClientDisplayName(key: KeySummary): string {
  return key.client_display_name.trim() || key.label;
}

export function KeysSection({ keys, subscriptionFormat, onRefresh }: Props) {
  const { toast } = useToast();

  const [collapsed, setCollapsed] = useState(false);
  const [hiddenPane, setHiddenPane] = useState<"informational" | "real" | null>(null);

  const { orderedKeys, reordering, hasUnsavedOrder, moveKeyToIndex, saveOrder } = useKeyListOrder({ keys, onRefresh, toast });
  const { keyCategories, loadKeyCategories } = useKeyListCategoryData();

  // Search and status filter states
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "inactive">("all");

  const [showBulkEdit, setShowBulkEdit] = useState(false);
  const [editKey, setEditKey] = useState<KeySummary | null>(null);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showCategoryEditor, setShowCategoryEditor] = useState(false);
  const [activeCategoryName, setActiveCategoryName] = useState("");
  const [createCategoryInitial, setCreateCategoryInitial] = useState("");

  const { dragging, setDragging, beginKeyDrag, dropKeyOnTarget, dropKeyAtEnd } = useKeyListDrag(moveKeyToIndex);

  const selectAllRef = useRef<HTMLInputElement | null>(null);
  const { uncategorizedBlockRef, categoryBlockRefs, scrollToCategory } = useKeyListCategoryNavigation();

  // Filter keys based on search query and status filter
  const filteredOrderedKeys = useMemo(() => filterKeyList(orderedKeys, { searchQuery, statusFilter }), [orderedKeys, searchQuery, statusFilter]);

  const { selectedKeyIDs, setSelectedKeyIDs, selectedCount, allSelected, partiallySelected, selectedKeys, toggleKeySelection, toggleSelectAllKeys } = useKeyListSelection(orderedKeys, filteredOrderedKeys);

  const informationalKeysCount = filteredOrderedKeys.filter((key) => key.kind === "informational").length;
  const realKeysCount = filteredOrderedKeys.length - informationalKeysCount;

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = selectedCount > 0 && !allSelected;
  }, [selectedCount, allSelected]);

  const keyIndexByID = useMemo(() => {
    return new Map<number, number>(orderedKeys.map((key, index) => [key.id, index]));
  }, [orderedKeys]);

  const allCategoryNames = useMemo(() => keyListCategoryNames(keyCategories, orderedKeys), [orderedKeys, keyCategories]);

  const { hiddenCategories, setHiddenCategories } = useKeyListHiddenCategories(allCategoryNames);

  const categoryCounts = useMemo(() => keyListCategoryCounts(filteredOrderedKeys), [filteredOrderedKeys]);

  const uncategorizedKeys = useMemo(() => {
    return filteredOrderedKeys.filter((key) => !normalizeCategory(key.category));
  }, [filteredOrderedKeys]);

  const uncategorizedCount = uncategorizedKeys.length;

  const categoryMeta = useMemo(() => keyListCategoryMetadata(keyCategories), [keyCategories]);

  const getCategoryColor = useCallback(
    (categoryName: string) => normalizeKeyCategoryColor(categoryMeta.get(categoryName)?.color || DEFAULT_KEY_CATEGORY_COLOR),
    [categoryMeta]
  );

  const categoryGroups = useMemo<CategoryGroup[]>(() => keyListCategoryGroups(allCategoryNames, filteredOrderedKeys), [allCategoryNames, filteredOrderedKeys]);

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

  const { deleteTarget, setDeleteTarget, deleting, handleDelete, confirmDelete } = useKeyListSingleDeletion({ refreshKeysData, toast });
const { bulkDeleteTargetIDs, setBulkDeleteTargetIDs, bulkDeleting, openDeleteSelectedDialog, confirmBulkDelete } = useKeyListBulkDeletion({ selectedKeyIDs, setSelectedKeyIDs, refreshKeysData, toast });

  const { checkingAll, handleCheck, handleCheckAll } = useKeyListHealth({ onRefresh, toast });

  const moveCategoryBlock = (categoryName: string, direction: -1 | 1) => moveKeyListCategoryBlock({ categoryName, direction, allCategoryNames, toast, refreshKeysData });

  const { showAddKey, setShowAddKey, insertAtIndex, setInsertAtIndex, addKeyCategoryHint, setAddKeyCategoryHint, addKeyKind, startAddConfiguration, closeKeyCreation, completeKeyCreation } = useKeyListCreation({ refreshKeysData });

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

  const moveKeyToCategory = (key: KeySummary, nextCategory: string) => moveKeyListKeyCategory({ key, nextCategory, refreshKeysData });

  const renderKeyCard = (key: KeySummary) => {
    const isReal = key.kind !== "informational";
    const isInactiveJSON = keyInactiveInLinkFormat(key, subscriptionFormat);
    const isDragging = dragging?.id === key.id;
    const isSelected = selectedKeyIDs.includes(key.id);
    const isExternalKey = key.external_source_id > 0;
    const globalIndex = keyIndexByID.get(key.id) ?? 0;

    return (
      <div
        draggable={!reordering}
        onDragStart={(event) => beginKeyDrag(event, key.id)}
        onDragEnd={() => setDragging(null)}
        onDragOver={(event) => {
          event.preventDefault();
        }}
        onDrop={(event) => dropKeyOnTarget(event, { id: key.id, index: globalIndex })}
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
          <div className="flex shrink-0 flex-wrap items-center justify-end gap-1.5">
            <StatusBadge status={key.status} />
            {isInactiveJSON ? (
              <span className="border border-border bg-surface-1 px-2.5 py-1 text-[12px] text-zinc-300">
                <EmojiText text="Неактивен JSON" />
              </span>
            ) : null}
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
                text={keyCardClientDisplayName(key)}
                truncate
                className="ui-key-card-client-name min-w-0 flex-1 text-zinc-300"
              />
            </div>
            {isInactiveJSON ? (
              <div className="mb-2 text-[13px] leading-5 text-zinc-500">
                JSON-конфиг отключен для ссылочного формата подписки
              </div>
            ) : null}
            <KeyHealthBadge healthKey={key} />
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
          onDrop={(event) => dropKeyAtEnd(event, { sectionKeys, keyIndexByID, totalCount: orderedKeys.length })}
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
                  onDrop={(event) => dropKeyOnTarget(event, { id: row.key.id, index: keyIndexByID.get(row.key.id) ?? rowIndex })}
                />
              )}
            </div>
          ))}
        </div>

        {/* Колонка конфигураций */}
        <div 
          className={`ui-key-order-column ui-key-order-column-real ${realHidden ? "hidden" : ""}`}
          onDragOver={(event) => event.preventDefault()}
          onDrop={(event) => dropKeyAtEnd(event, { sectionKeys, keyIndexByID, totalCount: orderedKeys.length })}
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
                  onDrop={(event) => dropKeyOnTarget(event, { id: row.key.id, index: keyIndexByID.get(row.key.id) ?? rowIndex })}
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
        onClose={closeKeyCreation}
        onRefresh={refreshKeysData}
        insertAtIndex={insertAtIndex}
        initialCategory={addKeyCategoryHint}
        initialKind={addKeyKind}
        onCreated={completeKeyCreation}
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
