"use client";

import { FormEvent, useEffect, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { routingSettings as routingSettingsApi } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";

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
  Name: "",
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

function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

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

function buildRoutingLink(config: RoutingConfig, autoActivate: boolean) {
  const payload = encodeURIComponent(encodeBase64(buildConfigJSON(config)));
  return `happ://routing/${autoActivate ? "onadd" : "add"}/${payload}`;
}

function buildDisableLink(config: RoutingConfig) {
  const payload = encodeURIComponent(encodeBase64(JSON.stringify({ Name: config.Name || "Routing" })));
  return `happ://routing/disable/${payload}`;
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
      {items.slice(0, 4).map((item, index) => (
        <div key={`${title}-${index}`} className="flex items-center gap-2">
          <input
            value={item}
            onChange={(e) => updateItem(index, e.target.value)}
            className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
          />
          <Button type="button" variant="danger" className="text-xs" onClick={() => removeItem(index)}>
            Удалить
          </Button>
        </div>
      ))}
      {items.length > 4 && <div className="text-xs text-zinc-500">Показаны первые 4 элемента из {items.length}</div>}
      <Button type="button" variant="ghost" className="text-xs self-start" onClick={() => onChange([...items, ""])}>
        Добавить элемент
      </Button>
    </div>
  );
}

export function RoutingSettingsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [tab, setTab] = useState<RoutingTab>("main");
  const [autoActivate, setAutoActivate] = useState(false);
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
        if (!response.config_json.trim()) {
          syncConfig(DEFAULT_ROUTING_CONFIG);
          return;
        }

        syncConfig(normalizeRoutingConfig(JSON.parse(response.config_json)));
      } catch {
        syncConfig(DEFAULT_ROUTING_CONFIG);
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
    setLoading(true);
    try {
      await routingSettingsApi.update(buildConfigJSON(config));
      toast("Настройки роутинга сохранены", "success");
      onClose();
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось сохранить настройки роутинга", "error");
    } finally {
      setLoading(false);
    }
  };

  const generatedLink = buildRoutingLink(config, autoActivate);
  const disableLink = buildDisableLink(config);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Роутинг"
      className="w-[96vw] max-w-[1660px] max-h-[94vh] overflow-y-auto"
    >
      <form onSubmit={handleSave} className="flex flex-col gap-4">
        <div className="grid items-start gap-4 xl:grid-cols-[25rem_minmax(0,1fr)] 2xl:grid-cols-[28rem_minmax(0,1fr)]">
          <div className="flex flex-col gap-4">
            <div className="grid gap-3 rounded-xl border border-border bg-surface-2/30 p-4">
              <div className="text-sm font-semibold text-zinc-200">Импорт конфигурации</div>
              <textarea
                value={importValue}
                onChange={(e) => setImportValue(e.target.value)}
                placeholder="Вставьте Happ-ссылку (happ://routing/add/...) или Base64/JSON конфигурации"
                className="min-h-20 rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
              />
              <div className="flex justify-end">
                <Button type="button" variant="ghost" className="text-xs" onClick={handleImport}>
                  Импортировать
                </Button>
              </div>
            </div>

            <div className="grid gap-3 rounded-xl border border-border bg-surface-2/30 p-4">
              <div className="text-sm font-semibold text-zinc-200">Ссылки</div>
              <div className="flex flex-col gap-2 2xl:flex-row 2xl:items-start">
                <Button
                  type="button"
                  variant={autoActivate ? "primary" : "ghost"}
                  className="min-w-36 text-xs"
                  onClick={() => setAutoActivate((prev) => !prev)}
                >
                  Авто-активация ({autoActivate ? "onadd" : "add"})
                </Button>
                <span className="text-xs text-zinc-500 2xl:pt-1">
                  Переключает тип happ-ссылки для добавления профиля.
                </span>
              </div>
              <div className="grid gap-2">
                <div className="text-xs font-medium text-zinc-300">Ссылка добавления</div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <input
                    value={generatedLink}
                    readOnly
                    className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-zinc-300"
                  />
                  <Button type="button" variant="ghost" className="shrink-0 text-xs" onClick={() => void copyToClipboard(generatedLink)}>
                    Скопировать
                  </Button>
                </div>
              </div>
              <div className="grid gap-2">
                <div className="text-xs font-medium text-zinc-300">Ссылка отключения</div>
                <div className="flex flex-col gap-2 sm:flex-row">
                  <input
                    value={disableLink}
                    readOnly
                    className="w-full rounded-lg border border-border bg-surface-2 px-3 py-2 text-xs text-zinc-300"
                  />
                  <Button type="button" variant="ghost" className="shrink-0 text-xs" onClick={() => void copyToClipboard(disableLink)}>
                    Скопировать
                  </Button>
                </div>
              </div>
            </div>

          </div>

          <div className="grid content-start self-start gap-4 rounded-xl border border-border bg-surface-2/30 p-4">
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
              <div className="grid grid-cols-1 gap-3 xl:grid-cols-2 2xl:grid-cols-3">
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
                  value={jsonText}
                  onChange={(e) => handleJsonChange(e.target.value)}
                  className="min-h-[56vh] w-full rounded-lg border border-border bg-surface-2 px-3 py-3 font-mono text-xs text-zinc-200 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                />
                {jsonError && <div className="text-xs text-red-300">{jsonError}</div>}
              </div>
            )}
          </div>
        </div>

        <div className="flex flex-col gap-2 sm:flex-row sm:justify-between">
          <Button type="button" variant="ghost" onClick={() => syncConfig(DEFAULT_ROUTING_CONFIG)}>
            Сбросить
          </Button>
          <Button type="submit" loading={loading}>
            Применить
          </Button>
        </div>
      </form>
    </Modal>
  );
}
