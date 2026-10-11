"use client";
import { useState } from "react";
import { keys as keysApi } from "@/lib/api";
import type { KeyListToast } from "./key-list-state";

export function useKeyListHealth({ onRefresh, toast }: { onRefresh: () => Promise<void>; toast: KeyListToast }) {
const [checkingAll, setCheckingAll] = useState(false);

const handleCheck = async (id: number) => {
    try {
      await keysApi.check(id);
      toast("Конфигурация проверена", "success");
      await onRefresh();
    } catch {
      toast("Не удалось проверить конфигурацию", "error");
    }
  };

const handleCheckAll = async () => {
    setCheckingAll(true);
    try {
      await keysApi.checkAll();
      toast("Проверка конфигураций запущена", "success");
    } catch {
      toast("Не удалось проверить конфигурации", "error");
    } finally {
      setCheckingAll(false);
    }
  };
return { checkingAll, handleCheck, handleCheckAll };
}
