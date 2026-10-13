"use client";

import { Input } from "@/components/ui/Input";

import type { KeyEditorModel } from "./use-key-editor-model";

const LABEL_LIMIT = 255;

export function KeyEditorClientName({ model }: { model: Pick<KeyEditorModel, "detail" | "displayName" | "isSourceOwned" | "label" | "setDisplayName" | "setIsDirty" | "sourceDefaultClientName">; }) {
  const { detail, displayName, isSourceOwned, label, setDisplayName, setIsDirty, sourceDefaultClientName } = model;
  return (
    <div className="min-w-0">
      <Input
        label="Название в клиенте"
        value={displayName}
        maxLength={LABEL_LIMIT}
        placeholder={isSourceOwned ? sourceDefaultClientName : (label || "Совпадает с названием в панели")}
        onChange={(event) => { setDisplayName(event.target.value); setIsDirty(true); }}
      />
      {isSourceOwned ? (
        <div className="mt-1 flex flex-wrap items-center justify-between gap-2 text-xs text-zinc-600">
          <span>Локальное название для подписки. Не изменяется при синхронизации источника. По умолчанию: {sourceDefaultClientName}</span>
          <button
            type="button"
            className="text-sky-300 transition-colors hover:text-sky-200 disabled:opacity-50"
            disabled={!displayName && !detail?.client_display_name_overridden}
            onClick={() => {
              setDisplayName("");
              setIsDirty(Boolean(detail?.client_display_name_overridden));
            }}
          >
            Использовать название источника
          </button>
        </div>
      ) : (
        <p className="mt-1 text-xs text-zinc-600">Отображается пользователю в приложении после добавления подписки.</p>
      )}
    </div>
  );
}
