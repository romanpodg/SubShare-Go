"use client";

import { KeyEditorCategoryDialog, KeyEditorCloseDialogs } from "./KeyEditorDialogs";
import { KeyEditorFooter, KeyEditorSourceOwnership } from "./KeyEditorFrame";

import { Modal } from "@/components/ui/Modal";

import type { KeyEditorModalProps } from "./key-editor-types";
import { KeyEditorBasics } from "./KeyEditorBasics";
import { KeyEditorInformational } from "./KeyEditorInformational";
import { KeyEditorProfile } from "./KeyEditorProfile";
import { useKeyEditorModel } from "./use-key-editor-model";
export type { KeyEditorModalProps } from "./key-editor-types";
export function KeyEditorModal(props: KeyEditorModalProps) {
  const model = useKeyEditorModel(props);
  const { handleCloseAttempt, handleSubmit, kind, loading, open, titleText } = model;
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
            <KeyEditorSourceOwnership model={model} />
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
            <KeyEditorFooter model={model} />
          </form>
        )}

        <KeyEditorCategoryDialog model={model} />
      </Modal>

      <KeyEditorCloseDialogs model={model} />
    </>

  );
}
