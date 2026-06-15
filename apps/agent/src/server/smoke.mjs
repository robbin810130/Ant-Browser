const baseUrl = process.env.AGENT_BASE_URL || "http://127.0.0.1:47831";
const serverBaseUrl = process.env.DESKTOP_SERVER_BASE_URL || "http://127.0.0.1:4174";

async function call(path, { method = "GET", body } = {}) {
  const response = await fetch(`${baseUrl}${path}`, {
    method,
    headers: {
      "content-type": "application/json"
    },
    body: body ? JSON.stringify(body) : undefined
  });

  const json = await response.json();
  if (!response.ok) {
    throw new Error(`${method} ${path} failed: ${JSON.stringify(json)}`);
  }
  return json;
}

async function serverLogin() {
  const response = await fetch(`${serverBaseUrl}/api/auth/login`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ username: "admin", password: "Admin@123" })
  });
  const json = await response.json();
  if (!response.ok) {
    throw new Error(`login failed: ${JSON.stringify(json)}`);
  }
  return json.data;
}

async function main() {
  const login = await serverLogin();

  const bootstrap = await call("/local/session/bootstrap", {
    method: "POST",
    body: {
      accessToken: login.accessToken,
      user: {
        userId: login.userSummary.id,
        displayName: login.userSummary.displayName
      },
      serverBaseUrl
    }
  });

  const localHealth = await call("/local/health");
  const deviceStatus = await call("/local/device/status");
  const shops = await call("/local/shops");
  const runs = await call("/local/runs?limit=5");
  const shopId = shops.data.items[0]?.shopId;
  let runStatus = null;

  if (shopId) {
    const open = await call(`/local/shops/${encodeURIComponent(shopId)}/open`, {
      method: "POST",
      body: { source: "desktop_shell" }
    });
    runStatus = await call(`/local/runs/${encodeURIComponent(open.data.runId)}`);
  }

  console.log(
    JSON.stringify(
      {
        ok: true,
        agentSessionId: bootstrap.data.agentSessionId,
        localHealthStatus: localHealth.data.status,
        deviceStatus: deviceStatus.data.deviceStatus,
        shopCount: shops.data.items.length,
        runStatus: runStatus?.data?.status || null,
        recentRunCount: runs.data.items.length
      },
      null,
      2
    )
  );
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
