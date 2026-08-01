import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Input } from "./Input";
import { PasswordInput } from "./PasswordInput";
import { Select } from "./Select";

describe("editable control interaction states", () => {
  it("keeps an accent focus class for mouse focus while retaining hover styling", () => {
    render(<Input label="Name" />);
    const input = screen.getByLabelText("Name");
    fireEvent.mouseDown(input);
    input.focus();
    fireEvent.mouseEnter(input);

    expect(input).toHaveFocus();
    expect(input).toHaveClass("hover:border-[var(--border-strong)]", "focus:border-accent");
  });

  it("exposes the keyboard focus-visible treatment on inputs and selects", () => {
    render(<><Input label="Name" /><Select label="Format" options={[{ value: "base64", label: "Base64" }]} value="base64" /></>);
    const input = screen.getByLabelText("Name");
    const select = screen.getByRole("button", { name: "Format" });
    fireEvent.keyDown(document, { key: "Tab" });
    input.focus();

    expect(input).toHaveFocus();
    expect(input).toHaveClass("focus-visible:border-accent");
    expect(select).toHaveClass("focus:border-accent", "focus-visible:border-accent");
  });

  it("uses the same mouse and keyboard focus treatment for password inputs", () => {
    render(<PasswordInput label="Password" />);
    const input = screen.getByLabelText("Password");
    fireEvent.mouseDown(input);
    input.focus();
    fireEvent.mouseEnter(input);

    expect(input).toHaveFocus();
    expect(input).toHaveClass("focus:border-accent", "focus-visible:border-accent");
  });
});
