"use client";

import { useEffect, useMemo, useState, FormEvent } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { users as usersApi } from "@/lib/api";
import type { User } from "@/lib/types";
import { Minus, Plus } from "lucide-react";

interface Props {
  user: User;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

export function connectedDevicesHeading(connected: number, maximum: number) {
  if (maximum === 0) {
    return `Подключенные устройства: ${connected} — без ограничений`;
  }
  return `Подключенные устройства (${connected}/${maximum})`;
}

export function HwidManager({ user, onClose, onRefresh }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [maxDevicesInput, setMaxDevicesInput] = useState(String(user.max_devices));

  useEffect(() => {
    setMaxDevicesInput(String(user.max_devices));
  }, [user.id, user.max_devices]);

  const parsedMaxDevices = useMemo(() => {
    const parsed = parseInt(maxDevicesInput, 10);
    if (Number.isNaN(parsed)) {
      return 0;
    }
    return Math.min(32, Math.max(0, parsed));
  }, [maxDevicesInput]);

  const hasMaxDevicesChanges = parsedMaxDevices !== user.max_devices;

  const connectedDevices = useMemo(() => {
    if (user.connected_devices && user.connected_devices.length > 0) {
      return user.connected_devices;
    }
    if (user.connected_hwids && user.connected_hwids.length > 0) {
      return user.connected_hwids.map((hwid) => ({
        hwid,
        normalized_hwid: "",
        device_name: "",
        device_model: "",
        device_brand: "",
        platform: "",
        os_version: "",
        app_name: "",
        app_version: "",
        client_app: "",
        client_version: "",
        user_agent: "",
        created_at: "",
        last_seen_at: "",
      }));
    }
    return [];
  }, [user.connected_devices, user.connected_hwids]);

  const handleSave = async (e: FormEvent) => {
    e.preventDefault();
    if (!hasMaxDevicesChanges) {
      return;
    }
    setLoading(true);
    try {
      await usersApi.updateHwid(user.id, parsedMaxDevices);
      toast("Настройки HWID обновлены", "success");
      await onRefresh();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить", "error");
    } finally {
      setLoading(false);
    }
  };

  const stepMaxDevices = (delta: number) => {
    const next = Math.min(32, Math.max(0, parsedMaxDevices + delta));
    setMaxDevicesInput(String(next));
  };

  const inferDeviceKind = (value: string) => {
    const normalized = value.toLowerCase();
    if (normalized.includes("iphone") || normalized.includes("ios")) return "iPhone";
    if (normalized.includes("ipad")) return "iPad";
    if (normalized.includes("android") || normalized.includes("miui") || normalized.includes("oneui")) return "Android";
    if (normalized.includes("windows") || normalized.includes("win32") || normalized.includes("win64")) return "Windows";
    if (normalized.includes("mac") || normalized.includes("darwin") || normalized.includes("macos")) return "macOS";
    if (normalized.includes("linux")) return "Linux";
    return "Устройство";
  };

  const shortHwid = (hwid: string) => {
    const value = hwid.trim();
    if (value.length <= 24) return value;
    return `${value.slice(0, 10)}…${value.slice(-8)}`;
  };

  const handleDeleteHwid = async (hwid: string) => {
    try {
      await usersApi.deleteHwid(user.id, hwid);
      toast("HWID удален", "success");
      await onRefresh();
    } catch {
      toast("Не удалось удалить HWID", "error");
    }
  };

  return (
    <Modal open onClose={onClose} title={`HWID — ${user.name}`}>
      <form onSubmit={handleSave} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <label htmlFor="hwid-max-devices" className="text-sm text-zinc-400">
            Макс. устройств
          </label>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={() => stepMaxDevices(-1)}
              disabled={parsedMaxDevices <= 0 || loading}
              className="ui-icon-button disabled:cursor-not-allowed disabled:opacity-50"
              aria-label="Уменьшить максимум устройств"
            >
              <Minus className="h-4 w-4" aria-hidden="true" />
            </button>
            <input
              id="hwid-max-devices"
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={maxDevicesInput}
              onChange={(e) => {
                const digits = e.target.value.replace(/\D/g, "");
                setMaxDevicesInput(digits === "" ? "" : String(Math.min(32, Math.max(0, parseInt(digits, 10)))));
              }}
              onBlur={() => setMaxDevicesInput(String(parsedMaxDevices))}
              className="ui-control w-full px-3 text-center text-sm"
              aria-label="Максимум устройств"
            />
            <button
              type="button"
              onClick={() => stepMaxDevices(1)}
              disabled={parsedMaxDevices >= 32 || loading}
              className="ui-icon-button disabled:cursor-not-allowed disabled:opacity-50"
              aria-label="Увеличить максимум устройств"
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
          <p className="text-xs text-zinc-600">0 — без ограничения числа устройств.</p>
        </div>
        <Button type="submit" loading={loading} disabled={!hasMaxDevicesChanges}>
          Сохранить
        </Button>
      </form>

      <div className="mt-4">
        <h3 className="text-sm text-zinc-400 mb-2">
          {connectedDevicesHeading(user.connected_device_count, user.max_devices)}
        </h3>
        {connectedDevices.length > 0 ? (
          <div className="flex flex-col gap-1">
            {connectedDevices.map((device, index) => {
              const sourceText = [device.device_name, device.device_model, device.platform, device.os_version, device.app_name, device.app_version, device.user_agent]
                .filter(Boolean)
                .join(" ");
              const title = [device.device_name, device.device_model, inferDeviceKind(sourceText || device.hwid)]
                .filter(Boolean)
                .join(" · ") || `Устройство ${index + 1}`;
              const clientLabel = [device.client_app || device.app_name, device.client_version || device.app_version]
                .filter(Boolean)
                .join(" ");
              const platformLabel = [device.platform, device.os_version].filter(Boolean).join(" ");
              const modelLabel = [device.device_brand, device.device_model].filter(Boolean).join(" ");

              return (
              <div
                key={device.hwid}
                className="flex items-center justify-between bg-surface-2 rounded-lg px-3 py-2"
              >
                <div className="min-w-0 pr-3">
                  <div className="text-sm text-zinc-200 truncate">
                    {title}
                  </div>
                  {(clientLabel || platformLabel || modelLabel) && (
                    <div className="text-xs text-zinc-400 truncate mt-0.5">
                      {clientLabel || "Клиент не определен"}
                      {platformLabel ? ` · ${platformLabel}` : ""}
                      {modelLabel ? ` · ${modelLabel}` : ""}
                    </div>
                  )}
                  <div className="font-mono text-xs text-zinc-500 truncate" title={device.hwid}>
                    {shortHwid(device.hwid)}
                  </div>
                  {device.normalized_hwid && device.normalized_hwid !== device.hwid && (
                    <div className="font-mono text-xs text-zinc-600 truncate" title={device.normalized_hwid}>
                      normalized: {shortHwid(device.normalized_hwid)}
                    </div>
                  )}
                  {device.last_seen_at && (
                    <div className="text-xs text-zinc-500 truncate mt-0.5">
                      Last seen: {device.last_seen_at}
                    </div>
                  )}
                </div>
                <Button
                  variant="danger"
                  className="text-xs"
                  onClick={() => handleDeleteHwid(device.hwid)}
                >
                  Удалить
                </Button>
              </div>
              );
            })}
          </div>
        ) : (
          <p className="text-sm text-zinc-400">Нет подключенных устройств</p>
        )}
      </div>
    </Modal>
  );
}
