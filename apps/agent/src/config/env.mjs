export const AGENT_LISTEN_HOST = process.env.AGENT_LISTEN_HOST || "127.0.0.1";
export const AGENT_LISTEN_PORT = Number(process.env.AGENT_LISTEN_PORT || 47831);

export const DESKTOP_SERVER_BASE_URL =
  process.env.DESKTOP_SERVER_BASE_URL || "http://127.0.0.1:4174";
export const AGENT_CLIENT_VERSION = process.env.AGENT_CLIENT_VERSION || "0.1.0";
export const AGENT_PLATFORM = process.env.AGENT_PLATFORM || "windows";

export const AGENT_STATE_DIR =
  process.env.AGENT_STATE_DIR || `${process.cwd()}/.local-agent-state`;
export const AGENT_RUN_TTL_DAYS = Number(process.env.AGENT_RUN_TTL_DAYS || 7);

export const ANT_RUNTIME_BASE_URL = process.env.ANT_RUNTIME_BASE_URL || "http://127.0.0.1:19876";
export const ANT_RUNTIME_AUTH_TOKEN = process.env.ANT_RUNTIME_AUTH_TOKEN || "";
