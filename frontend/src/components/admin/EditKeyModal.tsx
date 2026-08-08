"use client";

import type { KeySummary } from "@/lib/types";
import { KeyEditorModal } from "./KeyEditorModal";

interface Props {
  keyData: KeySummary;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditKeyModal({ keyData, onClose, onRefresh }: Props) {
  const rawUrl = (keyData as unknown as Record<string, unknown>).url || (keyData as unknown as Record<string, unknown>).url_short || "";
  const isRawJson = Boolean(typeof rawUrl === "string" && rawUrl.trim().startsWith("{"));
  const computedProtocol = keyData.protocol || (isRawJson ? "vless" : undefined);

  return (
    <KeyEditorModal
      open
      keyId={keyData.id}
      initialLabel={keyData.label}
      initialProtocol={computedProtocol}
      onClose={onClose}
      onRefresh={onRefresh}
    />
  );
}
