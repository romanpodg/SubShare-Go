"use client";

import { KeyEditorRawReveal } from "./KeyEditorRawReveal";

import { Button } from "@/components/ui/Button";
import {
  Copy
} from "lucide-react";

import { isXrayJSONRaw } from "./key-editor-raw-policy";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorRawFields({ model }: { model: KeyEditorModel; }) {
  const { changeRaw, copyRaw, isSourceOwned, keyId, mode, rawFormattingWarning, rawUri, rawValidationError, revealedRaw } = model;
  return (
    <div className={mode === "raw" ? "space-y-3" : "hidden space-y-3"}>
      <KeyEditorRawReveal model={model} />
      <textarea
        id="key-editor-raw"
        value={rawUri}
        aria-label="Raw-конфигурация"
        onChange={(e) => changeRaw(e.target.value)}
        placeholder="vless://... / vmess://... / trojan://... / ss://... / hysteria2://... / tuic://... / { XRAY-JSON }"
        rows={12}
        disabled={isSourceOwned || (keyId ? !revealedRaw : false)}
        spellCheck={false}
        className="min-h-64 w-full resize-y overflow-auto whitespace-pre rounded-sm border border-border bg-surface-2 p-3 font-mono text-xs leading-5 text-zinc-200"
      />

      {rawValidationError && (
        <div role="alert" className="rounded-sm border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-200">
          {rawValidationError}
        </div>
      )}

      {rawFormattingWarning && (
        <div role="status" className="rounded-sm border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-200">
          {rawFormattingWarning}
        </div>
      )}

      {showsOriginalRawHint(model) && (
        <div className="text-[11px] text-zinc-500">
          JSON отформатирован только для отображения. Сохранение и «Копировать raw» используют исходное представление, пока текст не изменён вручную.
        </div>
      )}

      {rawUri && (
        <div className="flex justify-end">
          <Button type="button" variant="ghost" onClick={copyRaw}>
            <Copy className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
            Копировать raw
          </Button>
        </div>
      )}
    </div>
  );
}

function showsOriginalRawHint(model: KeyEditorModel): boolean {
  if (!model.revealedRaw) return false;
  if (!isXrayJSONRaw(model.protocol, model.authoritativeRaw)) return false;
  if (model.rawEdited) return false;
  return model.authoritativeRaw !== model.rawUri;
}
