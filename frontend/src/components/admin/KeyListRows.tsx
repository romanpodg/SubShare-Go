"use client";
import React from "react";
import type { KeySummary } from "@/lib/types";
import { Plus } from "lucide-react";
import { alignKeysBySubscriptionOrder } from "./key-list-presentation";
import { KeyListCard } from "./KeyListCard";
import type { KeyListRowsContext } from "./key-list-presentation-types";
export function KeyListRows({ sectionKeys, categoryHint, context }: { sectionKeys: KeySummary[]; categoryHint?: string; context: KeyListRowsContext }) {
const { informationalHidden, realHidden, orderedKeys, keyIndexByID, startAddConfiguration, dropKeyOnTarget, dropKeyAtEnd } = context;
const renderKeyCard = (key: KeySummary) => <KeyListCard keyData={key} context={context} />;

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

}
