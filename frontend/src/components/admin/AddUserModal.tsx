"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
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
      toast("User created", "success");
      setName("");
      setEmail("");
      setCode("");
      setStatus("active");
      setDays("30");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to create user", "error");
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
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="paused">paused</option>
            <option value="blocked">blocked</option>
          </select>
        </div>
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
