// Analyze the real Next.js static export; never rebuild or pass tokens via argv.
const { createRequire } = require("node:module");
const { resolve } = require("node:path");
const { mkdirSync, writeFileSync } = require("node:fs");

async function main() {
  const dryRun = process.argv.includes("--dry-run");
  if (!process.env.CODECOV_BUNDLE_ANALYZER_DIR) {
    throw new Error("Set CODECOV_BUNDLE_ANALYZER_DIR to the isolated tool installation.");
  }
  if (!dryRun && !process.env.CODECOV_TOKEN) {
    throw new Error("CODECOV_TOKEN is required for bundle uploads.");
  }
  const toolRequire = createRequire(resolve(process.env.CODECOV_BUNDLE_ANALYZER_DIR, "package.json"));
  const { createAndUploadReport } = toolRequire("@codecov/bundle-analyzer");
  const report = await createAndUploadReport(
    [resolve(__dirname, "../frontend/out/_next/static")],
    {
      bundleName: "subshare-frontend",
      enableBundleAnalysis: true,
      uploadToken: dryRun ? undefined : process.env.CODECOV_TOKEN,
      dryRun,
      debug: false,
      telemetry: false,
      retryCount: 3,
    },
    {
      ignorePatterns: ["*.map"],
      beforeReportUpload: async (output) => {
        if (!output.assets?.some((asset) => /\.(js|css)$/.test(asset.name))) {
          throw new Error("No production JavaScript or CSS assets found.");
        }
        return output;
      },
    },
  );
  if (dryRun) {
    const directory = resolve(__dirname, "../.cache");
    mkdirSync(directory, { recursive: true });
    writeFileSync(resolve(directory, "bundle-report.json"), report);
  }
  console.log(dryRun ? "Bundle report generated (no upload)." : "Bundle analysis uploaded.");
}

main().catch(() => {
  // Avoid dumping SDK errors or request objects that could contain credentials.
  console.error("Bundle analysis failed; check the build output, tool installation and Codecov authentication.");
  process.exitCode = 1;
});
