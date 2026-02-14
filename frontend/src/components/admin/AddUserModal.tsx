"use client";

import { useState, useEffect, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function AddUserModal({ open, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [name, setName] = useState("");
  const [telegramUsername, setTelegramUsername] = useState("");
  const [code, setCode] = useState("");
  const [status, setStatus] = useState("active");
  const [days, setDays] = useState("30");

  const resetForm = () => {
    setName("");
    setTelegramUsername("");
    setCode("");
    setStatus("active");
    setDays("30");
  };

  useEffect(() => {
    if (open) {
      resetForm();
    }
  }, [open]);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      const normalizedTelegramUsername = telegramUsername.trim().replace(/^@+/, "");
      await usersApi.create({
        name,
        email: normalizedTelegramUsername,
        activation_code: code,
        status,
        issue_days: parseInt(days) || 30,
      });
      toast("Пользователь создан", "success");
      resetForm();
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось создать пользователя", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить пользователя">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input label="Имя" value={name} onChange={(e) => setName(e.target.value)} required />
        <div className="flex flex-col gap-1.5">
          <label htmlFor="telegram-username" className="text-sm text-zinc-400">
            Имя пользователя Telegram
          </label>
          <div className="flex items-center bg-surface-2 border border-border rounded-lg text-sm focus-within:ring-2 focus-within:ring-accent focus-within:ring-offset-2 focus-within:ring-offset-bg">
            <span className="pl-3 pr-1 text-zinc-500 select-none">@</span>
            <input
              id="telegram-username"
              type="text"
              value={telegramUsername}
              onChange={(e) => setTelegramUsername(e.target.value.replace(/@/g, ""))}
              placeholder="username"
              autoComplete="off"
              className="w-full bg-transparent py-2 pr-3 text-zinc-200 placeholder:text-zinc-500 focus:outline-none"
            />
          </div>
        </div>
        <Input
          label="Код активации"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          required
        />
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          options={[
            { value: "active", label: "Активен" },
            { value: "paused", label: "Приостановлен" },
            { value: "blocked", label: "Заблокирован" },
          ]}
        />
        <Input
          label="Дней подписки"
          type="number"
          value={days}
          onChange={(e) => setDays(e.target.value)}
          min="1"
          max="3650"
        />
        <Button type="submit" loading={loading}>
          Создать
        </Button>
      </form>
    </Modal>
  );
}
