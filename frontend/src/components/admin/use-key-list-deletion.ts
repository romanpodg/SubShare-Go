"use client";
import { useState, type Dispatch, type SetStateAction } from "react";
import { keys as keysApi } from "@/lib/api";
import type { KeyListToast } from "./key-list-state";

interface SingleDeletionOptions { refreshKeysData: () => Promise<void>; toast: KeyListToast }
interface BulkDeletionOptions extends SingleDeletionOptions { selectedKeyIDs: number[]; setSelectedKeyIDs: Dispatch<SetStateAction<number[]>> }
export function useKeyListSingleDeletion({ refreshKeysData, toast }: SingleDeletionOptions) {
const [deleteTarget, setDeleteTarget] = useState<{ id: number; label: string } | null>(null);

const [deleting, setDeleting] = useState(false);

const handleDelete = (id: number, label: string) => {
    setDeleteTarget({ id, label });
  };

const confirmDelete = async () => {
    if (!deleteTarget) {
      return;
    }

    setDeleting(true);
    try {
      await keysApi.delete(deleteTarget.id);
      toast("Конфигурация удалена", "success");
      await refreshKeysData();
    } catch {
      toast("Не удалось удалить конфигурацию", "error");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };
return { deleteTarget, setDeleteTarget, deleting, handleDelete, confirmDelete };
}
export function useKeyListBulkDeletion({ selectedKeyIDs, setSelectedKeyIDs, refreshKeysData, toast }: BulkDeletionOptions) {
const [bulkDeleteTargetIDs, setBulkDeleteTargetIDs] = useState<number[] | null>(null);

const [bulkDeleting, setBulkDeleting] = useState(false);

const openDeleteSelectedDialog = () => {
    if (selectedKeyIDs.length === 0) {
      return;
    }
    setBulkDeleteTargetIDs([...selectedKeyIDs]);
  };

const confirmBulkDelete = async () => {
    if (!bulkDeleteTargetIDs || bulkDeleteTargetIDs.length === 0) {
      return;
    }

    setBulkDeleting(true);
    try {
      const targetIDs = [...bulkDeleteTargetIDs];
      const response = await keysApi.bulkDelete(targetIDs);
      const deletedCount = response.deleted ?? targetIDs.length;
      toast(
        deletedCount === 1 ? "Конфигурация удалена" : `Удалено конфигураций: ${deletedCount}`,
        "success"
      );
      setSelectedKeyIDs([]);
      await refreshKeysData();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось удалить выбранные конфигурации", "error");
    } finally {
      setBulkDeleting(false);
      setBulkDeleteTargetIDs(null);
    }
  };
return { bulkDeleteTargetIDs, setBulkDeleteTargetIDs, bulkDeleting, openDeleteSelectedDialog, confirmBulkDelete };
}
