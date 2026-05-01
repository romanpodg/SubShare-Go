"use client";

import { useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { subscriptionPageConfig as subscriptionPageConfigApi } from "@/lib/api";
import { subscriptionPageConfig } from "@/app/subscription/pageConfig";

interface Props {
  open: boolean;
  onClose: () => void;
}

const CONFIG_KEY_ORDER = [
  "locale",
  "templateVars",
  "theme",
  "blocks",
  "type",
  "id",
  "brandTitle",
  "brandSubtitle",
  "subhead",
  "logo",
  "src",
  "alt",
  "statusBadge",
  "title",
  "steps",
  "description",
  "block",
  "buttons",
  "label",
  "href",
  "variant",
  "external",
  "icon",
  "addButtonLabel",
  "manualLinkLabel",
  "copyLabel",
  "copiedLabel",
  "copyright",
  "languageBadge",
  "footerLink",
];

function orderConfigValue(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(orderConfigValue);
  }

  if (!value || typeof value !== "object") {
    return value;
  }

  const source = value as Record<string, unknown>;
  const ordered: Record<string, unknown> = {};
  const used = new Set<string>();

  CONFIG_KEY_ORDER.forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(source, key)) {
      ordered[key] = orderConfigValue(source[key]);
      used.add(key);
    }
  });

  Object.keys(source).forEach((key) => {
    if (!used.has(key)) {
      ordered[key] = orderConfigValue(source[key]);
    }
  });

  return ordered;
}

function stringifyOrderedConfig(value: unknown) {
  return `${JSON.stringify(orderConfigValue(value), null, 2)}\n`;
}

function localDefaultConfigJSON() {
  return stringifyOrderedConfig(subscriptionPageConfig);
}

export function SubscriptionPageConfigModal({ open, onClose }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [configJSON, setConfigJSON] = useState("");
  const [defaultConfigJSON, setDefaultConfigJSON] = useState("");
  const [initialConfigJSON, setInitialConfigJSON] = useState("");

  const isDirty = useMemo(
    () => configJSON.trim() !== initialConfigJSON.trim(),
    [configJSON, initialConfigJSON]
  );

  useEffect(() => {
    if (!open) return;

    const load = async () => {
      setLoading(true);
      try {
        const response = await subscriptionPageConfigApi.getAdmin();
        const defaultJSON = response.default_config_json || localDefaultConfigJSON();
        const loadedJSON = response.config_json || defaultJSON;
        setConfigJSON(loadedJSON);
        setDefaultConfigJSON(defaultJSON);
        setInitialConfigJSON(loadedJSON);
      } catch (error: unknown) {
        const message = error instanceof Error ? error.message : "Не удалось загрузить конфиг страницы";
        const fallbackJSON = localDefaultConfigJSON();
        setConfigJSON(fallbackJSON);
        setDefaultConfigJSON(fallbackJSON);
        setInitialConfigJSON(fallbackJSON);
        toast(message, "error");
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [open, toast]);

  const handleFormat = () => {
    try {
      const parsed = JSON.parse(configJSON);
      setConfigJSON(stringifyOrderedConfig(parsed));
    } catch {
      toast("JSON содержит ошибку синтаксиса", "error");
    }
  };

  const handleResetToDefault = () => {
    if (!defaultConfigJSON) return;
    setConfigJSON(defaultConfigJSON);
  };

  const handleSave = async () => {
    try {
      JSON.parse(configJSON);
    } catch {
      toast("Исправьте синтаксис JSON перед сохранением", "error");
      return;
    }

    setSaving(true);
    try {
      const response = await subscriptionPageConfigApi.update(configJSON.trim());
      const saved = response.config_json || configJSON.trim();
      setConfigJSON(saved);
      setInitialConfigJSON(saved);
      toast("Конфиг страницы подписки сохранён", "success");
      onClose();
    } catch (error: unknown) {
      const message = error instanceof Error ? error.message : "Не удалось сохранить конфиг страницы";
      toast(message, "error");
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="🎨 Дизайн страницы /sub">
      <div className="flex flex-col gap-3">
        <p className="text-xs leading-relaxed text-zinc-400">
          JSON управляет блоками, текстами, кнопками и цветами страницы подписки по адресу
          <span className="font-mono text-zinc-300"> /sub/...</span>.
        </p>

        {loading ? (
          <div className="rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-400">
            Загрузка конфига...
          </div>
        ) : (
          <textarea
            value={configJSON}
            onChange={(event) => setConfigJSON(event.target.value)}
            className="h-[58vh] w-full resize-y rounded-xl border border-border bg-surface-2 p-3 font-mono text-xs leading-5 text-zinc-200 outline-none transition-colors focus:border-accent"
            spellCheck={false}
          />
        )}

        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="text-xs text-zinc-500">
            {isDirty ? "Есть несохранённые изменения" : "Изменений нет"}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="ghost" onClick={handleFormat} disabled={loading || saving}>
              Форматировать JSON
            </Button>
            <Button variant="ghost" onClick={handleResetToDefault} disabled={loading || saving || !defaultConfigJSON}>
              Сбросить к дефолту
            </Button>
            <Button variant="ghost" onClick={onClose} disabled={saving}>
              Отмена
            </Button>
            <Button onClick={handleSave} loading={saving} disabled={loading}>
              Сохранить
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
