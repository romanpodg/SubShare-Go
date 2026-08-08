import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { EmojiPickerButton } from "./EmojiPickerButton";

const mocks = vi.hoisted(() => ({ pickerOptions: [] as Array<Record<string, unknown>> }));

vi.mock("emoji-mart", () => ({
  Picker: class {
    constructor(options: Record<string, unknown>) {
      mocks.pickerOptions.push(options);
      const picker = document.createElement("em-emoji-picker");
      const shadow = picker.attachShadow({ mode: "open" });
      const search = document.createElement("input");
      search.type = "search";
      const select = document.createElement("button");
      select.textContent = "Select emoji";
      select.addEventListener("click", () => {
        (options.onEmojiSelect as (emoji: { native: string }) => void)({ native: "😀" });
      });
      shadow.append(search, select);
      return picker;
    }
  },
}));

function setViewport(width: number, height: number) {
  Object.defineProperty(window, "innerWidth", { configurable: true, value: width });
  Object.defineProperty(window, "innerHeight", { configurable: true, value: height });
}

function rect(top: number, right: number, bottom: number, left: number): DOMRect {
  return { top, right, bottom, left, width: right - left, height: bottom - top, x: left, y: top, toJSON: () => ({}) };
}

describe("EmojiPickerButton", () => {
  beforeEach(() => {
    mocks.pickerOptions.length = 0;
    setViewport(1280, 900);
  });

  it("portals a stable picker outside a scrollable modal and repositions on scroll", async () => {
    let triggerRect = rect(60, 900, 104, 856);
    render(<div data-testid="scroll-modal" style={{ overflow: "auto" }}><EmojiPickerButton iconOnly onSelect={vi.fn()} /></div>);
    const trigger = screen.getByRole("button", { name: "Показать эмодзи" });
    trigger.getBoundingClientRect = () => triggerRect;
    fireEvent.click(trigger);

    expect(screen.getByRole("status")).toHaveTextContent("Загрузка эмодзи");
    const dialog = await screen.findByRole("dialog", { name: "Выбор эмодзи" });
    await waitFor(() => expect(dialog.style.width).toBe("352px"));
    expect(dialog.parentElement).toBe(document.body);
    expect(dialog.style.height).toBe("400px");

    triggerRect = rect(700, 1018, 744, 974);
    fireEvent.scroll(screen.getByTestId("scroll-modal"));
    await waitFor(() => expect(dialog).toHaveAttribute("data-placement", "top"));
  });

  it("uses the available width on narrow viewports", async () => {
    setViewport(320, 700);
    render(<EmojiPickerButton iconOnly onSelect={vi.fn()} />);
    const trigger = screen.getByRole("button", { name: "Показать эмодзи" });
    trigger.getBoundingClientRect = () => rect(60, 304, 104, 260);
    fireEvent.click(trigger);
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(dialog.style.width).toBe("296px"));
    expect(dialog.style.left).toBe("12px");
  });

  it("closes on Escape and outside pointer interaction", async () => {
    render(<><EmojiPickerButton iconOnly onSelect={vi.fn()} /><button type="button">Outside</button></>);
    const trigger = screen.getByRole("button", { name: "Показать эмодзи" });
    trigger.getBoundingClientRect = () => rect(60, 120, 104, 76);
    fireEvent.click(trigger);
    await screen.findByRole("dialog");
    fireEvent.keyDown(document, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(trigger).toHaveFocus());

    fireEvent.click(trigger);
    await screen.findByRole("dialog");
    fireEvent.pointerDown(screen.getByRole("button", { name: "Outside" }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });

  it("selects a native emoji and restores focus", async () => {
    const onSelect = vi.fn();
    render(<EmojiPickerButton iconOnly onSelect={onSelect} />);
    const trigger = screen.getByRole("button", { name: "Показать эмодзи" });
    trigger.getBoundingClientRect = () => rect(60, 120, 104, 76);
    fireEvent.click(trigger);
    const dialog = await screen.findByRole("dialog");
    const picker = await waitFor(() => {
      const element = dialog.querySelector<HTMLElement>("em-emoji-picker");
      expect(element).not.toBeNull();
      return element!;
    });
    expect(mocks.pickerOptions.at(-1)).toMatchObject({ dynamicWidth: false, perLine: 9, set: "native" });
    (picker.shadowRoot?.querySelector("button") as HTMLButtonElement).click();

    expect(onSelect).toHaveBeenCalledWith("😀");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    await waitFor(() => expect(trigger).toHaveFocus());
  });
});
