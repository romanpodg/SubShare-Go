"use client";

import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import {
  Eye,
  Lock
} from "lucide-react";
import {
  isLegacyEditorProtocol
} from "./protocol-editor-capabilities";
import { TuicV4ReadOnlyBanner } from "./protocol-editors/TuicV4ReadOnlyBanner";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorConnection({ model }: { model: KeyEditorModel; }) {
  const { protocol, server, port, setServer, setPort, markStructuredChange, isSourceOwned } = model;
  if (isLegacyEditorProtocol(protocol)) return null;
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <Input
        label="Сервер (Host)"
        value={server}
        onChange={(e) => { setServer(e.target.value); markStructuredChange(); }}
        disabled={isSourceOwned}
      />
      <Input
        label="Порт / Выражение портов"
        value={port}
        placeholder="443 или 20000-50000"
        onChange={(e) => { setPort(e.target.value); markStructuredChange(); }}
        disabled={isSourceOwned}
      />
    </div>
  );
}

export function KeyEditorNativeReveal({ model }: { model: KeyEditorModel; }) {
  const { revealing, handleReveal } = model;
  if (!model.keyId) return null;
  if (model.isSourceOwned) return null;
  if (isLegacyEditorProtocol(model.protocol)) return null;
  return (
    <div className="flex items-center justify-between rounded-sm border border-border bg-surface-2/40 p-3 text-xs text-zinc-300">
      <div className="flex items-center gap-2">
        <Lock className="h-4 w-4 text-zinc-400" aria-hidden="true" />
        <span>Секретные поля скрыты по умолчанию.</span>
      </div>
      <Button
        type="button"
        variant="ghost"
        loading={revealing}
        onClick={() => handleReveal("structured-secrets")}
      >
        <Eye className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
        Раскрыть секреты
      </Button>
    </div>
  );
}

export function KeyEditorTuicReadOnly({ model }: { model: KeyEditorModel; }) {
  const tuic = model.detail?.safe_structured?.tuic;
  if (!tuic) return null;
  if (tuic.generation !== 4) return null;
  return <TuicV4ReadOnlyBanner tokenPresent={tuic.token_present} />;
}
