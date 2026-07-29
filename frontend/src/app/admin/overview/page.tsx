"use client";

import { useCallback, useEffect, useState } from "react";
import { Activity, AlertTriangle, KeyRound, RadioTower, RefreshCw, ShieldCheck, Users } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { BackgroundJob, DashboardData } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { Button } from "@/components/ui/Button";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { useToast } from "@/components/ui/Toast";

export default function OverviewPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [jobs, setJobs] = useState<BackgroundJob[]>([]);
  const [dashboardError, setDashboardError] = useState<unknown>(null);
  const [jobsError, setJobsError] = useState<unknown>(null);
  const { toast } = useToast();

  const load = useCallback(async () => {
    setLoading(true);
    setDashboardError(null);
    setJobsError(null);
    const [dashboardResult, jobsResult] = await Promise.allSettled([
      apiV1.dashboard(),
      apiV1.jobs.list(1, 6),
    ]);
    if (dashboardResult.status === "fulfilled") {
      setData(dashboardResult.value);
    } else {
      setDashboardError(dashboardResult.reason);
    }
    if (jobsResult.status === "fulfilled") {
      setJobs(jobsResult.value.data);
    } else {
      setJobsError(jobsResult.reason);
    }
    setLoading(false);
  }, []);

  const retryJob = async (id: number) => {
    try {
      await apiV1.jobs.retry(id);
      toast("Задание поставлено в очередь повторно", "success");
      await load();
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось повторить задание", "error");
    }
  };

  useEffect(() => {
    const timer = window.setTimeout(() => void load(), 0);
    return () => window.clearTimeout(timer);
  }, [load]);

  return (
    <div>
      <PageHeader
        title="Состояние SubShare"
        description="Пользователи, доступность конфигураций и проблемы синхронизации в одном месте."
        icon={<Activity className="h-5 w-5" />}
        actions={
          <Button variant="outline" onClick={load} disabled={loading}>
            <RefreshCw className={`h-4 w-4 ${loading ? "animate-spin" : ""}`} />
            Обновить
          </Button>
        }
      />

      {loading && !data ? (
        <InitialLoading label="Загрузка состояния SubShare…" />
      ) : dashboardError && !data ? (
        <ResourceError error={dashboardError} onRetry={() => void load()} title="Не удалось загрузить обзор" />
      ) : data ? (
        <>
          {dashboardError && (
            <div className="mb-5">
              <ResourceError error={dashboardError} onRetry={() => void load()} compact title="Не удалось обновить обзор" />
            </div>
          )}
          {(data.degraded_sections?.length ?? 0) > 0 && (
            <div className="mb-5 rounded-xl border border-amber-400/25 bg-amber-400/5 px-4 py-3 text-sm text-amber-200" role="status">
              Часть данных временно недоступна: {data.degraded_sections.join(", ")}.
            </div>
          )}
          <div className="grid gap-px bg-border sm:grid-cols-2 xl:grid-cols-5">
            <StatCard label="Всего пользователей" value={data.users.total} icon={<Users className="h-5 w-5" />} />
            <StatCard label="Активные" value={data.users.active} icon={<ShieldCheck className="h-5 w-5" />} tone="emerald" />
            <StatCard label="Истекли" value={data.users.expired} icon={<AlertTriangle className="h-5 w-5" />} tone="rose" />
            <StatCard label="Ключи доступны" value={`${data.keys.up}/${data.keys.total}`} icon={<KeyRound className="h-5 w-5" />} tone={data.keys.down ? "amber" : "cyan"} />
            <StatCard label="Источники" value={data.sources.total} hint={data.sources.errors ? `Ошибок: ${data.sources.errors}` : "Ошибок нет"} icon={<RadioTower className="h-5 w-5" />} tone={data.sources.errors ? "rose" : "zinc"} />
          </div>

          <div className="grid gap-px bg-border xl:grid-cols-[1.4fr_1fr]">
            <section className="technical-frame border border-border bg-surface-1">
              <div className="border-b border-border px-5 py-4">
                <h2 className="font-semibold text-zinc-200">Последние действия</h2>
                <p className="mt-1 text-xs text-zinc-600">Журнал не содержит токенов, HWID и содержимого ключей.</p>
              </div>
              <div className="divide-y divide-border">
                {data.recent_audit_events.length === 0 ? (
                  <div className="px-5 py-12 text-center text-sm text-zinc-600">Действий пока нет</div>
                ) : data.recent_audit_events.map((event) => (
                  <div key={event.id} className="flex items-center justify-between gap-4 px-5 py-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm text-zinc-300">{event.action}</div>
                      <div className="mt-0.5 text-xs text-zinc-600">{event.actor} · {event.target_type} #{event.target_id}</div>
                    </div>
                    <time className="shrink-0 text-xs text-zinc-600">{new Date(event.created_at).toLocaleString("ru-RU")}</time>
                  </div>
                ))}
              </div>
            </section>

            <section className="technical-frame border border-border bg-surface-1 p-5">
              <h2 className="font-semibold text-zinc-200">Операционный статус</h2>
              <div className="mt-5 space-y-3">
                {[
                  ["Приостановлены", data.users.paused, "text-amber-300"],
                  ["Заблокированы", data.users.blocked, "text-rose-300"],
                  ["Достигли HWID-лимита", data.users.limited, "text-cyan-300"],
                  ["Подключённые устройства", data.devices, "text-zinc-200"],
                  ["Ключи с ошибкой", data.keys.down, "text-rose-300"],
                  ["Ключи без проверки", data.keys.unknown, "text-zinc-400"],
                  [
                    "Резервная копия",
                    data.backup.status === "healthy"
                      ? "готова"
                      : data.backup.status === "pending"
                        ? "ожидается"
                        : data.backup.status === "error"
                          ? "ошибка"
                          : "выключена",
                    data.backup.status === "healthy"
                      ? "text-emerald-300"
                      : data.backup.status === "error"
                        ? "text-rose-300"
                        : "text-zinc-400",
                  ],
                ].map(([label, value, tone]) => (
                  <div key={String(label)} className="flex items-center justify-between border-b border-border bg-zinc-950/35 px-4 py-3 last:border-b-0">
                    <span className="text-sm text-zinc-500">{label}</span>
                    <span className={`font-mono text-sm font-semibold ${tone}`}>{value}</span>
                  </div>
                ))}
              </div>
            </section>
          </div>
          <section className="technical-frame border border-border bg-surface-1">
            <div className="border-b border-border px-5 py-4">
              <h2 className="font-semibold text-zinc-200">Фоновые задания</h2>
              <p className="mt-1 text-xs text-zinc-600">Синхронизации источников и массовые проверки ключей.</p>
            </div>
            {Boolean(jobsError) && (
              <div className="p-4">
                <ResourceError error={jobsError} onRetry={() => void load()} compact title="Не удалось загрузить фоновые задания" />
              </div>
            )}
            <div className="divide-y divide-border">
              {!jobsError && jobs.map((job) => (
                <div key={job.id} className="grid gap-3 px-5 py-4 text-sm md:grid-cols-[180px_120px_1fr_auto] md:items-center">
                  <span className="font-medium text-zinc-300">{job.kind}</span>
                  <span className={job.status === "succeeded" ? "text-emerald-400" : job.status === "failed" ? "text-rose-400" : "text-amber-300"}>{job.status}</span>
                  <span className="truncate text-xs text-zinc-600">{job.error_message || `${job.target_type} ${job.target_id}`}</span>
                  {job.status === "failed" && <Button variant="outline" onClick={() => retryJob(job.id)}>Повторить</Button>}
                </div>
              ))}
              {!jobsError && jobs.length === 0 && <div className="px-5 py-10 text-center text-sm text-zinc-600">Заданий пока нет</div>}
            </div>
          </section>
        </>
      ) : (
        <ResourceError error={dashboardError} onRetry={() => void load()} title="Не удалось загрузить обзор" />
      )}
    </div>
  );
}
