import { ReactNode } from "react";

export function SystemLabel({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <span className={`system-label ${className}`}>{children}</span>;
}

export function OperationalStatus({
  label = "SYSTEM OPERATIONAL",
  className = "",
}: {
  label?: string;
  className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-2 font-mono text-[10px] font-semibold uppercase tracking-[0.14em] text-zinc-400 ${className}`}>
      <span className="signal-dot" aria-hidden="true" />
      {label}
    </span>
  );
}

export function TechnicalFrame({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={`technical-frame ${className}`}>{children}</div>;
}

export function InfrastructureDiagram({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 520 360"
      className={className}
      role="img"
      aria-label="Схема доставки конфигураций SubShare"
    >
      <defs>
        <pattern id="subshare-grid" width="24" height="24" patternUnits="userSpaceOnUse">
          <path d="M 24 0 L 0 0 0 24" fill="none" stroke="currentColor" strokeOpacity=".08" />
        </pattern>
        <linearGradient id="subshare-signal" x1="0" y1="0" x2="1" y2="1">
          <stop offset="0" stopColor="#b7ff2a" stopOpacity=".95" />
          <stop offset="1" stopColor="#b7ff2a" stopOpacity=".18" />
        </linearGradient>
      </defs>
      <rect width="520" height="360" fill="url(#subshare-grid)" />
      <g fill="none" stroke="#737a74" strokeOpacity=".32">
        <path d="M88 180H204M316 180H432M260 76v62M260 222v62" />
        <path d="M112 116L208 162M408 116L312 162M112 244L208 198M408 244L312 198" />
      </g>
      <g className="data-flow" fill="none" stroke="url(#subshare-signal)" strokeWidth="1.5">
        <path d="M88 180H204M316 180H432" />
        <path d="M260 76v62M260 222v62" />
      </g>
      <g fill="#090a0b" stroke="#737a74" strokeOpacity=".62">
        <path d="M208 146h104v68H208z" />
        <path d="M52 152h72v56H52zM396 152h72v56h-72z" />
        <path d="M230 40h60v48h-60zM230 272h60v48h-60z" />
        <path d="M80 92h64v48H80zM376 92h64v48h-64zM80 220h64v48H80zM376 220h64v48h-64z" />
      </g>
      <g fill="#b7ff2a">
        <rect x="218" y="156" width="6" height="6" />
        <rect x="296" y="198" width="6" height="6" opacity=".55" />
        <rect x="82" y="176" width="12" height="8" />
        <rect x="426" y="176" width="12" height="8" opacity=".7" />
        <circle cx="260" cy="64" r="4" />
        <circle cx="260" cy="296" r="4" opacity=".55" />
      </g>
      <g fill="#f2f3ef" fontFamily="JetBrains Mono Variable, monospace" fontSize="10" letterSpacing="1">
        <text x="222" y="180">SUBSHARE</text>
        <text x="222" y="196" fill="#7f877f">ROUTING CORE</text>
        <text x="60" y="173">SOURCE</text>
        <text x="404" y="173">CLIENT</text>
      </g>
      <g fill="#737a74" fontFamily="JetBrains Mono Variable, monospace" fontSize="8" letterSpacing=".8">
        <text x="92" y="122">VALIDATE</text>
        <text x="384" y="122">DELIVER</text>
        <text x="92" y="250">NORMALIZE</text>
        <text x="384" y="250">SYNC</text>
        <text x="242" y="62">API/01</text>
        <text x="238" y="310">DB/SQL</text>
      </g>
    </svg>
  );
}
