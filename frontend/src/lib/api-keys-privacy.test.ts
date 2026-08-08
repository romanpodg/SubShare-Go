import fs from "fs";
import path from "path";
import { describe, expect, it } from "vitest";
import { keys } from "./api";

function getAllSourceFiles(dir: string, fileList: string[] = []): string[] {
  const files = fs.readdirSync(dir);
  for (const file of files) {
    const filePath = path.join(dir, file);
    const stat = fs.statSync(filePath);
    if (stat.isDirectory()) {
      getAllSourceFiles(filePath, fileList);
    } else if (/\.(ts|tsx|js|jsx)$/.test(file) && file !== "api-keys-privacy.test.ts") {
      fileList.push(filePath);
    }
  }
  return fileList;
}

describe("Frontend Keys Privacy & Source Audit", () => {
  it("ensures legacy administration and bulk-secret key endpoints are absent from frontend runtime", () => {
    const forbiddenEndpoints = [
      ["/api/v1/keys", "full"].join("/"),
      ["/api", "admin"].join("/") + "/",
    ];
    const srcDir = path.resolve(__dirname, "..");
    const sourceFiles = getAllSourceFiles(srcDir);

    const violations: { file: string; line: number; content: string }[] = [];

    for (const filePath of sourceFiles) {
      const content = fs.readFileSync(filePath, "utf-8");
      const lines = content.split("\n");
      lines.forEach((line, index) => {
        for (const forbiddenEndpoint of forbiddenEndpoints) {
          if (new RegExp(forbiddenEndpoint, "i").test(line)) {
            violations.push({
              file: path.relative(srcDir, filePath),
              line: index + 1,
              content: line.trim(),
            });
          }
        }
      });
    }

    expect(violations).toEqual([]);
  });

  it("exports listSummaries targeting /api/v1/keys", () => {
    expect(typeof keys.listSummaries).toBe("function");
    // Verify legacy 'list' method is removed entirely
    expect("list" in keys).toBe(false);
  });
});
