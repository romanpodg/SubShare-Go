"use client";

import { useState, useEffect, FormEvent } from "react";
import Link from "next/link";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { subscription } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";

export default function SubscriptionPage() {
  const [code, setCode] = useState("");
  const [subscriptionUrl, setSubscriptionUrl] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    document.title = "VPN-подписка — Xray Sub";
  }, []);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");
    setMessage("");
    setSubscriptionUrl("");
    setLoading(true);
    try {
      const data = await subscription.activate(code);
      setSubscriptionUrl(data.subscription_url);
      setMessage(data.message);
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Activation failed");
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    try {
      const copied = await copyToClipboard(subscriptionUrl);
      if (!copied) {
        if (typeof window !== "undefined") {
          window.prompt("Скопируйте ссылку вручную:", subscriptionUrl);
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
          return;
        }
        setError("Не удалось скопировать");
        return;
      }
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      if (typeof window !== "undefined") {
        window.prompt("Скопируйте ссылку вручную:", subscriptionUrl);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
        return;
      }
      setError("Не удалось скопировать");
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <h1 className="text-xl font-semibold mb-6 text-center">VPN-подписка</h1>

        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            label="Ключ активации"
            type="text"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            placeholder="Введите ключ активации"
            autoFocus
            required
          />
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button type="submit" loading={loading}>
            Активировать и получить ссылку
          </Button>
        </form>

        {subscriptionUrl && (
          <div className="mt-6">
            <p className="text-sm text-green-400 mb-3">{message}</p>
            <div className="bg-surface-2 rounded-lg p-3 flex items-center gap-2">
              <code className="text-xs text-zinc-300 flex-1 break-all font-mono">
                {subscriptionUrl}
              </code>
              <Button variant="ghost" onClick={handleCopy} className="shrink-0 text-xs">
                {copied ? "Скопировано" : "Копировать"}
              </Button>
            </div>
          </div>
        )}

        <div className="mt-6 text-center">
          <Link
            href="/admin/login"
            className="text-sm text-zinc-400 hover:text-zinc-300 transition-colors"
          >
            Панель управления
          </Link>
        </div>
      </Card>
    </div>
  );
}
