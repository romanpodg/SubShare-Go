"use client";

import type { KeyEditorModel } from "./use-key-editor-model";
export function KeyEditorTemplatePreview({ model }: { model: KeyEditorModel; }) {
  const { label, templatePreviewParts } = model;
  return (
    <div>
      <div className="mb-2 text-sm font-semibold text-zinc-200">Предпросмотр</div>
      <div className="rounded-sm border border-border bg-surface-2 p-3" data-testid="informational-preview">
        <div className="text-[11px] text-zinc-500">Пример для подписчика</div>
        <div className="mt-2 whitespace-pre-wrap break-words text-sm text-zinc-100">
          {templatePreviewParts.length === 0 ? (
            <span className="text-zinc-600">Введите текст информационного ключа</span>
          ) : templatePreviewParts.map((part, index) => part.unknown ? (
            <span
              key={`${part.text}-${index}`}
              data-unknown-placeholder
              className="rounded-sm bg-amber-500/15 px-1 text-amber-300 ring-1 ring-amber-500/30"
              title="Неизвестная переменная"
            >
              {part.text}
            </span>
          ) : <span key={`${part.text}-${index}`}>{part.text}</span>)}
        </div>
        <div className="mt-3 border-t border-border pt-2 text-xs text-zinc-500">
          Название в панели: <span className="text-zinc-300">{label || "Без названия"}</span>
        </div>
      </div>
    </div>
  );
}
