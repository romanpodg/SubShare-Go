"use client";

import { KeyEditorTemplatePreview } from "./KeyEditorTemplatePreview";

import { keyTemplateVariables } from "./keyTemplateVariables";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorInformational({ model }: { model: KeyEditorModel; }) {
  const { insertTemplateVariable, setIsDirty, setTemplateText, templateText, templateTextRef } = model;
  return (
    <section className="space-y-3 rounded-sm border border-border bg-surface-1 p-3.5">
      <div className="system-label">ИНФОРМАЦИОННЫЙ КЛЮЧ</div>
      <label className="block space-y-2">
        <span className="font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-500">
          Текст информационного ключа
        </span>
        <textarea
          ref={templateTextRef}
          aria-label="Текст информационного ключа"
          value={templateText}
          maxLength={8192}
          onChange={(event) => {
            setTemplateText(event.target.value);
            setIsDirty(true);
          }}
          className="min-h-32 w-full resize-y rounded-sm border border-border bg-surface-2 p-3 text-sm leading-6 text-zinc-100 focus:border-accent focus:outline-none"
          placeholder="Например: Подписка {user_name} действует до {expires_date}"
        />
      </label>

      <div>
        <div className="mb-2 text-sm font-semibold text-zinc-200">Доступные переменные</div>
        <div className="flex flex-wrap gap-2">
          {keyTemplateVariables.map((variable) => (
            <button
              key={variable.token}
              type="button"
              onClick={() => insertTemplateVariable(variable.token)}
              className="rounded-sm border border-border bg-surface-2 px-2.5 py-2 text-left transition-colors hover:border-accent"
              title={`Вставить ${variable.token}`}
            >
              <code className="block text-xs text-accent">{variable.token}</code>
              <span className="mt-0.5 block text-[11px] text-zinc-500">{variable.description}</span>
            </button>
          ))}
        </div>
      </div>

      <KeyEditorTemplatePreview model={model} />
    </section>
  );
}
