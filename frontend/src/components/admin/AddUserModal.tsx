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
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [status, setStatus] = useState("active");
  const [days, setDays] = useState("30");

  const resetForm = () => {
    setName("");
    setEmail("");
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
      await usersApi.create({
        name,
        email,
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
        <Input
          label="Email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
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
