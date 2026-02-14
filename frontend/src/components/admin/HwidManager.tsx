"use client";

import { useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import type { User } from "@/lib/types";

interface Props {
  user: User;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function HwidManager({ user, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [maxDevices, setMaxDevices] = useState(String(user.max_devices));

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await usersApi.updateHwid(user.id, parseInt(maxDevices) || 1);
      toast("HWID settings updated", "success");
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Failed to update", "error");
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteHwid = async (hwid: string) => {
    try {
      await usersApi.deleteHwid(user.id, hwid);
      toast("HWID removed", "success");
      await onRefresh();
    } catch {
      toast("Failed to remove HWID", "error");
    }
  };

  return (
    <Modal open onClose={onClose} title={`HWID — ${user.name}`}>
      <form onSubmit={handleSave} className="flex flex-col gap-4">
        <Input
          label="Макс. устройств"
          type="number"
          value={maxDevices}
          onChange={(e) => setMaxDevices(e.target.value)}
          min="1"
          max="32"
        />
        <Button type="submit" loading={loading}>
          Сохранить
        </Button>
      </form>

      <div className="mt-4">
        <h3 className="text-sm text-zinc-400 mb-2">
          Подключенные устройства ({user.connected_device_count}/{user.max_devices})
        </h3>
        {user.connected_hwids && user.connected_hwids.length > 0 ? (
          <div className="flex flex-col gap-1">
            {user.connected_hwids.map((hwid) => (
              <div
                key={hwid}
                className="flex items-center justify-between bg-surface-2 rounded-lg px-3 py-2"
              >
                <span className="font-mono text-xs text-zinc-300">{hwid}</span>
                <Button
                  variant="danger"
                  className="text-xs"
                  onClick={() => handleDeleteHwid(hwid)}
                >
                  Удалить
                </Button>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-sm text-zinc-500">Нет подключенных устройств</p>
        )}
      </div>
    </Modal>
  );
}
