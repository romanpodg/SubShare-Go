import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EmojiText } from "./EmojiText";

describe("EmojiText", () => {
  it("exposes the full value and enables the emoji-safe truncate mode", () => {
    const text = "Очень длинное название 👇 без гарантии";

    render(<EmojiText text={text} truncate className="custom-class" />);

    const value = screen.getByTitle(text);
    expect(value).toHaveClass("emoji-text", "emoji-text--truncate", "custom-class");
    expect(value).toHaveTextContent("Очень длинное название");
    expect(value).toHaveTextContent("без гарантии");
    expect(screen.getByRole("img", { name: "👇" })).toBeInTheDocument();
  });

  it("does not add a tooltip or truncate class in the default wrapping mode", () => {
    const text = "Обычный текст";

    const { container } = render(<EmojiText text={text} />);

    const value = container.querySelector(".emoji-text");
    expect(value).not.toBeNull();
    expect(value).toHaveClass("emoji-text");
    expect(value).not.toHaveClass("emoji-text--truncate");
    expect(value).not.toHaveAttribute("title");
  });
});
