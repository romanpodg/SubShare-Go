import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { InfrastructureDiagram, OperationalStatus, SignalCoreDiagram, TechnicalFrame } from "./Technical";
import { StatusBadge } from "./StatusBadge";

describe("SubShare technical design primitives", () => {
  it("exposes operational state as text in addition to color", () => {
    render(<OperationalStatus label="SYSTEM OPERATIONAL" />);

    expect(screen.getByText("SYSTEM OPERATIONAL")).toBeInTheDocument();
  });

  it("keeps status badges semantically announced", () => {
    render(<StatusBadge status="active" />);

    expect(screen.getByRole("status")).toHaveTextContent("Активен");
  });

  it("gives the infrastructure illustration an accessible description", () => {
    render(
      <TechnicalFrame>
        <InfrastructureDiagram />
      </TechnicalFrame>
    );

    expect(screen.getByRole("img", { name: /схема доставки конфигураций/i })).toBeInTheDocument();
  });

  it("describes the signal core and its four connected systems", () => {
    render(
      <TechnicalFrame>
        <SignalCoreDiagram />
      </TechnicalFrame>
    );

    expect(screen.getByRole("img", { name: /сигнальное ядро SubShare/i })).toBeInTheDocument();
    expect(screen.getByText("SUBSHARE")).toBeInTheDocument();
    expect(screen.getByText("SIGNAL CORE")).toBeInTheDocument();
    expect(screen.getByText("SOURCES")).toBeInTheDocument();
    expect(screen.getByText("KEYS")).toBeInTheDocument();
    expect(screen.getByText("USERS")).toBeInTheDocument();
    expect(screen.getByText("CLIENTS")).toBeInTheDocument();
  });
});
