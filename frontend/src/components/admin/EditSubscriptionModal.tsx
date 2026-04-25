"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { EmojiText } from "@/components/ui/EmojiText";
import { users as usersApi } from "@/lib/api";
import { normalizeDateTimeLocalValue } from "@/lib/datetime";
import type { User } from "@/lib/types";

interface Props {
  user: User;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function EditSubscriptionModal({ user, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState<"active" | "paused" | "blocked">(user.status);
  const [expiresAt, setExpiresAt] = useState(() => normalizeDateTimeLocalValue(user.expires_at));
  const [blockedReason, setBlockedReason] = useState(user.blocked_reason);
  const startsAt = normalizeDateTimeLocalValue(user.starts_at);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await usersApi.updateSubscription(user.id, {
        status,
        starts_at: startsAt,
        expires_at: expiresAt,
        blocked_reason: blockedReason,
      });
      toast("Подписка обновлена", "success");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить подписку", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open onClose={onClose} title={`Подписка — ${user.name}`}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value as "active" | "paused" | "blocked")}
          options={[
            { value: "active", label: "Активен" },
            { value: "paused", label: "Приостановлен" },
            { value: "blocked", label: "Заблокирован" },
          ]}
        />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="subscription-expires-at" className="text-sm text-zinc-400">
            Окончание
          </label>
          <div className="relative">
            <input
              id="subscription-expires-at"
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
        {status === "blocked" && (
          <Input
            label="Причина блокировки"
            value={blockedReason}
            onChange={(e) => setBlockedReason(e.target.value)}
          />
        )}
        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
