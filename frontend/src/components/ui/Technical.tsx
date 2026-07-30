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

export function SignalCoreDiagram({ className = "" }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 640 320"
      className={`signal-core-diagram ${className}`}
      role="img"
      aria-label="Сигнальное ядро SubShare и защищённые каналы доставки"
    >
      <title>Сигнальное ядро SubShare и защищённые каналы доставки</title>
      <defs>
        <pattern id="signal-core-grid" width="20" height="20" patternUnits="userSpaceOnUse">
          <path d="M20 0H0V20" fill="none" stroke="currentColor" strokeOpacity=".07" />
        </pattern>
        <filter id="signal-core-glow" x="-100%" y="-100%" width="300%" height="300%">
          <feGaussianBlur stdDeviation="5" result="blur" />
          <feMerge>
            <feMergeNode in="blur" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>
      </defs>

      <rect width="640" height="320" fill="url(#signal-core-grid)" />

      <g className="signal-core-coordinates" fill="none" stroke="currentColor" strokeOpacity=".22">
        <path d="M20 38V20h18M602 20h18v18M20 282v18h18M602 300h18v-18" />
        <path d="M308 14h24M308 278h24M12 134v24M628 134v24" />
      </g>

      <g className="signal-core-routes" fill="none" stroke="#737a74" strokeOpacity=".35">
        <path d="M126 84h62l58 36M514 84h-62l-58 36M126 224h62l58-52M514 224h-62l-58-52" />
        <path d="M188 84v140M452 84v140" strokeDasharray="2 8" strokeOpacity=".22" />
      </g>
      <g className="signal-core-flow" fill="none" stroke="var(--accent)" strokeWidth="1.5">
        <path d="M126 84h62l58 36" />
        <path d="M514 84h-62l-58 36" />
        <path d="M126 224h62l58-52" />
        <path d="M514 224h-62l-58-52" />
      </g>

      <g className="signal-core-orbit signal-core-orbit--outer" fill="none" stroke="currentColor" strokeOpacity=".32">
        <circle cx="320" cy="146" r="122" strokeDasharray="72 18 9 25" />
        <path d="M320 14v18M320 260v18M188 146h18M434 146h18" />
      </g>
      <g className="signal-core-orbit signal-core-orbit--inner" fill="none" stroke="var(--accent)" strokeOpacity=".34">
        <circle cx="320" cy="146" r="88" strokeDasharray="4 12 42 18" />
      </g>

      <g className="signal-core-node" transform="translate(34 56)">
        <rect width="92" height="56" fill="#090a0b" stroke="#737a74" strokeOpacity=".65" />
        <path d="M0 8V0h8M84 0h8v8" fill="none" stroke="var(--accent)" strokeOpacity=".55" />
        <rect className="signal-core-beacon signal-core-beacon--1" x="12" y="13" width="7" height="7" fill="var(--accent)" />
        <text x="28" y="19" fill="#f2f3ef">SOURCES</text>
        <text x="12" y="40" fill="#737a74">02 / READY</text>
      </g>
      <g className="signal-core-node" transform="translate(514 56)">
        <rect width="92" height="56" fill="#090a0b" stroke="#737a74" strokeOpacity=".65" />
        <path d="M0 8V0h8M84 0h8v8" fill="none" stroke="var(--accent)" strokeOpacity=".55" />
        <rect className="signal-core-beacon signal-core-beacon--2" x="12" y="13" width="7" height="7" fill="var(--accent)" />
        <text x="28" y="19" fill="#f2f3ef">KEYS</text>
        <text x="12" y="40" fill="#737a74">24 / VALID</text>
      </g>
      <g className="signal-core-node" transform="translate(34 196)">
        <rect width="92" height="56" fill="#090a0b" stroke="#737a74" strokeOpacity=".65" />
        <path d="M0 48v8h8M84 56h8v-8" fill="none" stroke="var(--accent)" strokeOpacity=".55" />
        <rect className="signal-core-beacon signal-core-beacon--3" x="12" y="13" width="7" height="7" fill="var(--accent)" />
        <text x="28" y="19" fill="#f2f3ef">USERS</text>
        <text x="12" y="40" fill="#737a74">01 / ACTIVE</text>
      </g>
      <g className="signal-core-node" transform="translate(514 196)">
        <rect width="92" height="56" fill="#090a0b" stroke="#737a74" strokeOpacity=".65" />
        <path d="M0 48v8h8M84 56h8v-8" fill="none" stroke="var(--accent)" strokeOpacity=".55" />
        <rect className="signal-core-beacon signal-core-beacon--4" x="12" y="13" width="7" height="7" fill="var(--accent)" />
        <text x="28" y="19" fill="#f2f3ef">CLIENTS</text>
        <text x="12" y="40" fill="#737a74">TLS / LIVE</text>
      </g>

      <g className="signal-core-core">
        <path d="M320 86l74 34v52l-74 34-74-34v-52z" fill="#080909" stroke="#737a74" strokeOpacity=".8" />
        <path d="M320 98l58 27v42l-58 27-58-27v-42z" fill="none" stroke="var(--accent)" strokeOpacity=".38" />
        <path d="M320 110l42 20v32l-42 20-42-20v-32z" fill="#0d0f11" stroke="var(--accent)" strokeOpacity=".7" />
        <circle
          className="signal-core-core-pulse"
          cx="320"
          cy="146"
          r="7"
          fill="var(--accent)"
          filter="url(#signal-core-glow)"
        />
        <text x="320" y="137" textAnchor="middle" fill="#f2f3ef" className="signal-core-title">SUBSHARE</text>
        <text x="320" y="163" textAnchor="middle" fill="#9aa09a" className="signal-core-caption">SIGNAL CORE</text>
      </g>

      <g className="signal-core-packets" fill="var(--accent)">
        <rect className="signal-core-beacon signal-core-beacon--2" x="184" y="80" width="5" height="5" />
        <rect className="signal-core-beacon signal-core-beacon--3" x="451" y="80" width="5" height="5" />
        <rect className="signal-core-beacon signal-core-beacon--4" x="184" y="220" width="5" height="5" />
        <rect className="signal-core-beacon signal-core-beacon--1" x="451" y="220" width="5" height="5" />
      </g>

      <g className="signal-core-axis-labels" fill="#737a74">
        <text x="22" y="16">CORE MAP / 01</text>
        <text x="618" y="16" textAnchor="end">ENCRYPTED MESH</text>
      </g>
    </svg>
  );
}
