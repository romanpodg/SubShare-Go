import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SubscriptionSettingsPage from "./page";

const mocks = vi.hoisted(() => ({
  getSettings: vi.fn(),
	getDelivery: vi.fn(),
	getRouting: vi.fn(),
  updateDelivery: vi.fn(),
  toast: vi.fn(),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
		subscriptionSettings: {
      ...original.subscriptionSettings,
      get: mocks.getSettings,
		},
		routingSettings: {
			...original.routingSettings,
			get: mocks.getRouting,
		},
    apiV1: {
      ...original.apiV1,
      deliverySettings: {
        get: mocks.getDelivery,
        update: mocks.updateDelivery,
      },
    },
  };
});

vi.mock("@/hooks/useAuth", () => ({
  useAuth: () => ({ role: "owner" }),
}));

vi.mock("@/components/ui/Toast", () => ({
  useToast: () => ({ toast: mocks.toast }),
}));

vi.mock("@/components/admin/GlobalSubscriptionSettingsModal", () => ({
  GlobalSubscriptionSettingsModal: () => null,
}));

vi.mock("@/components/admin/RoutingSettingsModal", () => ({
  RoutingSettingsModal: () => null,
}));

const emptyRemarks = () => ({
  expired: [],
  paused: [],
  blocked: [],
  limited: [],
  empty: [],
});

describe("SubscriptionSettingsPage delivery editor", () => {
  beforeEach(() => {
    mocks.getSettings.mockReset().mockResolvedValue({
      title: "AllKeys",
      refresh_hours: 12,
      info_url: "",
      extra_url: "",
      extra_status: "",
      subscription_format: "links",
      show_subscription_expiration: false,
      time_zone: "UTC",
      language: "ru",
      provider_id: "",
      happ_no_limit_mode: false,
      happ_no_limit_mode_xhttp_only: false,
      happ_mandatory_hwid: false,
      happ_notify_expiration: false,
      happ_hide_server_settings: false,
      happ_subscription_body: "",
    });
		mocks.getDelivery.mockReset().mockResolvedValue({
      response_headers: [],
      remarks: emptyRemarks(),
      capabilities: [],
      generation_exclusion_reason_codes: [],
		});
		mocks.getRouting.mockReset().mockResolvedValue({
			config_json: "",
			delivery_mode: "disabled",
			add_url: "",
			onadd_url: "",
			off_url: "happ://routing/off",
		});
    mocks.updateDelivery.mockReset().mockResolvedValue({ message: "ok" });
    mocks.toast.mockReset();
	});

	it("shows the persisted routing delivery state in the summary", async () => {
		mocks.getRouting.mockResolvedValue({
			config_json: '{"Name":"SubShare Routing"}',
			delivery_mode: "onadd",
			add_url: "happ://routing/add/payload",
			onadd_url: "happ://routing/onadd/payload",
			off_url: "happ://routing/off",
		});
		render(<SubscriptionSettingsPage />);
		expect(await screen.findByText("Передаётся с подпиской")).toBeInTheDocument();
		expect(screen.getByText("Добавлять и активировать")).toBeInTheDocument();
	});

  it("loads and edits header rows and round-trips ordered remark arrays", async () => {
    mocks.getDelivery.mockResolvedValue({
      response_headers: [{ key: "X-Trace", value: "enabled" }],
      remarks: {
        ...emptyRemarks(),
        expired: ["Первая ремарка", "Вторая ремарка"],
      },
      capabilities: [],
      generation_exclusion_reason_codes: [],
    });

    render(<SubscriptionSettingsPage />);

    expect(await screen.findByDisplayValue("X-Trace")).toBeInTheDocument();
    expect(screen.getByDisplayValue("enabled")).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Сохранить" });
    expect(save).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Добавить заголовок" }));
    fireEvent.change(screen.getByLabelText("Имя заголовка 2"), { target: { value: "X-New" } });
    fireEvent.change(screen.getByLabelText("Значение заголовка 2"), { target: { value: "new-value" } });
    fireEvent.click(screen.getByRole("button", { name: "Удалить заголовок 1" }));
    expect(screen.queryByDisplayValue("X-Trace")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("X-New")).toBeInTheDocument();
    expect(save).toBeEnabled();

    fireEvent.click(screen.getByRole("tab", { name: "Ремарки" }));
    const expiredCard = screen.getByTestId("remark-card-expired");
    expect(within(expiredCard).getByDisplayValue("Первая ремарка")).toBeInTheDocument();
    expect(within(expiredCard).getByDisplayValue("Вторая ремарка")).toBeInTheDocument();

    fireEvent.click(within(expiredCard).getByRole("button", { name: "Добавить ремарку для Подписка истекла" }));
    fireEvent.change(within(expiredCard).getByLabelText("Ремарка 3: Подписка истекла"), { target: { value: "  Третья ремарка  " } });
    fireEvent.click(within(expiredCard).getByRole("button", { name: "Удалить ремарку 2: Подписка истекла" }));
    fireEvent.click(save);

    await waitFor(() => expect(mocks.updateDelivery).toHaveBeenCalledWith({
      response_headers: [{ key: "X-New", value: "new-value" }],
      remarks: {
        ...emptyRemarks(),
        expired: ["Первая ремарка", "Третья ремарка"],
      },
    }));
    await waitFor(() => expect(save).toBeDisabled());
    expect(mocks.toast).toHaveBeenCalledWith("Настройки выдачи сохранены", "success");
  });

  it("renders empty status cards and drops blank remark rows on save", async () => {
    render(<SubscriptionSettingsPage />);

    await screen.findByText("Дополнительные заголовки не настроены.");
    fireEvent.click(screen.getByRole("tab", { name: "Ремарки" }));
    expect(screen.getAllByText("Ремарок пока нет.")).toHaveLength(5);
    expect(document.querySelector("textarea")).not.toBeInTheDocument();

    const pausedCard = screen.getByTestId("remark-card-paused");
    fireEvent.click(within(pausedCard).getByRole("button", { name: "Добавить ремарку для Подписка приостановлена" }));
    expect(within(pausedCard).getByLabelText("Ремарка 1: Подписка приостановлена")).toHaveValue("");
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => expect(mocks.updateDelivery).toHaveBeenCalledWith({
      response_headers: [],
      remarks: emptyRemarks(),
    }));
    await waitFor(() => expect(within(pausedCard).queryByRole("textbox")).not.toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Сохранить" })).toBeDisabled();
  });

  it("keeps server-side reserved-header rejection visible without clearing edits", async () => {
    mocks.updateDelivery.mockRejectedValueOnce(new Error('unsafe response header "Announce"'));
    render(<SubscriptionSettingsPage />);

    await screen.findByText("Дополнительные заголовки не настроены.");
    fireEvent.click(screen.getByRole("button", { name: "Добавить заголовок" }));
    fireEvent.change(screen.getByLabelText("Имя заголовка 1"), { target: { value: "Announce" } });
    fireEvent.change(screen.getByLabelText("Значение заголовка 1"), { target: { value: "override" } });
    const save = screen.getByRole("button", { name: "Сохранить" });
    fireEvent.click(save);

    await waitFor(() => expect(mocks.updateDelivery).toHaveBeenCalledWith({
      response_headers: [{ key: "Announce", value: "override" }],
      remarks: emptyRemarks(),
    }));
    expect(mocks.toast).toHaveBeenCalledWith('unsafe response header "Announce"', "error");
    expect(screen.getByDisplayValue("Announce")).toBeInTheDocument();
    expect(save).toBeEnabled();
  });
});
