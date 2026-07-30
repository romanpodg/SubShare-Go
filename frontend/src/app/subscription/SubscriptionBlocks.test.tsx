import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { isMonochromeButtonIcon, SubscriptionBlocks } from "./SubscriptionBlocks";
import type { LinkButtonConfig, PageBlock } from "./pageConfig";

const windowsButton: LinkButtonConfig = {
  id: "windows",
  label: "Скачать для Windows",
  href: "https://example.com/windows",
  variant: "secondary",
  external: true,
  icon: {
    type: "image",
    src: "/subscription/windows.svg",
    alt: "Windows",
  },
};

const googlePlayButton: LinkButtonConfig = {
  id: "google-play",
  label: "Google Play",
  href: "https://example.com/google-play",
  variant: "secondary",
  external: true,
  icon: {
    type: "image",
    src: "/subscription/googleplay.svg",
    alt: "Google Play",
  },
};

const blocks: PageBlock[] = [
  {
    type: "steps",
    id: "subscription",
    title: "Как подключиться",
    steps: [
      {
        id: "install",
        title: "Установите приложение",
        description: "Выберите платформу",
        block: {
          type: "linkButtons",
          buttons: [windowsButton, googlePlayButton],
        },
      },
    ],
  },
];

describe("SubscriptionBlocks platform controls", () => {
  beforeEach(() => {
    vi.stubGlobal("requestAnimationFrame", vi.fn(() => 1));
    vi.stubGlobal("cancelAnimationFrame", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("recognizes only bundled monochrome SVG assets as theme-aware masks", () => {
    expect(isMonochromeButtonIcon(windowsButton)).toBe(true);
    expect(
      isMonochromeButtonIcon({
        ...windowsButton,
        icon: { ...windowsButton.icon!, src: "/subscription/apple.svg?version=2" },
      })
    ).toBe(true);
    expect(isMonochromeButtonIcon(googlePlayButton)).toBe(false);
    expect(
      isMonochromeButtonIcon({
        ...windowsButton,
        icon: { ...windowsButton.icon!, src: "https://example.com/custom.svg" },
      })
    ).toBe(false);
  });

  it("renders separate pressed platform filters and preserves multicolor artwork", () => {
    const { container } = render(
      <SubscriptionBlocks
        blocks={blocks}
        themeMode="dark"
        activation={{
          code: "",
          loading: false,
          error: "",
          message: "",
          subscriptionUrl: "",
          copied: false,
          onCodeChange: vi.fn(),
          onSubmit: vi.fn(),
          onCopy: vi.fn(),
        }}
      />
    );

    const platformGroup = screen.getByRole("group", { name: "Выбор платформы" });
    expect(platformGroup).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Все" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Windows" })).toHaveAttribute("aria-pressed", "false");

    const windowsLink = container.querySelector('[data-button-id="windows"]');
    const googlePlayLink = container.querySelector('[data-button-id="google-play"]');
    expect(windowsLink?.querySelector('[style*="--button-icon-mask"]')).toBeInTheDocument();
    expect(googlePlayLink?.querySelector("img")).toBeInTheDocument();
  });
});
