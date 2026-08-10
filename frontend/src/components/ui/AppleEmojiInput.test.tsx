import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useRef, useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { AppleEmojiInput, type AppleEmojiInputHandle } from "./AppleEmojiInput";

const grin = "\u{1F600}";
const family = "\u{1F468}\u200D\u{1F469}\u200D\u{1F467}\u200D\u{1F466}";

function InteractiveInput({ initial = "", maxLength = 200 }: { initial?: string; maxLength?: number }) {
  const [value, setValue] = useState(initial);
  const ref = useRef<AppleEmojiInputHandle>(null);
  return <>
    <AppleEmojiInput ref={ref} label="Interactive" value={value} onChange={(event) => setValue(event.target.value)} maxLength={maxLength} showCounter />
    <button type="button" onClick={() => ref.current?.insertEmoji(grin)}>Insert grin</button>
    <button type="button" onClick={() => ref.current?.insertEmoji(family)}>Insert family</button>
  </>;
}

function selectText(editor: HTMLElement, start: number, end: number) {
  const text = editor.firstChild!;
  const range = document.createRange();
  range.setStart(text, start);
  range.setEnd(text, end);
  const selection = window.getSelection()!;
  selection.removeAllRanges();
  selection.addRange(range);
  fireEvent.mouseUp(editor);
}

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

  it("inserts at the caret, replaces selections, and supports repeated insertion", async () => {
    render(<InteractiveInput initial="hello world" />);
    const editor = screen.getByRole("textbox", { name: "Interactive" });
    editor.focus();
    selectText(editor, 5, 5);
    fireEvent.click(screen.getByRole("button", { name: "Insert grin" }));
    await waitFor(() => expect(editor).toHaveTextContent(`hello${grin} world`));
    expect(editor).toHaveFocus();

    selectText(editor, 0, 5);
    fireEvent.click(screen.getByRole("button", { name: "Insert family" }));
    await waitFor(() => expect(editor).toHaveTextContent(`${family}${grin} world`));

    fireEvent.click(screen.getByRole("button", { name: "Insert grin" }));
    await waitFor(() => expect(editor).toHaveTextContent(`${family}${grin}${grin} world`));
  });

  it("counts and limits multi-codepoint emoji as grapheme characters", async () => {
    render(<InteractiveInput maxLength={2} />);
    const editor = screen.getByRole("textbox", { name: "Interactive" });
    fireEvent.click(screen.getByRole("button", { name: "Insert family" }));
    fireEvent.click(screen.getByRole("button", { name: "Insert grin" }));
    await waitFor(() => expect(editor).toHaveTextContent(`${family}${grin}`));
    expect(screen.getByText("2/2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Insert grin" }));
    await waitFor(() => expect(editor).toHaveTextContent(`${family}${grin}`));
    expect(screen.getByText("2/2")).toBeInTheDocument();
  });
});
