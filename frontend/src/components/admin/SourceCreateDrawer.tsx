"use client";

import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, Check, ChevronLeft, ChevronRight, Eye, Plus } from "lucide-react";
import { ApiError, apiV1 } from "@/lib/api";
import type {
  ExternalSourceCategory,
  ExternalSourcePreview,
  KeyCategory,
  SourceDetail,
} from "@/lib/types";
import { Button } from "@/components/ui/Button";
import { Drawer } from "@/components/ui/Drawer";
import { Input } from "@/components/ui/Input";
import { ResourceError } from "@/components/ui/ResourceState";
import { Select } from "@/components/ui/Select";

interface Props {
  open: boolean;
  sourceCategories: ExternalSourceCategory[];
  keyCategories: KeyCategory[];
  onClose: () => void;
  onCreated: (source: SourceDetail) => Promise<void>;
}

function randomHWID() {
  if (typeof crypto !== "undefined" && crypto.randomUUID) return crypto.randomUUID();
  return Array.from(crypto.getRandomValues(new Uint8Array(16)), (value) =>
    value.toString(16).padStart(2, "0")
  ).join("");
}

function rawContentType(raw: string) {
  const value = raw.trim();
  return value.startsWith("{") || value.startsWith("[")
    ? "application/json; charset=utf-8"
    : "text/plain; charset=utf-8";
}

export function SourceCreateDrawer({
  open,
  sourceCategories,
  keyCategories,
  onClose,
  onCreated,
}: Props) {
  const [step, setStep] = useState(1);
  const [sourceURL, setSourceURL] = useState("");
  const [name, setName] = useState("");
  const [category, setCategory] = useState("Общее");
  const [keyCategory, setKeyCategory] = useState("");
  const [keyInsertMode, setKeyInsertMode] = useState<"top" | "bottom">("bottom");
  const [enabled, setEnabled] = useState(true);
  const [passHWID, setPassHWID] = useState(false);
  const [hwidVersion, setHWIDVersion] = useState("");
  const [hwidModelName, setHWIDModelName] = useState("");
  const [hwidValue, setHWIDValue] = useState("");
  const [manualOpen, setManualOpen] = useState(false);
  const [manualBody, setManualBody] = useState("");
  const [preview, setPreview] = useState<ExternalSourcePreview | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);

  const reset = () => {
    setStep(1);
    setSourceURL("");
    setName("");
    setCategory("Общее");
    setKeyCategory("");
    setKeyInsertMode("bottom");
    setEnabled(true);
    setPassHWID(false);
    setHWIDVersion("");
    setHWIDModelName("");
    setHWIDValue("");
    setManualOpen(false);
    setManualBody("");
    setPreview(null);
    setError(null);
  };

  useEffect(() => {
    if (open) reset();
  }, [open]);

  const close = () => {
    if (!busy) onClose();
  };

  const previewSource = async () => {
    if (!sourceURL.trim()) {
      setError(new ApiError(400, "source_url_required", "Введите URL источника", { source_url: ["Введите URL источника"] }));
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const response = await apiV1.sources.preview({
        source_url: sourceURL.trim(),
        pass_hwid: passHWID,
        hwid_version: hwidVersion.trim(),
        hwid_model_name: hwidModelName.trim(),
        hwid_value: hwidValue.trim(),
        raw_body: manualBody.trim() || undefined,
        raw_content_type: manualBody.trim() ? rawContentType(manualBody) : undefined,
        raw_final_url: manualBody.trim() ? sourceURL.trim() : undefined,
      });
      setPreview(response);
      if (!name.trim()) setName((response.suggested_name || "").slice(0, 24));
      setStep(2);
    } catch (requestError) {
      setError(requestError);
    } finally {
      setBusy(false);
    }
  };

  const createSource = async () => {
    if (!preview) return;
    setBusy(true);
    setError(null);
    try {
      const response = await apiV1.sources.create({
        name: name.trim(),
        category: category.trim() || "Общее",
        key_category: keyCategory,
        key_insert_mode: keyInsertMode,
        source_url: sourceURL.trim(),
        enabled,
        apply_remote_metadata: false,
        pass_hwid: passHWID,
        hwid_version: hwidVersion.trim(),
        hwid_model_name: hwidModelName.trim(),
        hwid_value: hwidValue.trim() || undefined,
        raw_body: manualBody.trim() || undefined,
        raw_content_type: manualBody.trim() ? rawContentType(manualBody) : undefined,
        raw_final_url: manualBody.trim() ? sourceURL.trim() : undefined,
      });
      await onCreated(response.data);
      onClose();
    } catch (requestError) {
      setError(requestError);
    } finally {
      setBusy(false);
    }
  };

  const fieldErrors = error instanceof ApiError ? error.fieldErrors : {};
  const sourceCategoryNames = useMemo(
    () => Array.from(new Set(sourceCategories.map((item) => item.name))),
    [sourceCategories]
  );

  const footer = (
    <div className="flex items-center justify-between gap-3">
      <div className="text-xs text-zinc-600">Шаг {step} из 3</div>
      <div className="flex gap-2">
        {step > 1 && (
          <Button variant="outline" onClick={() => setStep((value) => value - 1)} disabled={busy}>
            <ChevronLeft className="h-4 w-4" />
            Назад
          </Button>
        )}
        {step === 1 && (
          <Button onClick={() => void previewSource()} loading={busy}>
            <Eye className="h-4 w-4" />
            Проверить источник
          </Button>
        )}
        {step === 2 && (
          <Button
            onClick={() => setStep(3)}
            disabled={!name.trim() || name.trim().length > 24}
          >
            Проверить параметры
            <ChevronRight className="h-4 w-4" />
          </Button>
        )}
        {step === 3 && (
          <Button onClick={() => void createSource()} loading={busy}>
            <Plus className="h-4 w-4" />
            Добавить источник
          </Button>
        )}
      </div>
    </div>
  );

  return (
    <Drawer
      open={open}
      onClose={close}
      title="Добавить внешний источник"
      description="Подключение выполняется только сервером с защитой от SSRF."
      footer={footer}
    >
      <div className="ui-joined-grid mb-6 grid grid-cols-3" aria-label={`Шаг ${step} из 3`}>
        {["Подключение", "Параметры", "Проверка"].map((label, index) => {
          const value = index + 1;
          return (
            <div
              key={label}
              className={`rounded-lg border px-3 py-2 text-center text-xs ${
                value === step
                  ? "border-accent/40 bg-accent/10 text-accent"
                  : value < step
                    ? "border-success/20 text-success"
                    : "border-border text-zinc-600"
              }`}
            >
              {value < step && <Check className="mr-1 inline h-3 w-3" />}
              {label}
            </div>
          );
        })}
      </div>

      {Boolean(error) && (
        <div className="mb-5">
          <ResourceError error={error} compact title="Операция с источником не выполнена" />
        </div>
      )}

      {step === 1 && (
        <div className="space-y-5">
          <Input
            label="URL подписки"
            value={sourceURL}
            onChange={(event) => {
              setSourceURL(event.target.value);
              setPreview(null);
            }}
            placeholder="https://provider.example/sub/…"
            error={fieldErrors.source_url?.[0]}
            disabled={busy}
          />

          <section className="rounded-xl border border-border bg-surface-1 p-4">
            <label className="flex items-center gap-3 text-sm text-zinc-300">
              <input
                type="checkbox"
                checked={passHWID}
                onChange={(event) => setPassHWID(event.target.checked)}
                className="h-4 w-4 accent-accent"
              />
              Передавать HWID при запросе источника
            </label>
            {passHWID && (
              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <Input label="Версия клиента" value={hwidVersion} onChange={(event) => setHWIDVersion(event.target.value)} />
                <Input label="Модель устройства" value={hwidModelName} onChange={(event) => setHWIDModelName(event.target.value)} />
                <div className="sm:col-span-2 grid grid-cols-[1fr_auto] items-end gap-2">
                  <Input label="HWID" value={hwidValue} onChange={(event) => setHWIDValue(event.target.value)} />
                  <Button variant="outline" onClick={() => setHWIDValue(randomHWID())}>Сгенерировать</Button>
                </div>
              </div>
            )}
          </section>

          <section className="rounded-xl border border-border bg-surface-1 p-4">
            <button
              type="button"
              onClick={() => setManualOpen((value) => !value)}
              className="text-left text-sm font-medium text-zinc-300 hover:text-zinc-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              {manualOpen ? "Скрыть" : "Показать"} ручной fallback
            </button>
            <p className="mt-1 text-xs text-zinc-600">
              Используйте только если провайдер не отдаёт содержимое серверу. Браузер не обращается к URL автоматически.
            </p>
            {manualOpen && (
              <textarea
                value={manualBody}
                onChange={(event) => setManualBody(event.target.value.slice(0, 1 << 20))}
                className="mt-4 min-h-44 w-full rounded-lg border border-border bg-surface-2 p-3 font-mono text-xs text-zinc-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent"
                placeholder="JSON или список ссылок, максимум 1 MiB"
              />
            )}
          </section>
        </div>
      )}

      {step === 2 && (
        <div className="space-y-5">
          <Input
            label="Название"
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={24}
            error={fieldErrors.name?.[0]}
          />
          <div>
            <Input
              label="Категория источника"
              value={category}
              list="source-category-options"
              onChange={(event) => setCategory(event.target.value)}
            />
            <datalist id="source-category-options">
              {sourceCategoryNames.map((item) => <option key={item} value={item} />)}
            </datalist>
          </div>
          <Select
            label="Категория импортированных ключей"
            value={keyCategory}
            onChange={(event) => setKeyCategory(event.target.value)}
            options={[
              { value: "", label: "Без категории" },
              ...keyCategories.map((item) => ({ value: item.name, label: item.name })),
            ]}
          />
          <div>
            <div className="mb-2 text-sm text-zinc-400">Позиция новых ключей</div>
            <div className="ui-joined-grid grid grid-cols-2">
              {(["top", "bottom"] as const).map((value) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => setKeyInsertMode(value)}
                  className={`rounded-lg border px-3 py-2 text-sm ${
                    keyInsertMode === value
                      ? "border-accent/35 bg-accent/10 text-accent"
                      : "border-border text-zinc-500"
                  }`}
                >
                  {value === "top" ? "В начало" : "В конец"}
                </button>
              ))}
            </div>
          </div>
          <label className="flex items-center gap-3 rounded-xl border border-border bg-surface-1 p-4 text-sm text-zinc-300">
            <input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} className="h-4 w-4 accent-accent" />
            Источник активен
          </label>
        </div>
      )}

      {step === 3 && preview && (
        <div className="space-y-5">
          <section className="ui-joined-grid grid sm:grid-cols-2">
            {[
              ["Источник", name || preview.suggested_name],
              ["Формат", preview.detected_format],
              ["Найдено ключей", preview.key_count],
              ["Категория", category || "Общее"],
            ].map(([label, value]) => (
              <div key={String(label)} className="rounded-xl border border-border bg-surface-1 p-4">
                <div className="text-xs text-zinc-600">{label}</div>
                <div className="mt-1 break-words text-sm font-medium text-zinc-200">{value}</div>
              </div>
            ))}
          </section>
          {preview.warnings.length > 0 && (
            <section className="rounded-xl border border-amber-400/25 bg-amber-400/5 p-4 text-sm text-amber-200">
              <div className="mb-2 flex items-center gap-2 font-medium"><AlertTriangle className="h-4 w-4" />Предупреждения</div>
              <ul className="list-disc space-y-1 pl-5">
                {preview.warnings.map((warning) => <li key={warning}>{warning}</li>)}
              </ul>
            </section>
          )}
          <section className="overflow-hidden rounded-xl border border-border bg-surface-1">
            <div className="border-b border-border px-4 py-3 text-sm font-medium text-zinc-300">Все ключи</div>
            <div className="divide-y divide-border">
              {preview.keys.map((key, index) => (
                <div key={`${key.url_short}-${index}`} className="px-4 py-3">
                  <div className="text-sm text-zinc-300">{key.label}</div>
                  <div className="mt-1 truncate font-mono text-xs text-zinc-600">{key.scheme} · {key.url_short}</div>
                </div>
              ))}
            </div>
          </section>
        </div>
      )}
    </Drawer>
  );
}
