"use client";

import { KeyEditorModal } from "./KeyEditorModal";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
  insertAtIndex?: number | null;
  initialCategory?: string;
  initialKind?: "real" | "informational";
  onCreated?: () => void;
}

export function AddKeyModal({
  open,
  onClose,
  onRefresh,
  insertAtIndex,
  initialCategory,
  initialKind = "real",
  onCreated,
}: Props) {
  void insertAtIndex;

  return (
    <KeyEditorModal
      open={open}
      initialCategory={initialCategory}
      initialKind={initialKind}
      onClose={onClose}
      onRefresh={async () => {
        await onRefresh();
        onCreated?.();
      }}
    />
  );
}
