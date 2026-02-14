"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
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
  const [startsAt, setStartsAt] = useState(user.starts_at);
  const [expiresAt, setExpiresAt] = useState(user.expires_at);
  const [blockedReason, setBlockedReason] = useState(user.blocked_reason);

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
        <Input
          label="Начало"
          type="datetime-local"
          value={startsAt}
          onChange={(e) => setStartsAt(e.target.value)}
        />
        <Input
          label="Окончание"
          type="datetime-local"
          value={expiresAt}
          onChange={(e) => setExpiresAt(e.target.value)}
        />
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
