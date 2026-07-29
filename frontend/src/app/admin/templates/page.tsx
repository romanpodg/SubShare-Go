"use client";

import { useCallback, useEffect, useState } from "react";
import { Eye, FileJson2, Plus, Save, Trash2, X } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { SubscriptionTemplate, SubscriptionTemplateFormat } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { useAuth } from "@/hooks/useAuth";

const emptyDraft = {
  name: "",
  format: "base64" as SubscriptionTemplateFormat,
  content: "",
  enabled: true,
};

export default function TemplatesPage() {
  const [templates, setTemplates] = useState<SubscriptionTemplate[]>([]);
  const [selected, setSelected] = useState<SubscriptionTemplate | null>(null);
  const [draft, setDraft] = useState(emptyDraft);
  const [saving, setSaving] = useState(false);
  const [preview, setPreview] = useState<{ content: string; content_type: string } | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const { toast } = useToast();
  const { role } = useAuth();
  const isOwner = role === "owner";

  const load = useCallback(async () => {
    setError(null);
    try {
      const response = await apiV1.templates.list();
      setTemplates(response.data);
      setSelected((current) => {
        if (!current) return null;
        return response.data.find((item) => item.id === current.id) ?? null;
      });
    } catch (requestError) {
      setError(requestError);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => void load());
    return () => window.cancelAnimationFrame(frame);
  }, [load]);

  const edit = (item: SubscriptionTemplate) => {
    setSelected(item);
    setDraft({ name: item.name, format: item.format, content: item.content, enabled: item.enabled });
  };

  const create = () => {
    if (!isOwner) return;
    setSelected(null);
    setDraft(emptyDraft);
  };

  const save = async () => {
    if (!isOwner) return;
    setSaving(true);
    try {
      if (selected) {
        await apiV1.templates.update(selected.id, draft);
      } else {
        await apiV1.templates.create(draft);
      }
      toast("Шаблон сохранён", "success");
      await load();
      if (!selected) setDraft(emptyDraft);
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить шаблон", "error");
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!isOwner || !selected || selected.is_system) return;
    try {
      await apiV1.templates.delete(selected.id);
      setSelected(null);
      setDraft(emptyDraft);
      await load();
      toast("Шаблон удалён", "success");
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось удалить шаблон", "error");
    }
  };

  const showPreview = async () => {
    setPreviewing(true);
    try {
      setPreview(await apiV1.templates.preview(draft));
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось построить preview", "error");
    } finally {
      setPreviewing(false);
    }
  };

  if (loading && templates.length === 0) {
    return <InitialLoading label="Загрузка шаблонов…" />;
  }

  if (error && templates.length === 0) {
    return <ResourceError error={error} onRetry={() => void load()} />;
  }

  return (
    <div>
      <PageHeader
        title="Шаблоны подписок"
        description="Форматы Base64, Xray JSON, Mihomo и Sing-box. В пользовательском содержимом используйте {{subscription}} и {{title}}."
        icon={<FileJson2 className="h-5 w-5" />}
        actions={isOwner ? <Button onClick={create}><Plus className="h-4 w-4" />Новый шаблон</Button> : undefined}
      />

      {Boolean(error) && <div className="mb-4"><ResourceError error={error} compact onRetry={() => void load()} /></div>}
      {!isOwner && (
        <div className="mb-4 rounded-xl border border-amber-400/20 bg-amber-400/5 px-4 py-3 text-sm text-amber-200">
          Режим просмотра: изменять шаблоны может только владелец.
        </div>
      )}

      <div className="grid gap-px bg-border xl:grid-cols-[340px_1fr]">
        <aside className="technical-frame overflow-hidden border border-border bg-surface-1">
          <div className="border-b border-border px-4 py-3 text-xs font-semibold uppercase tracking-wider text-zinc-600">
            Доступные шаблоны
          </div>
          <div className="max-h-[680px] divide-y divide-border overflow-y-auto">
            {templates.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => edit(item)}
                className={`w-full px-4 py-4 text-left transition ${selected?.id === item.id ? "bg-cyan-400/10" : "hover:bg-zinc-900/40"}`}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="truncate text-sm font-medium text-zinc-200">{item.name}</span>
                  <span className="rounded-md border border-border bg-zinc-950/40 px-2 py-1 font-mono text-[10px] text-cyan-300">{item.format}</span>
                </div>
                <div className="mt-1 flex items-center gap-2 text-xs text-zinc-600">
                  <span>{item.is_system ? "Системный" : "Пользовательский"}</span>
                  <span>·</span>
                  <span className={item.enabled ? "text-emerald-400" : "text-zinc-600"}>{item.enabled ? "Включён" : "Выключен"}</span>
                </div>
              </button>
            ))}
          </div>
        </aside>

        <section className="technical-frame border border-border bg-surface-1">
          <div className="border-b border-border px-5 py-4">
            <h2 className="font-semibold text-zinc-200">{selected ? `Редактирование: ${selected.name}` : "Новый шаблон"}</h2>
            <p className="mt-1 text-xs text-zinc-600">Пустое содержимое использует стандартный генератор выбранного формата.</p>
          </div>
          <div className="space-y-5 p-5">
            <div className="grid gap-4 md:grid-cols-[1fr_240px]">
              <Input label="Название" value={draft.name} disabled={!isOwner} onChange={(event) => setDraft((value) => ({ ...value, name: event.target.value }))} />
              <label className="flex flex-col gap-1.5 text-sm text-zinc-400">
                Формат
                <select
                  value={draft.format}
                  disabled={!isOwner}
                  onChange={(event) => setDraft((value) => ({ ...value, format: event.target.value as SubscriptionTemplateFormat }))}
                  className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus:outline-none focus:ring-2 focus:ring-accent"
                >
                  <option value="base64">Base64</option>
                  <option value="plain">Plain links</option>
                  <option value="xray-json">Xray JSON</option>
                  <option value="mihomo">Mihomo YAML</option>
                  <option value="sing-box">Sing-box JSON</option>
                </select>
              </label>
            </div>

            <label className="flex flex-col gap-1.5 text-sm text-zinc-400">
              Содержимое
              <textarea
                value={draft.content}
                disabled={!isOwner}
                onChange={(event) => setDraft((value) => ({ ...value, content: event.target.value }))}
                spellCheck={false}
                className="min-h-[360px] resize-y rounded-xl border border-border bg-[#0a0f16] p-4 font-mono text-sm leading-6 text-zinc-300 focus:outline-none focus:ring-2 focus:ring-accent"
                placeholder={"# Для Mihomo/Sing-box добавьте {{subscription}}\n# Пустое значение использует стандартную выдачу"}
              />
            </label>

            <label className="flex items-center gap-3 rounded-xl border border-border bg-zinc-950/30 px-4 py-3 text-sm text-zinc-300">
              <input type="checkbox" checked={draft.enabled} disabled={!isOwner} onChange={(event) => setDraft((value) => ({ ...value, enabled: event.target.checked }))} className="h-4 w-4 accent-cyan-500" />
              Шаблон доступен для response rules
            </label>
          </div>
          <div className="flex items-center justify-between border-t border-border px-5 py-4">
            <div>{isOwner && selected && !selected.is_system && <Button variant="danger" onClick={remove}><Trash2 className="h-4 w-4" />Удалить</Button>}</div>
            <div className="flex gap-2">
              <Button variant="outline" onClick={showPreview} loading={previewing} disabled={!draft.name.trim()}><Eye className="h-4 w-4" />Preview</Button>
              {isOwner && <Button onClick={save} loading={saving} disabled={!draft.name.trim()}><Save className="h-4 w-4" />Сохранить</Button>}
            </div>
          </div>
        </section>
      </div>
      {preview && (
      <section className="technical-frame border border-accent/20 bg-surface-1">
          <div className="flex items-center justify-between border-b border-border px-5 py-4">
            <div><h2 className="font-semibold text-zinc-200">Предварительный просмотр</h2><p className="mt-1 font-mono text-xs text-cyan-400">{preview.content_type}</p></div>
            <button type="button" onClick={() => setPreview(null)} aria-label="Закрыть preview" className="rounded-lg p-2 text-zinc-500 hover:bg-zinc-800"><X className="h-4 w-4" /></button>
          </div>
          <pre className="max-h-[520px] overflow-auto whitespace-pre-wrap p-5 font-mono text-xs leading-6 text-zinc-300">{preview.content}</pre>
        </section>
      )}
    </div>
  );
}
