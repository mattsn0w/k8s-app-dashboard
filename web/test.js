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

// ---------- UX-04: log DOM cap (trimLogLines + isAtBottom) ----------
// Mirrors the pure helpers in web/index.html. trimLogLines drops the OLDEST
// children beyond `max`; isAtBottom reports whether a scroll container is at
// (within `threshold`px of) its bottom.

// Mirror of the real trimLogLines (same algorithm; a tiny fake container stands
// in for a real <pre> so this runs without a DOM).
function fakeContainer() {
  const kids = [];
  return {
    kids,
    get childElementCount() { return kids.length; },
    get firstElementChild() { return kids[0] || null; },
    appendChild(c) { kids.push(c); return c; },
    removeChild(c) { const i = kids.indexOf(c); if (i >= 0) kids.splice(i, 1); return c; },
    text() { return kids.map(k => k.t).join('\n'); },
  };
}
function line(t) { return { t }; }
function trimLogLines(container, max) {
  const overflow = container.childElementCount - max;
  if (overflow <= 0) return 0;
  let removed = 0;
  for (let i = 0; i < overflow; i++) {
    const first = container.firstElementChild;
    if (!first) break;
    container.removeChild(first);
    removed++;
  }
  return removed;
}
// Mirror of the real isAtBottom.
function isAtBottom(scrollEl, threshold = 40) {
  return scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight < threshold;
}

console.log('log DOM cap (UX-04):');
{
  // Under the cap: nothing removed.
  const c = fakeContainer();
  for (let i = 0; i < 5; i++) c.appendChild(line('L' + i));
  assertEq(trimLogLines(c, 10), 0, 'under cap -> 0 removed');
  assertEq(c.childElementCount, 5, 'under cap -> count unchanged');

  // Exactly at the cap: nothing removed.
  const c2 = fakeContainer();
  for (let i = 0; i < 10; i++) c2.appendChild(line('L' + i));
  assertEq(trimLogLines(c2, 10), 0, 'at cap -> 0 removed');

  // Over the cap: oldest dropped, newest kept, count == max.
  const c3 = fakeContainer();
  for (let i = 0; i < 15; i++) c3.appendChild(line('L' + i)); // L0..L14
  const removed = trimLogLines(c3, 10);
  assertEq(removed, 5, 'over cap -> removed the overflow (5)');
  assertEq(c3.childElementCount, 10, 'over cap -> count clamped to max');
  assertEq(c3.text(), 'L5\nL6\nL7\nL8\nL9\nL10\nL11\nL12\nL13\nL14',
    'over cap -> OLDEST (L0..L4) dropped, NEWEST kept');

  // Large single batch (simulates one WS frame with many lines): still bounded.
  const c4 = fakeContainer();
  for (let i = 0; i < 5000; i++) c4.appendChild(line('X' + i));
  trimLogLines(c4, 2000);
  assertEq(c4.childElementCount, 2000, '5000-line batch -> clamped to 2000');
  assertEq(c4.kids[0].t, 'X3000', 'large batch -> oldest (X0..X2999) dropped');
  assertEq(c4.kids[1999].t, 'X4999', 'large batch -> newest line retained');

  // trimLogLines is idempotent once at the cap.
  assertEq(trimLogLines(c4, 2000), 0, 'already at cap -> 0 further removed');

  // isAtBottom: within threshold of the bottom.
  const atBottom = { scrollHeight: 1000, scrollTop: 970, clientHeight: 100 }; // 30px from bottom
  const mid =      { scrollHeight: 1000, scrollTop: 400, clientHeight: 100 }; // 500px from bottom
  const top =      { scrollHeight: 1000, scrollTop: 0,   clientHeight: 100 };
  assertEq(isAtBottom(atBottom), true,  '30px from bottom (< 40) -> atBottom');
  assertEq(isAtBottom({ scrollHeight:1000, scrollTop:960, clientHeight:100 }), true, 'exactly 40px -> atBottom (< 40 is false, ==40 -> 0<40 true)');
  assertEq(isAtBottom(mid), false, 'mid-scroll -> not at bottom');
  assertEq(isAtBottom(top), false, 'top -> not at bottom');
  // Custom threshold: 50px from bottom (1000-850-100=50) with threshold 10 -> not at bottom.
  assertEq(isAtBottom({ scrollHeight:1000, scrollTop:850, clientHeight:100 }, 10), false, '50px from bottom with threshold 10 -> not at bottom');
  // ...but with the default 40 threshold it IS at bottom (50 < 40 is false, so not). Use 30px to be inside default.
  assertEq(isAtBottom({ scrollHeight:1000, scrollTop:870, clientHeight:100 }), true, '30px from bottom -> at bottom (default 40)');
}

// ---------- Log stream token: switching pods must not cause a stale reconnect ----------
// Models the token-guard in openLogModalFor/closeLogModal. The bug: when the
// user switches pod, the OLD socket's onclose fires AFTER the new one is
// opened; without a token guard it schedules a 3s reconnect that kills the NEW
// (correctly-streaming) socket. The fix: each socket captures a token at open;
// its onclose/onmessage/reconnect-timers only act while that token is still
// current. Bumping the token on a new open (or on close) invalidates the old
// socket.

// Simulate the open/close/reconnect lifecycle with the token guard.
function simulateSwitchBugs() {
  let token = 0;
  const timers = []; // {at, fn, token}
  const sockets = []; // {token, closed, onclose}
  function open() {
    const t = ++token;
    const sock = { token: t, closed: false, onclose: null };
    sock.onclose = () => {
      // Guard: only the current socket, while "modal open", reschedules.
      if (sock.token !== token) return;          // stale -> inert
      if (!modalOpen) return;
      timers.push({ fn: () => restart(), token: sock.token });
    };
    sockets.push(sock);
    return sock;
  }
  let modalOpen = true;
  function close() {
    token++; // invalidate any pending reconnect / in-flight onclose
    const last = sockets[sockets.length - 1];
    if (last) { last.closed = true; }
    modalOpen = false;
  }
  let restarts = 0;
  function restart() { restarts++; }
  // A socket closes (its onclose fires). Returns whether it scheduled a reconnect.
  function fireClose(sock) {
    const before = timers.length;
    sock.onclose();
    return timers.length - before;
  }
  // Flush timers whose token is still current (simulates the 3s elapsing).
  function flush() {
    for (const t of timers) { if (t.token === token) t.fn(); }
    timers.length = 0;
  }
  return { open, close, fireClose, flush, get token(){return token;}, get restarts(){return restarts;},
           get scheduled(){return timers.length;}, setModalOpen(v){modalOpen=v;} };
}

console.log('log stream token guard (pod-switch reconnect bug):');
{
  // Scenario: open pod A, switch to pod B, pod A's socket closes.
  const s = simulateSwitchBugs();
  const a = s.open();      // token 1
  const b = s.open();      // token 2 (supersedes A)
  // A's socket now closes (asynchronously). It must NOT schedule a reconnect.
  const scheduledByA = s.fireClose(a);
  assertEq(scheduledByA, 0, 'superseded socket A onclose -> NO reconnect scheduled');
  assertEq(s.scheduled, 0, 'no phantom reconnect pending after switch');
  assertEq(s.restarts, 0, 'no restart triggered by the stale close');
  // B is still current; nothing should have restarted the stream.
  assertEq(b.token === s.token, true, 'socket B is the current one');

  // Scenario: the CURRENT socket (B) closes while modal open -> schedules a reconnect.
  const s2 = simulateSwitchBugs();
  const b2 = s2.open();
  const scheduledByB = s2.fireClose(b2);
  assertEq(scheduledByB, 1, 'current socket onclose (modal open) -> reconnect scheduled');
  s2.flush();
  assertEq(s2.restarts, 1, 'pending reconnect fired (current token still valid)');

  // Scenario: close the modal while a reconnect is pending -> token bump kills it.
  const s3 = simulateSwitchBugs();
  const c = s3.open();
  s3.fireClose(c);          // schedules a reconnect for token 1
  assertEq(s3.scheduled, 1, 'reconnect pending before close');
  s3.close();               // bump token + close modal
  s3.flush();               // 3s elapses, but token is stale now
  assertEq(s3.restarts, 0, 'closing the modal cancels the pending reconnect');

  // Scenario: rapid A->B->C switches, each old socket closes; only C is current.
  const s4 = simulateSwitchBugs();
  const a4 = s4.open(), b4 = s4.open(), c4 = s4.open();
  s4.fireClose(a4); s4.fireClose(b4);
  assertEq(s4.scheduled, 0, 'rapid switches: no phantom reconnects from superseded sockets');
  assertEq(c4.token === s4.token, true, 'only the last socket (C) is current');
}

// --- Summary ---
console.log(`\n${passed} passed, ${failed} failed`);
process.exit(failed > 0 ? 1 : 0);
