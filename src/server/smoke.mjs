const baseUrl = process.env.ANT_RUNTIME_BASE_URL || "http://127.0.0.1:19876";

async function call(path, { method = "GET", body } = {}) {
  const res = await fetch(`${baseUrl}${path}`, {
    method,
    headers: {
      "content-type": "application/json"
    },
    body: body ? JSON.stringify(body) : undefined
  });
  const json = await res.json();
  if (!res.ok || json.ok === false) {
    throw new Error(`${method} ${path} failed: ${JSON.stringify(json)}`);
  }
  return json;
}

async function main() {
  const profileId = "alibaba:smoke-shop";
  await call("/api/local/profiles/upsert", {
    method: "POST",
    body: {
      profileId,
      shopId: "smoke-shop",
      platformCode: "alibaba",
      profileName: "Smoke Shop",
      managedMode: true,
      userDataDir: "/tmp/ant-smoke-shop"
    }
  });
  const launch = await call(`/api/local/profiles/${encodeURIComponent(profileId)}/launch`, {
    method: "POST",
    body: {
      headless: false
    }
  });
  const runtime = await call(`/api/local/profiles/${encodeURIComponent(profileId)}/runtime`);
  const closed = await call(`/api/local/profiles/${encodeURIComponent(profileId)}/close`, {
    method: "POST"
  });
  console.log(
    JSON.stringify(
      {
        ok: true,
        profileId,
        launch,
        runtime,
        closed
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
