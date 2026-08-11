"use client";

import { FormEvent, useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { routingSettings as routingSettingsApi } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import type { RoutingDeliveryMode, RoutingSettings } from "@/lib/types";

type RoutingTab = "main" | "dns" | "geo" | "rules" | "json";

interface RoutingConfig {
  Name: string;
  GlobalProxy: boolean;
  RouteOrder: string;
  DomainStrategy: string;
  FakeDNS: boolean;
  UseChunkFiles: boolean;
  RemoteDNSType: string;
  RemoteDNSDomain: string;
  RemoteDNSIP: string;
  DomesticDNSType: string;
  DomesticDNSDomain: string;
  DomesticDNSIP: string;
  Geoipurl: string;
  Geositeurl: string;
  LastUpdated: string;
  DnsHosts: Record<string, string>;
  DirectSites: string[];
  DirectIp: string[];
  ProxySites: string[];
  ProxyIp: string[];
  BlockSites: string[];
  BlockIp: string[];
}

const DEFAULT_ROUTING_CONFIG: RoutingConfig = {
	Name: "SubShare Routing",
  GlobalProxy: true,
  RouteOrder: "block-proxy-direct",
  DomainStrategy: "IPIfNonMatch",
  FakeDNS: false,
  UseChunkFiles: true,
  RemoteDNSType: "DoH",
  RemoteDNSDomain: "https://cloudflare-dns.com/dns-query",
  RemoteDNSIP: "1.1.1.1",
  DomesticDNSType: "DoH",
  DomesticDNSDomain: "https://dns.google/dns-query",
  DomesticDNSIP: "8.8.8.8",
  Geoipurl: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat",
  Geositeurl: "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat",
  LastUpdated: "",
  DnsHosts: {
    "cloudflare-dns.com": "1.1.1.1",
    "dns.google": "8.8.8.8",
  },
  DirectSites: [],
  DirectIp: ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "169.254.0.0/16", "224.0.0.0/4", "255.255.255.255"],
  ProxySites: [],
  ProxyIp: [],
  BlockSites: [],
  BlockIp: [],
};

const tabLabels: Record<RoutingTab, string> = {
  main: "Основные",
  dns: "DNS",
  geo: "GEO",
  rules: "Исключения и правила",
  json: "JSON-редактор",
};

function decodeBase64(value: string) {
  const normalized = value.replace(/-/g, "+").replace(/_/g, "/");
  const padding = normalized.length % 4;
  const padded = padding === 0 ? normalized : normalized + "=".repeat(4 - padding);
  const binary = atob(padded);
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
  return new TextDecoder().decode(bytes);
}

function normalizeRoutingConfig(input: unknown): RoutingConfig {
  const source = (input && typeof input === "object" ? input : {}) as Partial<RoutingConfig>;

  return {
    ...DEFAULT_ROUTING_CONFIG,
    ...source,
    DnsHosts: source.DnsHosts && typeof source.DnsHosts === "object" ? source.DnsHosts : DEFAULT_ROUTING_CONFIG.DnsHosts,
    DirectSites: Array.isArray(source.DirectSites) ? source.DirectSites.filter(Boolean) : DEFAULT_ROUTING_CONFIG.DirectSites,
    DirectIp: Array.isArray(source.DirectIp) ? source.DirectIp.filter(Boolean) : DEFAULT_ROUTING_CONFIG.DirectIp,
    ProxySites: Array.isArray(source.ProxySites) ? source.ProxySites.filter(Boolean) : DEFAULT_ROUTING_CONFIG.ProxySites,
    ProxyIp: Array.isArray(source.ProxyIp) ? source.ProxyIp.filter(Boolean) : DEFAULT_ROUTING_CONFIG.ProxyIp,
    BlockSites: Array.isArray(source.BlockSites) ? source.BlockSites.filter(Boolean) : DEFAULT_ROUTING_CONFIG.BlockSites,
    BlockIp: Array.isArray(source.BlockIp) ? source.BlockIp.filter(Boolean) : DEFAULT_ROUTING_CONFIG.BlockIp,
  };
}

function parseImportedConfig(raw: string): RoutingConfig {
  const trimmed = raw.trim();
  if (!trimmed) {
    throw new Error("Вставьте Happ-ссылку, Base64 или JSON");
  }

  if (trimmed.startsWith("{")) {
    return normalizeRoutingConfig(JSON.parse(trimmed));
  }

  const prefixes = ["happ://routing/add/", "happ://routing/onadd/"];
  for (const prefix of prefixes) {
    if (trimmed.startsWith(prefix)) {
      const payload = decodeURIComponent(trimmed.slice(prefix.length));
      return normalizeRoutingConfig(JSON.parse(decodeBase64(payload)));
    }
  }

  return normalizeRoutingConfig(JSON.parse(decodeBase64(trimmed)));
}

function buildConfigJSON(config: RoutingConfig) {
  return JSON.stringify(config, null, 2);
}

function ListEditor({
  title,
  items,
  onChange,
}: {
  title: string;
  items: string[];
  onChange: (next: string[]) => void;
}) {
  const updateItem = (index: number, value: string) => {
    const next = [...items];
    next[index] = value;
    onChange(next);
  };

  const removeItem = (index: number) => {
    onChange(items.filter((_, itemIndex) => itemIndex !== index));
  };

  return (
    <div className="flex flex-col gap-2 rounded-xl border border-border bg-surface-2/40 p-3">
      <div className="text-sm font-medium text-zinc-200">{title}</div>
      {items.length === 0 && <div className="text-xs text-zinc-500">Список пуст</div>}
		<div className="flex max-h-64 flex-col gap-2 overflow-y-auto pr-1">
			{items.map((item, index) => (
				<div key={`${title}-${index}`} className="flex items-center gap-2">
					<input
						aria-label={`${title}, элемент ${index + 1}`}
						value={item}
						onChange={(e) => updateItem(index, e.target.value)}
						className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
					/>
					<Button type="button" variant="danger" className="text-xs" onClick={() => removeItem(index)}>
						Удалить
					</Button>
				</div>
			))}
		</div>
      <Button type="button" variant="ghost" className="text-xs self-start" onClick={() => onChange([...items, ""])}>
        Добавить элемент
      </Button>
    </div>
  );
}

const deliveryModes: Array<{
  value: RoutingDeliveryMode;
  label: string;
  hint: string;
  description: string;
}> = [
  {
    value: "disabled",
    label: "Не передавать",
    hint: "Не управлять routing",
    description: "SubShare сохранит профиль, но не будет включать его в ответы подписки.",
  },
  {
    value: "add",
    label: "Добавлять профиль",
    hint: "happ://routing/add/...",
    description: "Happ получит профиль при добавлении или обновлении подписки, не меняя активный профиль автоматически.",
  },
  {
    value: "onadd",
    label: "Добавлять и активировать",
    hint: "happ://routing/onadd/...",
    description: "Happ получит профиль при добавлении или обновлении подписки и сделает его активным.",
  },
];

function ManualLinkRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-2 border-t border-border py-2.5 first:border-t-0 first:pt-0 last:pb-0">
      <div className="min-w-0">
        <div className="text-xs font-medium text-zinc-300">{label}</div>
        <output
          aria-label={`${label}, Happ-ссылка`}
          title={value}
          className="mt-1 block truncate font-mono text-xs text-zinc-600"
        >
          {value || "Ссылка появится после сохранения конфигурации"}
        </output>
      </div>
      <Button
        type="button"
        variant="ghost"
        className="shrink-0 px-2.5 text-xs"
        aria-label={`Скопировать: ${label}`}
        disabled={!value}
        onClick={() => void copyToClipboard(value)}
      >
        Копировать
      </Button>
    </div>
  );
}

interface RoutingSettingsModalProps {
	open: boolean;
	onClose: () => void;
	onSaved?: (settings: RoutingSettings) => void;
}

export function RoutingSettingsModal({ open, onClose, onSaved }: RoutingSettingsModalProps) {
	const { toast } = useToast();
	const [loading, setLoading] = useState(false);
	const [tab, setTab] = useState<RoutingTab>("main");
	const [deliveryMode, setDeliveryMode] = useState<RoutingDeliveryMode>("disabled");
	const [manualLinks, setManualLinks] = useState({ add: "", onadd: "", off: "happ://routing/off" });
	const [initialState, setInitialState] = useState("");
  const [initialConfigState, setInitialConfigState] = useState("");
  const [importValue, setImportValue] = useState("");
  const [dnsHostsText, setDnsHostsText] = useState(JSON.stringify(DEFAULT_ROUTING_CONFIG.DnsHosts, null, 2));
  const [jsonText, setJsonText] = useState(buildConfigJSON(DEFAULT_ROUTING_CONFIG));
  const [config, setConfig] = useState<RoutingConfig>(DEFAULT_ROUTING_CONFIG);
  const [jsonError, setJsonError] = useState("");

  const syncConfig = (nextConfig: RoutingConfig) => {
    setConfig(nextConfig);
    setJsonText(buildConfigJSON(nextConfig));
    setDnsHostsText(JSON.stringify(nextConfig.DnsHosts, null, 2));
    setJsonError("");
  };

  useEffect(() => {
    if (!open) {
      return;
    }

    const load = async () => {
      setLoading(true);
		try {
			const response = await routingSettingsApi.get();
			const hasStoredConfig = response.config_json.trim() !== "";
			const nextConfig = hasStoredConfig
				? normalizeRoutingConfig(JSON.parse(response.config_json))
				: DEFAULT_ROUTING_CONFIG;
			const nextMode = response.delivery_mode ?? "disabled";
			syncConfig(nextConfig);
			setDeliveryMode(nextMode);
			setManualLinks({
				add: response.add_url ?? "",
				onadd: response.onadd_url ?? "",
				off: response.off_url || "happ://routing/off",
			});
			setInitialState(JSON.stringify({ config: hasStoredConfig ? nextConfig : null, deliveryMode: nextMode }));
      setInitialConfigState(JSON.stringify(hasStoredConfig ? nextConfig : null));
		} catch {
			syncConfig(DEFAULT_ROUTING_CONFIG);
			setDeliveryMode("disabled");
			setManualLinks({ add: "", onadd: "", off: "happ://routing/off" });
			setInitialState(JSON.stringify({ config: DEFAULT_ROUTING_CONFIG, deliveryMode: "disabled" }));
      setInitialConfigState(JSON.stringify(DEFAULT_ROUTING_CONFIG));
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [open]);

  const updateField = <K extends keyof RoutingConfig>(key: K, value: RoutingConfig[K]) => {
    syncConfig({ ...config, [key]: value });
  };

  const handleImport = () => {
    try {
      syncConfig(parseImportedConfig(importValue));
      toast("Конфигурация роутинга импортирована", "success");
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось импортировать конфигурацию", "error");
    }
  };

  const handleDnsHostsBlur = () => {
    try {
      const parsed = JSON.parse(dnsHostsText) as Record<string, string>;
      updateField("DnsHosts", parsed);
      setJsonError("");
    } catch {
      setJsonError("DnsHosts должен быть валидным JSON-объектом");
    }
  };

  const handleJsonChange = (nextJson: string) => {
    setJsonText(nextJson);
    try {
      syncConfig(normalizeRoutingConfig(JSON.parse(nextJson)));
    } catch {
      setJsonError("JSON-конфигурация заполнена некорректно");
    }
  };

	const handleSave = async (e: FormEvent) => {
		e.preventDefault();
		if (deliveryMode !== "disabled" && !config.Name.trim()) {
			toast("Укажите Name профиля перед включением доставки с подпиской", "error");
			return;
		}
		setLoading(true);
		try {
			const response = await routingSettingsApi.update({
				config_json: buildConfigJSON(config),
				delivery_mode: deliveryMode,
			});
			setManualLinks({ add: response.add_url, onadd: response.onadd_url, off: response.off_url });
			setInitialState(JSON.stringify({ config, deliveryMode }));
      setInitialConfigState(JSON.stringify(config));
			onSaved?.(response);
			toast("Настройки роутинга сохранены", "success");
      onClose();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить настройки роутинга", "error");
    } finally {
      setLoading(false);
    }
  };

	const dirty = initialState !== "" && JSON.stringify({ config, deliveryMode }) !== initialState;
  const manualLinksStale = initialConfigState !== "" && JSON.stringify(config) !== initialConfigState;
  const selectedDeliveryMode = deliveryModes.find((mode) => mode.value === deliveryMode) ?? deliveryModes[0];

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Роутинг"
      className="max-h-[calc(100dvh-1.5rem)] w-[96vw] max-w-[1660px]"
      contentClassName="flex min-h-0 flex-1 flex-col overflow-hidden"
    >
      <form onSubmit={handleSave} className="flex min-h-0 flex-1 flex-col overflow-hidden">
        <div
          data-testid="routing-scroll-area"
          className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden overscroll-contain pr-1"
        >
          <fieldset
            data-testid="routing-delivery-section"
            className="grid gap-3 rounded-xl border border-border bg-surface-2/30 p-4"
          >
            <legend className="px-1 text-balance text-sm font-semibold text-zinc-200">Доставка с подпиской</legend>
            <p className="text-pretty text-xs leading-5 text-zinc-500">
              Автоматическая передача текущего routing-профиля при получении или обновлении подписки в Happ.
            </p>
            <div data-testid="routing-delivery-options" className="grid gap-2 lg:grid-cols-3">
              {deliveryModes.map((mode) => (
                <label
                  key={mode.value}
                  className={`min-w-0 cursor-pointer rounded-lg border px-3 py-2.5 transition-colors ${
                    deliveryMode === mode.value
                      ? "border-accent/50 bg-accent/8"
                      : "border-border bg-zinc-950/20 hover:border-zinc-700"
                  }`}
                >
                  <span className="flex items-start gap-2.5">
                    <input
                      type="radio"
                      name="routing-delivery-mode"
                      value={mode.value}
                      checked={deliveryMode === mode.value}
                      onChange={() => setDeliveryMode(mode.value)}
                      className="mt-0.5 size-4 accent-accent"
                    />
                    <span className="min-w-0">
                      <span className="block text-sm font-medium text-zinc-200">{mode.label}</span>
                      <span className="mt-0.5 block truncate font-mono text-xs text-zinc-600">{mode.hint}</span>
                    </span>
                  </span>
                </label>
              ))}
            </div>
            <div className="rounded-lg border border-border/70 bg-zinc-950/20 px-3 py-2.5">
              <div className="text-xs font-medium text-zinc-300">{selectedDeliveryMode.label}</div>
              <p className="mt-1 text-pretty text-xs leading-5 text-zinc-500">{selectedDeliveryMode.description}</p>
            </div>
          </fieldset>

          <div
            data-testid="routing-workspace"
            className="mt-4 grid min-h-0 items-start gap-4 xl:grid-cols-[minmax(18.75rem,21.25rem)_minmax(0,1fr)]"
          >
            <aside className="grid min-w-0 gap-4" aria-label="Утилиты роутинга">
              <div data-testid="routing-import-card" className="grid gap-3 rounded-xl border border-border bg-surface-2/30 p-4">
                <div>
                  <div className="text-balance text-sm font-semibold text-zinc-200">Импорт конфигурации</div>
                  <p className="mt-1 text-pretty text-xs leading-5 text-zinc-500">Happ-ссылка, Base64 или JSON.</p>
                </div>
                <textarea
                  aria-label="Импорт конфигурации"
                  value={importValue}
                  onChange={(e) => setImportValue(e.target.value)}
                  placeholder="Вставьте Happ-ссылку (happ://routing/add/...) или Base64/JSON конфигурации"
                  className="min-h-16 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                />
                <div className="flex justify-end">
                  <Button type="button" variant="ghost" className="text-xs" onClick={handleImport}>
                    Импортировать
                  </Button>
                </div>
              </div>

              <div data-testid="routing-manual-card" className="grid gap-3 rounded-xl border border-border bg-surface-2/30 p-4">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <div className="text-balance text-sm font-semibold text-zinc-200">Ручная установка</div>
                    <p className="mt-1 text-pretty text-xs leading-5 text-zinc-500">
                      Серверные ссылки для последней сохранённой конфигурации.
                    </p>
                  </div>
                  {manualLinksStale && (
                    <span
                      data-testid="manual-links-stale"
                      className="inline-flex items-center gap-1.5 rounded-sm border border-amber-400/20 bg-amber-400/5 px-2 py-1 text-xs text-amber-300"
                    >
                      <span className="size-1.5 rounded-full bg-amber-300" aria-hidden="true" />
                      Требуется сохранение
                    </span>
                  )}
                </div>
                <div>
                  <ManualLinkRow label="Добавить профиль" value={manualLinks.add} />
                  <ManualLinkRow label="Добавить и активировать" value={manualLinks.onadd} />
                  <ManualLinkRow label="Отключить роутинг" value={manualLinks.off} />
                </div>
              </div>
            </aside>

            <div data-testid="routing-editor" className="grid min-w-0 content-start gap-4 rounded-xl border border-border bg-surface-2/30 p-4">
              <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
                <div>
                  <div className="text-sm font-semibold text-zinc-200">Визуальный редактор</div>
                  <div className="mt-1 text-xs text-zinc-500">
                    Основные параметры занимают всю ширину окна. Служебные ссылки и импорт вынесены в левую колонку.
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  {(Object.keys(tabLabels) as RoutingTab[]).map((tabKey) => (
                    <Button
                      key={tabKey}
                      type="button"
                      variant={tab === tabKey ? "primary" : "ghost"}
                      className="text-xs"
                      onClick={() => setTab(tabKey)}
                    >
                      {tabLabels[tabKey]}
                    </Button>
                  ))}
                </div>
              </div>

            {tab === "main" && (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                <Input label="Name" value={config.Name} onChange={(e) => updateField("Name", e.target.value)} />
                <Select
                  label="GlobalProxy"
                  value={String(config.GlobalProxy)}
                  onChange={(e) => updateField("GlobalProxy", e.target.value === "true")}
                  options={[{ value: "true", label: "Включено" }, { value: "false", label: "Выключено" }]}
                />
                <Select
                  label="RouteOrder"
                  value={config.RouteOrder}
                  onChange={(e) => updateField("RouteOrder", e.target.value)}
                  options={[
                    { value: "block-proxy-direct", label: "Block -> Proxy -> Direct" },
                    { value: "block-direct-proxy", label: "Block -> Direct -> Proxy" },
                    { value: "proxy-direct-block", label: "Proxy -> Direct -> Block" },
                    { value: "proxy-block-direct", label: "Proxy -> Block -> Direct" },
                    { value: "direct-proxy-block", label: "Direct -> Proxy -> Block" },
                    { value: "direct-block-proxy", label: "Direct -> Block -> Proxy" },
                  ]}
                />
                <Select
                  label="DomainStrategy"
                  value={config.DomainStrategy}
                  onChange={(e) => updateField("DomainStrategy", e.target.value)}
                  options={[
                    { value: "IPIfNonMatch", label: "IPIfNonMatch" },
                    { value: "IPOnDemand", label: "IPOnDemand" },
                    { value: "AsIs", label: "AsIs" },
                  ]}
                />
                <Select
                  label="FakeDNS"
                  value={String(config.FakeDNS)}
                  onChange={(e) => updateField("FakeDNS", e.target.value === "true")}
                  options={[{ value: "true", label: "Включено" }, { value: "false", label: "Выключено" }]}
                />
                <Select
                  label="UseChunkFiles"
                  value={String(config.UseChunkFiles)}
                  onChange={(e) => updateField("UseChunkFiles", e.target.value === "true")}
                  options={[{ value: "true", label: "Включено" }, { value: "false", label: "Выключено" }]}
                />
              </div>
            )}

            {tab === "dns" && (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                <Select
                  label="RemoteDNSType"
                  value={config.RemoteDNSType}
                  onChange={(e) => updateField("RemoteDNSType", e.target.value)}
                  options={[{ value: "DoH", label: "DoH (DNS-over-HTTPS)" }, { value: "DoU", label: "DoU (DNS-over-UDP)" }]}
                />
                <Input label="Remote DNS Domain" value={config.RemoteDNSDomain} onChange={(e) => updateField("RemoteDNSDomain", e.target.value)} />
                <Input label="Remote DNS IP" value={config.RemoteDNSIP} onChange={(e) => updateField("RemoteDNSIP", e.target.value)} />
                <Select
                  label="DomesticDNSType"
                  value={config.DomesticDNSType}
                  onChange={(e) => updateField("DomesticDNSType", e.target.value)}
                  options={[{ value: "DoH", label: "DoH (DNS-over-HTTPS)" }, { value: "DoU", label: "DoU (DNS-over-UDP)" }]}
                />
                <Input label="Domestic DNS Domain" value={config.DomesticDNSDomain} onChange={(e) => updateField("DomesticDNSDomain", e.target.value)} />
                <Input label="Domestic DNS IP" value={config.DomesticDNSIP} onChange={(e) => updateField("DomesticDNSIP", e.target.value)} />
                <div className="md:col-span-2 xl:col-span-3 2xl:col-span-4">
                  <label htmlFor="dns-hosts" className="mb-1.5 block text-sm text-zinc-400">
                    DnsHosts
                  </label>
                  <textarea
                    id="dns-hosts"
                    value={dnsHostsText}
                    onChange={(e) => setDnsHostsText(e.target.value)}
                    onBlur={handleDnsHostsBlur}
                    className="min-h-28 w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                  />
                </div>
              </div>
            )}

            {tab === "geo" && (
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                <Input label="Geoipurl" value={config.Geoipurl} onChange={(e) => updateField("Geoipurl", e.target.value)} />
                <Input label="Geositeurl" value={config.Geositeurl} onChange={(e) => updateField("Geositeurl", e.target.value)} />
                <Input label="LastUpdated" value={config.LastUpdated} onChange={(e) => updateField("LastUpdated", e.target.value)} />
              </div>
            )}

            {tab === "rules" && (
              <div className="ui-joined-grid grid grid-cols-1 xl:grid-cols-2 2xl:grid-cols-3">
                <ListEditor title="DirectSites" items={config.DirectSites} onChange={(next) => updateField("DirectSites", next)} />
                <ListEditor title="DirectIp" items={config.DirectIp} onChange={(next) => updateField("DirectIp", next)} />
                <ListEditor title="ProxySites" items={config.ProxySites} onChange={(next) => updateField("ProxySites", next)} />
                <ListEditor title="ProxyIp" items={config.ProxyIp} onChange={(next) => updateField("ProxyIp", next)} />
                <ListEditor title="BlockSites" items={config.BlockSites} onChange={(next) => updateField("BlockSites", next)} />
                <ListEditor title="BlockIp" items={config.BlockIp} onChange={(next) => updateField("BlockIp", next)} />
              </div>
            )}

            {tab === "json" && (
              <div className="grid gap-3">
                <div className="text-xs text-zinc-500">
                  Прямое редактирование JSON-конфигурации. Изменения сразу синхронизируются с визуальными вкладками.
                </div>
                <textarea
                  aria-label="JSON-конфигурация роутинга"
                  value={jsonText}
                  onChange={(e) => handleJsonChange(e.target.value)}
                  className="min-h-96 w-full rounded-lg border border-border bg-surface-2 px-3 py-3 font-mono text-xs text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                />
                {jsonError && <div className="text-xs text-red-300">{jsonError}</div>}
              </div>
            )}
          </div>
        </div>
        </div>

        <div
          data-testid="routing-action-footer"
          className="mt-3 flex shrink-0 flex-col gap-2 border-t border-border bg-surface-1 pt-3 sm:flex-row sm:items-center"
        >
          <Button type="button" variant="ghost" className="w-full sm:w-auto" onClick={() => syncConfig(DEFAULT_ROUTING_CONFIG)}>
            Сбросить
          </Button>
          <span className={`text-center text-xs sm:mx-auto ${dirty ? "text-amber-300" : "text-zinc-600"}`}>
            {dirty ? "Есть несохранённые изменения" : "Все изменения сохранены"}
          </span>
          <Button type="submit" className="w-full sm:w-auto" loading={loading} disabled={!dirty}>
            Применить
          </Button>
        </div>
      </form>
    </Modal>
  );
}
