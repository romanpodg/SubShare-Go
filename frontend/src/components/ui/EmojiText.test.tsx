import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { EmojiGlyph, EmojiText } from "./EmojiText";

describe("EmojiText", () => {
  it("exposes the full value and enables the emoji-safe truncate mode", () => {
    const text = "Очень длинное название 👇 без гарантии";

    render(<EmojiText text={text} truncate className="custom-class" />);

    const value = screen.getByTitle(text);
    expect(value).toHaveClass("emoji-text", "emoji-text--truncate", "custom-class");
    expect(value).toHaveTextContent("Очень длинное название");
    expect(value).toHaveTextContent("без гарантии");
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
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

  it("falls back to native Unicode when an optional local image fails", () => {
    render(<EmojiGlyph emoji="😀" assetURL="/emoji/native/15/1f600.png" />);
    fireEvent.error(screen.getByRole("img", { name: "😀" }));
    expect(screen.queryByRole("img")).not.toBeInTheDocument();
    expect(screen.getByText("😀")).toHaveClass("emoji-native");
  });
});
