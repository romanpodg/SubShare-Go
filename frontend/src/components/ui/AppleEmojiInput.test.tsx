import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AppleEmojiInput } from "./AppleEmojiInput";

describe("AppleEmojiInput native fallback", () => {
  it("renders emoji without a remote image request", () => {
    render(<AppleEmojiInput label="Message" value="Hello 😀" />);
    const editor = screen.getByRole("textbox", { name: "Message" });
    expect(editor).toHaveTextContent("Hello 😀");
    expect(editor.querySelector("img")).toBeNull();
  });

  it("replaces a failed legacy image with its native alt text", () => {
    const onChange = vi.fn();
    render(<AppleEmojiInput label="Message" value="" onChange={onChange} />);
    const editor = screen.getByRole("textbox", { name: "Message" });
    editor.innerHTML = '<img src="/missing-emoji.png" alt="😀">';
    fireEvent.error(editor.querySelector("img")!);
    expect(editor.querySelector("img")).toBeNull();
    expect(editor).toHaveTextContent("😀");
    expect(onChange).toHaveBeenCalledWith({ target: { value: "😀" } });
  });
});
