import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RoutingSettingsModal } from "./RoutingSettingsModal";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
  toast: vi.fn(),
  copy: vi.fn(),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    routingSettings: {
      get: mocks.get,
      update: mocks.update,
    },
  };
});

vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: mocks.toast }),
}));

vi.mock("@/lib/clipboard", () => ({
  copyToClipboard: mocks.copy,
}));

const storedConfig = {
  Name: "Стабильный профиль",
  GlobalProxy: false,
  RouteOrder: "block-proxy-direct",
  DomainStrategy: "IPIfNonMatch",
  FakeDNS: false,
  UseChunkFiles: true,
  RemoteDNSType: "DoH",
  RemoteDNSDomain: "https://example.test/dns-query",
  RemoteDNSIP: "1.1.1.1",
  DomesticDNSType: "DoH",
  DomesticDNSDomain: "https://domestic.test/dns-query",
  DomesticDNSIP: "8.8.8.8",
  Geoipurl: "https://example.test/geoip.dat",
  Geositeurl: "https://example.test/geosite.dat",
  LastUpdated: "",
  DnsHosts: { "example.test": "1.1.1.1" },
  DirectSites: ["one.test", "two.test", "three.test", "four.test", "five.test", "six.test"],
  DirectIp: [],
  ProxySites: [],
  ProxyIp: [],
  BlockSites: [],
  BlockIp: [],
  ForwardCompatible: { enabled: true },
};

const apiResponse = {
  config_json: JSON.stringify(storedConfig),
  delivery_mode: "disabled" as const,
  add_url: "happ://routing/add/server-add-payload",
  onadd_url: "happ://routing/onadd/server-onadd-payload",
  off_url: "happ://routing/off",
};

function encodeUTF8Base64(value: string) {
	const binary = Array.from(new TextEncoder().encode(value), (byte) => String.fromCharCode(byte)).join("");
	return btoa(binary);
}

describe("RoutingSettingsModal", () => {
  beforeEach(() => {
    mocks.get.mockReset().mockResolvedValue(apiResponse);
    mocks.update.mockReset().mockImplementation(async (input) => ({
      ...apiResponse,
      config_json: input.config_json,
      delivery_mode: input.delivery_mode,
    }));
    mocks.toast.mockReset();
    mocks.copy.mockReset().mockResolvedValue(undefined);
  });

  it("loads the safe upgraded mode and uses server-generated manual links", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);

    expect(await screen.findByDisplayValue("Стабильный профиль")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Не передавать/ })).toBeChecked();
    expect(screen.getByLabelText("Добавить профиль, Happ-ссылка")).toHaveTextContent(apiResponse.add_url);
    expect(screen.getByLabelText("Добавить и активировать, Happ-ссылка")).toHaveTextContent(apiResponse.onadd_url);
    expect(screen.getByLabelText("Отключить роутинг, Happ-ссылка")).toHaveTextContent("happ://routing/off");
    fireEvent.click(screen.getByRole("button", { name: "Скопировать: Добавить профиль" }));
    expect(mocks.copy).toHaveBeenCalledWith(apiResponse.add_url);
    expect(screen.queryByText(/Авто-активация/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Применить" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Сбросить" })).toBeVisible();
  });

  it("places delivery above the utilities/editor workspace", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);
    await screen.findByDisplayValue("Стабильный профиль");

    const delivery = screen.getByTestId("routing-delivery-section");
    const workspace = screen.getByTestId("routing-workspace");
    expect(delivery.parentElement).toBe(workspace.parentElement);
    expect(workspace).not.toContainElement(delivery);
    expect(within(workspace).getByLabelText("Утилиты роутинга")).toBeInTheDocument();
    expect(within(workspace).queryByText("Утилиты", { exact: true })).not.toBeInTheDocument();
    expect(within(workspace).getByTestId("routing-import-card")).toBeInTheDocument();
    expect(within(workspace).getByTestId("routing-editor")).toBeInTheDocument();

    const dialog = screen.getByRole("dialog", { name: "Роутинг" });
    expect(dialog.querySelector(".ui-modal-content")).toHaveClass("flex", "min-h-0", "overflow-hidden");
    expect(screen.getByTestId("routing-scroll-area")).toHaveClass("min-h-0", "flex-1", "overflow-y-auto");
    expect(screen.getByTestId("routing-action-footer")).toHaveClass("shrink-0");
  });

  it("round-trips delivery mode without changing the routing JSON", async () => {
    const onSaved = vi.fn();
    render(<RoutingSettingsModal open onClose={vi.fn()} onSaved={onSaved} />);

    await screen.findByDisplayValue("Стабильный профиль");
    fireEvent.click(screen.getByRole("radio", { name: /Добавлять и активировать/ }));
    const save = screen.getByRole("button", { name: "Применить" });
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() => expect(mocks.update).toHaveBeenCalledTimes(1));
    const request = mocks.update.mock.calls[0][0];
    expect(request.delivery_mode).toBe("onadd");
    expect(JSON.parse(request.config_json)).toEqual(storedConfig);
    expect(onSaved).toHaveBeenCalledWith(expect.objectContaining({ delivery_mode: "onadd" }));
  });

  it("tracks dirty state for every delivery-mode selection", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);
    await screen.findByDisplayValue("Стабильный профиль");
    const save = screen.getByRole("button", { name: "Применить" });

    fireEvent.click(screen.getByRole("radio", { name: /Добавлять профиль/ }));
    expect(save).toBeEnabled();
    expect(screen.queryByTestId("manual-links-stale")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("radio", { name: /Не передавать/ }));
    expect(save).toBeDisabled();
    fireEvent.click(screen.getByRole("radio", { name: /Добавлять и активировать/ }));
    expect(save).toBeEnabled();
  });

  it("marks server-generated manual links stale after an unsaved config edit", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);
    const name = await screen.findByDisplayValue("Стабильный профиль");
    expect(screen.queryByTestId("manual-links-stale")).not.toBeInTheDocument();

    fireEvent.change(name, { target: { value: "Изменённый профиль" } });
    expect(screen.getByTestId("manual-links-stale")).toHaveTextContent("Требуется сохранение");
    expect(screen.getByText("Есть несохранённые изменения")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Применить" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Сбросить" })).toBeVisible();
  });

  it("imports add/onadd links without enabling automatic delivery", async () => {
    const imported = { ...storedConfig, Name: "Импортированный профиль" };
		const payload = encodeUTF8Base64(JSON.stringify(imported));
    render(<RoutingSettingsModal open onClose={vi.fn()} />);

    await screen.findByDisplayValue("Стабильный профиль");
    fireEvent.change(screen.getByPlaceholderText(/Вставьте Happ-ссылку/), {
      target: { value: `happ://routing/onadd/${encodeURIComponent(payload)}` },
    });
    fireEvent.click(screen.getByRole("button", { name: "Импортировать" }));

    expect(screen.getByDisplayValue("Импортированный профиль")).toBeInTheDocument();
    expect(screen.getByRole("radio", { name: /Не передавать/ })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Применить" }));
    await waitFor(() => expect(mocks.update).toHaveBeenCalledWith(expect.objectContaining({ delivery_mode: "disabled" })));
  });

  it("shows every configured routing-list entry in a bounded editor", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);
    await screen.findByDisplayValue("Стабильный профиль");
    fireEvent.click(screen.getByRole("button", { name: "Исключения и правила" }));

    expect(screen.getByDisplayValue("one.test")).toBeInTheDocument();
    expect(screen.getByDisplayValue("five.test")).toBeInTheDocument();
    expect(screen.getByDisplayValue("six.test")).toBeInTheDocument();
    expect(screen.queryByText(/Показаны первые 4/)).not.toBeInTheDocument();
  });

  it("keeps the JSON editor synchronized with visual fields", async () => {
    render(<RoutingSettingsModal open onClose={vi.fn()} />);
    await screen.findByDisplayValue("Стабильный профиль");
    fireEvent.click(screen.getByRole("button", { name: "JSON-редактор" }));
		const editor = screen.getByRole("textbox", { name: "JSON-конфигурация роутинга" });
    const changed = { ...storedConfig, Name: "JSON profile" };
    fireEvent.change(editor, { target: { value: JSON.stringify(changed) } });
    fireEvent.click(screen.getByRole("button", { name: "Основные" }));
    expect(screen.getByDisplayValue("JSON profile")).toBeInTheDocument();
  });
});
