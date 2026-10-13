"use client";

import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Modal } from "@/components/ui/Modal";
import {
  Info
} from "lucide-react";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";
import { KeyEditorConflictDialog } from "./KeyEditorConflictDialog";

import type { KeyEditorModalProps } from "./key-editor-types";
import { KeyEditorBasics } from "./KeyEditorBasics";
import { KeyEditorInformational } from "./KeyEditorInformational";
import { KeyEditorProfile } from "./KeyEditorProfile";
import { useKeyEditorModel } from "./use-key-editor-model";
export type { KeyEditorModalProps } from "./key-editor-types";
export function KeyEditorModal(props: KeyEditorModalProps) {
  const model = useKeyEditorModel(props);
  const {
    cloning,
    detail,
    handleClone,
    handleCloseAttempt,
    handleSubmit,
    isDirty,
    isSourceOwned,
    keyId,
    kind,
    loadDetailAndSchema,
    loading,
    onClose,
    open,
    saving,
    setAvailableCategories,
    setCategory,
    setShowConflictDialog,
    setShowCreateCategory,
    setShowDiscardConfirm,
    showConflictDialog,
    showCreateCategory,
    showDiscardConfirm,
    titleText
  } = model;
  return (
    <>
      <Modal
        open={open}
        onClose={handleCloseAttempt}
        title={titleText}
        className="max-h-[calc(100dvh-1.5rem)] w-[96vw] max-w-6xl overflow-hidden"
        contentClassName="flex min-h-0 flex-col overflow-hidden"
      >
        {loading ? (
          <div className="flex min-h-64 items-center justify-center text-sm text-zinc-400">
            Загрузка профиля и схем...
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="flex min-h-0 flex-col overflow-hidden">
            {/* Source ownership is fixed metadata, independent of the scrolling editor workspace. */}
            {isSourceOwned && (
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

            <div className="ui-key-editor-scroll-region min-h-0 flex-[0_1_auto] space-y-3 overflow-y-auto pr-1">

              <div
                data-testid="key-editor-workspace"
                className={`ui-key-editor-workspace ui-joined-grid grid min-w-0 grid-cols-1 rounded-sm ${kind === "real" ? "lg:grid-cols-2" : ""}`}
              >
                {/* Main Parameters */}
                <KeyEditorBasics model={model} />

                {kind === "informational" && (
                  <KeyEditorInformational model={model} />
                )}

                {/* Real Profile Workspace */}
                {kind === "real" && (
                  <KeyEditorProfile model={model} />
                )}
              </div>
            </div>

            {/* Modal Actions */}
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
          </form>
        )}

        <CreateKeyCategoryModal
          open={showCreateCategory}
          onClose={() => setShowCreateCategory(false)}
          onCreated={(nextCategory) => {
            setAvailableCategories((prev) => [...prev, nextCategory]);
            setCategory(nextCategory.name);
            setShowCreateCategory(false);
          }}
        />
      </Modal>

      <ConfirmDialog
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
      />
    </>

  );
}
