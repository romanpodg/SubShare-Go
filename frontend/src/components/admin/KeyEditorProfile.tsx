"use client";

import { Select } from "@/components/ui/Select";
import type {
  ExternalProfileProtocol
} from "@/lib/types";
import {
  isLegacyEditorProtocol,
  PROTOCOL_EDITOR_CAPABILITIES,
} from "./protocol-editor-capabilities";

import { emptyLegacyDraft } from "./key-editor-command-state";

import { KeyEditorRawFields } from "./KeyEditorRawFields";
import { KeyEditorStructuredFields } from "./KeyEditorStructuredFields";
import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorProfile({ model }: { model: KeyEditorModel; }) {
  const {
    keyId,
    mode,
    protocol,
    setAuthoritativeRaw,
    setIsDirty,
    setLegacyDraft,
    setLegacySecretsRevealed,
    setMode,
    setPort,
    setProtocol,
    setRawEdited,
    setRawUri,
    setRawValidationError,
    setServer,
    setStructuredEdited
  } = model;
  return (
    <section className="space-y-3 rounded-sm border border-border bg-surface-1 p-3.5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="system-label">РЕДАКТОР ПРОФИЛЯ</div>
        <div className="flex rounded-sm border border-border bg-surface-2 p-1">
          <button
            type="button"
            onClick={() => setMode("structured")}
            className={`px-3 py-1 text-xs font-medium rounded-sm ${mode === "structured" ? "bg-accent text-accent-fg" : "text-zinc-400"
              }`}
          >
            Структурированный
          </button>
          <button
            type="button"
            aria-label="XRAY-JSON"
            onClick={() => setMode("raw")}
            className={`px-3 py-1 text-xs font-medium rounded-sm ${mode === "raw" ? "bg-accent text-accent-fg" : "text-zinc-400"
              }`}
          >
            Raw / XRAY-JSON
          </button>
        </div>
      </div>

      {!keyId && (
        <Select
          label="Протокол"
          value={protocol}
          onChange={(e) => {
            const nextProtocol = e.target.value as ExternalProfileProtocol;
            setProtocol(nextProtocol);
            setRawUri("");
            setAuthoritativeRaw("");
            setRawEdited(false);
            setStructuredEdited(false);
            setRawValidationError("");
            if (isLegacyEditorProtocol(nextProtocol)) {
              setLegacyDraft(emptyLegacyDraft(nextProtocol));
              setLegacySecretsRevealed(true);
              setServer("");
              setPort("443");
            }
            setIsDirty(true);
          }}
          options={PROTOCOL_EDITOR_CAPABILITIES.map((item) => ({ value: item.protocol, label: item.label }))}
        />
      )}

      <KeyEditorStructuredFields model={model} />                  {/* Raw Mode */}
      <KeyEditorRawFields model={model} />
    </section>
  );
}
