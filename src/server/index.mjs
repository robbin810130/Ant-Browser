import crypto from "node:crypto";
import http from "node:http";

const ANT_LISTEN_HOST = process.env.ANT_LISTEN_HOST || "127.0.0.1";
const ANT_LISTEN_PORT = Number(process.env.ANT_LISTEN_PORT || 19876);
const ANT_RUNTIME_AUTH_TOKEN = process.env.ANT_RUNTIME_AUTH_TOKEN || "";
const ANT_RUNTIME_MODE = process.env.ANT_RUNTIME_MODE || "auto";
const ANT_UPSTREAM_BASE_URL = process.env.ANT_UPSTREAM_BASE_URL || "";
const ANT_UPSTREAM_TIMEOUT_MS = Number(process.env.ANT_UPSTREAM_TIMEOUT_MS || 8000);

const profiles = new Map();
const runtimes = new Map();
let lastResolvedMode = ANT_RUNTIME_MODE === "upstream" ? "upstream" : "mock";
let lastFallbackReason = null;

function sendJson(res, statusCode, payload) {
  res.writeHead(statusCode, { "content-type": "application/json; charset=utf-8" });
  res.end(JSON.stringify(payload));
}

function readErrorMessage(error) {
  if (error instanceof Error) {
    return error.message;
  }
  return "internal error";
}

async function readJsonBody(req) {
  const chunks = [];
  for await (const chunk of req) {
    chunks.push(chunk);
  }
  if (chunks.length === 0) {
    return {};
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8"));
  } catch {
    const error = new Error("invalid json body");
    error.statusCode = 400;
    throw error;
  }
}

function assertRequired(payload, field) {
  if (payload?.[field] === undefined || payload?.[field] === null || payload?.[field] === "") {
    const error = new Error(`missing field: ${field}`);
    error.statusCode = 400;
    throw error;
  }
}

function verifyManagedMode(payload) {
  if (payload.managedMode !== true) {
    const error = new Error("managedMode must be true");
    error.statusCode = 403;
    throw error;
  }
}

function verifyAuth(req) {
  if (!ANT_RUNTIME_AUTH_TOKEN) {
    return;
  }
  const authorization = req.headers.authorization || "";
  const token = authorization.startsWith("Bearer ") ? authorization.slice(7) : "";
  if (!token || token !== ANT_RUNTIME_AUTH_TOKEN) {
    const error = new Error("unauthorized");
    error.statusCode = 401;
    throw error;
  }
}

function normalizeUpstreamBaseUrl() {
  if (!ANT_UPSTREAM_BASE_URL) {
    return "";
  }
  return ANT_UPSTREAM_BASE_URL.endsWith("/")
    ? ANT_UPSTREAM_BASE_URL.slice(0, -1)
    : ANT_UPSTREAM_BASE_URL;
}

async function callUpstream(pathname, { method = "GET", body } = {}) {
  const baseUrl = normalizeUpstreamBaseUrl();
  if (!baseUrl) {
    const error = new Error("upstream base url is required in upstream mode");
    error.statusCode = 500;
    throw error;
  }

  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), ANT_UPSTREAM_TIMEOUT_MS);

  try {
    const response = await fetch(`${baseUrl}${pathname}`, {
      method,
      headers: {
        "content-type": "application/json",
        ...(ANT_RUNTIME_AUTH_TOKEN ? { authorization: `Bearer ${ANT_RUNTIME_AUTH_TOKEN}` } : {})
      },
      body: body ? JSON.stringify(body) : undefined,
      signal: controller.signal
    });

    let payload = null;
    try {
      payload = await response.json();
    } catch {
      payload = { ok: false, message: "invalid upstream response" };
    }

    if (!response.ok) {
      const error = new Error(payload?.message || `upstream error (${response.status})`);
      error.statusCode = response.status;
      throw error;
    }
    return payload;
  } catch (error) {
    const next = new Error(error?.message || "upstream request failed");
    next.statusCode = Number(error?.statusCode || 502);
    throw next;
  } finally {
    clearTimeout(timeout);
  }
}

function normalizeRuntimeMode(value) {
  if (value === "mock" || value === "upstream" || value === "auto") {
    return value;
  }
  return "auto";
}

const normalizedRuntimeMode = normalizeRuntimeMode(ANT_RUNTIME_MODE);

function canTryUpstream() {
  return Boolean(normalizeUpstreamBaseUrl());
}

async function callUpstreamIfEnabled(pathname, options = {}) {
  if (normalizedRuntimeMode === "mock") {
    lastResolvedMode = "mock";
    return null;
  }

  if (!canTryUpstream()) {
    if (normalizedRuntimeMode === "upstream") {
      const error = new Error("upstream base url is required in upstream mode");
      error.statusCode = 500;
      throw error;
    }
    lastResolvedMode = "mock";
    lastFallbackReason = "upstream base url is empty";
    return null;
  }

  try {
    const payload = await callUpstream(pathname, options);
    lastResolvedMode = "upstream";
    lastFallbackReason = null;
    return payload;
  } catch (error) {
    if (normalizedRuntimeMode === "upstream") {
      throw error;
    }
    lastResolvedMode = "mock";
    lastFallbackReason = readErrorMessage(error);
    return null;
  }
}

function upsertProfile(payload) {
  const now = new Date().toISOString();
  const current = profiles.get(payload.profileId);
  const next = {
    profileId: payload.profileId,
    shopId: payload.shopId,
    platformCode: payload.platformCode,
    profileName: payload.profileName,
    managedMode: true,
    userDataDir: payload.userDataDir,
    createdAt: current?.createdAt || now,
    updatedAt: now
  };
  profiles.set(payload.profileId, next);
  return {
    ok: true,
    profileId: payload.profileId,
    updated: Boolean(current)
  };
}

function launchProfile(profileId) {
  const profile = profiles.get(profileId);
  if (!profile) {
    const error = new Error("profile not found");
    error.statusCode = 404;
    throw error;
  }
  const now = new Date().toISOString();
  const runtime = {
    profileId,
    running: true,
    pid: Math.floor(10000 + Math.random() * 80000),
    debugPort: Math.floor(20000 + Math.random() * 20000),
    lastStartAt: now
  };
  runtimes.set(profileId, runtime);
  return {
    ok: true,
    profileId,
    pid: runtime.pid,
    debugPort: runtime.debugPort
  };
}

function getRuntime(profileId) {
  const runtime = runtimes.get(profileId);
  if (!runtime) {
    return {
      ok: true,
      profileId,
      running: false,
      pid: null,
      debugPort: null,
      lastStartAt: null
    };
  }
  return {
    ok: true,
    profileId,
    running: runtime.running,
    pid: runtime.pid,
    debugPort: runtime.debugPort,
    lastStartAt: runtime.lastStartAt
  };
}

function closeProfile(profileId) {
  const current = runtimes.get(profileId);
  runtimes.set(profileId, {
    profileId,
    running: false,
    pid: null,
    debugPort: null,
    lastStartAt: current?.lastStartAt || null
  });
  return {
    ok: true,
    profileId,
    closed: true
  };
}

function clearSession(profileId) {
  const profile = profiles.get(profileId);
  if (!profile) {
    const error = new Error("profile not found");
    error.statusCode = 404;
    throw error;
  }
  return {
    ok: true,
    profileId,
    cleared: true
  };
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url || "/", "http://127.0.0.1");
  const { pathname } = url;

  try {
    verifyAuth(req);

    if (req.method === "GET" && pathname === "/api/local/health") {
      const upstreamHealth = await callUpstreamIfEnabled("/api/local/health");
      if (upstreamHealth) {
        sendJson(res, 200, {
          ...upstreamHealth,
          mode: "upstream",
          modeRequested: normalizedRuntimeMode
        });
        return;
      }
      sendJson(res, 200, {
        ok: true,
        managedMode: true,
        mode: "mock",
        modeRequested: normalizedRuntimeMode,
        profileCount: profiles.size,
        runtimeCount: Array.from(runtimes.values()).filter((item) => item.running).length,
        ...(lastFallbackReason ? { fallbackReason: lastFallbackReason } : {})
      });
      return;
    }

    if (req.method === "POST" && pathname === "/api/local/profiles/upsert") {
      const payload = await readJsonBody(req);
      assertRequired(payload, "profileId");
      assertRequired(payload, "shopId");
      assertRequired(payload, "platformCode");
      assertRequired(payload, "profileName");
      assertRequired(payload, "userDataDir");
      verifyManagedMode(payload);
      const upstreamPayload = await callUpstreamIfEnabled("/api/local/profiles/upsert", {
        method: "POST",
        body: payload
      });
      if (upstreamPayload) {
        sendJson(res, 200, upstreamPayload);
        return;
      }
      sendJson(res, 200, upsertProfile(payload));
      return;
    }

    const launchMatch = pathname.match(/^\/api\/local\/profiles\/([^/]+)\/launch$/);
    if (req.method === "POST" && launchMatch) {
      const profileId = decodeURIComponent(launchMatch[1]);
      const payload = await readJsonBody(req);
      const upstreamPayload = await callUpstreamIfEnabled(
        `/api/local/profiles/${encodeURIComponent(profileId)}/launch`,
        {
          method: "POST",
          body: payload
        }
      );
      if (upstreamPayload) {
        sendJson(res, 200, upstreamPayload);
        return;
      }
      sendJson(res, 200, launchProfile(profileId));
      return;
    }

    const runtimeMatch = pathname.match(/^\/api\/local\/profiles\/([^/]+)\/runtime$/);
    if (req.method === "GET" && runtimeMatch) {
      const profileId = decodeURIComponent(runtimeMatch[1]);
      const upstreamPayload = await callUpstreamIfEnabled(
        `/api/local/profiles/${encodeURIComponent(profileId)}/runtime`
      );
      if (upstreamPayload) {
        sendJson(res, 200, upstreamPayload);
        return;
      }
      sendJson(res, 200, getRuntime(profileId));
      return;
    }

    const closeMatch = pathname.match(/^\/api\/local\/profiles\/([^/]+)\/close$/);
    if (req.method === "POST" && closeMatch) {
      const profileId = decodeURIComponent(closeMatch[1]);
      const upstreamPayload = await callUpstreamIfEnabled(
        `/api/local/profiles/${encodeURIComponent(profileId)}/close`,
        {
          method: "POST"
        }
      );
      if (upstreamPayload) {
        sendJson(res, 200, upstreamPayload);
        return;
      }
      sendJson(res, 200, closeProfile(profileId));
      return;
    }

    const clearMatch = pathname.match(/^\/api\/local\/profiles\/([^/]+)\/clear-session$/);
    if (req.method === "POST" && clearMatch) {
      const profileId = decodeURIComponent(clearMatch[1]);
      const payload = await readJsonBody(req);
      assertRequired(payload, "clearCookies");
      assertRequired(payload, "clearStorage");
      const upstreamPayload = await callUpstreamIfEnabled(
        `/api/local/profiles/${encodeURIComponent(profileId)}/clear-session`,
        {
          method: "POST",
          body: payload
        }
      );
      if (upstreamPayload) {
        sendJson(res, 200, upstreamPayload);
        return;
      }
      sendJson(res, 200, clearSession(profileId));
      return;
    }

    sendJson(res, 404, { ok: false, message: "not found" });
  } catch (error) {
    sendJson(res, Number(error?.statusCode || 500), {
      ok: false,
      message: readErrorMessage(error)
    });
  }
});

server.listen(ANT_LISTEN_PORT, ANT_LISTEN_HOST, () => {
  console.log("[ant-runtime] listening", {
    host: ANT_LISTEN_HOST,
    port: ANT_LISTEN_PORT,
    managedMode: true,
    mode: normalizedRuntimeMode,
    upstreamBaseUrl: ANT_UPSTREAM_BASE_URL || null
  });
});
