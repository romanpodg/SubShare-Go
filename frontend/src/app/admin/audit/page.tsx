"use client";

import { useCallback, useEffect, useState } from "react";
import { ChevronLeft, ChevronRight, ScrollText } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { AuditEvent, PageMeta } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";

export default function AuditPage() {
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [meta, setMeta] = useState<PageMeta>({
    page: 1,
    page_size: 25,
    total: 0,
    total_pages: 0,
  });
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const load = useCallback(async (page: number, initial = false) => {
    if (initial) setLoading(true);
    else setRefreshing(true);
    setError(null);
    try {
      const response = await apiV1.auditEvents(page, 25);
      setEvents(response.data);
      setMeta(response.meta);
    } catch (requestError) {
      setError(requestError);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => void load(1, true));
    return () => window.cancelAnimationFrame(frame);
  }, [load]);

  return (
    <div>
      <PageHeader
        title="Журнал действий"
        description="Структурированный аудит без ключей, токенов подписки и HWID."
        icon={<ScrollText className="h-5 w-5" />}
      />

      {loading && events.length === 0 ? (
        <InitialLoading label="Загрузка журнала…" />
      ) : error && events.length === 0 ? (
        <ResourceError error={error} onRetry={() => void load(meta.page, true)} />
      ) : (
        <>
          {Boolean(error) && (
            <div className="mb-4">
              <ResourceError
                error={error}
                compact
                onRetry={() => void load(meta.page)}
                title="Не удалось обновить журнал"
              />
            </div>
          )}
          <div
            className="ui-list-shell technical-frame"
            aria-busy={refreshing}
          >
            <div className="overflow-x-auto">
              <table className="ui-data-table min-w-[820px] text-left text-sm">
                <thead>
                  <tr>
                    <th className="px-5 py-3">Время</th>
                    <th className="px-5 py-3">Администратор</th>
                    <th className="px-5 py-3">Действие</th>
                    <th className="px-5 py-3">Объект</th>
                    <th className="px-5 py-3">Request ID</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {events.map((event) => (
                    <tr key={event.id} className="hover:bg-zinc-900/30">
                      <td className="whitespace-nowrap px-5 py-4 text-zinc-500">
                        {new Date(event.created_at).toLocaleString("ru-RU")}
                      </td>
                      <td className="px-5 py-4 font-medium text-zinc-300">{event.actor}</td>
                      <td className="px-5 py-4 text-cyan-300">{event.action}</td>
                      <td className="px-5 py-4 text-zinc-400">
                        {event.target_type} {event.target_id && `#${event.target_id}`}
                      </td>
                      <td className="px-5 py-4 font-mono text-xs text-zinc-600">
                        {event.request_id || "—"}
                      </td>
                    </tr>
                  ))}
                  {events.length === 0 && (
                    <tr>
                      <td colSpan={5} className="px-5 py-16 text-center text-zinc-600">
                        Журнал пока пуст
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
            <div className="ui-list-footer">
              <span className="text-xs text-zinc-600">{meta.total} событий</span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  disabled={meta.page <= 1 || refreshing}
                  onClick={() => void load(meta.page - 1)}
                  aria-label="Предыдущая страница"
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <span className="text-xs text-zinc-500">
                  {meta.page} / {Math.max(meta.total_pages, 1)}
                </span>
                <Button
                  variant="outline"
                  disabled={meta.page >= meta.total_pages || refreshing}
                  onClick={() => void load(meta.page + 1)}
                  aria-label="Следующая страница"
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
