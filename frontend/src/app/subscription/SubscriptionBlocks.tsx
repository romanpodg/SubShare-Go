"use client";

import { CSSProperties, FormEvent, useMemo, useState, useEffect } from "react";
import Image from "next/image";
import { Check, Clipboard, Languages, Layers, Link2, Monitor, Send, Smartphone, Terminal } from "lucide-react";
import type {
  ActivationStepBlockConfig,
  FooterBlockConfig,
  HeroBlockConfig,
  LinkButtonConfig,
  LinkButtonsStepBlockConfig,
  PageBlock,
  StepConfig,
} from "./pageConfig";
import { NumericGlobe } from "./NumericGlobe";
import styles from "./subscription-page.module.css";

interface ActivationRuntime {
  code: string;
  loading: boolean;
  error: string;
  message: string;
  subscriptionUrl: string;
  copied: boolean;
  onCodeChange: (nextValue: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onCopy: () => void;
}

interface SubscriptionBlocksProps {
  blocks: PageBlock[];
  panelLogoDataUrl?: string;
  activation: ActivationRuntime;
  themeMode: "light" | "dark";
}

type DevicePlatform = "android" | "ios" | "windows" | "macos" | "linux" | "unknown";

function detectDevicePlatform(): DevicePlatform {
  if (typeof navigator === "undefined") return "unknown";

  const navigatorWithUAData = navigator as Navigator & {
    userAgentData?: {
      platform?: string;
    };
  };

  const platformFromUAData = navigatorWithUAData.userAgentData?.platform?.toLowerCase() || "";
  const platform = (navigator.platform || "").toLowerCase();
  const userAgent = (navigator.userAgent || "").toLowerCase();
  const fingerprint = `${platformFromUAData} ${platform} ${userAgent}`;

  if (fingerprint.includes("android")) return "android";
  if (fingerprint.includes("iphone") || fingerprint.includes("ipad") || fingerprint.includes("ipod")) return "ios";

  // iPadOS desktop mode can report itself as Mac.
  if (platform.includes("mac") && navigator.maxTouchPoints > 1) return "ios";

  if (fingerprint.includes("win")) return "windows";
  if (fingerprint.includes("mac")) return "macos";
  if (fingerprint.includes("linux")) return "linux";
  return "unknown";
}

function recommendedButtonIDsByPlatform(platform: DevicePlatform): Set<string> {
  switch (platform) {
    case "android":
      return new Set(["google-play", "android-apk"]);
    case "ios":
      return new Set(["app-store-ru", "app-store-global"]);
    case "windows":
      return new Set(["windows"]);
    case "macos":
      return new Set(["macos"]);
    case "linux":
      return new Set(["linux"]);
    default:
      return new Set();
  }
}

function DownloadIcon() {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      className={styles.buttonIcon}
      aria-hidden
    >
      <path
        d="M12 3v12m0 0-3-3m3 3 3-3M5 17h14"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <rect x="4" y="19" width="16" height="2" fill="currentColor" stroke="none" />
    </svg>
  );
}

const MONOCHROME_ICON_SOURCES = new Set([
  "/subscription/apple.svg",
  "/subscription/linux.svg",
  "/subscription/windows.svg",
]);

export function isMonochromeButtonIcon(button: LinkButtonConfig): boolean {
  if (button.icon?.type !== "image" || !button.icon.src) return false;
  const sourcePath = button.icon.src.split(/[?#]/, 1)[0].toLowerCase();
  return MONOCHROME_ICON_SOURCES.has(sourcePath);
}

function renderButtonIcon(button: LinkButtonConfig) {
  if (!button.icon) return null;
  if (button.icon.type === "download") return <DownloadIcon />;
  if (!button.icon.src) return null;

  if (isMonochromeButtonIcon(button)) {
    return (
      <span
        className={`${styles.buttonIcon} ${styles.buttonIconMonochrome}`}
        style={{ "--button-icon-mask": `url("${button.icon.src}")` } as CSSProperties}
        aria-hidden="true"
      />
    );
  }

  return (
    <Image
      src={button.icon.src}
      alt=""
      className={styles.buttonIcon}
      width={20}
      height={20}
      draggable={false}
      unoptimized
    />
  );
}

function buttonClassName(variant: LinkButtonConfig["variant"], recommended: boolean) {
  if (recommended) return `${styles.button} ${styles.buttonRecommended}`;
  if (variant === "primary") return `${styles.button} ${styles.buttonPrimary}`;
  if (variant === "subscribe") return `${styles.button} ${styles.buttonSubscribe}`;
  return `${styles.button} ${styles.buttonSecondary}`;
}

type RecommendedTone = "neutral" | "windows" | "android" | "linux" | "apple";

function recommendedToneByButtonID(buttonID: string): RecommendedTone {
  if (buttonID === "google-play") return "neutral";
  if (buttonID === "android-apk") return "android";
  if (buttonID === "windows") return "windows";
  if (buttonID === "linux") return "linux";
  if (buttonID === "app-store-ru" || buttonID === "app-store-global" || buttonID === "macos") return "apple";
  return "neutral";
}

function recommendedToneClass(buttonID: string, themeMode: "light" | "dark"): string {
  if (themeMode === "dark") return "";

  switch (recommendedToneByButtonID(buttonID)) {
    case "windows":
      return styles.buttonRecommendedWindows;
    case "android":
      return styles.buttonRecommendedAndroid;
    case "linux":
      return styles.buttonRecommendedLinux;
    case "apple":
      return styles.buttonRecommendedApple;
    default:
      return styles.buttonRecommendedNeutral;
  }
}

function buttonClassNameWithTone({
  variant,
  recommended,
  buttonID,
  themeMode,
}: {
  variant: LinkButtonConfig["variant"];
  recommended: boolean;
  buttonID: string;
  themeMode: "light" | "dark";
}) {
  const className = buttonClassName(variant, recommended);
  if (!recommended) return className;

  const toneClassName = recommendedToneClass(buttonID, themeMode);
  return toneClassName ? `${className} ${toneClassName}` : className;
}

function LinkButtonsBlock({
  block,
  recommendedButtonIDs,
  themeMode,
}: {
  block: LinkButtonsStepBlockConfig;
  recommendedButtonIDs: Set<string>;
  themeMode: "light" | "dark";
}) {
  type PlatformTab = "android" | "apple" | "windows" | "linux" | "all";
  const [selectedPlatform, setSelectedPlatform] = useState<PlatformTab>("all");

  useEffect(() => {
    const frame = window.requestAnimationFrame(() => {
      const platform = detectDevicePlatform();
      if (platform === "android") setSelectedPlatform("android");
      else if (platform === "ios" || platform === "macos") setSelectedPlatform("apple");
      else if (platform === "windows") setSelectedPlatform("windows");
      else if (platform === "linux") setSelectedPlatform("linux");
      else setSelectedPlatform("all");
    });
    return () => window.cancelAnimationFrame(frame);
  }, []);

  const platformTabs: Array<{ id: PlatformTab; label: string; icon: React.ReactNode }> = [
    { id: "all", label: "Все", icon: <Layers className={styles.platformFilterIcon} /> },
    { id: "android", label: "Android", icon: <Smartphone className={styles.platformFilterIcon} /> },
    { id: "apple", label: "Apple", icon: <svg className={styles.platformFilterIcon} viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg"><path d="M12.152 6.896c-.948 0-2.415-1.078-3.96-1.078-2.04 0-3.905 1.158-4.966 3.002-2.117 3.69-.54 9.139 1.519 12.096 1.007 1.452 2.207 3.074 3.774 3.074 1.51 0 2.09-.91 3.916-.91 1.815 0 2.348.91 3.927.91 1.597 0 2.68-1.474 3.675-2.923 1.158-1.688 1.637-3.32 1.67-3.41-.035-.02-3.197-1.22-3.23-4.832-.027-3.013 2.47-4.463 2.585-4.532-1.42-2.073-3.602-2.31-4.38-2.373-2.031-.157-3.23 1.078-4.13 1.078zM15.98 3.82c.835-1.013 1.393-2.422 1.242-3.82-1.2.049-2.657.801-3.52 1.814-.755.877-1.414 2.301-1.233 3.682 1.336.103 2.705-.688 3.511-1.676z"/></svg> },
    { id: "windows", label: "Windows", icon: <Monitor className={styles.platformFilterIcon} /> },
    { id: "linux", label: "Linux", icon: <Terminal className={styles.platformFilterIcon} /> },
  ];

  const getButtonsForPlatform = () => {
    switch (selectedPlatform) {
      case "android":
        return block.buttons.filter((b) => b.id === "google-play" || b.id === "android-apk");
      case "apple":
        return block.buttons.filter((b) => b.id === "app-store-ru" || b.id === "app-store-global" || b.id === "macos");
      case "windows":
        return block.buttons.filter((b) => b.id === "windows");
      case "linux":
        return block.buttons.filter((b) => b.id === "linux");
      default:
        // By default, sort recommended buttons to the top
        return [...block.buttons].sort((left, right) => {
          const leftRank = recommendedButtonIDs.has(left.id) ? 0 : 1;
          const rightRank = recommendedButtonIDs.has(right.id) ? 0 : 1;
          return leftRank - rightRank;
        });
    }
  };

  const visibleButtons = getButtonsForPlatform();

  return (
    <div className={styles.linkButtonsBlock}>
      <div className={styles.platformFilters} role="group" aria-label="Выбор платформы">
        {platformTabs.map((tab) => (
          <button
            type="button"
            key={tab.id}
            onClick={() => setSelectedPlatform(tab.id)}
            className={`${styles.platformFilter} ${
              selectedPlatform === tab.id ? styles.platformFilterActive : ""
            }`}
            aria-pressed={selectedPlatform === tab.id}
          >
            {tab.icon}
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      <div className={styles.buttons}>
        {visibleButtons.map((button) => {
          const recommended = recommendedButtonIDs.has(button.id);
          const visualVariant =
            recommendedButtonIDs.size > 0 && !recommended ? "secondary" : button.variant;

          return (
            <a
              key={button.id}
              className={buttonClassNameWithTone({
                variant: visualVariant,
                recommended: recommended && selectedPlatform === "all", // only show highlight glow if on "All" tab or auto-selected
                buttonID: button.id,
                themeMode,
              })}
              href={button.href}
              target={button.external ? "_blank" : undefined}
              rel={button.external ? "noreferrer" : undefined}
              data-button-id={button.id}
            >
              {renderButtonIcon(button)}
              <span>{button.label}</span>
            </a>
          );
        })}
      </div>
    </div>
  );
}

function ActivationBlock({
  block,
  activation,
}: {
  block: ActivationStepBlockConfig;
  activation: ActivationRuntime;
}) {
  return (
    <>
      <form className={styles.activationForm} onSubmit={activation.onSubmit}>
        <label htmlFor="activation-code" className={styles.activationLabel}>
          {block.formLabel}
        </label>
        <div className={styles.activationRow}>
          <input
            id="activation-code"
            type="text"
            value={activation.code}
            onChange={(event) => activation.onCodeChange(event.target.value)}
            placeholder={block.formPlaceholder}
            required
            autoFocus
            className={styles.activationInput}
          />
          <button type="submit" className={`${styles.button} ${styles.buttonPrimary}`} disabled={activation.loading}>
            {activation.loading ? block.submitLoadingLabel : block.submitLabel}
          </button>
        </div>
      </form>

      {activation.error ? <p className={styles.errorMessage}>{activation.error}</p> : null}
      {activation.message ? <p className={styles.successMessage}>{activation.message}</p> : null}

      {activation.subscriptionUrl ? (
        <>
          <div className={styles.buttons}>
            <a href={activation.subscriptionUrl} className={`${styles.button} ${styles.buttonSubscribe}`}>
              <Send className={styles.buttonIcon} aria-hidden="true" />
              <span>{block.addButtonLabel}</span>
            </a>
          </div>

          <div className={styles.subLinkRow}>
            <span className={styles.subLinkLabel}>
              <Link2 className={styles.inlineIcon} aria-hidden="true" />
              <span>{block.manualLinkLabel}</span>
            </span>
            <code className={styles.subLinkCode}>{activation.subscriptionUrl}</code>
            <button type="button" className={styles.copyLink} onClick={activation.onCopy}>
              {activation.copied ? (
                <Check className={styles.inlineIcon} aria-hidden="true" />
              ) : (
                <Clipboard className={styles.inlineIcon} aria-hidden="true" />
              )}
              <span>{activation.copied ? block.copiedLabel : block.copyLabel}</span>
            </button>
          </div>
        </>
      ) : null}
    </>
  );
}

function StepBlock({
  step,
  index,
  activation,
  recommendedButtonIDs,
  themeMode,
}: {
  step: StepConfig;
  index: number;
  activation: ActivationRuntime;
  recommendedButtonIDs: Set<string>;
  themeMode: "light" | "dark";
}) {
  return (
    <article className={styles.step}>
      <div className={styles.stepNumber}>{index + 1}</div>
      <div className={styles.stepContent}>
        <h3 className={styles.stepTitle}>{step.title}</h3>
        <p className={styles.stepText}>{step.description}</p>

        {step.block.type === "linkButtons" ? (
          <LinkButtonsBlock
            block={step.block}
            recommendedButtonIDs={recommendedButtonIDs}
            themeMode={themeMode}
          />
        ) : null}
        {step.block.type === "activation" ? <ActivationBlock block={step.block} activation={activation} /> : null}
      </div>
    </article>
  );
}

function HeroBlock({
  block,
  panelLogoDataUrl,
}: {
  block: HeroBlockConfig;
  panelLogoDataUrl?: string;
}) {
  const logoSrc = panelLogoDataUrl || block.logo.src;

  return (
    <header className={styles.hero}>
      <div className={styles.heroCopy}>
        <div className={styles.heroTop}>
          <div className={styles.brand}>
            <div className={styles.logoWrap}>
              <Image
                src={logoSrc}
                alt={block.logo.alt}
                className={styles.logo}
                fill
                sizes="56px"
                priority
                draggable={false}
                unoptimized
              />
            </div>
            <div className={styles.brandText}>
              <h1>{block.brandTitle}</h1>
              <p>{block.brandSubtitle}</p>
            </div>
          </div>
        </div>

        <div className={styles.heroLabel}>SUBSCRIPTION DELIVERY / ACCESS NODE</div>
        <p className={styles.subhead}>{block.subhead}</p>

        {block.statusBadge ? (
          <div className={styles.statusBadge}>
            <span className={styles.statusDot} aria-hidden="true" />
            <span className={styles.statusText}>{block.statusBadge}</span>
          </div>
        ) : null}
      </div>
      <div className={styles.heroDiagram}>
        <NumericGlobe className={styles.numericGlobe} />
      </div>
    </header>
  );
}

function FooterBlock({ block }: { block: FooterBlockConfig }) {
  return (
    <footer className={styles.footer}>
      <span>{block.copyright}</span>
      <div className={styles.footerRight}>
        {block.footerLink ? (
          <a href={block.footerLink.href} className={styles.footerLink}>
            {block.footerLink.label}
          </a>
        ) : null}
        <span className={styles.languageBadge}>
          <Languages className={styles.inlineIcon} aria-hidden="true" />
          <span>{block.languageBadge}</span>
        </span>
      </div>
    </footer>
  );
}

export function SubscriptionBlocks({
  blocks,
  panelLogoDataUrl,
  activation,
  themeMode,
}: SubscriptionBlocksProps) {
  const recommendedButtonIDs = useMemo(
    () => recommendedButtonIDsByPlatform(detectDevicePlatform()),
    []
  );

  return (
    <>
      {blocks.map((block) => {
        if (block.type === "hero") {
          return (
            <HeroBlock
              key={block.id}
              block={block}
              panelLogoDataUrl={panelLogoDataUrl}
            />
          );
        }

        if (block.type === "steps") {
          return (
            <section className={styles.section} id={block.id} key={block.id}>
              <h2>{block.title}</h2>
              <div className={styles.steps}>
                {block.steps.map((step, index) => (
                  <StepBlock
                    key={step.id}
                    step={step}
                    index={index}
                    activation={activation}
                    recommendedButtonIDs={recommendedButtonIDs}
                    themeMode={themeMode}
                  />
                ))}
              </div>
            </section>
          );
        }

        return <FooterBlock key={block.id} block={block} />;
      })}
    </>
  );
}
