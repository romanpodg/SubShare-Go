"use client";

import { Select } from "@/components/ui/Select";
import {
  FolderPlus
} from "lucide-react";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorCategory({ model }: { model: Pick<KeyEditorModel, "availableCategories" | "category" | "isSourceOwned" | "setCategory" | "setIsDirty" | "setShowCreateCategory">; }) {
  const { availableCategories, category, isSourceOwned, setCategory, setIsDirty, setShowCreateCategory } = model;
  return (
    <div className="min-w-0 md:col-span-2">
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_44px] items-end gap-2">
        <Select
          label="Категория"
          value={category}
          onChange={(e) => { setCategory(e.target.value); setIsDirty(true); }}
          options={[{ value: "", label: "Без категории" }].concat(
            availableCategories.map((item) => ({ value: item.name, label: item.name }))
          )}
          disabled={Boolean(isSourceOwned)}
        />
        <button
          type="button"
          className="flex h-11 w-11 items-center justify-center rounded-sm border border-border text-zinc-400 transition-colors hover:bg-surface-2 hover:text-zinc-100"
          onClick={() => setShowCreateCategory(true)}
          disabled={Boolean(isSourceOwned)}
          title="Добавить категорию"
        >
          <FolderPlus className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
    </div>
  );
}
