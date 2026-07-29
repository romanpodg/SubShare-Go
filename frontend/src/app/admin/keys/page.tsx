"use client";

import { useCallback, useEffect, useState } from "react";
import { KeyRound } from "lucide-react";
import { keys as keysApi, subscriptionSettings } from "@/lib/api";
import type { VLESSKey } from "@/lib/types";
import { KeysSection } from "@/components/admin/KeysSection";
import { PageHeader } from "@/components/admin/PageHeader";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";

export default function KeysPage() {
  const [keys, setKeys] = useState<VLESSKey[]>([]);
  const [format, setFormat] = useState<"links" | "xray-json">("links");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [settingsError, setSettingsError] = useState<unknown>(null);

  const load = useCallback(async () => {
    setError(null);
    setSettingsError(null);
    const [keysResult, settingsResult] = await Promise.allSettled([
      keysApi.list(),
      subscriptionSettings.get(),
    ]);
    if (keysResult.status === "fulfilled") setKeys(keysResult.value.keys || []);
    else setError(keysResult.reason);
    if (settingsResult.status === "fulfilled") {
      setFormat(settingsResult.value.subscription_format);
    } else {
      setSettingsError(settingsResult.reason);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      void load();
    });
    return () => window.cancelAnimationFrame(frame);
  }, [load]);

  return (
    <div>
      <PageHeader title="Ключи и конфигурации" description="Проверка доступности, категории, источники и порядок выдачи." icon={<KeyRound className="h-5 w-5" />} />
      {loading && keys.length === 0 ? (
        <InitialLoading label="Загрузка ключей…" />
      ) : error && keys.length === 0 ? (
        <ResourceError error={error} onRetry={() => void load()} />
      ) : (
        <>
          {Boolean(error) && (
            <div className="mb-4">
              <ResourceError error={error} compact onRetry={() => void load()} title="Не удалось обновить ключи" />
            </div>
          )}
          {Boolean(settingsError) && (
            <div className="mb-4">
              <ResourceError
                error={settingsError}
                compact
                onRetry={() => void load()}
                title="Настройки формата недоступны; используется формат links"
              />
            </div>
          )}
          <KeysSection keys={keys} subscriptionFormat={format} onRefresh={load} />
        </>
      )}
    </div>
  );
}
