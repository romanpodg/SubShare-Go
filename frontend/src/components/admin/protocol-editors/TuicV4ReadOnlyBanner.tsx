"use client";

import { Info } from "lucide-react";

interface Props {
  tokenPresent: boolean;
}

export function TuicV4ReadOnlyBanner({ tokenPresent }: Props) {
  return (
    <div className="space-y-3 rounded-sm border border-amber-500/30 bg-amber-500/10 p-4 text-xs leading-5 text-amber-200">
      <div className="flex items-center gap-2 font-mono text-[11px] font-semibold uppercase tracking-wider text-amber-300">
        <Info className="h-4 w-4 shrink-0" aria-hidden="true" />
        TUIC v4 (Compatibility Only)
      </div>
      <p>
        Этот профиль использует устаревший протокол TUIC v4. Структурированное редактирование
        полей отключено. Доставка подписки происходит исключительно в виде сырого URL (Raw Delivery).
      </p>
      <div className="font-mono text-[11px] text-zinc-400">
        Статус токена: {tokenPresent ? "Токен присутствует (скрыт)" : "Токен отсутствует"}
      </div>
    </div>
  );
}
