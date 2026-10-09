import type { KeyProfileDetailResponse } from "@/lib/types";
import type { EditorCommandState } from "./key-editor-command-state";
import { buildEditorCreateCommand } from "./key-editor-create-command";
import { buildEditorUpdateCommand } from "./key-editor-update-command";

export function buildEditorSaveCommand(state: EditorCommandState, keyId: number | undefined, detail: KeyProfileDetailResponse | null) {
  if (keyId) {
    if (!detail) throw new Error("Профиль не загружен. Откройте его заново.");
    return {
      type: "update" as const, keyId, input: buildEditorUpdateCommand(state, detail),
      message: detail.ownership === "external_source" ? "Локальные параметры профиля обновлены" : "Профиль успешно обновлён",
    };
  }
  return { type: "create" as const, input: buildEditorCreateCommand(state), message: "Профиль успешно создан" };
}
