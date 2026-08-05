"use client";

import { FormEvent, useCallback, useEffect, useMemo, useState } from "react";
import {
  ChevronLeft,
  ChevronRight,
  Fingerprint,
  KeyRound,
  Plus,
  Search,
  Settings2,
  Trash2,
  UserRound,
  Users,
  X,
} from "lucide-react";
import { apiV1, keys as keysApi, users as usersApi } from "@/lib/api";
import type { KeySummary, PageMeta, User, UserSummary } from "@/lib/types";
import { AddUserModal } from "@/components/admin/AddUserModal";
import { EditSubscriptionModal } from "@/components/admin/EditSubscriptionModal";
import { HwidManager } from "@/components/admin/HwidManager";
import { KeyAssignerModal } from "@/components/admin/KeyAssignerModal";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Input } from "@/components/ui/Input";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";
import { Select } from "@/components/ui/Select";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { useToast } from "@/components/ui/Toast";

type DrawerTab = "subscription" | "keys" | "devices" | "history";

export default function UsersPage() {
  const [items, setItems] = useState<UserSummary[]>([]);
  const [meta, setMeta] = useState<PageMeta>({ page: 1, page_size: 25, total: 0, total_pages: 0 });
  const [query, setQuery] = useState("");
  const [submittedQuery, setSubmittedQuery] = useState("");
  const [status, setStatus] = useState("all");
  const [sort, setSort] = useState("created_desc");
  const [loading, setLoading] = useState(true);
  const [selectedIDs, setSelectedIDs] = useState<number[]>([]);
  const [deleteIDs, setDeleteIDs] = useState<number[] | null>(null);
  const [adding, setAdding] = useState(false);
  const [detail, setDetail] = useState<User | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [drawerTab, setDrawerTab] = useState<DrawerTab>("subscription");
  const [keys, setKeys] = useState<KeySummary[]>([]);
  const [editSubscription, setEditSubscription] = useState(false);
  const [editKeys, setEditKeys] = useState(false);
  const [editHWID, setEditHWID] = useState(false);
  const { toast } = useToast();

  const load = useCallback(async (page = meta.page) => {
    setLoading(true);
    try {
      const response = await apiV1.users({
        query: submittedQuery,
        status,
        sort,
        page,
        page_size: meta.page_size,
      });
      setItems(response.data);
      setMeta(response.meta);
      setSelectedIDs((current) => current.filter((id) => response.data.some((user) => user.id === id)));
    } finally {
      setLoading(false);
    }
  }, [meta.page, meta.page_size, sort, status, submittedQuery]);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      load(1).catch(() => setLoading(false));
      keysApi.listSummaries({ page_size: 200 }).then((response) => setKeys(response.data || [])).catch(() => undefined);
    });
    return () => window.cancelAnimationFrame(frame);
  }, [load]);

  const openDetail = async (id: number) => {
    setDetailLoading(true);
    setDrawerTab("subscription");
    try {
      const response = await apiV1.user(id);
      setDetail(response.data);
    } catch {
      toast("Не удалось загрузить пользователя", "error");
    } finally {
      setDetailLoading(false);
    }
  };

  const refreshDetail = async () => {
    await load(meta.page);
    if (detail) {
      const response = await apiV1.user(detail.id);
      setDetail(response.data);
    }
  };

  const submitSearch = (event: FormEvent) => {
    event.preventDefault();
    setSubmittedQuery(query.trim());
    setMeta((current) => ({ ...current, page: 1 }));
  };

  const allSelected = items.length > 0 && items.every((item) => selectedIDs.includes(item.id));
  const assignedKeys = useMemo(() => {
    if (!detail?.assigned_key_ids) return [];
    const ids = new Set(detail.assigned_key_ids.split(",").map(Number));
    return keys.filter((key) => ids.has(key.id));
  }, [detail, keys]);
  const connectedDevices = Array.isArray(detail?.connected_devices)
    ? detail.connected_devices
    : [];

  const confirmDelete = async () => {
    if (!deleteIDs?.length) return;
    const results = await Promise.allSettled(deleteIDs.map((id) => usersApi.delete(id)));
    const failed = results.filter((result) => result.status === "rejected").length;
    setDeleteIDs(null);
    if (detail && deleteIDs.includes(detail.id)) setDetail(null);
    await load(meta.page);
    toast(failed ? `Не удалено: ${failed}` : `Удалено: ${deleteIDs.length}`, failed ? "error" : "success");
  };

  return (
    <div>
      <PageHeader
        title="Пользователи"
        description="Серверная пагинация, effective status, массовые действия и подробности в боковой панели."
        icon={<Users className="h-5 w-5" />}
        actions={
          <Button onClick={() => setAdding(true)}>
            <Plus className="h-4 w-4" />
            Добавить
          </Button>
        }
      />

      <section className="ui-list-shell technical-frame">
        <div className="ui-list-filters xl:flex-row">
          <form onSubmit={submitSearch} className="flex min-w-0 flex-1 gap-2">
            <Input
              aria-label="Поиск пользователей"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Имя или Telegram"
              className="min-w-0"
            />
            <Button type="submit" variant="outline">
              <Search className="h-4 w-4" />
              Найти
            </Button>
          </form>
          <div className="ui-toolbar-actions">
            <Select
              ariaLabel="Effective status"
              value={status}
              onChange={(event) => setStatus(event.target.value)}
              className="w-44 min-w-44"
              options={[
                { value: "all", label: "Все статусы" },
                { value: "active", label: "Active" },
                { value: "expired", label: "Expired" },
                { value: "paused", label: "Paused" },
                { value: "blocked", label: "Blocked" },
                { value: "limited", label: "Limited" },
              ]}
            />
            <Select
              ariaLabel="Сортировка"
              value={sort}
              onChange={(event) => setSort(event.target.value)}
              className="w-52 min-w-52"
              options={[
                { value: "created_desc", label: "Сначала новые" },
                { value: "name_asc", label: "По имени" },
                { value: "expires_asc", label: "По сроку" },
              ]}
            />
            {selectedIDs.length > 0 && (
              <Button variant="danger" onClick={() => setDeleteIDs(selectedIDs)}>
                <Trash2 className="h-4 w-4" />
                Удалить ({selectedIDs.length})
              </Button>
            )}
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="ui-data-table min-w-[940px] text-left">
            <thead>
              <tr>
                <th className="w-12 px-4 py-3">
                  <input
                    aria-label="Выбрать всех пользователей"
                    type="checkbox"
                    checked={allSelected}
                    onChange={() => setSelectedIDs(allSelected ? [] : items.map((item) => item.id))}
                    className="accent-accent"
                  />
                </th>
                <th className="px-4 py-3">Пользователь</th>
                <th className="px-4 py-3">Статус</th>
                <th className="px-4 py-3">Срок</th>
                <th className="px-4 py-3">Устройства</th>
                <th className="px-4 py-3 text-right">Детали</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {items.map((user) => (
                <tr key={user.id} className="transition hover:bg-zinc-900/30">
                  <td className="px-4 py-3">
                    <input
                      aria-label={`Выбрать ${user.name}`}
                      type="checkbox"
                      checked={selectedIDs.includes(user.id)}
                      onChange={() => setSelectedIDs((current) => current.includes(user.id) ? current.filter((id) => id !== user.id) : [...current, user.id])}
                      className="accent-accent"
                    />
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-medium text-zinc-200">{user.name}</div>
                    <div className="mt-0.5 text-xs text-zinc-600">{user.email ? `@${user.email}` : "Без Telegram"}</div>
                  </td>
                  <td className="px-4 py-3"><StatusBadge status={user.effective_status} /></td>
                  <td className="px-4 py-3 text-zinc-500">{user.expires_at || "Без ограничения"}</td>
                  <td className="px-4 py-3 font-mono text-zinc-400">{user.connected_device_count}/{user.max_devices > 0 ? user.max_devices : "∞"}</td>
                  <td className="px-4 py-3 text-right">
                    <Button variant="outline" onClick={() => openDetail(user.id)}>
                      <UserRound className="h-4 w-4" />
                      Открыть
                    </Button>
                  </td>
                </tr>
              ))}
              {!loading && items.length === 0 && (
                <tr><td colSpan={6} className="px-5 py-16 text-center text-zinc-600">Пользователи не найдены</td></tr>
              )}
            </tbody>
          </table>
          {loading && <div className="flex min-h-48 items-center justify-center"><LoadingSpinner /></div>}
        </div>

        <div className="ui-list-footer">
          <span className="text-xs text-zinc-600">Всего: {meta.total} · страница {meta.page} из {Math.max(meta.total_pages, 1)}</span>
          <div className="flex gap-2">
            <Button variant="outline" disabled={meta.page <= 1 || loading} onClick={() => load(meta.page - 1)}>
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <Button variant="outline" disabled={meta.page >= meta.total_pages || loading} onClick={() => load(meta.page + 1)}>
              <ChevronRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </section>

      {(detail || detailLoading) && (
        <div className="fixed inset-0 z-50 flex justify-end bg-black/55" onMouseDown={(event) => {
          if (event.target === event.currentTarget) setDetail(null);
        }}>
          <aside
            role="dialog"
            aria-label="Детали пользователя"
            className="h-full w-full max-w-xl overflow-y-auto border-l border-border bg-bg shadow-2xl"
          >
            <div className="sticky top-0 z-10 flex items-center justify-between border-b border-border bg-bg/95 px-5 py-4 backdrop-blur">
              <div>
                <h2 className="font-semibold text-zinc-100">{detail?.name || "Загрузка…"}</h2>
                <p className="mt-1 text-xs text-zinc-600">{detail?.email ? `@${detail.email}` : ""}</p>
              </div>
              <button type="button" onClick={() => setDetail(null)} aria-label="Закрыть детали" className="ui-icon-button">
                <X className="h-5 w-5" />
              </button>
            </div>
            {detailLoading && !detail ? (
              <div className="flex min-h-80 items-center justify-center"><LoadingSpinner /></div>
            ) : detail && (
              <>
                <div className="p-5">
                  <div className="ui-joined-grid grid grid-cols-2">
                    <div className="rounded-xl border border-border bg-surface-1 p-3"><div className="text-xs text-zinc-600">Effective status</div><div className="mt-2"><StatusBadge status={detail.effective_status} /></div></div>
                    <div className="rounded-xl border border-border bg-surface-1 p-3"><div className="text-xs text-zinc-600">HWID</div><div className="mt-2 font-mono text-zinc-300">{detail.connected_device_count}/{detail.max_devices > 0 ? detail.max_devices : "∞"}</div></div>
                  </div>
                </div>
                <div className="flex overflow-x-auto border-y border-border px-5">
                  {([
                    ["subscription", "Подписка"],
                    ["keys", "Ключи"],
                    ["devices", "Устройства"],
                    ["history", "История"],
                  ] as const).map(([value, label]) => (
                    <button key={value} type="button" onClick={() => setDrawerTab(value)} role="tab" aria-selected={drawerTab === value} className="ui-tab ui-tab--underline px-3 py-3 text-sm">
                      {label}
                    </button>
                  ))}
                </div>
                <div className="p-5">
                  {drawerTab === "subscription" && (
                    <div>
                      <div className="ui-joined-list">
                        <DetailRow label="Сохранённый статус" value={detail.status} />
                        <DetailRow label="Начало" value={detail.starts_at || "—"} />
                        <DetailRow label="Окончание" value={detail.expires_at || "—"} />
                        <DetailRow label="Название" value={detail.subscription_name || "—"} />
                      </div>
                      <Button className="mt-3" onClick={() => setEditSubscription(true)}><Settings2 className="h-4 w-4" />Изменить подписку</Button>
                    </div>
                  )}
                  {drawerTab === "keys" && (
                    <div>
                      <div className="ui-joined-list">
                        {assignedKeys.map((key) => <DetailRow key={key.id} label={key.label} value={key.check_status} />)}
                        {assignedKeys.length === 0 && <EmptyDrawerState text="Ключи не назначены" />}
                      </div>
                      <Button className="mt-3" onClick={() => setEditKeys(true)}><KeyRound className="h-4 w-4" />Назначить ключи</Button>
                    </div>
                  )}
                  {drawerTab === "devices" && (
                    <div>
                      <div className="ui-joined-list">
                        {connectedDevices.map((device) => (
                          <div key={device.normalized_hwid || device.hwid} className="border border-border bg-surface-1 p-4 transition-colors hover:border-[var(--border-strong)]">
                            <div className="font-medium text-zinc-300">{device.device_name || device.device_model || "Устройство"}</div>
                            <div className="mt-1 text-xs text-zinc-600">{device.platform} {device.os_version} · {device.app_name}</div>
                          </div>
                        ))}
                        {connectedDevices.length === 0 && <EmptyDrawerState text="Устройства ещё не подключались" />}
                      </div>
                      <Button className="mt-3" onClick={() => setEditHWID(true)}><Fingerprint className="h-4 w-4" />Управление HWID</Button>
                    </div>
                  )}
                  {drawerTab === "history" && <EmptyDrawerState text="События пользователя доступны в разделе Audit с фильтром target=user." />}
                </div>
              </>
            )}
          </aside>
        </div>
      )}

      <AddUserModal open={adding} onClose={() => setAdding(false)} onRefresh={() => load(1)} />
      {detail && editSubscription && <EditSubscriptionModal user={detail} onClose={() => setEditSubscription(false)} onRefresh={refreshDetail} />}
      {detail && editKeys && <KeyAssignerModal user={detail} assignableKeys={keys.filter((key) => key.status === "active")} onClose={() => setEditKeys(false)} onRefresh={refreshDetail} />}
      {detail && editHWID && <HwidManager user={detail} onClose={() => setEditHWID(false)} onRefresh={refreshDetail} />}
      <ConfirmDialog
        open={Boolean(deleteIDs?.length)}
        title="Удалить пользователей?"
        message="Пользователи, их устройства и персональные ссылки будут удалены."
        confirmLabel="Удалить"
        onCancel={() => setDeleteIDs(null)}
        onConfirm={confirmDelete}
      />
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return <div className="flex min-h-12 items-center justify-between gap-4 border border-border bg-surface-1 px-4 py-3 transition-colors hover:border-[var(--border-strong)]"><span className="text-sm text-zinc-600">{label}</span><span className="max-w-[65%] truncate font-mono text-xs text-zinc-300">{value}</span></div>;
}

function EmptyDrawerState({ text }: { text: string }) {
  return <div className="border border-dashed border-border px-4 py-8 text-center text-sm text-zinc-600">{text}</div>;
}
