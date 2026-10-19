/**
 * Frontend unit tests for devpod.
 *
 * Tests pure functions extracted from the SPA logic.
 * Run with: node web/test.js
 */

// ---------- Theme logic ----------

const THEME_KEY = 'devpod-theme';
const THEMES = ['latte', 'frappe'];

function setTheme(theme) {
  localStorage.setItem(THEME_KEY, theme);
  document.documentElement.setAttribute('data-color-mode', theme === 'latte' ? 'light' : 'dark');
}

function cycleTheme() {
  const current = localStorage.getItem(THEME_KEY) || 'latte';
  const next = THEMES[(THEMES.indexOf(current) + 1) % THEMES.length];
  setTheme(next);
  return next;
}

// ---------- API path construction (subpath support) ----------

function apiPath(basePath, path) {
  // basePath is location.pathname with trailing slash removed
  // path is a relative API path like "api/deployments"
  const base = basePath.replace(/\/$/, '');
  return base + '/' + path;
}

function wsURL(basePath, host) {
  // Construct WebSocket URL from the current page path
  // e.g. page at /apps/dashboard/ -> ws://host/apps/dashboard/api/logs-ws
  const base = basePath.replace(/\/$/, '');
  const proto = location.protocol === 'https:' ? 'wss' : 'ws';
  return `${proto}://${host}${base}/api/logs-ws`;
}

// ---------- Log line processing ----------

function processLogLines(data) {
  return data.split('\n').filter(l => l.length > 0);
}

function formatLogLine(line) {
  // Parse a log line: "2024-01-01T00:00:00Z [INFO] message"
  const match = line.match(/^(\S+)\s+\[(\w+)\]\s+(.*)$/);
  if (!match) return { raw: line, level: 'INFO', msg: line };
  return { timestamp: match[1], level: match[2], msg: match[3] };
}

// ---------- Deployment state ----------

function deploymentStatus(d) {
  if (d.ready > 0 && d.ready === d.replicas) return 'running';
  if (d.replicas === 0) return 'stopped';
  if (d.ready === 0) return 'pending';
  return 'degraded';
}

function targetReplicas(d, run) {
  if (run) return d.replicas === 0 ? 1 : d.replicas;
  return 0;
}

// ---------- Node ready state (pure logic) ----------

// Mirrors the SPA's ready-dot color mapping.
function readyColor(status) {
  if (status === 'Ready') return 'success';
  if (status === 'NotReady') return 'attention';
  return 'danger'; // Unknown / anything else
}

// ---------- Node resource bars (pure logic) ----------

// Mirrors levelClass in the SPA: utilization -> CSS level.
function levelClass(pct) {
  if (pct >= 0.9) return 'crit';
  if (pct >= 0.7) return 'warn';
  return 'ok';
}
// Mirrors resBar's width computation (returns a 0..100 number).
function barWidth(used, total) {
  if (!(total > 0) || used < 0) return 0;
  return Math.min(1, Math.max(0, used / total)) * 100;
}

// ---------- Tests ----------

let passed = 0;
let failed = 0;

function assert(cond, msg) {
  if (cond) {
    passed++;
  } else {
    failed++;
    console.error(`  FAIL: ${msg}`);
  }
}

function assertEq(a, b, msg) {
  assert(a === b, `${msg} (got ${JSON.stringify(a)}, want ${JSON.stringify(b)})`);
}

// --- Theme tests ---
console.log('Theme:');

// Mock localStorage
global.localStorage = {
  _data: {},
  getItem(k) { return this._data[k] ?? null; },
  setItem(k, v) { this._data[k] = v; },
};
global.document = {
  documentElement: {
    _attrs: {},
    setAttribute(k, v) { this._attrs[k] = v; },
    getAttribute(k) { return this._attrs[k]; },
  },
};
global.location = { protocol: 'http:', pathname: '/apps/dashboard/', host: 'localhost:8080' };

// setTheme latte
localStorage.setItem(THEME_KEY, null);
setTheme('latte');
assertEq(localStorage.getItem(THEME_KEY), 'latte', 'setTheme stores latte');
assertEq(document.documentElement.getAttribute('data-color-mode'), 'light', 'latte -> light');

// setTheme frappe
setTheme('frappe');
assertEq(localStorage.getItem(THEME_KEY), 'frappe', 'setTheme stores frappe');
assertEq(document.documentElement.getAttribute('data-color-mode'), 'dark', 'frappe -> dark');

// cycleTheme: latte -> frappe
localStorage.setItem(THEME_KEY, 'latte');
let next = cycleTheme();
assertEq(next, 'frappe', 'cycleTheme latte -> frappe');

// cycleTheme: frappe -> latte
next = cycleTheme();
assertEq(next, 'latte', 'cycleTheme frappe -> latte');

// cycleTheme: default (no stored value) -> frappe
localStorage.setItem(THEME_KEY, null);
next = cycleTheme();
assertEq(next, 'frappe', 'cycleTheme default -> frappe');

// --- API path tests ---
console.log('API path:');
assertEq(apiPath('/apps/dashboard/', 'api/deployments'), '/apps/dashboard/api/deployments', 'subpath + api');
assertEq(apiPath('/', 'api/deployments'), '/api/deployments', 'root + api');
assertEq(apiPath('/apps/dashboard', 'api/deployments'), '/apps/dashboard/api/deployments', 'no trailing slash');
assertEq(apiPath('/a/b/c/', 'healthz'), '/a/b/c/healthz', 'nested path');

// --- WebSocket URL tests ---
console.log('WebSocket URL:');
assertEq(wsURL('/apps/dashboard/', 'localhost:8080'), 'ws://localhost:8080/apps/dashboard/api/logs-ws', 'ws URL subpath');
assertEq(wsURL('/', 'localhost:8080'), 'ws://localhost:8080/api/logs-ws', 'ws URL root');
assertEq(wsURL('/a/b/c/', 'example.com:443'), 'ws://example.com:443/a/b/c/api/logs-ws', 'ws URL nested');

// --- Log line tests ---
console.log('Log lines:');
assertEq(JSON.stringify(processLogLines('line1\nline2\n\nline4\n')), JSON.stringify(['line1', 'line2', 'line4']), 'processLogLines filters empty');
assertEq(JSON.stringify(processLogLines('single\n')), JSON.stringify(['single']), 'processLogLines single');
assertEq(JSON.stringify(processLogLines('')), JSON.stringify([]), 'processLogLines empty');

const parsed = formatLogLine('2024-01-01T00:00:00Z [INFO] hello world');
assertEq(parsed.level, 'INFO', 'formatLogLine level');
assertEq(parsed.msg, 'hello world', 'formatLogLine msg');

const parsed2 = formatLogLine('garbage line');
assertEq(parsed2.level, 'INFO', 'formatLogLine fallback level');
assertEq(parsed2.msg, 'garbage line', 'formatLogLine fallback msg');

// --- Deployment status tests ---
console.log('Deployment status:');
assertEq(deploymentStatus({ ready: 3, replicas: 3 }), 'running', 'all ready');
assertEq(deploymentStatus({ ready: 0, replicas: 3 }), 'pending', 'none ready');
assertEq(deploymentStatus({ ready: 0, replicas: 0 }), 'stopped', 'scaled to 0');
assertEq(deploymentStatus({ ready: 1, replicas: 3 }), 'degraded', 'partially ready');

assertEq(targetReplicas({ replicas: 3 }, true), 3, 'scale up keeps replicas');
assertEq(targetReplicas({ replicas: 0 }, true), 1, 'scale up from 0 -> 1');
assertEq(targetReplicas({ replicas: 3 }, false), 0, 'scale down -> 0');

// --- Node bar logic tests ---
console.log('Node bars:');
assertEq(levelClass(0.2), 'ok', 'low -> ok');
assertEq(levelClass(0.69), 'ok', 'just under warn -> ok');
assertEq(levelClass(0.7), 'warn', 'warn threshold -> warn');
assertEq(levelClass(0.89), 'warn', 'high -> warn');
assertEq(levelClass(0.9), 'crit', 'crit threshold -> crit');
assertEq(levelClass(1.0), 'crit', 'full -> crit');

assertEq(barWidth(500, 2000), 25, '500/2000 = 25%');
assertEq(barWidth(2000, 2000), 100, 'full = 100%');
assertEq(barWidth(3000, 2000), 100, 'clamped to 100%');
assertEq(barWidth(-1, 2000), 0, 'unknown used -> 0%');
assertEq(barWidth(500, 0), 0, 'zero total -> 0%');

// --- Node ready color tests ---
console.log('Node ready:');
assertEq(readyColor('Ready'), 'success', 'Ready -> green');
assertEq(readyColor('NotReady'), 'attention', 'NotReady -> yellow');
assertEq(readyColor('Unknown'), 'danger', 'Unknown -> red');
assertEq(readyColor(''), 'danger', 'empty -> red');

// --- Summary ---
console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
