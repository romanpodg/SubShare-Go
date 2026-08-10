import { describe, expect, it } from "vitest";
import {
  externalImportReasonMessage,
  externalImportProtocolLabel,
  externalImportStatusMessage,
  externalImportWarningMessage,
  safeExternalPreviewEndpoint,
} from "./external-import-messages";

describe("external import diagnostics", () => {
  it("renders stable codes as human-readable Russian messages", () => {
    expect(externalImportWarningMessage("partial_import")).toContain("Часть профилей");
    expect(externalImportStatusMessage("unsupported")).toBe("Не поддерживается");
    expect(externalImportReasonMessage("unsupported_hysteria_v1")).toBe("Hysteria v1 пока не поддерживается.");
    expect(externalImportReasonMessage("invalid_hysteria2_json")).toContain("Объект Hysteria2");
    const invalidHysteria2 = `${externalImportProtocolLabel("hysteria2")} · ${externalImportReasonMessage("invalid_hysteria2_json")}`;
    expect(invalidHysteria2).toContain("Hysteria 2");
    expect(invalidHysteria2).not.toContain("Hysteria v1");
    expect(externalImportProtocolLabel("hysteria-unknown")).toContain("версия не определена");
    expect(externalImportReasonMessage("ambiguous_hysteria_version")).toContain("settings.version");
  });

  it("builds endpoints only from safe host and port metadata", () => {
    expect(safeExternalPreviewEndpoint({ label: "HY", scheme: "xray-json", url_short: "hysteria", host: "hy.example", port: "443" })).toBe("hy.example:443");
    expect(safeExternalPreviewEndpoint({ label: "Hidden", scheme: "xray-json", url_short: "xray-json" })).toBe("");
  });
});
