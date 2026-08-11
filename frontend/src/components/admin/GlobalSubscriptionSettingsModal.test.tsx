import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { GlobalSubscriptionSettingsModal } from "./GlobalSubscriptionSettingsModal";

const mocks = vi.hoisted(() => ({
  get: vi.fn(),
  update: vi.fn(),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    subscriptionSettings: mocks,
  };
});

describe("GlobalSubscriptionSettingsModal", () => {
  it("shows the confirmed Provider ID format and preserves deprecated Happ values", async () => {
    mocks.get.mockResolvedValue({
      title: "AllKeys",
      refresh_hours: 12,
      info_url: "https://example.test/info",
      extra_url: "",
      extra_status: "",
      subscription_format: "links",
      show_subscription_expiration: false,
      time_zone: "UTC",
      language: "ru",
      provider_id: "A1B2C3D4",
      happ_no_limit_mode: true,
      happ_no_limit_mode_xhttp_only: false,
      happ_mandatory_hwid: true,
      happ_notify_expiration: true,
      happ_hide_server_settings: true,
      happ_subscription_body: "legacy-value",
    });
    mocks.update.mockResolvedValue({ message: "ok" });

    render(<GlobalSubscriptionSettingsModal open onClose={vi.fn()} />);

    const providerInput = await screen.findByLabelText("Provider ID");
    expect(providerInput).toHaveAttribute("placeholder", "Например, A1B2C3D4");
    expect(screen.queryByText("No Limit Mode")).not.toBeInTheDocument();
    const expirationCheckbox = screen.getByRole("checkbox", { name: /Показывать дату окончания подписки/ });
    expect(expirationCheckbox).not.toBeChecked();
    expect(screen.getByText(/Передаёт дату окончания подписки совместимым клиентам/)).toBeInTheDocument();
    const localization = screen.getByTestId("settings-localization");
    expect(screen.getByTestId("settings-secondary-column")).toContainElement(localization);
    expect(screen.getByTestId("settings-primary-column")).not.toContainElement(localization);
    expect(localization).toHaveTextContent("Локализация сервиса");
    expect(localization).toHaveTextContent("Часовой пояс");
    expect(localization).toHaveTextContent("Язык интерфейса");

    fireEvent.click(expirationCheckbox);

    fireEvent.change(screen.getByLabelText("Название подписки"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Сохранить" }));

    await waitFor(() => {
      expect(mocks.update).toHaveBeenCalledWith(expect.objectContaining({
        title: "Updated",
        info_url: "https://example.test/info",
        show_subscription_expiration: true,
        happ_no_limit_mode: true,
        happ_mandatory_hwid: true,
        happ_notify_expiration: true,
        happ_hide_server_settings: true,
        happ_subscription_body: "legacy-value",
      }));
    });
  });
});
