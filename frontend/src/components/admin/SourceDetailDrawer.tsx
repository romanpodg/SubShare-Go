"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { History, RefreshCw, Save, Trash2 } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { SourceDetail, SourceSummary, SourceSyncRun } from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Drawer } from "@/components/ui/Drawer";
import { Input } from "@/components/ui/Input";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";

export function SourceDetailDrawer({
  source,
  open,
  onClose,
  onChanged,
}: {
  source: SourceSummary | null;
  open: boolean;
  onClose: () => void;
  onChanged: () => Promise<void>;
}) {
  const [detail, setDetail] = useState<SourceDetail | null>(null);
  const [runs, setRuns] = useState<SourceSyncRun[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [newHWID, setNewHWID] = useState("");
  const [clearHWID, setClearHWID] = useState(false);
  const activeRef = useRef(false);

  const load = useCallback(async () => {
    if (!source) return;
    setLoading(true);
    setError(null);
    const [detailResult, runsResult] = await Promise.allSettled([
      apiV1.sources.get(source.id),
      apiV1.sources.syncRuns(source.id),
    ]);
    if (detailResult.status === "fulfilled") {
      setDetail(detailResult.value.data);
    } else {
      setError(detailResult.reason);
    }
    if (runsResult.status === "fulfilled") setRuns(runsResult.value.data);
    setLoading(false);
  }, [source]);

  useEffect(() => {
    activeRef.current = open;
    if (open) void load();
    if (!open) {
      setDetail(null);
      setRuns([]);
      setError(null);
      setNewHWID("");
      setClearHWID(false);
    }
    return () => {
      activeRef.current = false;
    };
  }, [open, load]);

  const update = <K extends keyof SourceDetail>(key: K, value: SourceDetail[K]) => {
    setDetail((current) => current ? { ...current, [key]: value } : current);
  };

  const save = async () => {
    if (!detail) return;
    setSaving(true);
    setError(null);
    try {
      await apiV1.sources.update(detail.id, {
        name: detail.name,
        category: detail.category,
        key_category: detail.key_category,
        key_insert_mode: detail.key_insert_mode,
        source_url: detail.source_url,
        enabled: detail.enabled,
        apply_remote_metadata: false,
        pass_hwid: detail.pass_hwid,
        hwid_version: detail.hwid_version,
        hwid_model_name: detail.hwid_model_name,
        hwid_value: newHWID.trim() || undefined,
        clear_hwid_value: clearHWID,
      });
      setNewHWID("");
      setClearHWID(false);
      await onChanged();
      await load();
    } catch (requestError) {
      setError(requestError);
    } finally {
      setSaving(false);
    }
  };

  const pollJob = async (jobID: number) => {
    for (let attempt = 0; attempt < 60 && activeRef.current; attempt += 1) {
      const response = await apiV1.jobs.get(jobID);
      if (response.data.status === "succeeded") return;
      if (response.data.status === "failed") {
        throw new Error(response.data.error_message || "Синхронизация завершилась ошибкой");
      }
      await new Promise((resolve) => window.setTimeout(resolve, 2000));
    }
    if (activeRef.current) throw new Error("Синхронизация продолжается в фоне. Обновите страницу позже.");
  };

  const sync = async () => {
    if (!detail) return;
    setSyncing(true);
    setError(null);
    try {
      const queued = await apiV1.sources.sync(detail.id);
      await pollJob(queued.job_id);
      await onChanged();
      await load();
    } catch (requestError) {
      await onChanged();
      await load();
      setError(requestError);
    } finally {
      setSyncing(false);
    }
  };

  const remove = async () => {
    if (!detail) return;
    setDeleting(true);
    setError(null);
    try {
      await apiV1.sources.delete(detail.id);
      setConfirmDelete(false);
      onClose();
      await onChanged();
    } catch (requestError) {
      setError(requestError);
      setConfirmDelete(false);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <>
      <Drawer
        open={open}
        onClose={onClose}
        title={detail?.name || source?.name || "Источник"}
        description="Параметры, синхронизация и история запусков"
        footer={detail ? (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Button variant="danger" onClick={() => setConfirmDelete(true)} disabled={saving || syncing}>
              <Trash2 className="h-4 w-4" />
              Удалить
            </Button>
            <div className="flex gap-2">
              <Button variant="outline" onClick={() => void sync()} loading={syncing} disabled={saving}>
                <RefreshCw className="h-4 w-4" />
                Синхронизировать
              </Button>
              <Button onClick={() => void save()} loading={saving} disabled={syncing}>
                <Save className="h-4 w-4" />
                Сохранить
              </Button>
            </div>
          </div>
        ) : undefined}
      >
        {loading && !detail ? (
          <InitialLoading label="Загрузка источника…" />
        ) : error && !detail ? (
          <ResourceError error={error} onRetry={() => void load()} />
        ) : detail ? (
          <div className="space-y-6">
            {Boolean(error) && <ResourceError error={error} compact onRetry={() => void load()} title="Операция не выполнена" />}
            <section className="space-y-4 rounded-xl border border-border bg-surface-1 p-4">
              <Input label="Название" value={detail.name} maxLength={24} onChange={(event) => update("name", event.target.value)} />
              <Input label="URL источника" value={detail.source_url} onChange={(event) => update("source_url", event.target.value)} />
              <div className="grid gap-4 sm:grid-cols-2">
                <Input label="Категория источника" value={detail.category} onChange={(event) => update("category", event.target.value)} />
                <Input label="Категория ключей" value={detail.key_category} onChange={(event) => update("key_category", event.target.value)} />
              </div>
              <div className="ui-joined-grid grid sm:grid-cols-2">
                <label className="flex items-center gap-3 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-300">
                  <input type="checkbox" checked={detail.enabled} onChange={(event) => update("enabled", event.target.checked)} className="accent-cyan-500" />
                  Источник активен
                </label>
                <label className="flex items-center gap-3 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-300">
                  <input type="checkbox" checked={detail.pass_hwid} onChange={(event) => update("pass_hwid", event.target.checked)} className="accent-cyan-500" />
                  Передавать HWID
                </label>
              </div>
              {detail.pass_hwid && (
                <div className="space-y-3 rounded-lg border border-border p-3">
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Input label="Версия" value={detail.hwid_version} onChange={(event) => update("hwid_version", event.target.value)} />
                    <Input label="Модель" value={detail.hwid_model_name} onChange={(event) => update("hwid_model_name", event.target.value)} />
                  </div>
                  <Input
                    label={detail.has_hwid_value ? "Новый HWID (текущий сохранён и скрыт)" : "HWID"}
                    value={newHWID}
                    onChange={(event) => setNewHWID(event.target.value)}
                    placeholder={detail.has_hwid_value ? "Оставьте пустым, чтобы сохранить текущий" : ""}
                    disabled={clearHWID}
                  />
                  {detail.has_hwid_value && (
                    <label className="flex items-center gap-2 text-xs text-zinc-500">
                      <input type="checkbox" checked={clearHWID} onChange={(event) => setClearHWID(event.target.checked)} className="accent-rose-500" />
                      Удалить сохранённый HWID
                    </label>
                  )}
                </div>
              )}
            </section>

            <section className="ui-joined-grid grid sm:grid-cols-2">
              {[
                ["Состояние", detail.import_status],
                ["Импортировано", detail.imported_keys],
                ["Последний запуск", detail.last_synced_at || "Ещё не запускался"],
                ["URL в списке", detail.source_url_masked],
              ].map(([label, value]) => (
                <div key={String(label)} className="rounded-xl border border-border bg-surface-1 p-4">
                  <div className="text-xs text-zinc-600">{label}</div>
                  <div className="mt-1 break-words text-sm text-zinc-300">{value}</div>
                </div>
              ))}
            </section>

            <section className="overflow-hidden rounded-xl border border-border bg-surface-1">
              <div className="flex items-center gap-2 border-b border-border px-4 py-3 text-sm font-medium text-zinc-300">
                <History className="h-4 w-4" />
                История синхронизаций
              </div>
              <div className="divide-y divide-border">
                {runs.map((run) => (
                  <div key={run.id} className="px-4 py-3 text-sm">
                    <div className="flex items-center justify-between gap-3">
                      <span className={run.status === "succeeded" ? "text-emerald-300" : run.status === "failed" ? "text-rose-300" : "text-amber-300"}>
                        {run.status}
                      </span>
                      <time className="text-xs text-zinc-600">{new Date(run.started_at).toLocaleString("ru-RU")}</time>
                    </div>
                    <div className="mt-1 text-xs text-zinc-500">
                      Импортировано: {run.imported_count} · пропущено: {run.skipped_count}
                      {run.error_message ? ` · ${run.error_message}` : ""}
                    </div>
                  </div>
                ))}
                {runs.length === 0 && <div className="px-4 py-8 text-center text-sm text-zinc-600">Запусков пока нет</div>}
              </div>
            </section>
          </div>
        ) : null}
      </Drawer>

      <ConfirmDialog
        open={confirmDelete}
        title="Удалить источник?"
        message={`Будет удалён источник «${detail?.name || ""}» и ${detail?.imported_keys || 0} связанных ключей. Отменить действие будет нельзя.`}
        confirmLabel="Удалить источник"
        onConfirm={() => void remove()}
        onCancel={() => setConfirmDelete(false)}
        loading={deleting}
      />
    </>
  );
}
