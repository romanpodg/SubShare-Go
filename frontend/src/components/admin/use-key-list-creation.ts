"use client";
import { useState } from "react";
import { keys as keysApi } from "@/lib/api";
import type { KeySummary } from "@/lib/types";
import { normalizeCategory } from "./key-list-state";

export function useKeyListCreation({ refreshKeysData }: { refreshKeysData: () => Promise<void> }) {
const [showAddKey, setShowAddKey] = useState(false);

const [insertAtIndex, setInsertAtIndex] = useState<number | null>(null);

const [addKeyCategoryHint, setAddKeyCategoryHint] = useState<string | undefined>(undefined);

const [addKeyKind, setAddKeyKind] = useState<"real" | "informational">("real");

const startAddConfiguration = (
    index: number,
    categoryHint?: string,
    kind: "real" | "informational" = "real"
  ) => {
    setInsertAtIndex(index);
    setAddKeyCategoryHint(categoryHint ? normalizeCategory(categoryHint) : undefined);
    setAddKeyKind(kind);
    setShowAddKey(true);
  };

const closeKeyCreation = () => {
          setShowAddKey(false);
          setInsertAtIndex(null);
          setAddKeyCategoryHint(undefined);
          setAddKeyKind("real");
        };

const completeKeyCreation = () => {
          if (insertAtIndex !== null) {
            setTimeout(async () => {
              try {
                const { data: freshKeys } = await keysApi.listSummaries({ page_size: 200 });
                if (!freshKeys || freshKeys.length === 0) {
                  return;
                }
                const newKey = freshKeys[freshKeys.length - 1];
                const withoutNew = freshKeys.filter((item: KeySummary) => item.id !== newKey.id);
                const targetIndex = Math.min(insertAtIndex, withoutNew.length);
                withoutNew.splice(targetIndex, 0, newKey);
                await keysApi.reorder(withoutNew.map((item: KeySummary) => item.id));
                await refreshKeysData();
              } catch {
                // The key is still created even if reorder fails.
              }
              setInsertAtIndex(null);
              setAddKeyCategoryHint(undefined);
              setAddKeyKind("real");
            }, 100);
          }
        };
return { showAddKey, setShowAddKey, insertAtIndex, setInsertAtIndex, addKeyCategoryHint, setAddKeyCategoryHint, addKeyKind, setAddKeyKind, startAddConfiguration, closeKeyCreation, completeKeyCreation };
}
