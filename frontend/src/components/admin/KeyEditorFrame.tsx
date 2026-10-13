"use client";

import { Button } from "@/components/ui/Button";
import {
  Info
} from "lucide-react";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorSourceOwnership({ model }: { model: KeyEditorModel; }) {
  const { isSourceOwned, detail, cloning, handleClone } = model;
  return (
    <>            {isSourceOwned && (
      <div className="ui-key-editor-source-banner mb-3 flex shrink-0 items-center justify-between gap-3 rounded-sm border border-sky-500/30 bg-sky-500/10 p-3 text-xs text-sky-200">
        <div className="flex items-center gap-2">
          <Info className="h-4 w-4 shrink-0 text-sky-300" aria-hidden="true" />
          <span>
            Управляется источником <strong>{detail?.external_source_name}</strong>. Подключение заблокировано; статус и название в клиенте задаются локально.
          </span>
        </div>
        <Button
          type="button"
          variant="ghost"
          loading={cloning}
          onClick={handleClone}
          className="border-sky-500/40 text-sky-200 hover:bg-sky-500/20"
        >
          Клонировать как локальный
        </Button>
      </div>
    )}
    </>
  );
}

export function KeyEditorFooter({ model }: { model: KeyEditorModel; }) {
  const { handleCloseAttempt, saving, keyId, isSourceOwned, isDirty } = model;
  return (
    <div className="ui-key-editor-footer mt-3 flex shrink-0 items-center justify-end border-t border-border bg-surface-1 py-4">
      <div className="ui-key-editor-footer-actions grid w-full grid-cols-2 gap-3 sm:flex sm:w-auto">
        <Button className="w-full sm:w-36" type="button" variant="outline" onClick={handleCloseAttempt} disabled={saving}>
          Отмена
        </Button>
        <Button className={`w-full ${keyId ? "sm:w-36" : "sm:w-48"}`} type="submit" loading={saving} disabled={Boolean(isSourceOwned && !isDirty)}>
          {keyId ? "Сохранить" : "Добавить профиль"}
        </Button>
      </div>
    </div>
  );
}
