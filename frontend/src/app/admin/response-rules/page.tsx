"use client";

import { useCallback, useEffect, useState } from "react";
import { Activity, GripVertical, Plus, Save, Trash2, X } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { ResponseRule, ResponseRuleCondition, SubscriptionTemplate } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { useToast } from "@/components/ui/Toast";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";
import { useAuth } from "@/hooks/useAuth";

type RuleDraft = Omit<ResponseRule, "id" | "is_system" | "created_at" | "updated_at">;

const newCondition = (): ResponseRuleCondition => ({
  headerName: "user-agent",
  operator: "CONTAINS",
  value: "",
  caseSensitive: false,
});

const emptyRule = (): RuleDraft => ({
  name: "",
  description: "",
  enabled: true,
  priority: 100,
  operator: "AND",
  conditions: [newCondition()],
  response_type: "base64",
  template_id: null,
  headers: [],
});

export default function ResponseRulesPage() {
  const [rules, setRules] = useState<ResponseRule[]>([]);
  const [templates, setTemplates] = useState<SubscriptionTemplate[]>([]);
  const [selected, setSelected] = useState<ResponseRule | null>(null);
  const [draft, setDraft] = useState<RuleDraft>(emptyRule);
  const [jsonDraft, setJSONDraft] = useState(() => JSON.stringify(emptyRule(), null, 2));
  const [jsonMode, setJSONMode] = useState(false);
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const { toast } = useToast();
  const { role } = useAuth();
  const isOwner = role === "owner";

  const load = useCallback(async () => {
    setError(null);
    const [ruleResult, templateResult] = await Promise.allSettled([
      apiV1.responseRules.list(),
      apiV1.templates.list(),
    ]);
    if (ruleResult.status === "fulfilled") setRules(ruleResult.value.data);
    else setError(ruleResult.reason);
    if (templateResult.status === "fulfilled") {
      setTemplates(templateResult.value.data.filter((item) => item.enabled));
    } else if (ruleResult.status === "fulfilled") {
      setError(templateResult.reason);
    }
    setLoading(false);
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => void load());
    return () => window.cancelAnimationFrame(frame);
  }, [load]);

  const edit = (rule: ResponseRule) => {
    setSelected(rule);
    setDraft({
      name: rule.name,
      description: rule.description,
      enabled: rule.enabled,
      priority: rule.priority,
      operator: rule.operator,
      conditions: rule.conditions.length ? rule.conditions.map((item) => ({ ...item })) : [],
      response_type: rule.response_type,
      template_id: rule.template_id,
      headers: rule.headers.map((item) => ({ ...item })),
    });
    setJSONDraft(JSON.stringify({
      name: rule.name,
      description: rule.description,
      enabled: rule.enabled,
      priority: rule.priority,
      operator: rule.operator,
      conditions: rule.conditions,
      response_type: rule.response_type,
      template_id: rule.template_id,
      headers: rule.headers,
    }, null, 2));
  };

  const create = () => {
    if (!isOwner) return;
    setSelected(null);
    const next = emptyRule();
    setDraft(next);
    setJSONDraft(JSON.stringify(next, null, 2));
  };

  const save = async () => {
    if (!isOwner) return;
    setSaving(true);
    try {
      if (selected) await apiV1.responseRules.update(selected.id, draft);
      else await apiV1.responseRules.create(draft);
      await load();
      toast("Правило сохранено", "success");
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить правило", "error");
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!isOwner || !selected || selected.is_system) return;
    try {
      await apiV1.responseRules.delete(selected.id);
      setSelected(null);
      setDraft(emptyRule());
      await load();
      toast("Правило удалено", "success");
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось удалить правило", "error");
    }
  };

  const updateCondition = (index: number, patch: Partial<ResponseRuleCondition>) => {
    setDraft((current) => ({
      ...current,
      conditions: current.conditions.map((condition, conditionIndex) =>
        conditionIndex === index ? { ...condition, ...patch } : condition
      ),
    }));
  };

  const applyJSONDraft = () => {
    if (!isOwner) return;
    try {
      const parsed = JSON.parse(jsonDraft) as RuleDraft;
      setDraft(parsed);
      toast("JSON применён к визуальному редактору", "success");
      setJSONMode(false);
    } catch {
      toast("JSON содержит синтаксическую ошибку", "error");
    }
  };

  if (loading && rules.length === 0) {
    return <InitialLoading label="Загрузка правил ответов…" />;
  }

  if (error && rules.length === 0) {
    return <ResourceError error={error} onRetry={() => void load()} />;
  }

  return (
    <div>
      <PageHeader
        title="Правила ответов"
        description="Правила выполняются сверху вниз. Первое совпадение определяет формат, шаблон и дополнительные заголовки."
        icon={<Activity className="h-5 w-5" />}
        actions={isOwner ? <Button onClick={create}><Plus className="h-4 w-4" />Новое правило</Button> : undefined}
      />

      {Boolean(error) && <div className="mb-4"><ResourceError error={error} compact onRetry={() => void load()} /></div>}
      {!isOwner && (
        <div className="mb-4 rounded-xl border border-amber-400/20 bg-amber-400/5 px-4 py-3 text-sm text-amber-200">
          Режим просмотра: изменять правила ответов может только владелец.
        </div>
      )}

      <div className="ui-joined-grid technical-frame grid xl:grid-cols-[360px_1fr]">
        <aside className="technical-frame overflow-hidden border border-border bg-surface-1">
          <div className="border-b border-border px-4 py-3 text-xs font-semibold uppercase tracking-wider text-zinc-600">
            Порядок выполнения
          </div>
          <div className="ui-selection-list">
            {rules.map((rule) => (
              <button
                key={rule.id}
                type="button"
                onClick={() => edit(rule)}
                aria-current={selected?.id === rule.id}
                className="ui-selection-row flex items-start gap-3 px-4 py-4"
              >
                <GripVertical className="mt-0.5 h-4 w-4 shrink-0 text-zinc-700" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-3">
                    <span className="truncate text-sm font-medium text-zinc-200">{rule.name}</span>
                    <span className="font-mono text-[10px] text-zinc-600">{rule.priority}</span>
                  </div>
                  <div className="mt-1 flex items-center gap-2 text-xs">
                    <span className="text-cyan-400">{rule.response_type}</span>
                    <span className={rule.enabled ? "text-emerald-400" : "text-zinc-600"}>{rule.enabled ? "включено" : "выключено"}</span>
                  </div>
                </div>
              </button>
            ))}
          </div>
        </aside>

        <fieldset disabled={!isOwner} className="min-w-0">
        <section className="technical-frame border border-border bg-surface-1">
          <div className="border-b border-border px-5 py-4">
            <div className="flex items-start justify-between gap-4">
              <div><h2 className="font-semibold text-zinc-200">{selected ? selected.name : "Новое правило"}</h2><p className="mt-1 text-xs text-zinc-600">Пустой список условий соответствует любому запросу и подходит только для fallback.</p></div>
              <Button variant="outline" onClick={() => {
                if (!jsonMode) setJSONDraft(JSON.stringify(draft, null, 2));
                setJSONMode((current) => !current);
              }}>{jsonMode ? "Визуальный режим" : "JSON-режим"}</Button>
            </div>
          </div>

          <div className="space-y-6 p-5">
            {jsonMode && (
              <div className="rounded-xl border border-cyan-400/20 bg-[#080d13] p-4">
                <textarea
                  aria-label="JSON правила"
                  value={jsonDraft}
                  onChange={(event) => setJSONDraft(event.target.value)}
                  spellCheck={false}
                  className="min-h-[420px] w-full resize-y bg-transparent font-mono text-sm leading-6 text-zinc-300 outline-none"
                />
                <div className="mt-3 flex justify-end"><Button onClick={applyJSONDraft}>Применить JSON</Button></div>
              </div>
            )}
            {!jsonMode && (
              <>
            <div className="grid gap-4 md:grid-cols-[1fr_140px]">
              <Input label="Название" value={draft.name} onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} />
              <Input label="Приоритет" type="number" min={0} max={100000} value={draft.priority} onChange={(event) => setDraft((current) => ({ ...current, priority: Number(event.target.value) }))} />
            </div>
            <Input label="Описание" value={draft.description} onChange={(event) => setDraft((current) => ({ ...current, description: event.target.value }))} />

            <div className="grid gap-4 md:grid-cols-3">
              <Select
                label="Логика условий"
                value={draft.operator}
                onChange={(event) => setDraft((current) => ({ ...current, operator: event.target.value as "AND" | "OR" }))}
                options={[
                  { value: "AND", label: "Все условия (AND)" },
                  { value: "OR", label: "Любое условие (OR)" },
                ]}
              />
              <Select
                label="Тип ответа"
                value={draft.response_type}
                onChange={(event) => setDraft((current) => ({ ...current, response_type: event.target.value as RuleDraft["response_type"] }))}
                options={[
                  { value: "browser", label: "Browser page" },
                  { value: "base64", label: "Base64" },
                  { value: "plain", label: "Plain" },
                  { value: "xray-json", label: "Xray JSON" },
                  { value: "mihomo", label: "Mihomo" },
                  { value: "sing-box", label: "Sing-box" },
                  { value: "block", label: "HTTP 403" },
                  { value: "not-found", label: "HTTP 404" },
                ]}
              />
              <Select
                label="Шаблон"
                value={String(draft.template_id ?? "")}
                onChange={(event) => setDraft((current) => ({ ...current, template_id: event.target.value ? Number(event.target.value) : null }))}
                options={[
                  { value: "", label: "Стандартный генератор" },
                  ...templates.map((template) => ({ value: String(template.id), label: `${template.name} (${template.format})` })),
                ]}
              />
            </div>

            <div>
              <div className="mb-3 flex items-center justify-between">
                <div><h3 className="text-sm font-semibold text-zinc-300">Условия</h3><p className="mt-1 text-xs text-zinc-600">Сопоставление HTTP-заголовков без учёта регистра имени.</p></div>
                <Button variant="outline" onClick={() => setDraft((current) => ({ ...current, conditions: [...current.conditions, newCondition()] }))}><Plus className="h-4 w-4" />Добавить</Button>
              </div>
              <div className="ui-joined-list">
                {draft.conditions.map((condition, index) => (
                  <div key={index} className="grid gap-3 rounded-xl border border-border bg-zinc-950/30 p-3 md:grid-cols-[1fr_180px_1fr_auto]">
                    <Input aria-label="Заголовок" placeholder="user-agent" value={condition.headerName} onChange={(event) => updateCondition(index, { headerName: event.target.value })} />
                    <Select
                      ariaLabel="Оператор условия"
                      value={condition.operator}
                      onChange={(event) => updateCondition(index, { operator: event.target.value as ResponseRuleCondition["operator"] })}
                      options={[
                        { value: "EQUALS", label: "EQUALS" },
                        { value: "NOT_EQUALS", label: "NOT EQUALS" },
                        { value: "CONTAINS", label: "CONTAINS" },
                        { value: "NOT_CONTAINS", label: "NOT CONTAINS" },
                        { value: "STARTS_WITH", label: "STARTS WITH" },
                        { value: "ENDS_WITH", label: "ENDS WITH" },
                        { value: "REGEX", label: "REGEX" },
                        { value: "NOT_REGEX", label: "NOT REGEX" },
                      ]}
                    />
                    <Input aria-label="Значение" placeholder="happ" value={condition.value} onChange={(event) => updateCondition(index, { value: event.target.value })} />
                    <button type="button" onClick={() => setDraft((current) => ({ ...current, conditions: current.conditions.filter((_, itemIndex) => itemIndex !== index) }))} className="rounded-lg p-2 text-zinc-600 hover:bg-rose-500/10 hover:text-rose-300" aria-label="Удалить условие"><X className="h-4 w-4" /></button>
                    <label className="flex items-center gap-2 text-xs text-zinc-500 md:col-span-4">
                      <input type="checkbox" checked={condition.caseSensitive} onChange={(event) => updateCondition(index, { caseSensitive: event.target.checked })} className="accent-cyan-500" />
                      Учитывать регистр
                    </label>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <div className="mb-3 flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-semibold text-zinc-300">Заголовки ответа</h3>
                  <p className="mt-1 text-xs text-zinc-600">
                    Cookie, Content-Type, Content-Length и transport/security-заголовки запрещены сервером.
                  </p>
                </div>
                <Button
                  variant="outline"
                  onClick={() =>
                    setDraft((current) => ({
                      ...current,
                      headers: [...current.headers, { key: "", value: "" }],
                    }))
                  }
                >
                  <Plus className="h-4 w-4" />
                  Добавить
                </Button>
              </div>
              <div className="ui-joined-list">
                {draft.headers.map((header, index) => (
                  <div
                    key={index}
                    className="grid gap-3 rounded-xl border border-border bg-zinc-950/30 p-3 md:grid-cols-[minmax(180px,0.4fr)_1fr_auto]"
                  >
                    <Input
                      aria-label="Имя заголовка"
                      placeholder="X-Subscription-Title"
                      value={header.key}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          headers: current.headers.map((item, itemIndex) =>
                            itemIndex === index ? { ...item, key: event.target.value } : item
                          ),
                        }))
                      }
                    />
                    <Input
                      aria-label="Значение заголовка"
                      placeholder="SubShare"
                      value={header.value}
                      onChange={(event) =>
                        setDraft((current) => ({
                          ...current,
                          headers: current.headers.map((item, itemIndex) =>
                            itemIndex === index ? { ...item, value: event.target.value } : item
                          ),
                        }))
                      }
                    />
                    <button
                      type="button"
                      onClick={() =>
                        setDraft((current) => ({
                          ...current,
                          headers: current.headers.filter((_, itemIndex) => itemIndex !== index),
                        }))
                      }
                      className="rounded-lg p-2 text-zinc-600 hover:bg-rose-500/10 hover:text-rose-300"
                      aria-label="Удалить заголовок"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  </div>
                ))}
                {draft.headers.length === 0 && (
                  <div className="rounded-xl border border-dashed border-border px-4 py-6 text-center text-xs text-zinc-600">
                    Дополнительные заголовки не заданы.
                  </div>
                )}
              </div>
            </div>

            <label className="flex items-center gap-3 rounded-xl border border-border bg-zinc-950/30 px-4 py-3 text-sm text-zinc-300">
              <input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))} className="h-4 w-4 accent-cyan-500" />
              Правило включено
            </label>
              </>
            )}
          </div>

          <div className="flex items-center justify-between border-t border-border px-5 py-4">
            <div>{isOwner && selected && !selected.is_system && <Button variant="danger" onClick={remove}><Trash2 className="h-4 w-4" />Удалить</Button>}</div>
            {isOwner && <Button onClick={save} loading={saving} disabled={!draft.name.trim()}><Save className="h-4 w-4" />Сохранить</Button>}
          </div>
        </section>
        </fieldset>
      </div>
    </div>
  );
}
