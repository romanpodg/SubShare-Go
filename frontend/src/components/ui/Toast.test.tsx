import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider, useToast } from "./Toast";

function ToastHarness() {
  const { toast } = useToast();

  return (
    <>
      <button onClick={() => toast("Операция завершена", "success")}>Success</button>
      <button onClick={() => toast("Операция не выполнена", "error")}>Error</button>
      <button onClick={() => toast("Проверьте параметры", "warning")}>Warning</button>
      <button onClick={() => toast("Новая информация", "info")}>Info</button>
      <button
        onClick={() =>
          toast(
            "Очень длинное техническое уведомление, которое должно естественно переноситься и оставаться внутри компактного контейнера.",
            "info"
          )
        }
      >
        Long
      </button>
    </>
  );
}

function renderToasts() {
  return render(
    <ToastProvider>
      <ToastHarness />
    </ToastProvider>
  );
}

describe("ToastProvider", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it.each([
    ["Success", "success", "Операция завершена", "status"],
    ["Error", "error", "Операция не выполнена", "alert"],
    ["Warning", "warning", "Проверьте параметры", "status"],
    ["Info", "info", "Новая информация", "status"],
  ])("renders the %s treatment with accessible message output", (button, type, message, role) => {
    renderToasts();

    fireEvent.click(screen.getByRole("button", { name: button }));

    const notification = screen.getByText(message).closest(`[role="${role}"]`);
    if (!notification) {
      throw new Error(`Expected toast notification with role ${role}`);
    }

    expect(notification).toHaveTextContent(message);
    expect(notification).toHaveAttribute("data-toast-type", type);
    expect(notification).toHaveClass("bg-surface-1", "rounded-sm");
    expect(notification.querySelector("svg")).toHaveAttribute("aria-hidden", "true");
  });

  it("wraps long text within the compact maximum width", () => {
    renderToasts();

    fireEvent.click(screen.getByRole("button", { name: "Long" }));

    const notification = screen.getByRole("status");
    expect(notification).toHaveClass("max-w-sm");
    expect(notification.lastElementChild).toHaveClass("break-words", "text-pretty");
  });

  it("stacks multiple notifications with a compact consistent gap", () => {
    const { container } = renderToasts();

    fireEvent.click(screen.getByRole("button", { name: "Success" }));
    fireEvent.click(screen.getByRole("button", { name: "Info" }));

    expect(screen.getAllByRole("status")).toHaveLength(2);
    expect(container.querySelector('[aria-label="Уведомления"]')).toHaveClass("flex-col", "gap-2");
  });

  it("keeps the existing automatic dismissal timing", () => {
    renderToasts();
    fireEvent.click(screen.getByRole("button", { name: "Success" }));

    act(() => vi.advanceTimersByTime(4_000));
    expect(screen.getByRole("status")).toHaveClass("animate-fade-out");

    act(() => vi.advanceTimersByTime(300));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });
});
