"use client";

import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";

import { KeyEditorCategory } from "./KeyEditorCategory";
import { KeyEditorClientName } from "./KeyEditorClientName";
import type { KeyEditorModel } from "./use-key-editor-model";
const LABEL_LIMIT = 255;

export function KeyEditorBasics({ model }: { model: KeyEditorModel; }) {
  const { isSourceOwned, kind, label, setIsDirty, setKind, setLabel, setStatus, status } = model;
  return (
    <section className="rounded-sm border border-border bg-surface-2/25 p-3.5">
      <div className="system-label mb-3">ОСНОВНЫЕ ПАРАМЕТРЫ</div>
      <div className="grid min-w-0 grid-cols-1 gap-3 md:grid-cols-2">
        <div className={`min-w-0 ${kind === "real" ? "" : "md:col-span-2"}`}>
          <Input
            label="Название"
            value={label}
            maxLength={LABEL_LIMIT}
            onChange={(e) => { setLabel(e.target.value); setIsDirty(true); }}
            disabled={Boolean(isSourceOwned)}
            required
          />
          <p className="mt-1 text-xs text-zinc-600">Используется только в панели управления.</p>
        </div>
        {kind === "real" && (
          <KeyEditorClientName model={model} />
        )}
        <div className="min-w-0">
          <Select
            label="Статус"
            value={status}
            onChange={(e) => { setStatus(e.target.value as "active" | "non-active"); setIsDirty(true); }}
            options={[
              { value: "active", label: "Активен" },
              { value: "non-active", label: "Неактивен" },
            ]}
          />
        </div>
        <Select
          label="Тип"
          value={kind}
          onChange={(e) => { setKind(e.target.value as "real" | "informational"); setIsDirty(true); }}
          options={[
            { value: "real", label: "Конфигурация" },
            { value: "informational", label: "Информационный ключ" },
          ]}
          disabled={Boolean(isSourceOwned)}
        />
        <KeyEditorCategory model={model} />
      </div>
    </section>
  );
}
