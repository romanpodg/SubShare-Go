"use client";

import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { KeyEditorConflictDialog } from "./KeyEditorConflictDialog";

import type { KeyEditorModel } from "./use-key-editor-model";

export function KeyEditorCategoryDialog({ model }: { model: KeyEditorModel; }) {
  const { showCreateCategory, setShowCreateCategory, setAvailableCategories, setCategory } = model;
  return (
    <CreateKeyCategoryModal
      open={showCreateCategory}
      onClose={() => setShowCreateCategory(false)}
      onCreated={(nextCategory) => {
        setAvailableCategories((prev) => [...prev, nextCategory]);
        setCategory(nextCategory.name);
        setShowCreateCategory(false);
      }}
    />
  );
}

export function KeyEditorCloseDialogs({ model }: { model: KeyEditorModel; }) {
  const { showDiscardConfirm, setShowDiscardConfirm, onClose, showConflictDialog, setShowConflictDialog, loadDetailAndSchema } = model;
  return (
    <>      <ConfirmDialog
      open={showDiscardConfirm}
      title="Закрыть без сохранения?"
      message="Внесённые изменения будут потеряны."
      confirmLabel="Закрыть"
      onCancel={() => setShowDiscardConfirm(false)}
      onConfirm={() => {
        setShowDiscardConfirm(false);
        onClose();
      }}
    />

      <KeyEditorConflictDialog
        open={showConflictDialog}
        onRefreshLatest={async () => {
          setShowConflictDialog(false);
          await loadDetailAndSchema();
        }}
        onClose={() => setShowConflictDialog(false)}
      /></>
  );
}
