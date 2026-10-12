import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import fixtures from "../../../../testdata/subscription-page/contracts.json";
import SubscriptionPage from "./page";
import { subscriptionPageConfig } from "./pageConfig";

const mocks = vi.hoisted(() => ({
  getPublic: vi.fn(), activate: vi.fn(), copy: vi.fn(),
  settings: { panelTitle: "", logoDataUrl: "", pageTitles: { subscription: "Fixture page" } },
}));
vi.mock("@/lib/api", () => ({ subscriptionPageConfig: { getPublic: mocks.getPublic }, subscription: { activate: mocks.activate } }));
vi.mock("@/context/PanelSettingsContext", () => ({ usePanelSettings: () => ({ settings: mocks.settings }) }));
vi.mock("@/lib/clipboard", () => ({ copyToClipboard: mocks.copy }));
vi.mock("./NumericGlobe", () => ({ NumericGlobe: () => null }));

async function renderConfig(raw: string) {
  mocks.getPublic.mockResolvedValue({ config_json: raw });
  let view!: ReturnType<typeof render>;
  await act(async () => { view = render(<SubscriptionPage />); });
  return view;
}

beforeEach(() => {
  vi.stubGlobal("requestAnimationFrame", vi.fn(() => 1));
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  mocks.settings.panelTitle = "";
  mocks.activate.mockReset();
  mocks.copy.mockReset().mockResolvedValue(true);
});
afterEach(() => vi.unstubAllGlobals());

describe("shared public-page supported semantics", () => {
  it("renders all supported blocks with escaped text, attributes and external link policy", async () => {
    const { container } = await renderConfig(JSON.stringify(fixtures.config));
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent(fixtures.expected.brand);
    for (const text of [fixtures.expected.subtitle, fixtures.expected.subhead, fixtures.expected.section, fixtures.expected.copyright, "Text-only step"]) {
      expect(screen.getByText(text)).toBeInTheDocument();
    }
    const link = screen.getByRole("link", { name: "Install & go" });
    expect(link).toHaveAttribute("href", "https://example.test/client?a=1&b=2");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noreferrer");
    expect(screen.getByRole("link", { name: "Local help" })).toHaveAttribute("href", "/help?q=one&lang=ru");
    expect(screen.getByRole("link", { name: "Admin" })).toHaveAttribute("href", "/admin/login");
    expect(container.querySelector("script")).toBeNull();
    expect(container.firstElementChild).toHaveStyle({ "--sub-btn-primary-hover": fixtures.expected.frontendPrimaryHover });
    expect(screen.getByLabelText("Fixture code")).toHaveAttribute("placeholder", "Code here");
    expect(screen.queryByRole("link", { name: "Import fixture" })).toBeNull();
    expect(document.title).toBe("Fixture page");
  });

  it("retains activation loading, Happ import, escaped success text and clipboard behavior", async () => {
    let finish!: (value: { subscription_url: string; message: string }) => void;
    mocks.activate.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
    await renderConfig(JSON.stringify(fixtures.config));
    fireEvent.change(screen.getByLabelText("Fixture code"), { target: { value: "fixture-code" } });
    fireEvent.click(screen.getByRole("button", { name: "Activate fixture" }));
    expect(screen.getByRole("button", { name: "Waiting fixture" })).toBeDisabled();
    expect(mocks.activate).toHaveBeenCalledWith("fixture-code");
    await act(async () => finish({ subscription_url: fixtures.expected.importURL, message: "<script>success</script>" }));
    expect(screen.getByRole("link", { name: "Import fixture" })).toHaveAttribute("href", fixtures.expected.importURL);
    expect(screen.getByText("<script>success</script>")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Activate fixture" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Copy fixture" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Copied fixture" })).toBeInTheDocument());
    expect(mocks.copy).toHaveBeenCalledWith(fixtures.expected.importURL);
  });

  it("hydrates missing activation labels and uses the configured failure fallback", async () => {
    const config = { ...fixtures.config, blocks: [{ type: "steps", id: "s", title: "Hydration", steps: [{ id: "a", title: "Activate", description: "", block: { type: "activation", activateErrorFallback: "Custom fallback" } }] }] };
    mocks.activate.mockRejectedValue("non-Error fixture");
    await renderConfig(JSON.stringify(config));
    fireEvent.change(screen.getByLabelText("Ключ активации"), { target: { value: "code" } });
    fireEvent.click(screen.getByRole("button", { name: "Активировать" }));
    await screen.findByText("Custom fallback");
  });
});

describe("shared compatibility boundaries", () => {
  it.each(fixtures.admission)("preserves frontend fallback for $name", async ({ raw, goAccepted }) => {
    await renderConfig(raw);
    expect(screen.getByText(goAccepted ? "Admission fixture" : subscriptionPageConfig.blocks[0].type === "hero" ? subscriptionPageConfig.blocks[0].brandSubtitle : "")).toBeInTheDocument();
  });

  it.each(fixtures.links)("characterizes $name button URLs", async ({ href, frontendHref }) => {
    const config = structuredClone(fixtures.config);
    config.blocks[1].steps![0].block.buttons![0].href = href;
    await renderConfig(JSON.stringify(config));
    const actual = screen.getByRole("link", { name: "Install & go" }).getAttribute("href");
    if (frontendHref === "react-blocked") expect(actual).toMatch(/^javascript:throw new Error\(/);
    else expect(actual).toBe(frontendHref);
  });

  it.each(fixtures.themes)("retains frontend CSS-property behavior for $name", async ({ value }) => {
    const config = structuredClone(fixtures.config);
    config.theme.badgeText = value;
    const { container } = await renderConfig(JSON.stringify(config));
    expect((container.firstElementChild as HTMLElement).style.getPropertyValue("--sub-badge-text")).toBe(value);
    expect(container.querySelector("script")).toBeNull();
  });

  it("preserves template unknown-field behavior and trimmed panel branding override", async () => {
    const config = structuredClone(fixtures.config);
    config.blocks[0].brandTitle = fixtures.template.source;
    mocks.settings.panelTitle = " Fixture ";
    await renderConfig(JSON.stringify(config));
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe(fixtures.template.frontend);
  });

  it("preserves localized activation defaults and supported download layout", () => {
    expect(subscriptionPageConfig.locale).toBe(fixtures.defaults.locale);
    expect(subscriptionPageConfig.templateVars.brandName).toBe(fixtures.defaults.frontendBrand);
    expect(subscriptionPageConfig.blocks.map((block) => block.type)).toEqual(fixtures.defaults.blockTypes);
    const steps = subscriptionPageConfig.blocks.flatMap((block) => block.type === "steps" ? block.steps : []);
    expect(steps.map((step) => step.block.type)).toEqual(fixtures.defaults.stepTypes);
    expect(steps.flatMap((step) => step.block.type === "linkButtons" ? step.block.buttons.map((button) => button.id) : [])).toEqual(fixtures.defaults.downloadIDs);
  });
});
