"use client";
import { useCallback, useEffect, useState } from "react";
import { keys as keysApi } from "@/lib/api";
import type { KeySummary } from "@/lib/types";
import { moveArrayItem, type KeyListToast } from "./key-list-state";

export function useKeyListOrder({ keys, onRefresh, toast }: { keys: KeySummary[]; onRefresh: () => Promise<void>; toast: KeyListToast }) {
const [orderedKeys, setOrderedKeys] = useState<KeySummary[]>(keys);

const [reordering, setReordering] = useState(false);

const [hasUnsavedOrder, setHasUnsavedOrder] = useState(false);

useEffect(() => {
    if (!hasUnsavedOrder) {
      setOrderedKeys(keys);
    }
  }, [keys, hasUnsavedOrder]);

const moveKeyToIndex = useCallback((keyID: number, targetIndex: number) => {
    setOrderedKeys((previous) => {
      const fromIndex = previous.findIndex((item) => item.id === keyID);
      if (fromIndex === -1) {
        return previous;
      }
      const next = moveArrayItem(previous, fromIndex, targetIndex);
      if (next === previous) {
        return previous;
      }
      return next;
    });
    setHasUnsavedOrder(true);
  }, []);

const saveOrder = async () => {
    const ids = orderedKeys.map((item) => item.id);
    setReordering(true);
    try {
      await keysApi.reorder(ids);
      await onRefresh();
      setHasUnsavedOrder(false);
      toast("Порядок конфигураций сохранен", "success");
    } catch {
      toast("Не удалось изменить порядок конфигураций", "error");
    } finally {
      setReordering(false);
    }
  };
return { orderedKeys, reordering, hasUnsavedOrder, moveKeyToIndex, saveOrder };
}
