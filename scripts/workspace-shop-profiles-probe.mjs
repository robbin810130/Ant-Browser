#!/usr/bin/env node

const agentBaseUrl = process.env.ANT_BROWSER_WORKSPACE_AGENT_BASE_URL || "http://127.0.0.1:47831";
const serverBaseUrl = process.env.ANT_BROWSER_WORKSPACE_SERVER_ORIGIN || "http://127.0.0.1:4174";
const accessToken = process.env.DESKTOP_ACCESS_TOKEN || process.env.ANT_BROWSER_DESKTOP_ACCESS_TOKEN || "";

async function readJson(response) {
  let payload = null;
  try {
    payload = await response.json();
  } catch {
    payload = { message: "invalid json response" };
  }
  return payload;
}

async function probeJson(name, url, options = {}) {
  const startedAt = Date.now();
  try {
    const response = await fetch(url, options);
    const payload = await readJson(response);
    return {
      name,
      ok: response.ok,
      status: response.status,
      durationMs: Date.now() - startedAt,
      payload
    };
  } catch (error) {
    return {
      name,
      ok: false,
      status: 0,
      durationMs: Date.now() - startedAt,
      error: error instanceof Error ? error.message : String(error)
    };
  }
}

function extractItems(result) {
  const data = result.payload?.data || result.payload;
  return Array.isArray(data?.items) ? data.items : [];
}

function summarizeProfiles(result) {
  const items = extractItems(result);
  const sourceCounts = new Map();
  for (const item of items) {
    const source = String(item?.source || "missing");
    sourceCounts.set(source, (sourceCounts.get(source) || 0) + 1);
  }
  return {
    count: items.length,
    sourceCounts: Object.fromEntries(sourceCounts),
    first: items.slice(0, 5).map((item) => ({
      shopId: item.shopId,
      shopName: item.shopName,
      asmShopId: item.asmShopId,
      shopCode: item.shopCode,
      fullShopName: item.fullShopName,
      source: item.source,
      lastSyncedAt: item.lastSyncedAt
    }))
  };
}

function printResult(result) {
  console.log(`\n[${result.name}] ${result.ok ? "OK" : "FAIL"} status=${result.status} duration=${result.durationMs}ms`);
  if (result.error) {
    console.log(`error=${result.error}`);
    return;
  }
  const summary = summarizeProfiles(result);
  console.log(JSON.stringify(summary, null, 2));
  if (!result.ok) {
    console.log(JSON.stringify(result.payload, null, 2));
  }
}

const agentResult = await probeJson("agent /local/shop-profiles", `${agentBaseUrl}/local/shop-profiles`);
printResult(agentResult);

if (accessToken) {
  const serverResult = await probeJson("server /api/desktop/shop-profiles", `${serverBaseUrl}/api/desktop/shop-profiles`, {
    headers: {
      authorization: `Bearer ${accessToken}`
    }
  });
  printResult(serverResult);
} else {
  console.log("\n[server /api/desktop/shop-profiles] SKIP missing DESKTOP_ACCESS_TOKEN");
}

const agentItems = extractItems(agentResult);
const hasNonAsmSource = agentItems.some((item) => item?.source && item.source !== "asm");
if (!agentResult.ok || hasNonAsmSource) {
  process.exitCode = 1;
}
