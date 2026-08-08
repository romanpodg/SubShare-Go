"use client";

import { AlertTriangle, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/Button";
import { Modal } from "@/components/ui/Modal";

interface Props {
  open: boolean;
  onRefreshLatest: () => void;
  onClose: () => void;
}

export function KeyEditorConflictDialog({ open, onRefreshLatest, onClose }: Props) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Конфликт версий профиля (409 Conflict)"
      className="max-w-md"
    >
      <div className="space-y-4">
        <div className="flex items-start gap-3 rounded-sm border border-amber-500/30 bg-amber-500/10 p-3.5 text-xs leading-5 text-amber-200">
          <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-400" aria-hidden="true" />
          <div>
            <p className="font-semibold text-amber-300">Профиль был изменён другой операцией</p>
            <p className="mt-1 text-zinc-300">
              Ревизия профиля на сервере изменилась (`profile_revision_conflict`). Автоматическая
              перезапись запрещена для предотвращения потери данных.
            </p>
          </div>
        </div>

        <p className="text-xs text-zinc-400">
          Несохранённые изменения сохранены в локальном черновике. Для продолжения необходимо
          загрузить актуальную версию профиля с сервера.
        </p>

        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose}>
            Отмена
          </Button>
          <Button variant="primary" onClick={onRefreshLatest}>
            <RefreshCw className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Загрузить свежую версию
          </Button>
        </div>
      </div>
    </Modal>
  );
}
