"use client";

import { FormEvent, useMemo } from "react";
import Image from "next/image";
import { MoonStar, SunMedium } from "lucide-react";
import type {
  ActivationStepBlockConfig,
  FooterBlockConfig,
  HeroBlockConfig,
  LinkButtonConfig,
  LinkButtonsStepBlockConfig,
  PageBlock,
  StepConfig,
} from "./pageConfig";
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
  onToggleTheme: () => void;
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

function renderButtonIcon(button: LinkButtonConfig) {
  if (!button.icon) return null;
  if (button.icon.type === "download") return <DownloadIcon />;
  if (!button.icon.src) return null;

  return (
    <Image
      src={button.icon.src}
      alt={button.icon.alt || ""}
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
  const buttons =
    recommendedButtonIDs.size === 0
      ? block.buttons
      : [...block.buttons].sort((left, right) => {
          const leftRank = recommendedButtonIDs.has(left.id) ? 0 : 1;
          const rightRank = recommendedButtonIDs.has(right.id) ? 0 : 1;
          return leftRank - rightRank;
        });

  return (
    <div className={styles.buttons}>
      {buttons.map((button) => {
        const recommended = recommendedButtonIDs.has(button.id);
        const visualVariant =
          recommendedButtonIDs.size > 0 && !recommended ? "secondary" : button.variant;

        return (
          <a
            key={button.id}
            className={buttonClassNameWithTone({
              variant: visualVariant,
              recommended,
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
              {block.addButtonLabel}
            </a>
          </div>

          <div className={styles.subLinkRow}>
            <span>{block.manualLinkLabel}</span>
            <code className={styles.subLinkCode}>{activation.subscriptionUrl}</code>
            <button type="button" className={styles.copyLink} onClick={activation.onCopy}>
              {activation.copied ? block.copiedLabel : block.copyLabel}
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

function ThemeToggle({
  themeMode,
  onToggleTheme,
}: {
  themeMode: "light" | "dark";
  onToggleTheme: () => void;
}) {
  const isDark = themeMode === "dark";

  return (
    <button
      type="button"
      className={`${styles.themeToggle} ${isDark ? styles.themeToggleDark : styles.themeToggleLight}`}
      onClick={onToggleTheme}
      aria-label={isDark ? "Переключить на светлую тему" : "Переключить на тёмную тему"}
      aria-pressed={isDark}
    >
      <span className={styles.themeToggleTrack}>
        <span className={styles.themeToggleThumb}>
          {isDark ? <MoonStar size={20} strokeWidth={2.2} /> : <SunMedium size={20} strokeWidth={2.2} />}
        </span>
      </span>
    </button>
  );
}

function HeroBlock({
  block,
  panelLogoDataUrl,
  themeMode,
  onToggleTheme,
}: {
  block: HeroBlockConfig;
  panelLogoDataUrl?: string;
  themeMode: "light" | "dark";
  onToggleTheme: () => void;
}) {
  const logoSrc = panelLogoDataUrl || block.logo.src;

  return (
    <header className={styles.hero}>
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
        <ThemeToggle themeMode={themeMode} onToggleTheme={onToggleTheme} />
      </div>

      <p className={styles.subhead}>{block.subhead}</p>

      {block.statusBadge ? (
        <div className={styles.statusBadge}>
          <span className={styles.statusDot} />
          {block.statusBadge}
        </div>
      ) : null}
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
        <span className={styles.languageBadge}>{block.languageBadge}</span>
      </div>
    </footer>
  );
}

export function SubscriptionBlocks({
  blocks,
  panelLogoDataUrl,
  activation,
  themeMode,
  onToggleTheme,
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
              themeMode={themeMode}
              onToggleTheme={onToggleTheme}
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
