"use client";

import { FormEvent, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { EmojiText } from "@/components/ui/EmojiText";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import { normalizeDateTimeLocalValue } from "@/lib/datetime";
import type { User } from "@/lib/types";

interface Props {
  users: User[];
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function BulkEditUsersModal({ users, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [expiresAt, setExpiresAt] = useState("");
  const [maxDevicesInput, setMaxDevicesInput] = useState("");

  const parsedMaxDevices = useMemo(() => {
    if (maxDevicesInput.trim() === "") {
      return null;
    }

    const parsed = parseInt(maxDevicesInput, 10);
    if (Number.isNaN(parsed)) {
      return null;
    }

    return Math.min(32, Math.max(1, parsed));
  }, [maxDevicesInput]);

  const hasChanges = expiresAt.trim() !== "" || parsedMaxDevices !== null;

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!hasChanges) {
      return;
    }

    setLoading(true);
    try {
      const results = await Promise.allSettled(
        users.map(async (user) => {
          if (expiresAt.trim() !== "") {
            await usersApi.updateSubscription(user.id, {
              status: user.status,
              starts_at: normalizeDateTimeLocalValue(user.starts_at),
              expires_at: expiresAt,
              blocked_reason: user.blocked_reason,
              subscription_name: user.subscription_name,
              subscription_refresh_hours: user.subscription_refresh_hours,
              subscription_info_url: user.subscription_info_url,
              subscription_extra_url: user.subscription_extra_url,
              subscription_extra_status: user.subscription_extra_status,
            });
          }

          if (parsedMaxDevices !== null) {
            await usersApi.updateHwid(user.id, parsedMaxDevices);
          }
        })
      );

      const failedUsers = results
        .map((result, index) => (result.status === "rejected" ? users[index] : null))
        .filter((user): user is User => user !== null);
      const successCount = users.length - failedUsers.length;

      if (successCount > 0 && failedUsers.length === 0) {
        toast(
          successCount === 1 ? "Параметры пользователя обновлены" : `Параметры обновлены у ${successCount} пользователей`,
          "success"
        );
        onClose();
      } else if (successCount > 0) {
        toast(`Изменения применены к ${successCount} из ${users.length} пользователей`, "error");
      } else {
        toast("Не удалось применить массовые изменения", "error");
      }

      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось применить массовые изменения", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Изменить пользователей (${users.length})`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <p className="text-sm text-zinc-400">
          Выберите параметры, которые нужно изменить у отмеченных пользователей. Пустые поля не будут применены.
        </p>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="bulk-expires-at" className="text-sm text-zinc-400">
            Дата окончания подписки
          </label>
          <div className="relative">
            <input
              id="bulk-expires-at"
              type="datetime-local"
              value={expiresAt}
              onChange={(e) => setExpiresAt(e.target.value)}
              className="input-calendar-emoji bg-surface-2 border border-border rounded-lg px-3 py-2 pr-11 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg w-full"
            />
            <span className="pointer-events-none absolute inset-y-0 right-0 flex w-11 items-center justify-center text-lg">
              <EmojiText text="🗓️" />
            </span>
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="bulk-max-devices" className="text-sm text-zinc-400">
            Доступные HWID
          </label>
          <input
            id="bulk-max-devices"
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            value={maxDevicesInput}
            onChange={(e) => setMaxDevicesInput(e.target.value.replace(/\D/g, ""))}
            onBlur={() => setMaxDevicesInput(parsedMaxDevices === null ? "" : String(parsedMaxDevices))}
            placeholder="Например, 3"
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          />
          <p className="text-xs text-zinc-500">Допустимый диапазон: от 1 до 32 устройств.</p>
        </div>

        <Button type="submit" loading={loading} disabled={!hasChanges}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
