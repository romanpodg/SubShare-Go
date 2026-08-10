import { describe, expect, it } from "vitest";
import { PROTOCOL_EDITOR_CAPABILITIES, protocolEditorCapability } from "./protocol-editor-capabilities";

describe("protocol editor capabilities", () => {
  it("has one structured capability entry for all six create protocols", () => {
    expect(PROTOCOL_EDITOR_CAPABILITIES.map((item) => item.protocol)).toEqual([
      "vless", "vmess", "trojan", "shadowsocks", "hysteria2", "tuic",
    ]);
    for (const item of PROTOCOL_EDITOR_CAPABILITIES) {
      expect(protocolEditorCapability(item.protocol)?.structured).toBe(true);
    }
  });
});
