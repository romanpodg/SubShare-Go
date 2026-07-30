"use client";

import { useCallback, useEffect, useState } from "react";
import { CheckCircle2, Copy, KeyRound, LockKeyhole, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { apiV1 } from "@/lib/api";
import type { APIToken } from "@/lib/types";
import { PageHeader } from "@/components/admin/PageHeader";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { useToast } from "@/components/ui/Toast";
import { copyToClipboard } from "@/lib/clipboard";
import { InitialLoading, ResourceError } from "@/components/ui/ResourceState";

interface BuildInfo {
  version: string;
  commit: string;
  build_time: string;
  schema_version: number;
}

export default function SecurityPage() {
  const [build, setBuild] = useState<BuildInfo | null>(null);
  const [tokens, setTokens] = useState<APIToken[]>([]);
  const [tokenName, setTokenName] = useState("");
  const [tokenExpiresAt, setTokenExpiresAt] = useState("");
  const [newToken, setNewToken] = useState("");
  const [creating, setCreating] = useState(false);
  const [scopes, setScopes] = useState(["read"]);
  const [loading, setLoading] = useState(true);
  const [buildError, setBuildError] = useState<unknown>(null);
  const [tokensError, setTokensError] = useState<unknown>(null);
  const { toast } = useToast();

  const loadTokens = useCallback(async () => {
    setTokensError(null);
    try {
      const response = await apiV1.apiTokens.list();
      setTokens(response.data);
    } catch (requestError) {
      setTokensError(requestError);
    }
  }, []);

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      const loadBuild = async () => {
        setBuildError(null);
        try {
          setBuild(await apiV1.buildInfo());
        } catch (requestError) {
          setBuildError(requestError);
        }
      };
      void Promise.allSettled([loadBuild(), loadTokens()]).finally(() => setLoading(false));
    });
    return () => window.cancelAnimationFrame(frame);
  }, [loadTokens]);

  const toggleScope = (scope: string) => {
    setScopes((current) => current.includes(scope) ? current.filter((item) => item !== scope) : [...current, scope]);
  };

  const createToken = async () => {
    setCreating(true);
    try {
      const response = await apiV1.apiTokens.create({
        name: tokenName,
        scopes,
        expires_at: tokenExpiresAt ? new Date(tokenExpiresAt).toISOString() : "",
      });
      setNewToken(response.token);
      setTokenName("");
      setTokenExpiresAt("");
      await loadTokens();
      toast("API-токен создан", "success");
    } catch (error) {
      toast(error instanceof Error ? error.message : "Не удалось создать токен", "error");
    } finally {
      setCreating(false);
    }
  };

  const revoke = async (id: number) => {
    await apiV1.apiTokens.revoke(id);
    await loadTokens();
    toast("API-токен отозван", "success");
  };

  const checks = [
    "CSRF для изменяющих admin-запросов",
    "HttpOnly/SameSite session cookies",
    "SSRF-защита внешних источников и redirect",
    "Ограничение тела запросов и rate limiting",
    "Audit log без subscription IDs, ключей и HWID",
  ];

  if (loading && !build && tokens.length === 0) {
    return <InitialLoading label="Загрузка настроек безопасности…" />;
  }

  return (
    <div>
      <PageHeader title="Безопасность и сборка" description="Текущее состояние защитных механизмов и схема базы." icon={<ShieldCheck className="h-5 w-5" />} />
      {Boolean(buildError) && <div className="mb-4"><ResourceError error={buildError} compact title="Build info недоступна" /></div>}
      {Boolean(tokensError) && <div className="mb-4"><ResourceError error={tokensError} compact onRetry={() => void loadTokens()} title="Не удалось загрузить API-токены" /></div>}
      <div className="ui-joined-grid technical-frame grid lg:grid-cols-2">
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex items-center gap-3"><LockKeyhole className="h-5 w-5 text-cyan-300" /><h2 className="font-semibold text-zinc-200">Защитные механизмы</h2></div>
          <div className="ui-joined-list mt-5">
            {checks.map((check) => <div key={check} className="flex items-center gap-3 rounded-xl border border-border bg-zinc-950/30 px-4 py-3 text-sm text-zinc-400"><CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-400" />{check}</div>)}
          </div>
        </section>
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <h2 className="font-semibold text-zinc-200">Build info</h2>
          <dl className="ui-joined-list mt-5">
            {[["Версия", build?.version], ["Commit", build?.commit], ["Время сборки", build?.build_time], ["Версия схемы", build?.schema_version]].map(([label, value]) => (
              <div key={String(label)} className="flex items-center justify-between gap-4 rounded-xl border border-border bg-zinc-950/30 px-4 py-3"><dt className="text-sm text-zinc-600">{label}</dt><dd className="font-mono text-sm text-cyan-300">{value ?? "—"}</dd></div>
            ))}
          </dl>
        </section>
      </div>

      <section className="technical-frame border border-border bg-surface-1">
        <div className="flex flex-col justify-between gap-4 border-b border-border px-5 py-4 lg:flex-row lg:items-center">
          <div className="flex items-center gap-3"><KeyRound className="h-5 w-5 text-cyan-300" /><div><h2 className="font-semibold text-zinc-200">API-токены</h2><p className="mt-1 text-xs text-zinc-600">Секрет показывается только один раз. В базе хранится SHA-256 hash.</p></div></div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
            <Input label="Название токена" value={tokenName} onChange={(event) => setTokenName(event.target.value)} placeholder="Automation" />
            <Input label="Истекает (необязательно)" type="datetime-local" value={tokenExpiresAt} onChange={(event) => setTokenExpiresAt(event.target.value)} />
            <Button onClick={createToken} loading={creating} disabled={!tokenName.trim() || scopes.length === 0}><Plus className="h-4 w-4" />Создать</Button>
          </div>
        </div>
        <div className="border-b border-border px-5 py-4">
          <div className="flex flex-wrap gap-2">
            {["read", "users:write", "keys:write", "settings:write"].map((scope) => (
              <label key={scope} className={`flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-xs ${scopes.includes(scope) ? "border-cyan-400/30 bg-cyan-400/10 text-cyan-200" : "border-border text-zinc-500"}`}>
                <input type="checkbox" checked={scopes.includes(scope)} onChange={() => toggleScope(scope)} className="accent-cyan-500" />
                {scope}
              </label>
            ))}
          </div>
        </div>
        {newToken && (
          <div className="border-b border-amber-400/20 bg-amber-400/5 px-5 py-4">
            <div className="text-xs font-semibold uppercase tracking-wider text-amber-300">Скопируйте секрет сейчас</div>
            <div className="mt-2 flex items-center gap-2">
              <code className="min-w-0 flex-1 overflow-x-auto rounded-lg border border-amber-400/20 bg-black/30 px-3 py-2 font-mono text-sm text-amber-100">{newToken}</code>
              <Button variant="outline" onClick={async () => { await copyToClipboard(newToken); toast("Токен скопирован", "success"); }}><Copy className="h-4 w-4" /></Button>
            </div>
          </div>
        )}
        <div className="divide-y divide-border">
          {tokens.map((token) => (
            <div key={token.id} className="flex flex-col justify-between gap-3 px-5 py-4 sm:flex-row sm:items-center">
              <div>
                <div className="font-medium text-zinc-300">{token.name}</div>
                <div className="mt-1 font-mono text-xs text-zinc-600">{token.prefix}… · {token.scopes.join(", ")}</div>
                <div className="mt-1 text-xs text-zinc-700">
                  {token.expires_at ? `Истекает: ${new Date(token.expires_at).toLocaleString("ru-RU")}` : "Без срока"}
                  {token.last_used_at ? ` · использован: ${new Date(token.last_used_at).toLocaleString("ru-RU")}` : ""}
                </div>
              </div>
              <div className="flex items-center gap-3"><span className={`text-xs ${token.revoked_at ? "text-rose-400" : "text-emerald-400"}`}>{token.revoked_at ? "Отозван" : "Активен"}</span>{!token.revoked_at && <Button variant="danger" onClick={() => revoke(token.id)}><Trash2 className="h-4 w-4" />Отозвать</Button>}</div>
            </div>
          ))}
          {tokens.length === 0 && <div className="px-5 py-12 text-center text-sm text-zinc-600">API-токенов пока нет</div>}
        </div>
      </section>
    </div>
  );
}
