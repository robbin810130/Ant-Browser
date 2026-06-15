import {
  AGENT_CLIENT_VERSION,
  AGENT_PLATFORM,
  ANT_RUNTIME_AUTH_TOKEN,
  ANT_RUNTIME_BASE_URL,
  DESKTOP_SERVER_BASE_URL
} from "../config/env.mjs";
import { gzipSync } from "node:zlib";

function buildHeaders(accessToken) {
  return {
    "content-type": "application/json",
    authorization: `Bearer ${accessToken}`
  };
}

function buildJsonBody(payload, { gzip = false } = {}) {
  const json = JSON.stringify(payload);
  return gzip ? gzipSync(Buffer.from(json, "utf8")) : json;
}

function splitString(value, chunkSize) {
  const chunks = [];
  for (let index = 0; index < value.length; index += chunkSize) {
    chunks.push(value.slice(index, index + chunkSize));
  }
  return chunks.length ? chunks : [""];
}

function buildAntHeaders() {
  return {
    "content-type": "application/json",
    ...(ANT_RUNTIME_AUTH_TOKEN ? { authorization: `Bearer ${ANT_RUNTIME_AUTH_TOKEN}` } : {})
  };
}

async function readResponse(response) {
  let payload = null;
  try {
    payload = await response.json();
  } catch {
    payload = { message: "invalid server response" };
  }

  if (!response.ok) {
    const message = payload?.message || `server request failed (${response.status})`;
    throw new Error(message);
  }

  return payload;
}

function describeFetchError(error) {
  const message = error instanceof Error ? error.message : String(error);
  const cause = error?.cause instanceof Error ? `; cause=${error.cause.message}` : "";
  return `${message}${cause}`;
}

async function fetchAntRuntime(path, options = {}) {
  const url = `${ANT_RUNTIME_BASE_URL}${path}`;
  const attempts = Number(options.attempts || 3);
  const init = { ...options };
  delete init.attempts;

  let lastError = null;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      return await fetch(url, init);
    } catch (error) {
      lastError = error;
      if (attempt < attempts) {
        await new Promise((resolve) => setTimeout(resolve, attempt * 500));
      }
    }
  }

  throw new Error(`Ant Runtime request failed: ${url}; ${describeFetchError(lastError)}`);
}

async function fetchWithRetry(url, options = {}) {
  const attempts = Number(options.attempts || 3);
  const init = { ...options };
  delete init.attempts;

  let lastError = null;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    try {
      return await fetch(url, init);
    } catch (error) {
      lastError = error;
      if (attempt < attempts) {
        await new Promise((resolve) => setTimeout(resolve, attempt * 500));
      }
    }
  }

  throw new Error(`Request failed: ${url}; ${describeFetchError(lastError)}`);
}

export async function bootstrapDesktopSession({ accessToken, serverBaseUrl }) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/session/bootstrap`, {
    method: "POST",
    headers: buildHeaders(accessToken),
    body: JSON.stringify({
      clientType: "desktop",
      clientVersion: AGENT_CLIENT_VERSION,
      platform: AGENT_PLATFORM
    })
  });

  return readResponse(response);
}

export async function registerDesktopDevice({
  accessToken,
  serverBaseUrl,
  deviceFingerprint,
  deviceName
}) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/devices/register`, {
    method: "POST",
    headers: buildHeaders(accessToken),
    body: JSON.stringify({
      deviceFingerprint,
      deviceName,
      platform: AGENT_PLATFORM,
      clientVersion: AGENT_CLIENT_VERSION
    })
  });

  return readResponse(response);
}

export async function heartbeatDesktopDevice({
  accessToken,
  serverBaseUrl,
  deviceId,
  antRuntimeReachable = false
}) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/devices/${encodeURIComponent(deviceId)}/heartbeat`,
    {
      method: "POST",
      headers: buildHeaders(accessToken),
      body: JSON.stringify({
        clientVersion: AGENT_CLIENT_VERSION,
        agentVersion: "0.1.0",
        antRuntimeVersion: "0.1.0",
        health: {
          agentStatus: "ready",
          antRuntimeReachable
        }
      })
    }
  );

  return readResponse(response);
}

export async function registerDeviceBridgeAgent({
  accessToken,
  serverBaseUrl,
  deviceFingerprint,
  deviceName,
  platform = AGENT_PLATFORM,
  agentVersion = AGENT_CLIENT_VERSION,
  browserwingVersion = ""
}) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/device-bridge/register`, {
    method: "POST",
    headers: buildHeaders(accessToken),
    body: JSON.stringify({
      deviceFingerprint,
      deviceName,
      platform,
      agentVersion,
      browserwingVersion
    })
  });
  return readResponse(response);
}

export async function heartbeatDeviceBridgeAgent({
  accessToken,
  serverBaseUrl,
  deviceFingerprint,
  deviceName,
  platform = AGENT_PLATFORM,
  agentVersion = AGENT_CLIENT_VERSION,
  browserwingVersion = ""
}) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/device-bridge/heartbeat`, {
    method: "POST",
    headers: buildHeaders(accessToken),
    body: JSON.stringify({
      deviceFingerprint,
      deviceName,
      platform,
      agentVersion,
      browserwingVersion
    })
  });
  return readResponse(response);
}

export async function pullDeviceBridgeTasks({ accessToken, serverBaseUrl, deviceFingerprint, limit = 3 }) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/device-bridge/tasks/pull`, {
    method: "POST",
    headers: buildHeaders(accessToken),
    body: JSON.stringify({
      deviceFingerprint,
      limit
    })
  });
  return readResponse(response);
}

export async function reportDeviceBridgeTask({
  accessToken,
  serverBaseUrl,
  taskId,
  status,
  message = "",
  errorMessage = "",
  failureCode = "",
  result = {},
  attempts = 3
}) {
  const response = await fetchWithRetry(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/device-bridge/tasks/${encodeURIComponent(taskId)}/report`,
    {
      method: "POST",
      headers: {
        ...buildHeaders(accessToken),
        "content-encoding": "gzip"
      },
      body: buildJsonBody({
        status,
        message,
        errorMessage,
        failureCode,
        result
      }, { gzip: true }),
      attempts
    }
  );
  return readResponse(response);
}

export async function uploadDeviceBridgeTaskSessionBundle({
  accessToken,
  serverBaseUrl,
  taskId,
  sessionBundle,
  chunkSize = 24 * 1024
}) {
  const compressed = gzipSync(Buffer.from(JSON.stringify(sessionBundle), "utf8"));
  const chunks = splitString(compressed.toString("base64"), chunkSize);
  let uploadId = "";
  let lastPayload = null;

  for (let chunkIndex = 0; chunkIndex < chunks.length; chunkIndex += 1) {
    const response = await fetchWithRetry(
      `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/device-bridge/tasks/${encodeURIComponent(taskId)}/session-bundle-chunks`,
      {
        method: "POST",
        headers: buildHeaders(accessToken),
        body: JSON.stringify({
          uploadId,
          chunkIndex,
          chunkCount: chunks.length,
          data: chunks[chunkIndex]
        }),
        attempts: 5
      }
    );
    lastPayload = await readResponse(response);
    uploadId = lastPayload?.data?.uploadId || uploadId;
  }

  if (!uploadId || !lastPayload?.data?.complete) {
    throw new Error("共享会话分片上传未完成");
  }

  return {
    uploadId,
    chunkCount: chunks.length,
    compressedBytes: compressed.byteLength
  };
}

export async function listDesktopShops({ accessToken, serverBaseUrl }) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shops`, {
    method: "GET",
    headers: buildHeaders(accessToken)
  });

  return readResponse(response);
}

export async function listDesktopShopProfiles({ accessToken, serverBaseUrl }) {
  const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shop-profiles`, {
    method: "GET",
    headers: buildHeaders(accessToken)
  });

  return readResponse(response);
}

export async function requestOpenShop({ accessToken, serverBaseUrl, shopId, deviceId }) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shops/${encodeURIComponent(shopId)}/open`,
    {
      method: "POST",
      headers: buildHeaders(accessToken),
      body: JSON.stringify({
        deviceId,
        clientVersion: AGENT_CLIENT_VERSION
      })
    }
  );

  return readResponse(response);
}

export async function requestDesktopSharedLoginBind({ accessToken, serverBaseUrl, shopId }) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shops/${encodeURIComponent(shopId)}/bind`,
    {
      method: "POST",
      headers: buildHeaders(accessToken)
    }
  );
  return readResponse(response);
}

export async function requestDesktopSharedLoginValidate({ accessToken, serverBaseUrl, shopId }) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shops/${encodeURIComponent(shopId)}/validate`,
    {
      method: "POST",
      headers: buildHeaders(accessToken)
    }
  );
  return readResponse(response);
}

export async function getDesktopSharedLoginBindSession({ accessToken, serverBaseUrl, bindSessionId }) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/shared-login-bind-sessions/${encodeURIComponent(bindSessionId)}`,
    {
      method: "GET",
      headers: buildHeaders(accessToken)
    }
  );
  return readResponse(response);
}

export async function reportOpenShopResult({
  accessToken,
  serverBaseUrl,
  openRequestId,
  deviceId,
  status,
  runtime,
  failureCode,
  failureMessage
}) {
  const response = await fetch(
    `${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/desktop/open-requests/${encodeURIComponent(openRequestId)}/report`,
    {
      method: "POST",
      headers: buildHeaders(accessToken),
      body: JSON.stringify({
        deviceId,
        status,
        runtime,
        failureCode,
        failureMessage
      })
    }
  );

  return readResponse(response);
}

export async function checkServerReachable(serverBaseUrl) {
  try {
    const response = await fetch(`${serverBaseUrl || DESKTOP_SERVER_BASE_URL}/api/health`, {
      method: "GET"
    });
    return response.ok;
  } catch {
    return false;
  }
}

export async function checkAntRuntimeReachable(baseUrl = ANT_RUNTIME_BASE_URL) {
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

export async function upsertAntProfile({
  profileId,
  shopId,
  platformCode,
  profileName,
  userDataDir,
  managedMode = true
}) {
  const response = await fetchAntRuntime("/api/local/profiles/upsert", {
    method: "POST",
    headers: buildAntHeaders(),
    body: JSON.stringify({
      profileId,
      shopId,
      platformCode,
      profileName,
      managedMode,
      userDataDir
    })
  });
  return readResponse(response);
}

export async function launchAntProfile({ profileId, headless = false, targetUrl = null, sessionBundle = null }) {
  const response = await fetchAntRuntime(`/api/local/profiles/${encodeURIComponent(profileId)}/launch`, {
    method: "POST",
    headers: buildAntHeaders(),
    body: JSON.stringify({
      headless,
      targetUrl,
      sessionBundle
    })
  });
  return readResponse(response);
}

export async function closeAntProfile({ profileId }) {
  const response = await fetchAntRuntime(`/api/local/profiles/${encodeURIComponent(profileId)}/close`, {
    method: "POST",
    headers: buildAntHeaders(),
    body: JSON.stringify({})
  });
  return readResponse(response);
}

export async function navigateAntProfile({ profileId, targetUrl }) {
  const response = await fetchAntRuntime(`/api/local/profiles/${encodeURIComponent(profileId)}/navigate`, {
    method: "POST",
    headers: buildAntHeaders(),
    body: JSON.stringify({ targetUrl })
  });
  return readResponse(response);
}

export async function getAntProfileRuntime({ profileId }) {
  const response = await fetchAntRuntime(`/api/local/profiles/${encodeURIComponent(profileId)}/runtime`, {
    method: "GET",
    headers: buildAntHeaders()
  });
  return readResponse(response);
}

export async function captureAntProfileSessionBundle({ profileId, platformCode, captureStartedAt }) {
  const response = await fetchAntRuntime(`/api/local/profiles/${encodeURIComponent(profileId)}/session-bundle`, {
    method: "POST",
    headers: buildAntHeaders(),
    body: JSON.stringify({ platformCode, captureStartedAt }),
    attempts: 4
  });
  return readResponse(response);
}
