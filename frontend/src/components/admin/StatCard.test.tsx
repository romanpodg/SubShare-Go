import { render, screen } from "@testing-library/react";
import { Activity } from "lucide-react";
import { describe, expect, it } from "vitest";
import { StatCard } from "./StatCard";

describe("StatCard", () => {
  it("renders an accessible summary without hiding its hint", () => {
    render(
      <StatCard
        label="Активные пользователи"
        value={42}
        hint="из 50"
        icon={<Activity aria-label="Статистика" />}
        tone="emerald"
      />
    );

    expect(screen.getByText("Активные пользователи")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("из 50")).toBeInTheDocument();
    expect(screen.getByLabelText("Статистика")).toBeInTheDocument();
  });

  it("forwards a semantic layout modifier to its technical frame", () => {
    const { container } = render(
      <StatCard
        className="technical-frame--joined-accent"
        label="Активные"
        value={42}
        icon={<Activity />}
      />
    );

    expect(container.firstElementChild).toHaveClass(
      "technical-frame",
      "technical-frame--joined-accent"
    );
  });
});
