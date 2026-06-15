import http from "node:http";
import { AGENT_LISTEN_HOST, AGENT_LISTEN_PORT } from "../config/env.mjs";
import { handleLocalRequest } from "../routes/index.mjs";
import { ensureManagedAntRuntime, stopManagedAntRuntime } from "../runtime/antRuntimeBootstrap.mjs";

const antRuntime = await ensureManagedAntRuntime();
if (!antRuntime.ok) {
  console.warn("[agent] ant runtime bootstrap not ready", antRuntime);
} else if (antRuntime.started) {
  console.log("[agent] managed ant runtime started", antRuntime);
}

const server = http.createServer(async (req, res) => {
  await handleLocalRequest(req, res);
});

const gracefulShutdown = () => {
  server.close(() => {
    stopManagedAntRuntime();
    process.exit(0);
  });
};

process.on("SIGINT", gracefulShutdown);
process.on("SIGTERM", gracefulShutdown);

server.listen(AGENT_LISTEN_PORT, AGENT_LISTEN_HOST, () => {
  console.log(`[agent] listening on http://${AGENT_LISTEN_HOST}:${AGENT_LISTEN_PORT}`);
});
