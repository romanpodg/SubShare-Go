import { spawn } from "node:child_process";
import { once } from "node:events";
import { resolve } from "node:path";
import { startStaticServer } from "./static-server.mjs";

const server = startStaticServer();
await once(server, "listening");

const cli = resolve("node_modules/@playwright/test/cli.js");
const runner = spawn(process.execPath, [cli, "test", ...process.argv.slice(2)], {
  env: { ...process.env, E2E_EXTERNAL_SERVER: "1" },
  stdio: "inherit",
});

const [code, signal] = await once(runner, "exit");
server.close();
server.closeAllConnections();

if (signal) {
  process.kill(process.pid, signal);
} else {
  process.exitCode = code ?? 1;
}
