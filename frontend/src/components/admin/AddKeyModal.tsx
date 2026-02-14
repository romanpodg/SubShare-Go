"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
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

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await keysApi.create({ label, url, status });
      toast("Key added", "success");
      setLabel("");
      setUrl("");
      setStatus("active");
      onClose();
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to add key", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Добавить ключ">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <Input
          label="Label"
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
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Статус</label>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value)}
            className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-200"
          >
            <option value="active">active</option>
            <option value="non-active">non-active</option>
          </select>
        </div>
        <Button type="submit" loading={loading}>
          Добавить
        </Button>
      </form>
    </Modal>
  );
}
