"use client";
import { useState } from "react";
import type { KeySummary } from "@/lib/types";

export function useKeyListSelection(orderedKeys: KeySummary[], filteredOrderedKeys: KeySummary[]) {
const [selectedKeyIDs, setSelectedKeyIDs] = useState<number[]>([]);
const [previousKeys, setPreviousKeys] = useState(orderedKeys);

// Apply the existing permanent pruning rule before committing the new input.
if (previousKeys !== orderedKeys) {
  setPreviousKeys(orderedKeys);
  setSelectedKeyIDs((previous) => previous.filter((id) => orderedKeys.some((key) => key.id === id)));
}

const selectedCount = selectedKeyIDs.length;

const allSelected = filteredOrderedKeys.length > 0 && filteredOrderedKeys.every((key) => selectedKeyIDs.includes(key.id));

const partiallySelected = selectedCount > 0 && !allSelected;

const selectedKeys = filteredOrderedKeys.filter((key) => selectedKeyIDs.includes(key.id));

const toggleKeySelection = (id: number) => {
    setSelectedKeyIDs((previous) =>
      previous.includes(id) ? previous.filter((value) => value !== id) : [...previous, id]
    );
  };

const toggleSelectAllKeys = () => {
    if (allSelected) {
      setSelectedKeyIDs((previous) => previous.filter((id) => !filteredOrderedKeys.some((key) => key.id === id)));
    } else {
      setSelectedKeyIDs((previous) => {
        const next = [...previous];
        filteredOrderedKeys.forEach((key) => {
          if (!next.includes(key.id)) {
            next.push(key.id);
          }
        });
        return next;
      });
    }
  };
return { selectedKeyIDs, setSelectedKeyIDs, selectedCount, allSelected, partiallySelected, selectedKeys, toggleKeySelection, toggleSelectAllKeys };
}
