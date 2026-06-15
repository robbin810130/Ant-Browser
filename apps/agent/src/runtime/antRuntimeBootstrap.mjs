import { spawn } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { ANT_RUNTIME_AUTH_TOKEN, ANT_RUNTIME_BASE_URL } from "../config/env.mjs";

let managedBridgeProcess = null;

function buildAntHeaders() {
  return {
    ...(ANT_RUNTIME_AUTH_TOKEN ? { authorization: `Bearer ${ANT_RUNTIME_AUTH_TOKEN}` } : {})
  };
}

async function isRuntimeReachable(baseUrl = ANT_RUNTIME_BASE_URL) {
  try {
    const response = await fetch(`${baseUrl}/api/local/health`, {
      method: "GET",
      headers: buildAntHeaders()
    });
    return response.ok;
  } catch {
    return false;
  }
}

function isLoopbackRuntime(baseUrl) {
  try {
    const parsed = new URL(baseUrl);
    return parsed.hostname === "127.0.0.1" || parsed.hostname === "localhost";
  } catch {
    return false;
  }
}

function resolveBridgeEntry() {
  const fromEnv = (process.env.ANT_RUNTIME_BRIDGE_ENTRY || "").trim();
  const candidates = [
    fromEnv,
    path.resolve(process.cwd(), "installer/windows/scripts/ant-runtime-bridge.mjs")
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) {
      return candidate;
    }
  }

  return null;
}

async function waitForReachable(baseUrl, timeoutMs = 8000) {
  const startedAt = Date.now();
  while (Date.now() - startedAt < timeoutMs) {
    if (await isRuntimeReachable(baseUrl)) {
      return true;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  return false;
}

export async function ensureManagedAntRuntime() {
  if (await isRuntimeReachable()) {
    return {
      ok: true,
      started: false,
      mode: "reachable"
    };
  }

  if (String(process.env.ANT_RUNTIME_AUTO_BOOT || "false").trim().toLowerCase() !== "true") {
    return {
      ok: false,
      started: false,
      mode: "disabled",
      reason: "auto_boot_disabled"
    };
  }

  if (!isLoopbackRuntime(ANT_RUNTIME_BASE_URL)) {
    return {
      ok: false,
      started: false,
      mode: "external",
      reason: "runtime_not_loopback"
    };
  }

  if (managedBridgeProcess && !managedBridgeProcess.killed) {
    const reachable = await waitForReachable(ANT_RUNTIME_BASE_URL, 3000);
    return {
      ok: reachable,
      started: false,
      mode: "managed",
      reason: reachable ? null : "existing_bridge_unhealthy"
    };
  }

  const bridgeEntry = resolveBridgeEntry();
  if (!bridgeEntry) {
    return {
      ok: false,
      started: false,
      mode: "missing",
      reason: "bridge_entry_not_found"
    };
  }

  const parsed = new URL(ANT_RUNTIME_BASE_URL);
  managedBridgeProcess = spawn(process.execPath, [bridgeEntry], {
    cwd: process.cwd(),
    stdio: "inherit",
    env: {
      ...process.env,
      ANT_LISTEN_HOST: parsed.hostname,
      ANT_LISTEN_PORT: String(parsed.port || 80)
    }
  });

  const reachable = await waitForReachable(ANT_RUNTIME_BASE_URL);
  return {
    ok: reachable,
    started: true,
    mode: "managed",
    pid: managedBridgeProcess.pid || null,
    bridgeEntry,
    reason: reachable ? null : "bridge_boot_timeout"
  };
}

export function stopManagedAntRuntime() {
  if (managedBridgeProcess && !managedBridgeProcess.killed) {
    managedBridgeProcess.kill("SIGTERM");
  }
  managedBridgeProcess = null;
}
