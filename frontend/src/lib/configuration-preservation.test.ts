import { describe, expect, it } from "vitest";
import { corpusProtocols, preservationCorpus } from "./__fixtures__/configuration-preservation";
import { sourceOnlyConversionGolden, presentConversionGolden, transportGolden, rawTcpGolden, realityAliasGolden, realityAliasShapes } from "./__fixtures__/configuration-preservation-expectations";
import { singleTransportGoldens, absentTransportGoldens, singleSecurityGoldens, absentSecurityGoldens, singleConnectionGoldens, tlsOnlyNoneGoldens } from "./__fixtures__/configuration-field-goldens";
import { partialTransportGoldens, nonObjectGoldens, lowercaseHostGoldens, combinedModeGoldens, combinedProtocolGoldens, trueNoneGoldens, trueNoneTransitions, missingConnectionGoldens, commaListGoldens, trueNoneControlGoldens, intermediateConnectionGoldens, noncanonicalGoldens, zeroAlterIDGoldens, portNormalizationGoldens } from "./__fixtures__/configuration-edge-goldens";
import { parseXrayJSONConfiguration, patchEditableConfiguration, patchXrayJSONConfiguration } from "./configuration";
import { DuplicateJSONKeyError, formatXrayJSON } from "./xray-json-document";

describe("R09 configuration preservation corpus", () => {
  it.each([...partialTransportGoldens, ...nonObjectGoldens, ...lowercaseHostGoldens, ...combinedModeGoldens, ...combinedProtocolGoldens, ...trueNoneTransitions, ...missingConnectionGoldens, ...commaListGoldens, ...trueNoneControlGoldens, ...intermediateConnectionGoldens, ...noncanonicalGoldens, ...zeroAlterIDGoldens, ...portNormalizationGoldens])("retains imported and combined-edit edge contracts: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(trueNoneGoldens)("does not invent security branches in true none mode: $name", ({ raw, expected }) => {
    expect(parseXrayJSONConfiguration(raw)?.draft.security).toBe("none");
    expect(patchXrayJSONConfiguration(raw, {})).toBe(formatXrayJSON(expected));
    expect(patchXrayJSONConfiguration(raw, { sni: "ignored.example", publicKey: "updated", alpn: "h2" })).toBe(formatXrayJSON(expected));
  });

  it.each([
    { raw: "{", message: "ошибку синтаксиса" }, { raw: "[]", message: "должен быть JSON-объектом" },
    { raw: "null", message: "должен быть JSON-объектом" }, { raw: "{}", message: "Не найден outbound" },
    { raw: '{"outbounds":{}}', message: "Не найден outbound" }, { raw: '{"outbounds":[{"protocol":"freedom"}]}', message: "Не найден outbound" },
  ])("rejects invalid document selection before returning a patch: $raw", ({ raw, message }) => {
    expect(() => patchXrayJSONConfiguration(raw, { server: "invalid.example" })).toThrow(message);
  });
  it.each(singleTransportGoldens)("preserves unchanged siblings during one transport-control edit: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(absentTransportGoldens)("creates missing transport parents for one control: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(singleSecurityGoldens)("patches only one represented security control: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(absentSecurityGoldens)("creates missing security parents for one control: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(singleConnectionGoldens)("patches only one connection or protocol-user control: $name", ({ raw, patch, expected }) => {
    expect(patchXrayJSONConfiguration(raw, patch)).toBe(formatXrayJSON(expected));
  });

  it.each(tlsOnlyNoneGoldens)("retains none-to-TLS fallback without Reality settings: $name", ({ raw, expected }) => {
    expect(parseXrayJSONConfiguration(raw)?.draft.security).toBe("tls");
    expect(patchXrayJSONConfiguration(raw, {})).toBe(formatXrayJSON(expected));
    expect(patchXrayJSONConfiguration(raw, { security: "none" })).toBe(formatXrayJSON(expected));
    const enabled = patchXrayJSONConfiguration(raw, { security: "tls" });
    expect(enabled).toBe(formatXrayJSON(expected).replace('"security": "none"', '"security": "tls"'));
    expect(patchXrayJSONConfiguration(enabled, { security: "none" })).toBe(formatXrayJSON(expected));
  });
  it.each(preservationCorpus)("preserves normalized no-op bytes and token lexemes: $name", ({ raw, protocol, network, security }) => {
    const normalized = formatXrayJSON(raw);
    // The current editor infers Reality from its dormant settings even when
    // the stored discriminator is none; no-op editing retains the raw none.
    expect(parseXrayJSONConfiguration(raw)?.draft).toMatchObject({ protocol, network, security: security === "none" ? "reality" : security });
    expect(patchXrayJSONConfiguration(raw, {})).toBe(normalized);
    expect(patchXrayJSONConfiguration(normalized, {})).toBe(normalized);
    expect(normalized).toContain("1e400");
    expect(normalized).toContain("900719925474099312345");
    expect(normalized).toContain('"\\u0061"');
  });

  it.each(preservationCorpus)("matches the whole-document golden for one endpoint edit: $name", ({ raw }) => {
    const expected = formatXrayJSON(raw).replace('"edge.matrix.example"', '"changed.matrix.example"');
    const result = patchXrayJSONConfiguration(raw, { server: "changed.matrix.example" });
    expect(result).toBe(expected);
    expect(patchXrayJSONConfiguration(result, { server: "changed.matrix.example" })).toBe(expected);
  });

  it.each(preservationCorpus.filter(({ network, security }) => network === "tcp" && security === "none"))("keeps absent stream settings absent during connection edits: $name", ({ protocol, document }) => {
    const raw = JSON.stringify({ outbounds: [{ protocol, settings: document.outbounds[1].settings }] });
    expect(parseXrayJSONConfiguration(raw)?.draft).toMatchObject({ network: "tcp", security: "none" });
    const expected = formatXrayJSON(raw).replace('"edge.matrix.example"', '"changed.matrix.example"');
    expect(patchXrayJSONConfiguration(raw, { server: "changed.matrix.example" })).toBe(expected);
  });

  it.each(preservationCorpus.filter(({ network, security }) => network === "tcp" && security === "none"))("rejects explicit required-field clears before document mutation: $name", ({ raw, protocol }) => {
    expect(() => patchXrayJSONConfiguration(raw, { server: "" })).toThrow("Укажите сервер");
    expect(() => patchXrayJSONConfiguration(raw, { identifier: "" })).toThrow(protocol === "trojan" ? "Укажите пароль" : "Укажите UUID / ID");
  });

  it.each(preservationCorpus)("changes only discriminators while retaining dormant branches: $name", ({ raw, network, security }) => {
    const nextNetwork = network === "ws" ? "grpc" : "ws";
    const nextSecurity = security === "tls" ? "reality" : "tls";
    const expected = formatXrayJSON(raw)
      .replace(`"network": "${network}"`, `"network": "${nextNetwork}"`)
      .replace(`"security": "${security}"`, `"security": "${nextSecurity}"`);
    expect(patchXrayJSONConfiguration(raw, { network: nextNetwork, security: nextSecurity })).toBe(expected);
  });

  it.each(preservationCorpus)("edits only the projected security branch SNI: $name", ({ raw, security }) => {
    const oldName = security === "tls" ? "tls.matrix.example" : "reality.matrix.example";
    const expected = formatXrayJSON(raw).replace(`"${oldName}"`, '"changed-sni.example"');
    expect(patchXrayJSONConfiguration(raw, { sni: "changed-sni.example" })).toBe(expected);
  });

  it.each(preservationCorpus.filter(({ security }) => security === "tls"))("explicit false clears only the represented TLS flag: $name", ({ raw }) => {
    const expected = formatXrayJSON(raw).replace(/^\s+"allowInsecure": false,\n/m, "");
    expect(patchXrayJSONConfiguration(raw, { allowInsecure: false })).toBe(expected);
  });

  it.each(preservationCorpus.filter(({ security }) => security !== "tls"))("explicit key clear removes both represented Reality aliases: $name", ({ raw }) => {
    const expected = formatXrayJSON(raw)
      .replace(/^\s+"publicKey": "public",\n/m, "")
      .replace(/^\s+"password": "public",\n/m, "");
    expect(patchXrayJSONConfiguration(raw, { publicKey: "" })).toBe(expected);
  });

  it.each(preservationCorpus)("preserves extra entries and dormant branches during protocol changes: $name", (corpus) => {
    const { raw, protocol } = corpus;
    for (const target of corpusProtocols.filter((value) => value !== protocol)) {
      const result = patchXrayJSONConfiguration(raw, { protocol: target });
      expect(result).toBe(formatXrayJSON(presentConversionGolden(corpus, target)));
      expect(parseXrayJSONConfiguration(result)?.draft).toMatchObject({ protocol: target, server: "edge.matrix.example", port: "2443", identifier: "11111111-1111-4111-8111-111111111111" });
      expect(result).toContain("1e400");
      expect(result).toContain("900719925474099312345");
      expect(result).toContain('"\\u0061"');
    }
  });

  it.each(preservationCorpus)("creates absent target connection branches with exact conversion bytes: $name", (corpus) => {
    for (const target of corpusProtocols.filter((value) => value !== corpus.protocol)) {
      const golden = sourceOnlyConversionGolden(corpus, target);
      expect(patchXrayJSONConfiguration(golden.raw, { protocol: target })).toBe(formatXrayJSON(golden.expected));
    }
  });

  it.each(preservationCorpus)("patches and clears active transport fields without changing other branches: $name", (corpus) => {
    for (const mode of ["set", "clear"] as const) {
      const golden = transportGolden(corpus, mode);
      expect(patchXrayJSONConfiguration(corpus.raw, golden.patch)).toBe(formatXrayJSON(golden.expected));
    }
  });

  it.each(preservationCorpus.filter(({ network }) => network === "tcp"))("prefers existing raw TCP settings and retains dormant TCP settings: $name", (corpus) => {
    for (const mode of ["set", "clear"] as const) {
      const golden = rawTcpGolden(corpus, mode);
      expect(patchXrayJSONConfiguration(golden.raw, transportGolden(corpus, mode).patch)).toBe(formatXrayJSON(golden.expected));
    }
  });

  it.each(preservationCorpus)("retains dormant branches through every explicit security transition: $name", ({ raw, security }) => {
    for (const target of (["none", "tls", "reality"] as const).filter((value) => value !== security)) {
      const expected = formatXrayJSON(raw).replace(`"security": "${security}"`, `"security": "${target}"`);
      const result = patchXrayJSONConfiguration(raw, { security: target });
      expect(result).toBe(expected);
      expect(parseXrayJSONConfiguration(result)?.draft.security).toBe(target === "none" ? "reality" : target);
    }
  });

  it.each(preservationCorpus.filter(({ security }) => security !== "tls").flatMap((corpus) => realityAliasShapes.map((shape) => ({ ...corpus, name: `${corpus.name}/${shape}`, shape }))))("updates existing Reality aliases without changing document shape: $name", (corpus) => {
    const golden = realityAliasGolden(corpus, corpus.shape);
    expect(patchXrayJSONConfiguration(golden.raw, { publicKey: "updated" })).toBe(formatXrayJSON(golden.expected));
  });

  it.each(["vless", "trojan"])("retains ordered unknown/duplicate URI parameters and explicit false semantics: %s", (protocol) => {
    const prefix = `${protocol}://11111111-1111-4111-8111-111111111111@edge.example:443?`;
    const raw = `${prefix}x-first=one&allowInsecure=1&x-other=two&allowinsecure=true&x-first=three&type=ws&allow_insecure=1&security=tls&path=%2Fold#Node`;
    expect(patchEditableConfiguration(raw, {})).toBe(raw);
    expect(patchEditableConfiguration(raw, { path: "/new" })).toBe(raw.replace("path=%2Fold", "path=%2Fnew"));
    expect(patchEditableConfiguration(raw, { allowInsecure: false })).toBe(`${prefix}x-first=one&x-other=two&x-first=three&type=ws&security=tls&path=%2Fold#Node`);
  });

  it("retains unrepresented VMess payload data and explicit zero/false values", () => {
    const payload = { v: "2", ps: "Node", add: "edge.example", port: "443", id: "11111111-1111-4111-8111-111111111111", net: "ws", path: "/old", custom: { future: [1, 2] }, aid: 3, allowInsecure: true };
    const raw = `vmess://${btoa(JSON.stringify(payload))}`;
    expect(patchEditableConfiguration(raw, {})).toBe(raw);
    const result = patchEditableConfiguration(raw, { vmessAlterId: "0", allowInsecure: false, path: "/new" });
    const expected: Record<string, unknown> = { ...payload, aid: "0", path: "/new" };
    delete expected.allowInsecure;
    expect(JSON.parse(atob(result.slice("vmess://".length)))).toEqual(expected);
  });

  it("rejects duplicate unknown JSON keys before any representable edit", () => {
    const raw = preservationCorpus[0].raw.replace('"future": true', '"future": true, "future": false');
    expect(() => patchXrayJSONConfiguration(raw, { server: "changed.example" })).toThrow(DuplicateJSONKeyError);
  });
});
