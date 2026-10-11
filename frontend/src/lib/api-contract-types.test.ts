import path from "node:path";
import ts from "typescript";
import { expect, it } from "vitest";
import fixture from "../../../testdata/api/contracts.json";

function contractTypeSource() {
  const user = { ...fixture.cases["user-detail"].response.data };
  const { token: redactedToken, ...typedUser } = user;
  if (redactedToken !== "") throw new Error("User fixture contains an unredacted token");
  const samples: [string, string, unknown][] = [
    ["user", "User", typedUser],
    ["userSummary", "UserSummary", fixture.cases["user-list"].response.data[0]],
    ["page", "PageMeta", fixture.cases["user-list"].response.meta],
    ["emptyUsers", "{ data: UserSummary[]; meta: PageMeta }", fixture.cases["users-empty"].response],
    ["emptySources", "{ data: SourceSummary[]; meta: PageMeta }", fixture.cases["sources-empty"].response],
    ["source", "SourceDetail", fixture.cases["source-detail"].response.data],
    ["sourceSummary", "SourceSummary", fixture.cases["source-list"].response.data[0]],
    ["settings", "SubscriptionSettings", fixture.cases.settings.response],
    ["key", "KeyProfileDetailResponse", fixture.cases["key-detail"].response.data],
    ["info", "KeyProfileDetailResponse", fixture.cases["key-info"].response.data],
    ["panel", "PanelSettingsResponse", fixture.cases["panel-settings"].response],
    ["error", "ErrorContract", fixture.cases["missing-user"].response],
  ];
  return [
    'import type { User, UserSummary, SourceDetail, SourceSummary, SubscriptionSettings, KeyProfileDetailResponse, PageMeta, PanelSettingsResponse } from "./types";',
    'import type { ApiError, users } from "./api";',
    'type ErrorContract = Pick<ApiError, "code" | "message" | "field_errors" | "request_id"> & { error?: string };',
    ...samples.map(([name, type, value]) => `const ${name} = ${JSON.stringify(value)} satisfies ${type};`),
    ...(["patch-omitted", "patch-null", "patch-empty", "patch-status-null", "patch-refresh-max", "patch-refresh-over-max"] as const).map((name) => {
      const value = fixture.cases[name].body;
      return `const ${name.replaceAll("-", "_")} = ${JSON.stringify(value)} satisfies Parameters<typeof users.updateSubscription>[1];`;
    }),
  ].join("\n");
}

function contractDiagnostics(source: string) {
  const file = path.resolve(__dirname, "api-contract-generated.ts");
  const host = ts.createCompilerHost({});
  const original = host.getSourceFile;
  const exists = host.fileExists;
  host.fileExists = (name) => path.resolve(name) === file || exists(name);
  host.getSourceFile = (name, language, error, fresh) => path.resolve(name) === file
    ? ts.createSourceFile(name, source, language, true)
    : original(name, language, error, fresh);
  const program = ts.createProgram([file], { strict: true, noEmit: true, skipLibCheck: true, target: ts.ScriptTarget.ES2021, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler, types: [] }, host);
  return ts.getPreEmitDiagnostics(program).map((item) => ts.flattenDiagnosticMessageText(item.messageText, "\n"));
}

it("shared actual-response fixtures satisfy the declared TypeScript boundaries", () => {
  const diagnostics = contractDiagnostics(contractTypeSource());
  expect(diagnostics, diagnostics.join("\n")).toEqual([]);
});

it("rejects an empty protocol in a real structured create declaration", () => {
  const diagnostics = contractDiagnostics('import type { CreateKeyProfileInput } from "./types"; const input = { label: "fixture", status: "active", kind: "real", creation_mode: "structured", protocol: "", structured: {} } satisfies CreateKeyProfileInput;');
  expect(diagnostics).not.toEqual([]);
});

it("allows the existing empty informational create protocol", () => {
  const diagnostics = contractDiagnostics('import type { CreateKeyProfileInput } from "./types"; const input = { label: "fixture", status: "active", kind: "informational", creation_mode: "raw", protocol: "", template_text: "info" } satisfies CreateKeyProfileInput;');
  expect(diagnostics, diagnostics.join("\n")).toEqual([]);
});
