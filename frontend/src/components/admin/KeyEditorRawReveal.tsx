"use client";

import { Button } from "@/components/ui/Button";
import {
  Eye,
  Lock
} from "lucide-react";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorRawReveal({ model }: { model: KeyEditorModel; }) {
  const { keyId, revealedRaw, revealing, handleReveal } = model;
  return (<>      {keyId && !revealedRaw && (
    <div className="flex items-center justify-between rounded-sm border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs text-amber-200">
      <div className="flex items-center gap-2">
        <Lock className="h-4 w-4 shrink-0 text-amber-400" aria-hidden="true" />
        <span>Сырой URI содержит расшифрованные учетные данные.</span>
      </div>
      <Button
        type="button"
        variant="ghost"
        loading={revealing}
        onClick={() => handleReveal("raw")}
      >
        <Eye className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
        Раскрыть сырую ссылку
      </Button>
    </div>
  )}
  </>);
}
