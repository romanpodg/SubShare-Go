"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { externalSources, keys as keysApi } from "@/lib/api";
import type { ExternalSourceCategory, ExternalSubscriptionSource, KeyCategory, VLESSKey } from "@/lib/types";
import { EditKeyModal } from "./EditKeyModal";
import { CreateExternalCategoryModal } from "./CreateExternalCategoryModal";
import { ExternalSourceCategoriesModal } from "./ExternalSourceCategoriesModal";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";

interface Props {
  open: boolean;
  onClose: () => void;
  onChanged: () => Promise<void>;
}

const CREATE_CATEGORY_VALUE = "__create_category__";
const CREATE_KEY_CATEGORY_VALUE = "__create_key_category__";

function generateRandomHWID(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  if (typeof crypto !== "undefined" && typeof crypto.getRandomValues === "function") {
    crypto.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20, 32)}`;
}

function statusLabel(status: ExternalSubscriptionSource["import_status"]) {
  switch (status) {
    case "ok":
      return "Синхронизировано";
    case "error":
      return "Ошибка";
    case "syncing":
      return "Синхронизация";
    default:
      return "Ожидание";
  }
}

export function ExternalSourcesManagerModal({ open, onClose, onChanged }: Props) {
  const { toast } = useToast();
  const [sources, setSources] = useState<ExternalSubscriptionSource[]>([]);
  const [categories, setCategories] = useState<ExternalSourceCategory[]>([]);
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);
  const [externalKeys, setExternalKeys] = useState<VLESSKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [savingById, setSavingById] = useState<Record<number, boolean>>({});
  const [syncingById, setSyncingById] = useState<Record<number, boolean>>({});
  const [removingById, setRemovingById] = useState<Record<number, boolean>>({});
  const [deletingKeyID, setDeletingKeyID] = useState<number | null>(null);
  const [editKey, setEditKey] = useState<VLESSKey | null>(null);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [createCategoryTargetSourceID, setCreateCategoryTargetSourceID] = useState<number | null>(null);
  const [showCategoriesManager, setShowCategoriesManager] = useState(false);
  const [showCreateKeyCategory, setShowCreateKeyCategory] = useState(false);
  const [createKeyCategoryTargetSourceID, setCreateKeyCategoryTargetSourceID] = useState<number | null>(null);

  const grouped = useMemo(() => {
    const groups = new Map<string, ExternalSubscriptionSource[]>();
    for (const source of sources) {
      const category = source.category.trim() || "Общее";
      const list = groups.get(category) ?? [];
      list.push(source);
      groups.set(category, list);
    }
    return Array.from(groups.entries()).sort((a, b) => a[0].localeCompare(b[0], "ru"));
  }, [sources]);

  const categoryOptions = useMemo(() => {
    return categories
      .map((item) => ({
        value: item.name,
        label: `${item.name} (${item.sources_count})`,
      }))
      .sort((left, right) => left.label.localeCompare(right.label, "ru"));
  }, [categories]);

  const keyCategoryOptions = useMemo(() => {
    return [{ value: "", label: "Без категории" }]
      .concat(
        keyCategories.map((item) => ({
          value: item.name,
          label: `${item.name} (${item.keys_count})`,
        }))
      )
      .sort((left, right) => left.label.localeCompare(right.label, "ru"));
  }, [keyCategories]);

  const categoryOptionsForValue = useCallback(
    (value: string) => {
      const options = [...categoryOptions];
      const current = value.trim();
      if (current && !options.some((item) => item.value === current)) {
        options.push({ value: current, label: current });
      }
      options.push({ value: CREATE_CATEGORY_VALUE, label: "+ Создать категорию" });
      return options;
    },
    [categoryOptions]
  );

  const keyCategoryOptionsForValue = useCallback(
    (value: string) => {
      const options = [...keyCategoryOptions];
      const current = value.trim();
      if (current && !options.some((item) => item.value === current)) {
        options.push({ value: current, label: current });
      }
      options.push({ value: CREATE_KEY_CATEGORY_VALUE, label: "+ Создать категорию ключей" });
      return options;
    },
    [keyCategoryOptions]
  );

  const loadCategoriesOnly = useCallback(async () => {
    try {
      const [categoryResponse, keyCategoryResponse] = await Promise.all([
        externalSources.listCategories(),
        keysApi.listCategories(),
      ]);
      setCategories(categoryResponse.categories || []);
      setKeyCategories(keyCategoryResponse.categories || []);
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось загрузить категории", "error");
    }
  }, [toast]);

  const loadSources = useCallback(async () => {
    setLoading(true);
    try {
      const [response, keyResponse, categoryResponse, keyCategoryResponse] = await Promise.all([
        externalSources.list(),
        keysApi.list(),
        externalSources.listCategories(),
        keysApi.listCategories(),
      ]);
      setSources(
        (response.sources || []).map((source) => ({
          ...source,
          key_category: source.key_category || "",
          key_insert_mode: source.key_insert_mode || "bottom",
          pass_hwid: Boolean(source.pass_hwid),
          hwid_version: source.hwid_version || "",
          hwid_model_name: source.hwid_model_name || "",
          hwid_value: source.hwid_value || "",
        }))
      );
      setCategories(categoryResponse.categories || []);
      setKeyCategories(keyCategoryResponse.categories || []);
      setExternalKeys((keyResponse.keys || []).filter((key) => key.external_source_id > 0));
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось загрузить источники", "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (!open) {
      return;
    }
    void loadSources();
  }, [open, loadSources]);

  const updateLocalSource = (id: number, patch: Partial<ExternalSubscriptionSource>) => {
    setSources((previous) =>
      previous.map((source) => (source.id === id ? { ...source, ...patch } : source))
    );
  };

  const saveSource = async (source: ExternalSubscriptionSource) => {
    setSavingById((previous) => ({ ...previous, [source.id]: true }));
    try {
      await externalSources.update(source.id, {
        name: source.name,
        category: source.category,
        key_category: source.key_category || "",
        key_insert_mode: source.key_insert_mode || "bottom",
        source_url: source.source_url,
        enabled: source.enabled,
        pass_hwid: source.pass_hwid,
        hwid_version: source.hwid_version || "",
        hwid_model_name: source.hwid_model_name || "",
        hwid_value: source.hwid_value || "",
      });
      toast(`Источник "${source.name}" сохранён`, "success");
      await onChanged();
      await loadSources();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить источник", "error");
    } finally {
      setSavingById((previous) => ({ ...previous, [source.id]: false }));
    }
  };

  const syncSource = async (source: ExternalSubscriptionSource) => {
    setSyncingById((previous) => ({ ...previous, [source.id]: true }));
    try {
      const response = await externalSources.sync(source.id);
      const summary =
        response.skipped_count > 0
          ? `Синхронизировано: ${response.imported_count}, пропущено: ${response.skipped_count}`
          : `Синхронизировано ключей: ${response.imported_count}`;
      toast(summary, "success");
      await onChanged();
      await loadSources();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось синхронизировать источник", "error");
    } finally {
      setSyncingById((previous) => ({ ...previous, [source.id]: false }));
    }
  };

  const deleteSource = async (source: ExternalSubscriptionSource) => {
    const confirmed = window.confirm(
      `Удалить источник "${source.name}" и все связанные импортированные ключи?`
    );
    if (!confirmed) {
      return;
    }

    setRemovingById((previous) => ({ ...previous, [source.id]: true }));
    try {
      await externalSources.remove(source.id);
      toast(`Источник "${source.name}" удалён`, "success");
      await onChanged();
      await loadSources();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось удалить источник", "error");
    } finally {
      setRemovingById((previous) => ({ ...previous, [source.id]: false }));
    }
  };

  const deleteExternalKey = async (key: VLESSKey) => {
    const confirmed = window.confirm(`Удалить внешний ключ "${key.label}"?`);
    if (!confirmed) {
      return;
    }
    setDeletingKeyID(key.id);
    try {
      await keysApi.delete(key.id);
      toast(`Ключ "${key.label}" удалён`, "success");
      await onChanged();
      await loadSources();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось удалить ключ", "error");
    } finally {
      setDeletingKeyID(null);
    }
  };

  return (
    <>
      <Modal
        open={open}
        onClose={onClose}
        title="Управление сторонними ключами"
        className="max-w-5xl max-h-[90vh] overflow-y-auto"
      >
        <div className="space-y-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-sm text-zinc-400">
            Здесь можно редактировать источники, обновлять подписки и управлять импортированными ключами.
          </p>
          <div className="flex items-center gap-2">
            <Button variant="ghost" className="text-xs" onClick={() => setShowCategoriesManager(true)} disabled={loading}>
              Категории
            </Button>
            <Button variant="ghost" onClick={() => void loadSources()} loading={loading}>
              Обновить список
            </Button>
          </div>
        </div>

        {loading ? <div className="text-sm text-zinc-500">Загрузка источников...</div> : null}

        {!loading && grouped.length === 0 ? (
          <div className="rounded-lg border border-border bg-surface-2/50 px-4 py-8 text-center text-zinc-500">
            Пока нет сторонних источников. Добавьте источник через кнопку &quot;Экспорт подписки&quot;.
          </div>
        ) : null}

        {!loading &&
          grouped.map(([category, items]) => (
            <div key={category} className="space-y-3 rounded-xl border border-border bg-surface-2/40 p-3">
              <div className="text-sm font-semibold text-zinc-200">
                {category} ({items.length})
              </div>
              <div className="space-y-3">
                {items.map((source) => {
                  const sourceKeys = externalKeys.filter((key) => key.external_source_id === source.id);
                  const isBusy =
                    Boolean(savingById[source.id]) ||
                    Boolean(syncingById[source.id]) ||
                    Boolean(removingById[source.id]);
                  return (
                    <div key={source.id} className="rounded-lg border border-border bg-bg/30 p-3">
                      <div className="grid grid-cols-1 gap-3 md:grid-cols-3">
                        <Input
                          label="Название"
                          value={source.name}
                          onChange={(event) => updateLocalSource(source.id, { name: event.target.value })}
                          maxLength={24}
                          disabled={isBusy}
                        />
                        <Select
                          label="Категория"
                          value={source.category || "Общее"}
                          onChange={(event) => {
                            const nextValue = event.target.value;
                            if (nextValue === CREATE_CATEGORY_VALUE) {
                              setCreateCategoryTargetSourceID(source.id);
                              setShowCreateCategory(true);
                              return;
                            }
                            updateLocalSource(source.id, { category: nextValue });
                          }}
                          options={categoryOptionsForValue(source.category || "Общее")}
                          disabled={isBusy}
                        />
                        <div className="flex flex-col gap-1.5">
                          <label className="text-sm text-zinc-400">Статус</label>
                          <div
                            className={`rounded-lg border px-3 py-2 text-sm ${
                              source.import_status === "ok"
                                ? "border-green-500/30 bg-green-500/10 text-green-200"
                                : source.import_status === "error"
                                  ? "border-red-500/30 bg-red-500/10 text-red-200"
                                  : "border-border bg-surface-2 text-zinc-300"
                            }`}
                          >
                            {statusLabel(source.import_status)}
                          </div>
                        </div>
                      </div>

	                      <div className="mt-3">
	                        <Input
	                          label="URL источника"
	                          value={source.source_url}
	                          onChange={(event) => updateLocalSource(source.id, { source_url: event.target.value })}
	                          disabled={isBusy}
	                        />
	                      </div>

                      <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_240px]">
                        <Select
                          label="Категория ключей в панели"
                          value={source.key_category || ""}
                          onChange={(event) => {
                            const nextValue = event.target.value;
                            if (nextValue === CREATE_KEY_CATEGORY_VALUE) {
                              setCreateKeyCategoryTargetSourceID(source.id);
                              setShowCreateKeyCategory(true);
                              return;
                            }
                            updateLocalSource(source.id, { key_category: nextValue });
                          }}
                          options={keyCategoryOptionsForValue(source.key_category || "")}
                          disabled={isBusy}
                        />
                        <div className="flex flex-col gap-1.5">
                          <span className="text-sm text-zinc-400">Позиция в категории</span>
                          <div className="grid grid-cols-2 gap-1 rounded-xl border border-border bg-surface-2 p-1">
                            <button
                              type="button"
                              onClick={() => updateLocalSource(source.id, { key_insert_mode: "top" })}
                              className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                                (source.key_insert_mode || "bottom") === "top"
                                  ? "bg-accent text-white shadow-sm"
                                  : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                              }`}
                              disabled={isBusy}
                            >
                              В начало
                            </button>
                            <button
                              type="button"
                              onClick={() => updateLocalSource(source.id, { key_insert_mode: "bottom" })}
                              className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                                (source.key_insert_mode || "bottom") === "bottom"
                                  ? "bg-accent text-white shadow-sm"
                                  : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                              }`}
                              disabled={isBusy}
                            >
                              В конец
                            </button>
                          </div>
                        </div>
                      </div>

	                      <div className="mt-3 grid grid-cols-1 gap-2">
                        <label className="flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200">
                          <input
                            type="checkbox"
                            checked={source.enabled}
                            onChange={(event) => updateLocalSource(source.id, { enabled: event.target.checked })}
                            disabled={isBusy}
                          />
                          Источник активен
                        </label>
                      </div>

                      <div className="mt-3 rounded-lg border border-border bg-surface-2/40 p-3">
                        <label className="flex items-center gap-2 text-sm text-zinc-200">
                          <input
                            type="checkbox"
                            checked={source.pass_hwid}
                            onChange={(event) => updateLocalSource(source.id, { pass_hwid: event.target.checked })}
                            disabled={isBusy}
                          />
                          Передавать HWID
                        </label>
                        {source.pass_hwid ? (
                          <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-3">
                            <Input
                              label="Version"
                              value={source.hwid_version}
                              onChange={(event) => updateLocalSource(source.id, { hwid_version: event.target.value })}
                              disabled={isBusy}
                            />
                            <Input
                              label="Model name"
                              value={source.hwid_model_name}
                              onChange={(event) => updateLocalSource(source.id, { hwid_model_name: event.target.value })}
                              disabled={isBusy}
                            />
                            <div className="flex flex-col gap-1.5">
                              <label className="text-sm text-zinc-400">HWID</label>
                              <div className="relative">
                                <input
                                  value={source.hwid_value}
                                  onChange={(event) => updateLocalSource(source.id, { hwid_value: event.target.value })}
                                  disabled={isBusy}
                                  className="w-full bg-surface-2 border border-border rounded-lg px-3 py-2 pr-11 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                                />
                                <button
                                  type="button"
                                  onClick={() => updateLocalSource(source.id, { hwid_value: generateRandomHWID() })}
                                  disabled={isBusy}
                                  className="absolute right-1 top-1/2 -translate-y-1/2 h-8 w-8 rounded-md border border-border bg-white/5 text-zinc-300 hover:bg-white/10 hover:text-zinc-100 disabled:opacity-50 disabled:cursor-not-allowed"
                                  aria-label="Сгенерировать HWID"
                                  title="Сгенерировать HWID"
                                >
                                  <svg
                                    xmlns="http://www.w3.org/2000/svg"
                                    width="16"
                                    height="16"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    stroke="currentColor"
                                    strokeWidth="2"
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    className="mx-auto"
                                  >
                                    <path d="M21 2v6h-6" />
                                    <path d="M3 12a9 9 0 0 1 15.5-6.36L21 8" />
                                    <path d="M3 22v-6h6" />
                                    <path d="M21 12a9 9 0 0 1-15.5 6.36L3 16" />
                                  </svg>
                                </button>
                              </div>
                            </div>
                          </div>
                        ) : null}
                      </div>

                      <div className="mt-3 grid grid-cols-1 gap-2 text-xs text-zinc-400 md:grid-cols-2">
                        <div>Импортировано ключей: <span className="text-zinc-200">{source.last_import_count}</span></div>
                        <div>Последняя синхронизация: <span className="text-zinc-200">{source.last_synced_at || "—"}</span></div>
                        <div className="md:col-span-2 break-all">
                          Meta title: <span className="text-zinc-200">{source.meta_title || "—"}</span>
                        </div>
                        <div className="md:col-span-2 break-all">
                          Meta announce: <span className="text-zinc-200">{source.meta_announce || "—"}</span>
                        </div>
                        {source.last_error ? (
                          <div className="md:col-span-2 rounded-md border border-red-500/30 bg-red-500/10 px-2 py-1 text-red-200">
                            Ошибка: {source.last_error}
                          </div>
                        ) : null}
                      </div>

                      <div className="mt-3 rounded-lg border border-border bg-surface-2/25 p-3">
                        <div className="mb-2 text-sm font-medium text-zinc-200">
                          Ключи источника ({sourceKeys.length})
                        </div>
                        {sourceKeys.length === 0 ? (
                          <div className="text-xs text-zinc-500">Ключей пока нет. Нажмите &quot;Обновить подписку&quot;.</div>
                        ) : (
                          <div className="space-y-2">
                            {sourceKeys.map((key) => (
                              <div
                                key={key.id}
                                className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/70 bg-bg/40 px-2 py-2"
                              >
                                <div className="min-w-0 flex-1">
                                  <div className="truncate text-sm text-zinc-200">{key.label}</div>
                                  <div className="truncate text-xs text-zinc-500">
                                    В клиенте: {key.client_display_name || key.label}
                                  </div>
                                </div>
                                <div className="flex items-center gap-2">
                                  <Button
                                    variant="ghost"
                                    className="text-xs"
                                    onClick={() => setEditKey(key)}
                                    disabled={isBusy || deletingKeyID === key.id}
                                  >
                                    Изменить
                                  </Button>
                                  <Button
                                    variant="danger"
                                    className="text-xs"
                                    onClick={() => void deleteExternalKey(key)}
                                    loading={deletingKeyID === key.id}
                                    disabled={isBusy}
                                  >
                                    Удалить
                                  </Button>
                                </div>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>

                      <div className="mt-3 flex flex-wrap gap-2">
                        <Button
                          variant="ghost"
                          onClick={() => void saveSource(source)}
                          loading={Boolean(savingById[source.id])}
                          disabled={Boolean(syncingById[source.id]) || Boolean(removingById[source.id])}
                        >
                          Сохранить
                        </Button>
                        <Button
                          onClick={() => void syncSource(source)}
                          loading={Boolean(syncingById[source.id])}
                          disabled={Boolean(savingById[source.id]) || Boolean(removingById[source.id])}
                        >
                          Обновить подписку
                        </Button>
                        <Button
                          variant="danger"
                          onClick={() => void deleteSource(source)}
                          loading={Boolean(removingById[source.id])}
                          disabled={Boolean(savingById[source.id]) || Boolean(syncingById[source.id])}
                        >
                          Удалить источник
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      </Modal>
      {editKey ? (
        <EditKeyModal
          keyData={editKey}
          onClose={() => setEditKey(null)}
          onRefresh={async () => {
            await onChanged();
            await loadSources();
          }}
        />
      ) : null}

      <CreateExternalCategoryModal
        open={showCreateCategory}
        onClose={() => {
          setShowCreateCategory(false);
          setCreateCategoryTargetSourceID(null);
        }}
        initialValue={
          createCategoryTargetSourceID
            ? (sources.find((source) => source.id === createCategoryTargetSourceID)?.category || "Общее")
            : "Общее"
        }
        onCreated={(categoryName) => {
          if (createCategoryTargetSourceID) {
            updateLocalSource(createCategoryTargetSourceID, { category: categoryName });
          }
          void loadCategoriesOnly();
        }}
      />

      <CreateKeyCategoryModal
        open={showCreateKeyCategory}
        onClose={() => {
          setShowCreateKeyCategory(false);
          setCreateKeyCategoryTargetSourceID(null);
        }}
        initialValue={
          createKeyCategoryTargetSourceID
            ? (sources.find((source) => source.id === createKeyCategoryTargetSourceID)?.key_category || "")
            : ""
        }
        onCreated={(categoryValue) => {
          if (createKeyCategoryTargetSourceID) {
            updateLocalSource(createKeyCategoryTargetSourceID, { key_category: categoryValue.name });
          }
          void loadCategoriesOnly();
        }}
      />

      <ExternalSourceCategoriesModal
        open={showCategoriesManager}
        onClose={() => setShowCategoriesManager(false)}
        categories={categories}
        onSaved={async () => {
          await loadSources();
        }}
      />
    </>
  );
}
