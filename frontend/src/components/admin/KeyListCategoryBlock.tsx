"use client";
import React from "react";
import type { KeySummary } from "@/lib/types";
import { ArrowUp, ArrowDown, Pencil, Eye, EyeOff } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { alphaHexColor } from "./keyCategoryColors";
import { KeyListRows } from "./KeyListRows";
import type { KeyListCategoryContext } from "./key-list-presentation-types";
export function KeyListCategoryBlock({ categoryName, categoryKeys, empty, context }: { categoryName: string; categoryKeys: KeySummary[]; empty?: boolean; context: KeyListCategoryContext }) {
const { hiddenCategories, categoryCounts, getCategoryColor, allCategoryNames, categoryBlockRefs } = context;
const renderKeyRows = (keys: KeySummary[], hint?: string) => <KeyListRows sectionKeys={keys} categoryHint={hint} context={context} />;

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
          <KeyListCategoryHeader categoryName={categoryName} categoryKeys={categoryKeys} context={context} hidden={hidden} count={count} categoryColor={categoryColor} categoryIndex={categoryIndex} />

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

}

function KeyListCategoryHeader({ categoryName, categoryKeys, context: { allCategoryNames, moveCategoryBlock, openCategoryEditor, setHiddenCategories }, hidden, count, categoryColor, categoryIndex }: { categoryName: string; categoryKeys: KeySummary[]; context: KeyListCategoryContext; hidden: boolean; count: number; categoryColor: string; categoryIndex: number }) { return (
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
); }
