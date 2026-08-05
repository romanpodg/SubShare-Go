"use client";

import { AlertCircle, CheckCircle2, ShieldAlert } from "lucide-react";
import type { OutputCapability } from "@/lib/types";

interface Props {
  capabilities?: Record<string, OutputCapability>;
  exclusionReasonsCatalog?: Record<string, string>;
}

const FORMAT_LABELS: Record<string, string> = {
  plain: "Plain (SIP-URI)",
  base64: "Base64 (SubBody)",
  mihomo: "Mihomo (Clash)",
  "sing-box": "sing-box",
  "xray-json": "Xray JSON",
};

export function OutputCapabilitiesMatrix({
  capabilities,
  exclusionReasonsCatalog = {},
}: Props) {
  if (!capabilities || Object.keys(capabilities).length === 0) {
    return null;
  }

  return (
    <div className="rounded-sm border border-border bg-surface-2/40 p-4">
      <div className="system-label mb-3">Совместимость выгрузки (Generator Capabilities)</div>
      <div className="grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {Object.entries(capabilities).map(([fmt, cap]) => {
          const formatLabel = FORMAT_LABELS[fmt] || fmt;
          const status = cap.status;
          const reasonCode = cap.reason_code;
          const reasonText =
            reasonCode && exclusionReasonsCatalog[reasonCode]
              ? exclusionReasonsCatalog[reasonCode]
              : reasonCode || "";

          let badgeColor = "border-zinc-700 bg-zinc-800/50 text-zinc-400";
          let icon = <InfoIcon className="h-3.5 w-3.5 shrink-0 text-zinc-400" />;
          let statusLabel = "Не поддерживается";

          if (status === "supported") {
            badgeColor = "border-success/30 bg-success/10 text-success";
            icon = <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-success" />;
            statusLabel = "Поддерживается";
          } else if (status === "conditionally_supported") {
            badgeColor = "border-amber-500/30 bg-amber-500/10 text-amber-200";
            icon = <AlertCircle className="h-3.5 w-3.5 shrink-0 text-amber-300" />;
            statusLabel = "Ограничено";
          } else if (status === "compatibility_only") {
            badgeColor = "border-sky-500/30 bg-sky-500/10 text-sky-200";
            icon = <CheckCircle2 className="h-3.5 w-3.5 shrink-0 text-sky-300" />;
            statusLabel = "Только сырой формат";
          } else if (status === "unsupported") {
            badgeColor = "border-danger/30 bg-danger/10 text-red-200";
            icon = <ShieldAlert className="h-3.5 w-3.5 shrink-0 text-red-400" />;
            statusLabel = "Исключён";
          }

          return (
            <div
              key={fmt}
              className={`flex flex-col justify-between rounded-sm border p-2.5 text-xs ${badgeColor}`}
            >
              <div className="flex items-center justify-between gap-2 font-mono font-medium">
                <span>{formatLabel}</span>
                <span className="flex items-center gap-1">
                  {icon}
                  <span className="text-[11px]">{statusLabel}</span>
                </span>
              </div>
              {reasonText && (
                <div className="mt-1.5 font-mono text-[10px] opacity-80 break-words">
                  {reasonText}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function InfoIcon(props: React.SVGProps<SVGSVGElement>) {
  return (
    <svg fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" {...props}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M11.25 11.25l.041-.02a.75.75 0 011.063.852l-.708 2.836a.75.75 0 001.063.853l.041-.021M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9-3.75h.008v.008H12V8.25z" />
    </svg>
  );
}
