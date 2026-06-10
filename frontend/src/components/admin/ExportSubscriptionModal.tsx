"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { externalSources, keys as keysApi } from "@/lib/api";
import type { ExternalSourceCategory, ExternalSourcePreview, KeyCategory } from "@/lib/types";
import { CreateExternalCategoryModal } from "./CreateExternalCategoryModal";
import { CreateKeyCategoryModal } from "./CreateKeyCategoryModal";

interface Props {
  open: boolean;
  onClose: () => void;
  onImported: () => Promise<void>;
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

export function ExportSubscriptionModal({ open, onClose, onImported }: Props) {
  const { toast } = useToast();
  const fallbackRawBodyRef = useRef("");
  const fallbackRawContentTypeRef = useRef("");
  const fallbackRawFinalURLRef = useRef("");
  const [sourceURL, setSourceURL] = useState("");
  const [sourceName, setSourceName] = useState("");
  const [category, setCategory] = useState("Общее");
  const [categories, setCategories] = useState<ExternalSourceCategory[]>([]);
  const [keyCategory, setKeyCategory] = useState("");
  const [keyInsertMode, setKeyInsertMode] = useState<"top" | "bottom">("bottom");
  const [keyCategories, setKeyCategories] = useState<KeyCategory[]>([]);
  const [categoriesLoading, setCategoriesLoading] = useState(false);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showCreateKeyCategory, setShowCreateKeyCategory] = useState(false);
  const [enabled, setEnabled] = useState(true);
  const [passHWID, setPassHWID] = useState(false);
  const [hwidVersion, setHWIDVersion] = useState("");
  const [hwidModelName, setHWIDModelName] = useState("");
  const [hwidValue, setHWIDValue] = useState("");
  const [manualBody, setManualBody] = useState("");
  const [preview, setPreview] = useState<ExternalSourcePreview | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const [importing, setImporting] = useState(false);

  const resetState = () => {
    fallbackRawBodyRef.current = "";
    fallbackRawContentTypeRef.current = "";
    fallbackRawFinalURLRef.current = "";
    setSourceURL("");
    setSourceName("");
    setCategory("Общее");
    setKeyCategory("");
    setKeyInsertMode("bottom");
    setEnabled(true);
    setPassHWID(false);
    setHWIDVersion("");
    setHWIDModelName("");
    setHWIDValue("");
    setManualBody("");
    setPreview(null);
    setPreviewLoading(false);
    setImporting(false);
  };

  const inferRawContentType = (raw: string) => {
    const value = raw.trim();
    if (value.startsWith("{") || value.startsWith("[")) {
      return "application/json; charset=utf-8";
    }
    return "text/plain; charset=utf-8";
  };

  const loadCategories = useCallback(async (preferredCategory = "") => {
    setCategoriesLoading(true);
    try {
      const [response, keyCategoryResponse] = await Promise.all([
        externalSources.listCategories(),
        keysApi.listCategories(),
      ]);
      const list = response.categories || [];
      setCategories(list);
      setKeyCategories(keyCategoryResponse.categories || []);
      setCategory((previous) => {
        const preferred = preferredCategory.trim();
        if (preferred) {
          return preferred;
        }
        const current = previous.trim();
        if (current) {
          return current;
        }
        return list[0]?.name || "Общее";
      });
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось загрузить категории", "error");
      setCategories([]);
      setKeyCategories([]);
    } finally {
      setCategoriesLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (!open) {
      return;
    }
    void loadCategories();
  }, [open, loadCategories]);

  const categoryOptions = useMemo(() => {
    const values = new Set<string>();
    const options: Array<{ value: string; label: string }> = [];
    for (const item of categories) {
      const name = item.name.trim();
      if (!name || values.has(name)) {
        continue;
      }
      values.add(name);
      options.push({ value: name, label: `${name} (${item.sources_count})` });
    }
    const current = category.trim();
    if (current && !values.has(current)) {
      options.push({ value: current, label: current });
    }
    options.sort((left, right) => left.label.localeCompare(right.label, "ru"));
    options.push({ value: CREATE_CATEGORY_VALUE, label: "+ Создать категорию" });
    return options;
  }, [categories, category]);

  const keyCategoryOptions = useMemo(() => {
    const options = [{ value: "", label: "Без категории" }];
    const values = new Set<string>([""]);
    for (const item of keyCategories) {
      const name = item.name.trim();
      if (!name || values.has(name)) {
        continue;
      }
      values.add(name);
      options.push({ value: name, label: `${name} (${item.keys_count})` });
    }
    if (keyCategory.trim() && !values.has(keyCategory.trim())) {
      options.push({ value: keyCategory.trim(), label: keyCategory.trim() });
    }
    options.push({ value: CREATE_KEY_CATEGORY_VALUE, label: "+ Создать категорию ключей" });
    return options;
  }, [keyCategories, keyCategory]);

  const handleClose = () => {
    if (previewLoading || importing) {
      return;
    }
    resetState();
    onClose();
  };

  const handlePreview = async () => {
    const url = sourceURL.trim();
    if (!url) {
      toast("Введите ссылку на стороннюю подписку", "error");
      return;
    }

    setPreviewLoading(true);
    fallbackRawBodyRef.current = "";
    fallbackRawContentTypeRef.current = "";
    fallbackRawFinalURLRef.current = "";
    try {
      if (manualBody.trim()) {
        fallbackRawBodyRef.current = manualBody;
        fallbackRawContentTypeRef.current = inferRawContentType(manualBody);
        fallbackRawFinalURLRef.current = url;
        const response = await externalSources.preview({
          source_url: url,
          pass_hwid: passHWID,
          hwid_version: hwidVersion.trim(),
          hwid_model_name: hwidModelName.trim(),
          hwid_value: hwidValue.trim(),
          raw_body: manualBody,
          raw_content_type: fallbackRawContentTypeRef.current,
          raw_final_url: fallbackRawFinalURLRef.current,
        });
        setPreview(response);
        if (!sourceName.trim()) {
          setSourceName((response.suggested_name || "").slice(0, 24));
        }
        toast(`Найдено ключей: ${response.key_count} — использовано вставленное тело подписки`, "success");
        return;
      }

      const response = await externalSources.preview({
        source_url: url,
        pass_hwid: passHWID,
        hwid_version: hwidVersion.trim(),
        hwid_model_name: hwidModelName.trim(),
        hwid_value: hwidValue.trim(),
      });
      setPreview(response);
      if (!sourceName.trim()) {
        setSourceName((response.suggested_name || "").slice(0, 24));
      }
      toast(`Найдено ключей: ${response.key_count}`, "success");
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "Не удалось проанализировать подписку";
      if (!/(empty body|headers but empty body)/i.test(message)) {
        toast(message, "error");
        return;
      }

      try {
        const buildBrowserFetchOptions = (withHWIDHeaders: boolean): RequestInit => {
          const headers = new Headers({
            Accept: "application/json,text/plain,*/*",
          });
          if (withHWIDHeaders && passHWID) {
            if (hwidValue.trim()) {
              headers.set("X-HWID", hwidValue.trim());
              headers.set("X-Device-ID", hwidValue.trim());
            }
            if (hwidModelName.trim()) {
              headers.set("X-Device-Model", hwidModelName.trim());
            }
            if (hwidVersion.trim()) {
              headers.set("X-App-Version", hwidVersion.trim());
              headers.set("X-Client-Version", hwidVersion.trim());
            }
            if (hwidModelName.trim() || hwidVersion.trim()) {
              headers.set("X-Device-Info", `model=${hwidModelName.trim()};version=${hwidVersion.trim()}`);
            }
          }
          return {
            method: "GET",
            mode: "cors",
            cache: "no-store",
            headers,
          };
        };

        let browserResponse: Response;
        let browserBody = "";
        let lastBrowserError = "";

        try {
          browserResponse = await fetch(url, buildBrowserFetchOptions(true));
          browserBody = await browserResponse.text();
          if (!browserResponse.ok) {
            throw new Error(`Браузерный запрос вернул HTTP ${browserResponse.status}`);
          }
        } catch (hwidBrowserError: unknown) {
          lastBrowserError =
            hwidBrowserError instanceof Error ? hwidBrowserError.message : "Не удалось выполнить браузерный запрос с HWID";
          browserResponse = await fetch(url, buildBrowserFetchOptions(false));
          browserBody = await browserResponse.text();
          if (!browserResponse.ok) {
            throw new Error(`Браузерный запрос вернул HTTP ${browserResponse.status}`);
          }
        }

        if (!browserBody.trim()) {
          if (lastBrowserError) {
            throw new Error(`${message}. Browser fallback: ${lastBrowserError}`);
          }
          throw new Error(message);
        }

        fallbackRawBodyRef.current = browserBody;
        fallbackRawContentTypeRef.current = browserResponse.headers.get("content-type") || "";
        fallbackRawFinalURLRef.current = browserResponse.url || url;

        const response = await externalSources.preview({
          source_url: url,
          pass_hwid: passHWID,
          hwid_version: hwidVersion.trim(),
          hwid_model_name: hwidModelName.trim(),
          hwid_value: hwidValue.trim(),
          raw_body: browserBody,
          raw_content_type: fallbackRawContentTypeRef.current,
          raw_final_url: fallbackRawFinalURLRef.current,
        });
        setPreview(response);
        if (!sourceName.trim()) {
          setSourceName((response.suggested_name || "").slice(0, 24));
        }
        toast(`Найдено ключей: ${response.key_count} — использован прямой импорт из браузера`, "success");
      } catch (browserError: unknown) {
        fallbackRawBodyRef.current = "";
        fallbackRawContentTypeRef.current = "";
        fallbackRawFinalURLRef.current = "";
        const browserMessage =
          browserError instanceof Error ? browserError.message : "Не удалось выполнить браузерный fallback";
        toast(browserMessage, "error");
      }
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleImport = async () => {
    if (!preview) {
      toast("Сначала выполните анализ подписки", "error");
      return;
    }

    const url = sourceURL.trim();
    const name = sourceName.trim() || preview.suggested_name || "Сторонняя подписка";
    const categoryName = category.trim() || "Общее";

    setImporting(true);
    try {
      const response = await externalSources.import({
        name,
        category: categoryName,
        key_category: keyCategory.trim(),
        key_insert_mode: keyInsertMode,
        source_url: url,
        enabled,
        pass_hwid: passHWID,
        hwid_version: hwidVersion.trim(),
        hwid_model_name: hwidModelName.trim(),
        hwid_value: hwidValue.trim(),
        raw_body: fallbackRawBodyRef.current || undefined,
        raw_content_type: fallbackRawContentTypeRef.current || undefined,
        raw_final_url: fallbackRawFinalURLRef.current || undefined,
      });
      const message =
        response.skipped_count > 0
          ? `Импортировано ${response.imported_count}, пропущено ${response.skipped_count}`
          : `Импортировано ключей: ${response.imported_count}`;
      toast(message, "success");
      await onImported();
      resetState();
      onClose();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось импортировать подписку", "error");
    } finally {
      setImporting(false);
    }
  };

  const previewWarnings = Array.isArray(preview?.warnings) ? preview.warnings : [];
  const previewKeys = Array.isArray(preview?.keys) ? preview.keys : [];

  return (
    <>
      <Modal open={open} onClose={handleClose} title="Экспорт подписки" className="max-w-3xl max-h-[90vh] overflow-y-auto">
        <div className="space-y-4">
        <p className="text-sm text-zinc-400">
          Вставьте URL сторонней подписки. Панель попробует получить ключи и мета-данные (`profile-title`, `announce`, `support-url`) и импортирует их в текущий проект.
        </p>

        <Input
          label="URL подписки"
          value={sourceURL}
          onChange={(event) => setSourceURL(event.target.value)}
          placeholder="https://example.com/sub/..."
          disabled={previewLoading || importing}
        />

        <div className="space-y-2 rounded-lg border border-border bg-surface-2/50 px-3 py-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="text-sm font-medium text-zinc-200">Ручное тело подписки</div>
              <p className="mt-1 text-xs text-zinc-500">
                Если провайдер не отдает тело подписки серверу или браузеру, сюда можно вставить JSON-конфигурации или список ссылок-ключей.
              </p>
            </div>
            <label className="inline-flex cursor-pointer items-center rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 hover:bg-surface-2/80">
              Загрузить файл
              <input
                type="file"
                accept=".json,.txt,application/json,text/plain"
                className="hidden"
                disabled={previewLoading || importing}
                onChange={async (event) => {
                  const file = event.target.files?.[0];
                  if (!file) {
                    return;
                  }
                  try {
                    const text = await file.text();
                    setManualBody(text);
                    toast(`Файл загружен: ${file.name}`, "success");
                  } catch {
                    toast("Не удалось прочитать файл подписки", "error");
                  } finally {
                    event.currentTarget.value = "";
                  }
                }}
              />
            </label>
          </div>
          <textarea
            value={manualBody}
            onChange={(event) => setManualBody(event.target.value)}
            placeholder="Вставьте JSON-массив/объект или список ссылок-ключей..."
            disabled={previewLoading || importing}
            className="min-h-[120px] w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          />
        </div>

        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <Input
            label="Название источника"
            value={sourceName}
            onChange={(event) => setSourceName(event.target.value)}
            placeholder="Например, Partner RU Feed"
            maxLength={24}
            disabled={previewLoading || importing}
          />
          <Select
            label="Категория источника"
            value={category}
            onChange={(event) => {
              const nextValue = event.target.value;
              if (nextValue === CREATE_CATEGORY_VALUE) {
                setShowCreateCategory(true);
                return;
              }
              setCategory(nextValue);
            }}
            options={categoryOptions}
            disabled={previewLoading || importing || categoriesLoading}
          />
        </div>

        <div className="rounded-lg border border-border bg-surface-2/50 px-3 py-3">
          <div className="mb-3 text-sm font-medium text-zinc-200">Размещение ключей в панели</div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
            <Select
              label="Категория ключей"
              value={keyCategory}
              onChange={(event) => {
                const nextValue = event.target.value;
                if (nextValue === CREATE_KEY_CATEGORY_VALUE) {
                  setShowCreateKeyCategory(true);
                  return;
                }
                setKeyCategory(nextValue);
              }}
              options={keyCategoryOptions}
              disabled={previewLoading || importing || categoriesLoading}
            />
            <div className="flex flex-col gap-1.5">
              <span className="text-sm text-zinc-400">Позиция в категории</span>
              <div className="grid grid-cols-2 gap-1 rounded-xl border border-border bg-surface-2 p-1">
                <button
                  type="button"
                  onClick={() => setKeyInsertMode("top")}
                  className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                    keyInsertMode === "top"
                      ? "bg-accent text-accent-fg shadow-sm"
                      : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                  }`}
                >
                  В начало
                </button>
                <button
                  type="button"
                  onClick={() => setKeyInsertMode("bottom")}
                  className={`rounded-lg px-3 py-2 text-sm font-medium transition ${
                    keyInsertMode === "bottom"
                      ? "bg-accent text-accent-fg shadow-sm"
                      : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
                  }`}
                >
                  В конец
                </button>
              </div>
            </div>
          </div>
          <p className="mt-2 text-xs text-zinc-500">
            Если выбрана категория ключей, импортированные конфигурации попадут в нее и сохранятся там при последующих обновлениях источника.
          </p>
        </div>

        <div className="grid grid-cols-1 gap-2">
          <label className="flex items-center gap-2 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200">
            <input
              type="checkbox"
              checked={enabled}
              onChange={(event) => setEnabled(event.target.checked)}
              disabled={previewLoading || importing}
            />
            Источник активен сразу после импорта
          </label>
        </div>

        <div className="rounded-lg border border-border bg-surface-2/50 px-3 py-3">
          <label className="flex items-center gap-2 text-sm text-zinc-200">
            <input
              type="checkbox"
              checked={passHWID}
              onChange={(event) => setPassHWID(event.target.checked)}
              disabled={previewLoading || importing}
            />
            Передавать HWID
          </label>
          <p className="mt-1 text-xs text-zinc-500">
            Некоторые провайдеры проверяют HWID-заголовки при добавлении подписки.
          </p>

          {passHWID ? (
            <div className="mt-3 grid grid-cols-1 gap-3 md:grid-cols-3">
              <Input
                label="Version"
                value={hwidVersion}
                onChange={(event) => setHWIDVersion(event.target.value)}
                placeholder="3.8.1"
                disabled={previewLoading || importing}
              />
              <Input
                label="Model name"
                value={hwidModelName}
                onChange={(event) => setHWIDModelName(event.target.value)}
                placeholder="SM-S918B"
                disabled={previewLoading || importing}
              />
              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-zinc-400">HWID</label>
                <div className="relative">
                  <input
                    value={hwidValue}
                    onChange={(event) => setHWIDValue(event.target.value)}
                    placeholder="Нажмите кнопку справа для генерации"
                    disabled={previewLoading || importing}
                    className="w-full bg-surface-2 border border-border rounded-lg px-3 py-2 pr-11 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                  />
                  <button
                    type="button"
                    onClick={() => setHWIDValue(generateRandomHWID())}
                    disabled={previewLoading || importing}
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

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="ghost" onClick={handlePreview} loading={previewLoading} disabled={importing}>
            Проверить и подготовить
          </Button>
          <Button onClick={handleImport} loading={importing} disabled={!preview || previewLoading}>
            Импортировать ключи
          </Button>
          <Button variant="ghost" onClick={handleClose} disabled={previewLoading || importing}>
            Отмена
          </Button>
        </div>

        {preview && (
          <div className="space-y-3 rounded-xl border border-border bg-surface-2/60 p-4">
            <div className="flex flex-wrap items-center gap-2 text-sm text-zinc-300">
              <span className="rounded-full border border-border bg-surface-2 px-2 py-1">
                Формат: {preview.detected_format.toUpperCase()}
              </span>
              <span className="rounded-full border border-border bg-surface-2 px-2 py-1">
                Ключей: {preview.key_count}
              </span>
              {preview.metadata.http_status ? (
                <span className="rounded-full border border-border bg-surface-2 px-2 py-1">
                  HTTP: {preview.metadata.http_status}
                </span>
              ) : null}
            </div>

            <div className="grid grid-cols-1 gap-2 text-xs text-zinc-400 md:grid-cols-2">
              <div>Название: <span className="text-zinc-200">{preview.metadata.title || "—"}</span></div>
              <div>Обновление: <span className="text-zinc-200">{preview.metadata.refresh_hours || "—"} ч</span></div>
              <div className="md:col-span-2 break-all">Support URL: <span className="text-zinc-200">{preview.metadata.support_url || "—"}</span></div>
              <div className="md:col-span-2 break-all">Profile URL: <span className="text-zinc-200">{preview.metadata.profile_web_page || "—"}</span></div>
              <div className="md:col-span-2 break-all">Final URL: <span className="text-zinc-200">{preview.metadata.final_url || "—"}</span></div>
              <div className="md:col-span-2">Объявление: <span className="text-zinc-200">{preview.metadata.announce || "—"}</span></div>
            </div>

            {previewWarnings.length > 0 ? (
              <div className="rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-200">
                {previewWarnings.slice(0, 5).map((warning, index) => (
                  <div key={`warning-${index}`}>{warning}</div>
                ))}
                {previewWarnings.length > 5 ? <div>... и ещё {previewWarnings.length - 5}</div> : null}
              </div>
            ) : null}

            <div className="space-y-1 rounded-lg border border-border bg-bg/40 p-3">
              <div className="text-xs uppercase tracking-wide text-zinc-500">Первые ключи</div>
              {previewKeys.map((item, index) => (
                <div key={`${item.scheme}-${index}`} className="rounded-md border border-border/60 bg-surface-2 px-2 py-1">
                  <div className="text-xs text-zinc-300">{item.label}</div>
                  <div className="text-[11px] text-zinc-500">{item.scheme}</div>
                  <div className="font-mono text-[11px] text-zinc-400">{item.url_short}</div>
                </div>
              ))}
            </div>
          </div>
        )}
        </div>
      </Modal>
      <CreateExternalCategoryModal
        open={showCreateCategory}
        onClose={() => setShowCreateCategory(false)}
        initialValue={category}
        onCreated={(categoryName) => {
          setCategory(categoryName);
          void loadCategories(categoryName);
        }}
      />
      <CreateKeyCategoryModal
        open={showCreateKeyCategory}
        onClose={() => setShowCreateKeyCategory(false)}
        initialValue={keyCategory}
        onCreated={(categoryValue) => {
          setKeyCategory(categoryValue.name);
          void loadCategories(category.trim());
        }}
      />
    </>
  );
}
