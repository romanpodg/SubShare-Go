import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SourceCreateDrawer } from "./SourceCreateDrawer";

const mocks = vi.hoisted(() => ({
  preview: vi.fn(),
  create: vi.fn(),
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...original,
    apiV1: {
      sources: {
        preview: mocks.preview,
        create: mocks.create,
      },
    },
  };
});

describe("SourceCreateDrawer", () => {
  it("uses server preview and preserves the explicit manual fallback through creation", async () => {
    const onCreated = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    mocks.preview.mockResolvedValue({
      source_url: "https://provider.example/sub",
      suggested_name: "Provider",
      detected_format: "links",
      key_count: 1,
      metadata: {},
      warnings: [],
      keys: [{ label: "Edge", scheme: "vless", url_short: "vless://•••" }],
    });
    mocks.create.mockResolvedValue({
      data: {
        id: 1,
        name: "Provider",
      },
      imported_count: 1,
      skipped_count: 0,
      warnings: [],
      detected_format: "links",
    });

    render(
      <SourceCreateDrawer
        open
        sourceCategories={[]}
        keyCategories={[]}
        onClose={onClose}
        onCreated={onCreated}
      />
    );

    fireEvent.change(screen.getByLabelText("URL подписки"), {
      target: { value: "https://provider.example/sub" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Показать ручной fallback/i }));
    const rawBody = "vless://example";
    fireEvent.change(screen.getByPlaceholderText(/JSON или список ссылок/i), {
      target: { value: rawBody },
    });
    fireEvent.click(screen.getByRole("button", { name: /Проверить источник/i }));

    expect(await screen.findByDisplayValue("Provider")).toBeInTheDocument();
    expect(mocks.preview).toHaveBeenCalledWith(
      expect.objectContaining({
        source_url: "https://provider.example/sub",
        raw_body: rawBody,
      })
    );

    fireEvent.click(screen.getByRole("button", { name: /Проверить параметры/i }));
    expect(await screen.findByText("Edge")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Добавить источник/i }));

    await waitFor(() => {
      expect(mocks.create).toHaveBeenCalledWith(
        expect.objectContaining({
          source_url: "https://provider.example/sub",
          raw_body: rawBody,
        })
      );
      expect(onCreated).toHaveBeenCalled();
      expect(onClose).toHaveBeenCalled();
    });
  });
});
