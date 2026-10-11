"use client";
import React from "react";
import type { KeySummary } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { EmojiText } from "@/components/ui/EmojiText";
import { KeyHealthBadge } from "./KeyHealthBadge";
import { normalizeCategory, keyInactiveInLinkFormat } from "./key-list-state";
import { keyCardClientDisplayName } from "./key-list-presentation";
import type { KeyListCardContext } from "./key-list-presentation-types";
const UNCATEGORIZED_LABEL = "Без категории";
function categoryDisplayName(value: string) { return value || UNCATEGORIZED_LABEL; }
export function KeyListCard({ keyData: key, context }: { keyData: KeySummary; context: KeyListCardContext }) {
const { subscriptionFormat, selectedKeyIDs, dragging, reordering, keyIndexByID, beginKeyDrag, dropKeyOnTarget, setDragging, handleCheck, handleDelete, setEditKey } = context;

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
        <KeyListCardHeader keyData={key} context={context} isInactiveJSON={isInactiveJSON} isSelected={isSelected} />

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

}

function KeyListCardHeader({ keyData: key, context: { toggleKeySelection }, isInactiveJSON, isSelected }: { keyData: KeySummary; context: KeyListCardContext; isInactiveJSON: boolean; isSelected: boolean }) { return (
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
); }
