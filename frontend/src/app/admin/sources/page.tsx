"use client";

import { FormEvent, useCallback, useEffect, useState } from "react";
import {
  AlertCircle,
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Plus,
  RadioTower,
  RefreshCw,
  Search,
} from "lucide-react";
import { apiV1 } from "@/lib/api";
import type {
  ExternalSourceCategory,
  KeyCategory,
  PageMeta,
  SourceSummary,
} from "@/lib/types";
import { SourceCreateDrawer } from "@/components/admin/SourceCreateDrawer";
import { SourceDetailDrawer } from "@/components/admin/SourceDetailDrawer";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { Select } from "@/components/ui/Select";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { useToast } from "@/components/ui/Toast";
import { useAuth } from "@/hooks/useAuth";

const INITIAL_META: PageMeta = {
  page: 1,
  page_size: 20,
  total: 0,
  total_pages: 1,
};

function formatDate(value: string) {
  if (!value) return "Ещё не запускался";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("ru-RU");
}

function sourceStatus(source: SourceSummary) {
  if (!source.enabled) return "non-active";
  if (source.import_status === "ok") return "active";
  if (source.import_status === "error") return "blocked";
  return "paused";
}

export default function SourcesPage() {
  const [sources, setSources] = useState<SourceSummary[]>([]);
  const [meta, setMeta] = useState<PageMeta>(INITIAL_META);
  const [queryInput, setQueryInput] = useState("");
  const [query, setQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [sort, setSort] = useState("last_sync_desc");
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [categoriesError, setCategoriesError] = useState<unknown>(null);
  const [sourceCategories, setSourceCategories] = useState<ExternalSourceCategory[]>([]);
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedSource, setSelectedSource] = useState<SourceSummary | null>(null);
  const { toast } = useToast();
  const { role } = useAuth();
  const isOwner = role === "owner";

  const loadSources = useCallback(
    async (page = meta.page, initial = false) => {
      if (initial) setLoading(true);
      else setRefreshing(true);
      setError(null);
      try {
        const response = await apiV1.sources.list({
          query,
          status,
          sort,
          page,
          page_size: meta.page_size,
        });
        setSources(response.data);
        setMeta(response.meta);
      } catch (requestError) {
        setError(requestError);
      } finally {
        setLoading(false);
        setRefreshing(false);
      }
    },
    [meta.page, meta.page_size, query, sort, status]
  );

  const loadCategories = useCallback(async () => {
    setCategoriesError(null);
    const [sourceResult, keyResult] = await Promise.allSettled([
      apiV1.sources.categories(),
      apiV1.sources.keyCategories(),
    ]);

    if (sourceResult.status === "fulfilled") {
      setSourceCategories(sourceResult.value.data);
    }
    if (keyResult.status === "fulfilled") {
      setKeyCategories(keyResult.value.data);
    }

    if (sourceResult.status === "rejected") setCategoriesError(sourceResult.reason);
    else if (keyResult.status === "rejected") setCategoriesError(keyResult.reason);
  }, []);

  useEffect(() => {
    void loadSources(1, true);
    void loadCategories();
    // The first request is intentionally issued once; subsequent filters are explicit.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    if (!loading) void loadSources(1);
    // Pagination is handled separately and must not trigger a second request.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query, status, sort]);

  const search = (event: FormEvent) => {
    event.preventDefault();
    setQuery(queryInput.trim());
  };

  const refresh = async () => {
    await loadSources(meta.page);
  };

  if (loading && sources.length === 0) {
    return <InitialLoading label="Загрузка внешних источников…" />;
  }

  return (
    <div>
      <PageHeader
        title="Внешние источники"
        description="Импортируйте сторонние подписки и контролируйте состояние синхронизации."
        icon={<RadioTower className="h-5 w-5" />}
        actions={isOwner ? (
          <Button
            onClick={() => setCreateOpen(true)}
            disabled={Boolean(categoriesError)}
            title={categoriesError ? "Сначала повторите загрузку справочников" : undefined}
          >
            <Plus className="h-4 w-4" />
            Добавить источник
          </Button>
        ) : undefined}
      />

      {Boolean(categoriesError) && (
        <div className="mb-4">
          <ResourceError
            error={categoriesError}
            onRetry={() => void loadCategories()}
            compact
            title="Не удалось загрузить категории"
          />
        </div>
      )}
      {!isOwner && (
        <div className="mb-4 rounded-xl border border-amber-400/20 bg-amber-400/5 px-4 py-3 text-sm text-amber-200">
          Режим просмотра: добавлять, синхронизировать и изменять источники может только владелец.
        </div>
      )}

      {Boolean(error) && sources.length > 0 && (
        <div className="mb-4">
          <ResourceError
            error={error}
            onRetry={() => void refresh()}
            compact
            title="Не удалось обновить список"
          />
        </div>
      )}

      {error && sources.length === 0 ? (
        <ResourceError error={error} onRetry={() => void loadSources(1, true)} />
      ) : (
        <>
          <section className="ui-toolbar-shell">
            <div className="ui-toolbar">
              <form className="ui-toolbar-search" onSubmit={search}>
                <label className="ui-field">
                  <span className="ui-field-label">Поиск источников</span>
                  <span className="relative block">
                  <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-zinc-600" />
                  <input
                    value={queryInput}
                    onChange={(event) => setQueryInput(event.target.value)}
                    placeholder="Название, категория или адрес"
                    className="ui-control w-full pl-9 pr-3 text-sm placeholder:text-zinc-600"
                  />
                  </span>
                </label>
                <Button type="submit" variant="outline">
                  Найти
                </Button>
              </form>

              <Select
                label="Состояние"
                value={status}
                onChange={(event) => setStatus(event.target.value)}
                className="min-w-44"
                options={[
                  { value: "all", label: "Все" },
                  { value: "ok", label: "Успешные" },
                  { value: "error", label: "С ошибкой" },
                  { value: "syncing", label: "Синхронизация" },
                  { value: "idle", label: "Не запускались" },
                  { value: "disabled", label: "Отключённые" },
                ]}
              />

              <Select
                label="Сортировка"
                value={sort}
                onChange={(event) => setSort(event.target.value)}
                className="min-w-56"
                options={[
                  { value: "last_sync_desc", label: "Последняя синхронизация" },
                  { value: "name_asc", label: "По названию" },
                  { value: "created_asc", label: "Сначала старые" },
                ]}
              />

              <Button variant="outline" onClick={() => void refresh()} loading={refreshing}>
                <RefreshCw className="h-4 w-4" />
                Обновить
              </Button>
            </div>
          </section>

          <section
            className="ui-list-shell technical-frame"
            aria-busy={refreshing}
          >
            <div className="overflow-x-auto">
              <table className="ui-data-table min-w-[960px] text-left">
                <thead>
                  <tr>
                    <th className="px-5 py-3">Источник</th>
                    <th className="px-5 py-3">Категории</th>
                    <th className="px-5 py-3">Состояние</th>
                    <th className="px-5 py-3">Ключи</th>
                    <th className="px-5 py-3">Последний запуск</th>
                    <th className="px-5 py-3 text-right">Действия</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {sources.map((source) => (
                    <tr
                      key={source.id}
                      tabIndex={isOwner ? 0 : undefined}
                      onClick={() => {
                        if (isOwner) setSelectedSource(source);
                      }}
                      onKeyDown={(event) => {
                        if (event.key === "Enter" || event.key === " ") {
                          event.preventDefault();
                          if (isOwner) setSelectedSource(source);
                        }
                      }}
                      className={`${isOwner ? "cursor-pointer" : ""} transition hover:bg-zinc-900/30 focus-visible:bg-zinc-900/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent`}
                    >
                      <td className="px-5 py-4">
                        <div className="font-medium text-zinc-200">{source.name}</div>
                        <div
                          className="mt-1 max-w-[340px] truncate font-mono text-xs text-zinc-600"
                          title={source.source_url_masked}
                        >
                          {source.source_url_masked}
                        </div>
                      </td>
                      <td className="px-5 py-4 text-zinc-400">
                        <div>{source.category || "Без категории"}</div>
                        <div className="mt-1 text-xs text-zinc-600">
                          Ключи: {source.key_category || "без категории"}
                        </div>
                      </td>
                      <td className="px-5 py-4">
                        <StatusBadge status={sourceStatus(source)} />
                        {source.last_error && (
                          <div
                            className="mt-1 flex max-w-[260px] items-center gap-1 truncate text-xs text-rose-400"
                            title={source.last_error}
                          >
                            <AlertCircle className="h-3 w-3 shrink-0" />
                            {source.last_error}
                          </div>
                        )}
                      </td>
                      <td className="px-5 py-4">
                        <div className="font-mono text-zinc-300">{source.imported_keys}</div>
                        <div className="mt-1 text-xs text-zinc-600">
                          За последний запуск: {source.last_import_count}
                        </div>
                      </td>
                      <td className="px-5 py-4 text-zinc-500">
                        {formatDate(source.last_synced_at)}
                      </td>
                      <td className="px-5 py-4 text-right">
                        {isOwner ? <Button
                          variant="outline"
                          onClick={(event) => {
                            event.stopPropagation();
                            setSelectedSource(source);
                          }}
                        >
                          <ExternalLink className="h-4 w-4" />
                          Открыть
                        </Button> : <span className="text-xs text-zinc-700">Только просмотр</span>}
                      </td>
                    </tr>
                  ))}
                  {sources.length === 0 && (
                    <tr>
                      <td colSpan={6} className="px-5 py-16 text-center">
                        <RadioTower className="mx-auto h-8 w-8 text-zinc-700" />
                        <div className="mt-3 text-sm text-zinc-400">Источники не найдены</div>
                        <div className="mt-1 text-xs text-zinc-600">
                          Измените фильтры или добавьте первый источник.
                        </div>
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            <div className="ui-list-footer text-sm">
              <span className="text-zinc-600">
                {meta.total === 0
                  ? "0 источников"
                  : `${(meta.page - 1) * meta.page_size + 1}–${Math.min(
                      meta.page * meta.page_size,
                      meta.total
                    )} из ${meta.total}`}
              </span>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  disabled={meta.page <= 1 || refreshing}
                  onClick={() => void loadSources(meta.page - 1)}
                  aria-label="Предыдущая страница"
                >
                  <ChevronLeft className="h-4 w-4" />
                </Button>
                <span className="min-w-20 text-center text-zinc-500">
                  {meta.page} / {Math.max(meta.total_pages, 1)}
                </span>
                <Button
                  variant="outline"
                  disabled={meta.page >= meta.total_pages || refreshing}
                  onClick={() => void loadSources(meta.page + 1)}
                  aria-label="Следующая страница"
                >
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          </section>
        </>
      )}

      <div className="sr-only" aria-live="polite">
        {refreshing ? "Список источников обновляется" : ""}
      </div>

      <SourceCreateDrawer
        open={createOpen}
        sourceCategories={sourceCategories}
        keyCategories={keyCategories}
        onClose={() => setCreateOpen(false)}
        onCreated={async (source) => {
          setCreateOpen(false);
          toast(`Источник «${source.name}» добавлен`, "success");
          await loadSources(1);
        }}
      />

      <SourceDetailDrawer
        source={selectedSource}
        open={Boolean(selectedSource)}
        onClose={() => setSelectedSource(null)}
        onChanged={async () => {
          await loadSources(meta.page);
        }}
      />
    </div>
  );
}
