"use client";

import { useState } from "react";
import { ExternalLink, Paintbrush, PanelsTopLeft } from "lucide-react";
import Link from "next/link";
import { PageHeader } from "@/components/admin/PageHeader";
import { PanelSettingsModal } from "@/components/admin/PanelSettingsModal";
import { SubscriptionPageConfigModal } from "@/components/admin/SubscriptionPageConfigModal";
import { Button } from "@/components/ui/Button";

export default function BrandingPage() {
  const [panelOpen, setPanelOpen] = useState(false);
  const [pageOpen, setPageOpen] = useState(false);
  return (
    <div>
      <PageHeader title="Брендинг" description="Название панели, логотип, favicon и публичная страница активации." icon={<Paintbrush className="h-5 w-5" />} />
      <div className="grid gap-px bg-border lg:grid-cols-2">
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-cyan-400/20 bg-cyan-400/10 text-cyan-300"><PanelsTopLeft className="h-5 w-5" /></div>
          <h2 className="mt-4 font-semibold text-zinc-200">Панель управления</h2>
          <p className="mt-2 text-sm leading-6 text-zinc-600">Измените название, логотип, favicon и заголовки вкладок.</p>
          <Button className="mt-5" onClick={() => setPanelOpen(true)}>Настроить панель</Button>
        </section>
        <section className="technical-frame border border-border bg-surface-1 p-5">
          <div className="flex h-11 w-11 items-center justify-center rounded-xl border border-cyan-400/20 bg-cyan-400/10 text-cyan-300"><Paintbrush className="h-5 w-5" /></div>
          <h2 className="mt-4 font-semibold text-zinc-200">Публичная страница</h2>
          <p className="mt-2 text-sm leading-6 text-zinc-600">Редактируйте блоки, тексты, ссылки на клиенты и цветовую тему.</p>
          <div className="mt-5 flex flex-wrap gap-2">
            <Button onClick={() => setPageOpen(true)}>Открыть редактор</Button>
            <Link href="/subscription" target="_blank" className="inline-flex items-center gap-2 rounded-lg border border-border px-4 py-2 text-sm text-zinc-400 hover:bg-surface-2 hover:text-zinc-200">Предпросмотр <ExternalLink className="h-4 w-4" /></Link>
          </div>
        </section>
      </div>
      <PanelSettingsModal open={panelOpen} onClose={() => setPanelOpen(false)} />
      <SubscriptionPageConfigModal open={pageOpen} onClose={() => setPageOpen(false)} />
    </div>
  );
}
