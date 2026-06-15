import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { AGENT_RUN_TTL_DAYS, AGENT_STATE_DIR } from "../config/env.mjs";

const RUNS_FILE_PATH = path.join(AGENT_STATE_DIR, "runs.json");
const RUN_EVENTS_FILE_PATH = path.join(AGENT_STATE_DIR, "run-events.json");
const MAX_PERSISTED_RUNS = 500;
const MAX_EVENTS_PER_RUN = 100;
const RUN_TTL_MS = Math.max(1, AGENT_RUN_TTL_DAYS) * 24 * 60 * 60 * 1000;

function ensureStateDir() {
  fs.mkdirSync(AGENT_STATE_DIR, { recursive: true });
}

function loadPersistedRuns() {
  ensureStateDir();

  if (!fs.existsSync(RUNS_FILE_PATH)) {
    return new Map();
  }

  try {
    const text = fs.readFileSync(RUNS_FILE_PATH, "utf8");
    const rows = JSON.parse(text);
    if (!Array.isArray(rows)) {
      return new Map();
    }

    const map = new Map();
    for (const run of rows) {
      if (run && typeof run.runId === "string" && run.runId) {
        map.set(run.runId, run);
      }
    }
    return map;
  } catch {
    return new Map();
  }
}

function loadPersistedRunEvents() {
  ensureStateDir();

  if (!fs.existsSync(RUN_EVENTS_FILE_PATH)) {
    return new Map();
  }

  try {
    const text = fs.readFileSync(RUN_EVENTS_FILE_PATH, "utf8");
    const rows = JSON.parse(text);
    if (!rows || typeof rows !== "object") {
      return new Map();
    }

    const map = new Map();
    for (const [runId, events] of Object.entries(rows)) {
      if (typeof runId === "string" && Array.isArray(events)) {
        map.set(
          runId,
          events.filter((event) => event && typeof event.eventId === "string").slice(0, MAX_EVENTS_PER_RUN)
        );
      }
    }
    return map;
  } catch {
    return new Map();
  }
}

function persistRuns(runsMap) {
  ensureStateDir();

  const rows = Array.from(runsMap.values())
    .filter((run) => Date.now() - Date.parse(run.startedAt || 0) <= RUN_TTL_MS)
    .sort((a, b) => Date.parse(b.startedAt || 0) - Date.parse(a.startedAt || 0))
    .slice(0, MAX_PERSISTED_RUNS);

  const tmpFile = `${RUNS_FILE_PATH}.tmp`;
  fs.writeFileSync(tmpFile, JSON.stringify(rows, null, 2), "utf8");
  fs.renameSync(tmpFile, RUNS_FILE_PATH);
}

function persistRunEvents(eventsMap, runsMap) {
  ensureStateDir();

  const validRunIds = new Set(Array.from(runsMap.keys()));
  const payload = {};
  for (const [runId, events] of eventsMap.entries()) {
    if (!validRunIds.has(runId)) {
      continue;
    }
    payload[runId] = (events || []).slice(-MAX_EVENTS_PER_RUN);
  }

  const tmpFile = `${RUN_EVENTS_FILE_PATH}.tmp`;
  fs.writeFileSync(tmpFile, JSON.stringify(payload, null, 2), "utf8");
  fs.renameSync(tmpFile, RUN_EVENTS_FILE_PATH);
}

function pruneExpiredRuns(runsMap) {
  for (const [runId, run] of runsMap.entries()) {
    const startedAt = Date.parse(run?.startedAt || 0);
    if (!Number.isFinite(startedAt) || Date.now() - startedAt > RUN_TTL_MS) {
      runsMap.delete(runId);
    }
  }
}

function pruneEventsForMissingRuns(runsMap, eventsMap) {
  const runIds = new Set(Array.from(runsMap.keys()));
  for (const runId of eventsMap.keys()) {
    if (!runIds.has(runId)) {
      eventsMap.delete(runId);
    }
  }
}

const state = {
  session: null,
  device: null,
  shops: {
    items: [],
    syncedAt: null
  },
  runs: loadPersistedRuns(),
  runEvents: loadPersistedRunEvents()
};

pruneExpiredRuns(state.runs);
persistRuns(state.runs);
pruneEventsForMissingRuns(state.runs, state.runEvents);
persistRunEvents(state.runEvents, state.runs);

export function createAgentSessionId() {
  return `agent-session-${crypto.randomUUID()}`;
}

export function getState() {
  return state;
}

export function setSession(session) {
  state.session = session;
}

export function setDevice(device) {
  state.device = device;
}

export function setShops(items) {
  state.shops = {
    items,
    syncedAt: new Date().toISOString()
  };
}

export function upsertRun(run) {
  pruneExpiredRuns(state.runs);
  state.runs.set(run.runId, run);
  pruneEventsForMissingRuns(state.runs, state.runEvents);
  persistRuns(state.runs);
  persistRunEvents(state.runEvents, state.runs);
  return run;
}

export function getRun(runId) {
  return state.runs.get(runId) || null;
}

export function listRuns(options = {}) {
  const limit = Number(options.limit || 20);
  const status = options.status ? String(options.status) : "";
  const shopId = options.shopId ? String(options.shopId) : "";
  const failureCode = options.failureCode ? String(options.failureCode) : "";

  pruneExpiredRuns(state.runs);
  pruneEventsForMissingRuns(state.runs, state.runEvents);
  persistRuns(state.runs);
  persistRunEvents(state.runEvents, state.runs);
  let rows = Array.from(state.runs.values());
  if (status) {
    rows = rows.filter((run) => run.status === status);
  }
  if (shopId) {
    rows = rows.filter((run) => run.shopId === shopId);
  }
  if (failureCode) {
    rows = rows.filter((run) => (run.failureCode || "UNKNOWN_FAILURE") === failureCode);
  }

  return rows
    .sort((a, b) => Date.parse(b.startedAt || 0) - Date.parse(a.startedAt || 0))
    .slice(0, Math.max(1, limit));
}

export function appendRunEvent(runId, event) {
  if (!state.runs.has(runId)) {
    return null;
  }

  const existing = state.runEvents.get(runId) || [];
  const next = [...existing, event].slice(-MAX_EVENTS_PER_RUN);
  state.runEvents.set(runId, next);
  persistRunEvents(state.runEvents, state.runs);
  return event;
}

export function listRunEvents(runId, limit = 50) {
  const events = state.runEvents.get(runId) || [];
  return events
    .slice(-Math.max(1, Number(limit || 50)))
    .sort((a, b) => Date.parse(b.createdAt || 0) - Date.parse(a.createdAt || 0));
}
