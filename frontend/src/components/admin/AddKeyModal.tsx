"use client";

import { useState, useEffect, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";

interface Props {
  open: boolean;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function AddKeyModal({ open, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [label, setLabel] = useState("");
  const [url, setUrl] = useState("");
  const [status, setStatus] = useState("active");

  const resetForm = () => {
    setLabel("");
    setUrl("");
    setStatus("active");
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
      await keysApi.create({ label, url, status });
      toast("Ключ добавлен", "success");
      resetForm();
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось добавить ключ", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить ключ">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input
          label="Название"
          value={label}
          onChange={(e) => setLabel(e.target.value)}
          required
        />
        <Input
          label="VLESS URL"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="vless://..."
          required
        />
        <Select
          label="Статус"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          options={[
            { value: "active", label: "Активен" },
            { value: "non-active", label: "Неактивен" },
          ]}
        />
        <Button type="submit" loading={loading}>
          Добавить
        </Button>
      </form>
    </Modal>
  );
}
