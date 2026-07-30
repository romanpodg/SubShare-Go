import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Modal } from "./Modal";

describe("Modal overflow layout", () => {
  it("keeps the decorated frame clipped and scrolls only its content", () => {
    render(
      <Modal open onClose={vi.fn()} title="Короткое окно">
        <p>Короткое содержимое</p>
      </Modal>
    );

    const dialog = screen.getByRole("dialog", { name: "Короткое окно" });
    const content = dialog.querySelector(".ui-modal-content");

    expect(dialog).toHaveClass("ui-modal-frame", "overflow-hidden");
    expect(dialog).not.toHaveClass("overflow-y-auto");
    expect(content).toHaveClass("overflow-y-auto", "overflow-x-hidden");
  });

  it("allows complex dialogs to provide their own non-scrolling content shell", () => {
    render(
      <Modal
        open
        onClose={vi.fn()}
        title="Рабочая область"
        contentClassName="flex min-h-0 flex-1 flex-col overflow-hidden"
      >
        <div>Собственный scroll-контейнер</div>
      </Modal>
    );

    const content = screen
      .getByRole("dialog", { name: "Рабочая область" })
      .querySelector(".ui-modal-content");

    expect(content).toHaveClass("overflow-hidden");
    expect(content).not.toHaveClass("overflow-y-auto");
  });
});
