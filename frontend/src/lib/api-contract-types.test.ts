import fs from "node:fs";
import path from "node:path";
import ts from "typescript";
import { expect, it } from "vitest";
import fixture from "../../../testdata/api/contracts.json";

function contractTypeSource() {
  const user = { ...fixture.cases["user-detail"].response.data };
  const { token: _redactedToken, ...typedUser } = user;
  const context = ts.createSourceFile("context.tsx", fs.readFileSync(path.resolve(__dirname, "../context/PanelSettingsContext.tsx"), "utf8"), ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const panel = context.statements.find((node) => ts.isInterfaceDeclaration(node) && node.name.text === "APIPanelSettings");
  if (!panel) throw new Error("Panel API shape is missing");
  const samples: [string, string, unknown][] = [
    ["user", "User", typedUser],
    ["userSummary", "UserSummary", fixture.cases["user-list"].response.data[0]],
    ["source", "SourceDetail", fixture.cases["source-detail"].response.data],
    ["sourceSummary", "SourceSummary", fixture.cases["source-list"].response.data[0]],
    ["settings", "SubscriptionSettings", fixture.cases.settings.response],
    ["key", "KeyProfileDetailResponse", fixture.cases["key-detail"].response.data],
    ["info", "KeyProfileDetailResponse", fixture.cases["key-info"].response.data],
    ["panel", "APIPanelSettings", fixture.cases["panel-settings"].response],
    ["error", "ErrorContract", fixture.cases["missing-user"].response],
  ];
  return [
    'import type { User, UserSummary, SourceDetail, SourceSummary, SubscriptionSettings, KeyProfileDetailResponse } from "./types";',
    'import type { ApiError, users } from "./api";',
    panel.getText(context),
    'type ErrorContract = Pick<ApiError, "code" | "message" | "field_errors" | "request_id"> & { error?: string };',
    ...samples.map(([name, type, value]) => `const ${name} = ${JSON.stringify(value)} satisfies ${type};`),
    ...["patch-omitted", "patch-null", "patch-empty", "patch-status-null"].map((name) => {
      const value = fixture.cases[name as "patch-omitted"].body;
      return `const ${name.replaceAll("-", "_")} = ${JSON.stringify(value)} satisfies Parameters<typeof users.updateSubscription>[1];`;
    }),
  ].join("\n");
}

it("shared actual-response fixtures satisfy the declared TypeScript boundaries", () => {
  const file = path.resolve(__dirname, "api-contract-generated.ts");
  const source = contractTypeSource();
  const host = ts.createCompilerHost({});
  const original = host.getSourceFile;
  const exists = host.fileExists;
  host.fileExists = (name) => path.resolve(name) === file || exists(name);
  host.getSourceFile = (name, language, error, fresh) => path.resolve(name) === file
    ? ts.createSourceFile(name, source, language, true)
    : original(name, language, error, fresh);
  const program = ts.createProgram([file], { strict: true, noEmit: true, skipLibCheck: true, target: ts.ScriptTarget.ES2021, module: ts.ModuleKind.ESNext, moduleResolution: ts.ModuleResolutionKind.Bundler, types: [] }, host);
  const diagnostics = ts.getPreEmitDiagnostics(program).map((item) => ts.flattenDiagnosticMessageText(item.messageText, "\n"));
  expect(diagnostics, diagnostics.join("\n")).toEqual([]);
});
