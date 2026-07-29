"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Globe2, Plus, Route, Save, Settings, Trash2 } from "lucide-react";
import { apiV1, subscriptionSettings } from "@/lib/api";
import type { SubscriptionDeliverySettings, SubscriptionSettings } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { GlobalSubscriptionSettingsModal } from "@/components/admin/GlobalSubscriptionSettingsModal";
import { RoutingSettingsModal } from "@/components/admin/RoutingSettingsModal";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { useAuth } from "@/hooks/useAuth";

const emptyDeliverySettings = (): SubscriptionDeliverySettings => ({
  response_headers: [],
  announcement: "",
  remarks: { expired: [], paused: [], blocked: [], limited: [], empty: [] },
});

const remarkLabels: Record<keyof SubscriptionDeliverySettings["remarks"], string> = {
  expired: "Подписка истекла",
  paused: "Подписка приостановлена",
  blocked: "Подписка заблокирована",
  limited: "Превышен HWID-лимит",
  empty: "Нет доступных ключей",
};

export default function SubscriptionSettingsPage() {
  const [settings, setSettings] = useState<SubscriptionSettings | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [routingOpen, setRoutingOpen] = useState(false);
  const [delivery, setDelivery] = useState<SubscriptionDeliverySettings>(emptyDeliverySettings);
  const [initialDelivery, setInitialDelivery] = useState("");
  const [deliveryTab, setDeliveryTab] = useState<"announcement" | "headers" | "remarks">("announcement");
  const [savingDelivery, setSavingDelivery] = useState(false);
  const [loading, setLoading] = useState(true);
  const [settingsError, setSettingsError] = useState<unknown>(null);
  const [deliveryError, setDeliveryError] = useState<unknown>(null);
  const { toast } = useToast();
  const { role } = useAuth();
  const isOwner = role === "owner";

  const load = useCallback(async () => {
    setSettingsError(null);
    try {
      setSettings(await subscriptionSettings.get());
    } catch (requestError) {
      setSettingsError(requestError);
    }
  }, []);

  const loadDelivery = useCallback(async () => {
    setDeliveryError(null);
    try {
      const data = await apiV1.deliverySettings.get();
      setDelivery(data);
      setInitialDelivery(JSON.stringify(data));
    } catch (requestError) {
      setDeliveryError(requestError);
    }
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      void Promise.allSettled([load(), loadDelivery()]).finally(() => setLoading(false));
    });
    return () => window.cancelAnimationFrame(frame);
  }, [load, loadDelivery]);

  const deliveryDirty = useMemo(
    () => initialDelivery !== "" && JSON.stringify(delivery) !== initialDelivery,
    [delivery, initialDelivery]
  );

  const saveDelivery = async () => {
    if (!isOwner) return;
    setSavingDelivery(true);
    try {
      await apiV1.deliverySettings.update(delivery);
      setInitialDelivery(JSON.stringify(delivery));
      toast("Настройки выдачи сохранены", "success");
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить настройки выдачи", "error");
    } finally {
      setSavingDelivery(false);
    }
  };

  if (loading && !settings && initialDelivery === "") {
    return <InitialLoading label="Загрузка настроек подписки…" />;
  }

  return (
    <div>
      <PageHeader title="Настройки подписки" description="Глобальные метаданные, формат по умолчанию и параметры Happ." icon={<Settings className="h-5 w-5" />} />
      {Boolean(settingsError) && <div className="mb-4"><ResourceError error={settingsError} compact onRetry={() => void load()} title="Не удалось загрузить метаданные" /></div>}
      {Boolean(deliveryError) && <div className="mb-4"><ResourceError error={deliveryError} compact onRetry={() => void loadDelivery()} title="Не удалось загрузить настройки выдачи" /></div>}
      {!isOwner && (
        <div className="mb-4 rounded-xl border border-amber-400/20 bg-amber-400/5 px-4 py-3 text-sm text-amber-200">
          Режим просмотра: изменять настройки может только владелец.
        </div>
      )}
      <div className="grid gap-px bg-border lg:grid-cols-2">
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex items-start justify-between gap-4">
            <div className="flex gap-3"><div className="rounded-xl border border-cyan-400/20 bg-cyan-400/10 p-2.5 text-cyan-300"><Globe2 className="h-5 w-5" /></div><div><h2 className="font-semibold text-zinc-200">Метаданные</h2><p className="mt-1 text-sm text-zinc-600">Название, интервал обновления, ссылки и заголовки клиента.</p></div></div>
            <Button variant="outline" onClick={() => setSettingsOpen(true)} disabled={!isOwner}>Изменить</Button>
          </div>
          <dl className="mt-6 grid grid-cols-2 gap-3">
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Название</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.title || "—"}</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Формат fallback</dt><dd className="mt-1 font-mono text-sm text-cyan-300">{settings?.subscription_format || "—"}</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Обновление</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.refresh_hours ?? "—"} ч</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Язык</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.language || "—"}</dd></div>
          </dl>
        </section>

        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex items-start justify-between gap-4">
            <div className="flex gap-3"><div className="rounded-xl border border-cyan-400/20 bg-cyan-400/10 p-2.5 text-cyan-300"><Route className="h-5 w-5" /></div><div><h2 className="font-semibold text-zinc-200">Маршрутизация Happ</h2><p className="mt-1 text-sm text-zinc-600">DNS, исключения и routing-конфигурация для автоматического импорта.</p></div></div>
            <Button variant="outline" onClick={() => setRoutingOpen(true)} disabled={!isOwner}>Открыть редактор</Button>
          </div>
          <div className="mt-6 rounded-xl border border-border bg-zinc-950/30 p-4 text-sm text-zinc-500">
            Формат ответа теперь может выбираться автоматически на странице «Правила ответов».
          </div>
        </section>
      </div>

      <section className="technical-frame border border-border bg-surface-1">
        <div className="flex flex-col justify-between gap-4 border-b border-border px-5 py-4 lg:flex-row lg:items-center">
          <div>
            <h2 className="font-semibold text-zinc-200">Выдача и сообщения клиентам</h2>
            <p className="mt-1 text-xs text-zinc-600">Объявление, безопасные глобальные заголовки и сообщения для operational status.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {([
              ["announcement", "Объявление"],
              ["headers", "Заголовки"],
              ["remarks", "Ремарки"],
            ] as const).map(([value, label]) => (
              <button
                key={value}
                type="button"
                onClick={() => setDeliveryTab(value)}
                className={`rounded-lg border px-3 py-2 text-xs font-medium transition ${
                  deliveryTab === value
                    ? "border-cyan-400/40 bg-cyan-400/10 text-cyan-200"
                    : "border-border text-zinc-500 hover:text-zinc-300"
                }`}
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        <fieldset disabled={!isOwner} className="p-5">
          {deliveryTab === "announcement" && (
            <label className="flex flex-col gap-2 text-sm text-zinc-400">
              Объявление
              <textarea
                value={delivery.announcement}
                maxLength={200}
                rows={5}
                onChange={(event) => setDelivery((current) => ({ ...current, announcement: event.target.value }))}
                className="rounded-xl border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400"
                placeholder="Плановые работы 30 июля с 02:00 до 03:00"
              />
              <span className="text-right text-xs text-zinc-600">{delivery.announcement.length}/200</span>
            </label>
          )}

          {deliveryTab === "headers" && (
            <div className="space-y-3">
              {delivery.response_headers.map((header, index) => (
                <div key={index} className="grid gap-3 rounded-xl border border-border bg-zinc-950/30 p-3 md:grid-cols-[minmax(180px,0.4fr)_1fr_auto]">
                  <Input
                    aria-label={`Имя заголовка ${index + 1}`}
                    value={header.key}
                    placeholder="X-Provider-ID"
                    onChange={(event) => setDelivery((current) => ({
                      ...current,
                      response_headers: current.response_headers.map((item, itemIndex) => itemIndex === index ? { ...item, key: event.target.value } : item),
                    }))}
                  />
                  <Input
                    aria-label={`Значение заголовка ${index + 1}`}
                    value={header.value}
                    placeholder="subshare"
                    onChange={(event) => setDelivery((current) => ({
                      ...current,
                      response_headers: current.response_headers.map((item, itemIndex) => itemIndex === index ? { ...item, value: event.target.value } : item),
                    }))}
                  />
                  <button
                    type="button"
                    aria-label={`Удалить заголовок ${index + 1}`}
                    onClick={() => setDelivery((current) => ({
                      ...current,
                      response_headers: current.response_headers.filter((_, itemIndex) => itemIndex !== index),
                    }))}
                    className="rounded-lg p-2 text-zinc-600 hover:bg-rose-500/10 hover:text-rose-300"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              ))}
              <Button
                variant="outline"
                onClick={() => setDelivery((current) => ({
                  ...current,
                  response_headers: [...current.response_headers, { key: "", value: "" }],
                }))}
              >
                <Plus className="h-4 w-4" />
                Добавить заголовок
              </Button>
            </div>
          )}

          {deliveryTab === "remarks" && (
            <div className="grid gap-4 lg:grid-cols-2">
              {(Object.keys(remarkLabels) as Array<keyof SubscriptionDeliverySettings["remarks"]>).map((status) => (
                <label key={status} className="flex flex-col gap-2 rounded-xl border border-border bg-zinc-950/30 p-4 text-sm text-zinc-400">
                  <span className="font-medium text-zinc-300">{remarkLabels[status]}</span>
                  <textarea
                    rows={4}
                    value={delivery.remarks[status].join("\n")}
                    onChange={(event) => setDelivery((current) => ({
                      ...current,
                      remarks: {
                        ...current.remarks,
                        [status]: event.target.value.split("\n").filter((line) => line.trim() !== "").slice(0, 10),
                      },
                    }))}
                    className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400"
                    placeholder="Каждая строка — отдельная ремарка"
                  />
                </label>
              ))}
            </div>
          )}
        </fieldset>

        <div className="flex items-center justify-between border-t border-border px-5 py-4">
          <span className={`text-xs ${deliveryDirty ? "text-amber-300" : "text-zinc-600"}`}>
            {deliveryDirty ? "Есть несохранённые изменения" : "Все изменения сохранены"}
          </span>
          <Button onClick={saveDelivery} loading={savingDelivery} disabled={!deliveryDirty || !isOwner}>
            <Save className="h-4 w-4" />
            Сохранить
          </Button>
        </div>
      </section>
      <GlobalSubscriptionSettingsModal open={settingsOpen} onClose={() => setSettingsOpen(false)} onSaved={load} />
      <RoutingSettingsModal open={routingOpen} onClose={() => setRoutingOpen(false)} />
    </div>
  );
}
