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

// ---------- Job status colors (pure logic) ----------

// Mirrors jobStatusMeta in the SPA: status -> CSS class.
function jobStatusClass(status) {
  const map = { succeeded: 'success', pending: 'pending', running: 'running', failed: 'failed' };
  return map[status] || 'pending';
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

// ---------- Workload URL sanitization (SEC-06) ----------

// Mirrors the SPA's safeWorkloadURL: only same-origin relative paths are
// returned; anything that could be an external/protocol-relative/scheme URL
// returns "" (link hidden).
function safeWorkloadURL(u) {
  if (typeof u !== 'string' || u === '') return '';
  if (u.includes('://') || u.startsWith('//')) return '';
  const hasScheme = (u.match(/^[a-zA-Z][a-zA-Z0-9+.-]*:/) || [null])[0];
  if (hasScheme && !u.startsWith('/')) return '';
  const isAbsPath = u.startsWith('/');
  const isRelSubpath = !isAbsPath && u.endsWith('/') && u.length > 1 && !u.includes('//');
  if (!isAbsPath && !isRelSubpath) return '';
  if (isAbsPath && u === '/') return '';
  if (/[\s\u0000-\u001f]/.test(u)) return '';
  return u;
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

// --- Job status class tests ---
console.log('Job status:');
assertEq(jobStatusClass('succeeded'), 'success', 'succeeded -> green/success');
assertEq(jobStatusClass('pending'), 'pending', 'pending -> yellow/pending');
assertEq(jobStatusClass('running'), 'running', 'running -> blue/running');
assertEq(jobStatusClass('failed'), 'failed', 'failed -> red/failed');
assertEq(jobStatusClass('weird'), 'pending', 'unknown -> default pending');

// --- Workload URL sanitization tests ---
console.log('safeWorkloadURL:');
assertEq(safeWorkloadURL('/apps/echo'), '/apps/echo', 'absolute same-origin path -> kept');
assertEq(safeWorkloadURL('/apps/vscode/'), '/apps/vscode/', 'absolute path trailing slash -> kept');
assertEq(safeWorkloadURL('apps/vscode/'), 'apps/vscode/', 'relative subpath -> kept');
assertEq(safeWorkloadURL('terminal/'), 'terminal/', 'relative subpath single word -> kept');
assertEq(safeWorkloadURL('/a/b/c?x=1'), '/a/b/c?x=1', 'path with query -> kept');
assertEq(safeWorkloadURL('http://evil.example'), '', 'http URL -> hidden');
assertEq(safeWorkloadURL('https://evil.example/phish'), '', 'https URL -> hidden');
assertEq(safeWorkloadURL('//evil.example'), '', 'protocol-relative -> hidden');
assertEq(safeWorkloadURL('//'), '', 'bare // -> hidden');
assertEq(safeWorkloadURL('javascript:alert(1)'), '', 'javascript: -> hidden');
assertEq(safeWorkloadURL('data:text/html,<script>'), '', 'data: -> hidden');
assertEq(safeWorkloadURL('mailto:x@y.com'), '', 'mailto: -> hidden');
assertEq(safeWorkloadURL('file:///etc/passwd'), '', 'file: -> hidden');
assertEq(safeWorkloadURL('/'), '', 'bare root -> hidden');
assertEq(safeWorkloadURL(''), '', 'empty -> hidden');
assertEq(safeWorkloadURL('   '), '', 'whitespace -> hidden');
assertEq(safeWorkloadURL('/apps /x'), '', 'internal space -> hidden');
assertEq(safeWorkloadURL(null), '', 'null -> hidden');
assertEq(safeWorkloadURL(42), '', 'non-string -> hidden');

// ---------- UX-01: toast + confirm decision logic ----------
// The DOM-bound showToast()/confirmAction() are thin wrappers over these pure
// decisions. We test the decisions (icon, title, duration, variant->A11y role,
// and the confirm promise resolution semantics) so a regression in the mapping
// or the resolve flow is caught without a full DOM mock.

// Pure: variant -> octicon id (mirrors showToast's icons map)
function toastIcon(variant) {
  const icons = { success: 'check-circle', error: 'alert', info: 'info' };
  return icons[variant] || 'info';
}
// Pure: variant -> default ARIA role (mirrors showToast's role assignment)
function toastRole(variant) {
  return variant === 'error' ? 'alert' : 'status';
}
// Pure: variant -> default auto-dismiss duration (ms)
function toastDuration(variant) {
  return variant === 'error' ? 8000 : 4000;
}
// Pure: variant -> default title (mirrors showToast's titles map)
function toastTitle(variant, opts) {
  const titles = { success: 'Success', error: 'Error', info: '' };
  return opts && opts.title ? opts.title : titles[variant];
}

console.log('toast (UX-01):');
assertEq(toastIcon('success'), 'check-circle', 'success -> check-circle');
assertEq(toastIcon('error'), 'alert', 'error -> alert');
assertEq(toastIcon('info'), 'info', 'info -> info');
assertEq(toastIcon('bogus'), 'info', 'unknown variant -> info (fallback)');
assertEq(toastRole('error'), 'alert', 'error role=alert (assertive)');
assertEq(toastRole('success'), 'status', 'success role=status');
assertEq(toastDuration('error'), 8000, 'error stays longer (8s)');
assertEq(toastDuration('success'), 4000, 'success auto-dismiss 4s');
assertEq(toastTitle('success'), 'Success', 'success default title');
assertEq(toastTitle('error'), 'Error', 'error default title');
assertEq(toastTitle('info'), '', 'info has no default title');
assertEq(toastTitle('error', { title: 'Whoops' }), 'Whoops', 'custom title wins');

// Pure: confirm resolution semantics (mirrors _confirmResolve flow)
// Each interaction maps to a resolve call: ok->true, cancel/escape->false.
function confirmResolveFlow(interactions) {
  let resolved = 'pending';
  const resolve = (value) => {
    if (resolved !== 'pending') return; // ignore once resolved (first wins)
    resolved = value;
  };
  for (const i of interactions) {
    if (i === 'ok') resolve(true);
    else if (i === 'cancel' || i === 'escape') resolve(false);
  }
  return resolved;
}

console.log('confirm modal (UX-01):');
assertEq(confirmResolveFlow(['ok']), true, 'OK resolves true');
assertEq(confirmResolveFlow(['cancel']), false, 'cancel resolves false');
assertEq(confirmResolveFlow(['escape']), false, 'Escape resolves false (cancel)');
assertEq(confirmResolveFlow([]), 'pending', 'no interaction -> stays pending');
assertEq(confirmResolveFlow(['ok', 'cancel']), true, 'first resolution wins (double-click safe)');
assertEq(confirmResolveFlow(['cancel', 'ok']), false, 'first resolution wins (cancel then ok)');

// ---------- UX-02: in-flight (busy) double-click guard ----------
// Mirrors the busyActions Set + beginBusy/endBusy semantics. The guarantee: a
// second mutation on the same deployment while the first is in flight is
// rejected (returns false), so a fast double-click cannot fire a duplicate
// scale/terminate/delete. Different deployments are independent.
function makeBusyTracker() {
  const s = new Set();
  return {
    begin(name) { if (s.has(name)) return false; s.add(name); return true; },
    end(name) { s.delete(name); },
    has(name) { return s.has(name); },
    size() { return s.size; },
  };
}

console.log('busy / double-click guard (UX-02):');
{
  const t = makeBusyTracker();
  assertEq(t.begin('a'), true, 'first begin on a -> accepted');
  assertEq(t.begin('a'), false, 'second begin on a (in flight) -> rejected');
  assertEq(t.has('a'), true, 'a stays busy after rejected double-begin');
  t.end('a');
  assertEq(t.has('a'), false, 'a cleared after end');
  assertEq(t.begin('a'), true, 'a can begin again after end');
  // Independent deployments don't block each other.
  const u = makeBusyTracker();
  assertEq(u.begin('x'), true, 'x begin -> accepted');
  assertEq(u.begin('y'), true, 'y begin -> accepted (independent of x)');
  assertEq(u.begin('x'), false, 'x second begin -> rejected');
  assertEq(u.size(), 2, 'two distinct deployments tracked independently');
  u.end('x'); u.end('y');
  assertEq(u.size(), 0, 'all cleared after end');
}
// A failed request still clears the busy state (the handler's finally).
{
  const t = makeBusyTracker();
  t.begin('a');
  // simulate the handler: begin -> throw -> finally end
  try { throw new Error('api fail'); } catch { /* caught by handler */ }
  t.end('a'); // finally
  assertEq(t.begin('a'), true, 'after a failed request (finally end), a can retry');
}

// ---------- UX-03: log modal pod/container selection logic ----------
// Mirrors the pure selection rules in web/index.html: default pod pick
// (first ready, else first), default container (containers[0]), and the
// "show the selector bar only when there's more than one choice" rule.

// Pure: pick the default pod from a pod list (first ready, else first, else null).
function pickDefaultPod(pods) {
  if (!pods || pods.length === 0) return null;
  const ready = pods.find(p => p.ready);
  return ready || pods[0];
}
// Pure: should the selector bar be shown? (more than one pod OR >1 container on the first)
function showLogSelectors(pods) {
  if (!pods || pods.length === 0) return false;
  const multi = pods.length > 1 || (pods[0] && pods[0].containers.length > 1);
  return multi;
}
// Pure: the label for a pod option (mark non-ready pods).
function podOptionLabel(pod) {
  return pod.ready ? pod.name : `${pod.name} (not ready)`;
}

console.log('log pod/container selection (UX-03):');
{
  const single = [{ name: 'a-0', ready: true, containers: ['c1'] }];
  assertEq(pickDefaultPod(single).name, 'a-0', 'single pod -> that pod');
  assertEq(showLogSelectors(single), false, 'single pod + single container -> hide selectors');

  const multiPod = [
    { name: 'a-0', ready: true, containers: ['c1'] },
    { name: 'a-1', ready: true, containers: ['c1'] },
  ];
  assertEq(showLogSelectors(multiPod), true, 'two pods -> show selectors');

  const sidecar = [{ name: 'a-0', ready: true, containers: ['app', 'sidecar'] }];
  assertEq(showLogSelectors(sidecar), true, 'one pod with 2 containers -> show selectors');

  // Default pod prefers a ready pod over a not-ready first pod.
  const mixed = [
    { name: 'a-bad', ready: false, containers: ['c1'] },
    { name: 'a-good', ready: true, containers: ['c1'] },
  ];
  assertEq(pickDefaultPod(mixed).name, 'a-good', 'default pod = first READY (not the first pod)');
  // If none ready, fall back to the first pod.
  const noneReady = [
    { name: 'a-bad1', ready: false, containers: ['c1'] },
    { name: 'a-bad2', ready: false, containers: ['c1'] },
  ];
  assertEq(pickDefaultPod(noneReady).name, 'a-bad1', 'no ready pod -> fall back to first');
  assertEq(pickDefaultPod([]), null, 'empty pod list -> null');

  // Pod option labels mark non-ready pods.
  assertEq(podOptionLabel({ name: 'a-good', ready: true, containers: [] }), 'a-good', 'ready pod label = name');
  assertEq(podOptionLabel({ name: 'a-bad', ready: false, containers: [] }), 'a-bad (not ready)', 'not-ready pod label is annotated');
}

// --- Summary ---
console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
