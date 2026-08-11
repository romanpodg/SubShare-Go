"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Globe2, Plus, Route, Save, Settings, Trash2 } from "lucide-react";
import { apiV1, routingSettings as routingSettingsApi, subscriptionSettings } from "@/lib/api";
import type { RoutingSettings, SubscriptionDeliverySettingsUpdate, SubscriptionSettings } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { GlobalSubscriptionSettingsModal } from "@/components/admin/GlobalSubscriptionSettingsModal";
import { RoutingSettingsModal } from "@/components/admin/RoutingSettingsModal";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { useAuth } from "@/hooks/useAuth";

const emptyDeliverySettings = (): SubscriptionDeliverySettingsUpdate => ({
  response_headers: [],
  remarks: { expired: [], paused: [], blocked: [], limited: [], empty: [] },
});

const remarkLabels: Record<keyof SubscriptionDeliverySettingsUpdate["remarks"], string> = {
  expired: "Подписка истекла",
  paused: "Подписка приостановлена",
  blocked: "Подписка заблокирована",
  limited: "Превышен HWID-лимит",
  empty: "Нет доступных ключей",
};

type RemarkStatus = keyof SubscriptionDeliverySettingsUpdate["remarks"];

const remarkStatuses = Object.keys(remarkLabels) as RemarkStatus[];

const routingModeLabels = {
	disabled: "Не передаётся",
	add: "Добавлять профиль",
	onadd: "Добавлять и активировать",
} as const;

function normalizeDeliverySettingsForSave(
  value: SubscriptionDeliverySettingsUpdate
): SubscriptionDeliverySettingsUpdate {
  const normalizeRemarks = (remarks: string[]) =>
    remarks.map((remark) => remark.trim()).filter((remark) => remark !== "");

  return {
    ...value,
    remarks: {
      expired: normalizeRemarks(value.remarks.expired),
      paused: normalizeRemarks(value.remarks.paused),
      blocked: normalizeRemarks(value.remarks.blocked),
      limited: normalizeRemarks(value.remarks.limited),
      empty: normalizeRemarks(value.remarks.empty),
    },
  };
}

export default function SubscriptionSettingsPage() {
  const [settings, setSettings] = useState<SubscriptionSettings | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
	const [routingOpen, setRoutingOpen] = useState(false);
	const [routing, setRouting] = useState<RoutingSettings | null>(null);
  const [delivery, setDelivery] = useState<SubscriptionDeliverySettingsUpdate>(emptyDeliverySettings);
  const [initialDelivery, setInitialDelivery] = useState("");
  const [deliveryTab, setDeliveryTab] = useState<"headers" | "remarks">("headers");
  const [savingDelivery, setSavingDelivery] = useState(false);
  const [loading, setLoading] = useState(true);
  const [settingsError, setSettingsError] = useState<unknown>(null);
	const [deliveryError, setDeliveryError] = useState<unknown>(null);
	const [routingError, setRoutingError] = useState<unknown>(null);
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
      const mutable: SubscriptionDeliverySettingsUpdate = {
        response_headers: data.response_headers,
        remarks: data.remarks,
      };
      setDelivery(mutable);
      setInitialDelivery(JSON.stringify(mutable));
    } catch (requestError) {
      setDeliveryError(requestError);
    }
	}, []);

	const loadRouting = useCallback(async () => {
		setRoutingError(null);
		try {
			setRouting(await routingSettingsApi.get());
		} catch (requestError) {
			setRoutingError(requestError);
		}
	}, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
		void Promise.allSettled([load(), loadDelivery(), loadRouting()]).finally(() => setLoading(false));
    });
    return () => window.cancelAnimationFrame(frame);
	}, [load, loadDelivery, loadRouting]);

  const deliveryDirty = useMemo(
    () => initialDelivery !== "" && JSON.stringify(delivery) !== initialDelivery,
    [delivery, initialDelivery]
  );

  const saveDelivery = async () => {
    if (!isOwner) return;
    setSavingDelivery(true);
    try {
      const normalized = normalizeDeliverySettingsForSave(delivery);
      await apiV1.deliverySettings.update(normalized);
      setDelivery(normalized);
      setInitialDelivery(JSON.stringify(normalized));
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
		{Boolean(routingError) && <div className="mb-4"><ResourceError error={routingError} compact onRetry={() => void loadRouting()} title="Не удалось загрузить маршрутизацию" /></div>}
      {!isOwner && (
        <div className="mb-4 rounded-xl border border-amber-400/20 bg-amber-400/5 px-4 py-3 text-sm text-amber-200">
          Режим просмотра: изменять настройки может только владелец.
        </div>
      )}
      <div className="ui-joined-grid technical-frame grid lg:grid-cols-2">
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <div className="ui-settings-section-icon flex h-11 w-11 shrink-0 items-center justify-center self-start rounded-sm border">
                <Globe2 className="h-5 w-5" aria-hidden="true" />
              </div>
              <div className="min-w-0">
                <h2 className="font-semibold text-zinc-200">Метаданные</h2>
                <p className="mt-1 text-sm text-zinc-600">Название, интервал обновления, ссылки и заголовки клиента.</p>
              </div>
            </div>
            <Button variant="outline" onClick={() => setSettingsOpen(true)} disabled={!isOwner}>Изменить</Button>
          </div>
          <dl className="ui-joined-grid mt-6 grid grid-cols-2">
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Название</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.title || "—"}</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Формат fallback</dt><dd className="mt-1 font-mono text-sm text-info">{settings?.subscription_format || "—"}</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Обновление</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.refresh_hours ?? "—"} ч</dd></div>
            <div className="rounded-xl border border-border bg-zinc-950/30 p-3"><dt className="text-xs text-zinc-600">Язык</dt><dd className="mt-1 text-sm text-zinc-300">{settings?.language || "—"}</dd></div>
          </dl>
        </section>

        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-3">
              <div className="ui-settings-section-icon flex h-11 w-11 shrink-0 items-center justify-center self-start rounded-sm border">
                <Route className="h-5 w-5" aria-hidden="true" />
              </div>
              <div className="min-w-0">
                <h2 className="font-semibold text-zinc-200">Маршрутизация Happ</h2>
                <p className="mt-1 text-sm text-zinc-600">DNS, исключения и routing-конфигурация для автоматического импорта.</p>
              </div>
            </div>
            <Button variant="outline" onClick={() => setRoutingOpen(true)} disabled={!isOwner}>Открыть редактор</Button>
          </div>
			<dl className="mt-6 grid gap-2 rounded-xl border border-border bg-zinc-950/30 p-4 text-sm sm:grid-cols-2">
				<div>
					<dt className="text-xs text-zinc-600">Статус</dt>
					<dd className={`mt-1 font-medium ${routing?.delivery_mode && routing.delivery_mode !== "disabled" ? "text-accent" : "text-zinc-400"}`}>
						{routing?.delivery_mode && routing.delivery_mode !== "disabled" ? "Передаётся с подпиской" : "Не передаётся"}
					</dd>
				</div>
				<div>
					<dt className="text-xs text-zinc-600">Режим</dt>
					<dd className="mt-1 text-zinc-300">{routing ? routingModeLabels[routing.delivery_mode] : "—"}</dd>
				</div>
			</dl>
        </section>
      </div>

      <section className="technical-frame border border-border bg-surface-1">
        <div className="flex flex-col justify-between gap-4 border-b border-border px-5 py-4 lg:flex-row lg:items-center">
          <div>
            <h2 className="font-semibold text-zinc-200">Выдача и сообщения клиентам</h2>
            <p className="mt-1 text-xs text-zinc-600">Безопасные глобальные заголовки и сообщения для operational status.</p>
          </div>
          <div className="flex flex-wrap items-center gap-2" role="tablist" aria-label="Настройки выдачи">
            {([
              ["headers", "Заголовки"],
              ["remarks", "Ремарки"],
            ] as const).map(([value, label]) => (
              <button
                key={value}
                type="button"
                onClick={() => setDeliveryTab(value)}
                role="tab"
                aria-selected={deliveryTab === value}
                className="ui-tab px-3 py-2 text-xs font-medium"
              >
                {label}
              </button>
            ))}
          </div>
        </div>

        <fieldset disabled={!isOwner} className="p-5">
          {deliveryTab === "headers" && (
            <div className="overflow-hidden rounded-sm border border-border bg-zinc-950/20" role="tabpanel" aria-label="Заголовки">
              <div className="border-b border-border px-4 py-3.5">
                <h3 className="text-balance text-sm font-semibold text-zinc-200">Заголовки ответа</h3>
                <p className="mt-1 max-w-3xl text-pretty text-xs leading-5 text-zinc-500">
                  Безопасные глобальные заголовки, которые добавляются к ответам подписки. Стандартные метаданные управляются отдельно.
                </p>
              </div>

              <div data-testid="response-header-list">
                {delivery.response_headers.length === 0 && (
                  <p className="px-4 py-4 text-pretty text-sm text-zinc-600">Дополнительные заголовки не настроены.</p>
                )}
                {delivery.response_headers.map((header, index) => (
                  <div
                    key={index}
                    data-testid="response-header-row"
                    className="grid grid-cols-[2.75rem_minmax(0,0.8fr)_minmax(0,1.2fr)] items-center gap-2 border-b border-border px-3 py-2.5 last:border-b-0"
                  >
                    <button
                      type="button"
                      aria-label={`Удалить заголовок ${index + 1}`}
                      onClick={() => setDelivery((current) => ({
                        ...current,
                        response_headers: current.response_headers.filter((_, itemIndex) => itemIndex !== index),
                      }))}
                      className="inline-flex size-10 items-center justify-center rounded-sm border border-transparent text-zinc-600 transition-colors hover:border-danger/25 hover:bg-danger/8 hover:text-danger"
                    >
                      <Trash2 className="size-4" aria-hidden="true" />
                    </button>
                    <Input
                      aria-label={`Имя заголовка ${index + 1}`}
                      value={header.key}
                      placeholder="Key"
                      maxLength={80}
                      onChange={(event) => setDelivery((current) => ({
                        ...current,
                        response_headers: current.response_headers.map((item, itemIndex) => itemIndex === index ? { ...item, key: event.target.value } : item),
                      }))}
                    />
                    <Input
                      aria-label={`Значение заголовка ${index + 1}`}
                      value={header.value}
                      placeholder="Value"
                      maxLength={1024}
                      onChange={(event) => setDelivery((current) => ({
                        ...current,
                        response_headers: current.response_headers.map((item, itemIndex) => itemIndex === index ? { ...item, value: event.target.value } : item),
                      }))}
                    />
                  </div>
                ))}
              </div>

              <div className="flex items-center border-t border-border px-4 py-3">
                <Button
                  variant="outline"
                  disabled={delivery.response_headers.length >= 30}
                  onClick={() => setDelivery((current) => ({
                    ...current,
                    response_headers: [...current.response_headers, { key: "", value: "" }],
                  }))}
                >
                  <Plus className="size-4" aria-hidden="true" />
                  Добавить заголовок
                </Button>
              </div>
            </div>
          )}

          {deliveryTab === "remarks" && (
            <div role="tabpanel" aria-label="Ремарки">
              <div className="mb-4">
                <h3 className="text-balance text-sm font-semibold text-zinc-200">Пользовательские ремарки</h3>
                <p className="mt-1 max-w-3xl text-pretty text-xs leading-5 text-zinc-500">
                  Короткие сообщения для состояний подписки. Каждая ремарка хранится отдельно и выводится в заданном порядке.
                </p>
              </div>

              <div className="grid gap-3 lg:grid-cols-2" data-testid="remark-grid">
                {remarkStatuses.map((status) => (
                  <section
                    key={status}
                    data-testid={`remark-card-${status}`}
                    className="overflow-hidden rounded-sm border border-border bg-zinc-950/20 last:lg:col-span-2"
                  >
                    <div className="flex min-h-14 items-center justify-between gap-3 px-4 py-3">
                      <div className="flex min-w-0 items-center gap-2.5">
                        <span className="size-2 shrink-0 rounded-full bg-accent" aria-hidden="true" />
                        <h4 className="truncate text-sm font-medium text-zinc-300">{remarkLabels[status]}</h4>
                      </div>
                      <button
                        type="button"
                        aria-label={`Добавить ремарку для ${remarkLabels[status]}`}
                        disabled={delivery.remarks[status].length >= 10}
                        onClick={() => setDelivery((current) => ({
                          ...current,
                          remarks: {
                            ...current.remarks,
                            [status]: [...current.remarks[status], ""],
                          },
                        }))}
                        className="inline-flex h-9 shrink-0 items-center justify-center gap-1.5 rounded-sm border border-border px-3 font-mono text-xs font-semibold text-zinc-300 transition-colors hover:border-accent/40 hover:bg-accent/8 hover:text-accent disabled:cursor-not-allowed disabled:opacity-45"
                      >
                        <Plus className="size-3.5" aria-hidden="true" />
                        Добавить
                      </button>
                    </div>

                    <div className="border-t border-border p-3">
                      {delivery.remarks[status].length === 0 && (
                        <p className="px-1 py-2 text-pretty text-xs text-zinc-600">Ремарок пока нет.</p>
                      )}
                      <div className="flex flex-col gap-2">
                        {delivery.remarks[status].map((remark, index) => (
                          <div key={index} className="grid grid-cols-[minmax(0,1fr)_2.5rem] items-center gap-2">
                            <Input
                              aria-label={`Ремарка ${index + 1}: ${remarkLabels[status]}`}
                              value={remark}
                              placeholder="Текст ремарки"
                              maxLength={200}
                              onChange={(event) => setDelivery((current) => ({
                                ...current,
                                remarks: {
                                  ...current.remarks,
                                  [status]: current.remarks[status].map((item, itemIndex) => itemIndex === index ? event.target.value : item),
                                },
                              }))}
                            />
                            <button
                              type="button"
                              aria-label={`Удалить ремарку ${index + 1}: ${remarkLabels[status]}`}
                              onClick={() => setDelivery((current) => ({
                                ...current,
                                remarks: {
                                  ...current.remarks,
                                  [status]: current.remarks[status].filter((_, itemIndex) => itemIndex !== index),
                                },
                              }))}
                              className="inline-flex size-10 items-center justify-center rounded-sm border border-transparent text-zinc-600 transition-colors hover:border-danger/25 hover:bg-danger/8 hover:text-danger"
                            >
                              <Trash2 className="size-4" aria-hidden="true" />
                            </button>
                          </div>
                        ))}
                      </div>
                    </div>
                  </section>
                ))}
              </div>
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
		<RoutingSettingsModal open={routingOpen} onClose={() => setRoutingOpen(false)} onSaved={setRouting} />
    </div>
  );
}
