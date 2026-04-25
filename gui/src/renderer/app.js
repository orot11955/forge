// Forge GUI renderer — vanilla JS. Talks to the main process via window.forgeAPI
// (set up in preload.js). All forge invocations route through that bridge.

'use strict';

const $  = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
const api = window.forgeAPI;

const state = {
  workbench: null,
  projects: [],
  selectedPath: null,
  filter: '',
  activeTab: 'overview',
  lang: 'en',
  cache: emptyCache(),
  run: null, // { runId, name }
};

function emptyCache() {
  return { overview: null, checks: null, history: null, toolchain: null, scripts: null, logs: null };
}

document.addEventListener('DOMContentLoaded', () => {
  bindUI();
  api.onRunEvent(handleRunEvent);
  initVersion();
  refresh();
});

async function initVersion() {
  try {
    const v = await api.version();
    if (v && v.version) $('#version-badge').textContent = `v${v.version}`;
  } catch { /* ignore */ }
}

function bindUI() {
  $('#refresh').addEventListener('click', refresh);
  $('#workbench-chip').addEventListener('click', onPickWorkbench);
  $('#register-project').addEventListener('click', onRegisterProject);
  $('#open-doctor').addEventListener('click', openDoctor);

  $$('#tabs .tab').forEach((btn) => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });

  $$('.lang-btn').forEach((btn) => {
    btn.addEventListener('click', () => setLanguage(btn.dataset.lang));
  });

  $('#project-search').addEventListener('input', (e) => {
    state.filter = e.target.value.trim().toLowerCase();
    renderProjectList();
  });

  $('#run-check').addEventListener('click', () => runCheck());
  $('#reload-logs').addEventListener('click', () => loadLogs(true));
  $('#run-stop').addEventListener('click', stopRun);
  $('#run-clear').addEventListener('click', () => { $('#run-output').innerHTML = ''; });
  $('#ph-open').addEventListener('click', () => state.selectedPath && api.openInShell(state.selectedPath));

  // Modal close handlers.
  $$('#doctor-modal [data-close]').forEach((el) => {
    el.addEventListener('click', () => $('#doctor-modal').classList.add('hidden'));
  });

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'r') { e.preventDefault(); refresh(); }
    if (e.key === 'Escape') $('#doctor-modal').classList.add('hidden');
  });
}

// ─── Refresh / data loading ─────────────────────────────────────────────

async function refresh() {
  try {
    const ws = await api.workStatus().catch(() => null);
    state.workbench = ws;
    renderWorkbench(ws);

    if (!ws) {
      state.projects = [];
      renderProjectList();
      renderProjectHeader(null);
      return;
    }

    const list = await api.list();
    state.projects = (list && list.projects) || [];
    renderProjectList();

    if (state.projects.length && !findSelected()) {
      selectProject(state.projects[0].path);
    } else if (state.selectedPath) {
      // Re-render currently-selected project from refreshed list
      renderProjectHeader(findSelected());
      await loadActiveTab(true);
    } else {
      renderProjectHeader(null);
    }
  } catch (err) {
    showError(err.message || String(err));
  }
}

// ─── Workbench / project list ──────────────────────────────────────────

function renderWorkbench(ws) {
  if (!ws) {
    $('#wb-name').textContent = '(none)';
    $('#wb-path').textContent = 'click to select a Workbench';
    return;
  }
  $('#wb-name').textContent = ws.name || ws.id || 'workbench';
  $('#wb-path').textContent = ws.root + (ws.default ? '  · default' : '');
  $('#wb-path').title = ws.root;
}

function renderProjectList() {
  const ul = $('#project-list');
  ul.innerHTML = '';
  const filtered = filterProjects(state.projects, state.filter);
  $('#project-count').textContent = String(state.projects.length);

  if (!state.projects.length) {
    $('#project-empty').classList.remove('hidden');
    return;
  }
  $('#project-empty').classList.add('hidden');

  if (!filtered.length) {
    const li = document.createElement('li');
    li.className = 'muted small';
    li.style.padding = '8px 12px';
    li.textContent = 'No matches.';
    ul.appendChild(li);
    return;
  }

  for (const p of filtered) {
    const li = document.createElement('li');
    li.dataset.path = p.path;
    if (p.path === state.selectedPath) li.classList.add('active');
    const stale = p.state === 'stale';
    const statusCls = stale ? 'stale' : (p.status || 'subtle');
    const statusText = stale ? 'stale' : (p.status || 'unknown');
    li.innerHTML = `
      <div class="pavatar">${escapeHtml(initial(p.name))}</div>
      <div class="pmeta">
        <div class="pname">${escapeHtml(p.name)}</div>
        <div class="psub">${escapeHtml(p.type || 'generic')} · ${escapeHtml(p.location_type || '')}</div>
      </div>
      <span class="pill ${statusCls}">${escapeHtml(statusText)}</span>
    `;
    li.addEventListener('click', () => selectProject(p.path));
    ul.appendChild(li);
  }
}

function filterProjects(projects, q) {
  if (!q) return projects;
  return projects.filter((p) => {
    const hay = `${p.name} ${p.id} ${p.type} ${p.path} ${p.location_type}`.toLowerCase();
    return hay.includes(q);
  });
}

function findSelected() {
  return state.projects.find((p) => p.path === state.selectedPath) || null;
}

function selectProject(path) {
  state.selectedPath = path;
  state.cache = emptyCache();
  resetRun();
  renderProjectList();
  renderProjectHeader(findSelected());
  loadActiveTab(true);
}

function renderProjectHeader(p) {
  const header = $('#project-header');
  if (!p) { header.classList.add('hidden'); return; }
  header.classList.remove('hidden');
  $('#ph-avatar').textContent = initial(p.name);
  $('#ph-name').textContent = p.name;
  $('#ph-path').textContent = p.path;

  const lc = $('#ph-lifecycle');
  const lcVal = p.status || 'unknown';
  lc.textContent = lcVal;
  lc.className = `pill ${lcVal}`;

  const loc = $('#ph-location');
  loc.textContent = p.location_type || '—';
  loc.className = 'pill subtle';
}

// ─── Tab routing ───────────────────────────────────────────────────────

function switchTab(tab) {
  state.activeTab = tab;
  $$('#tabs .tab').forEach((b) => b.classList.toggle('active', b.dataset.tab === tab));
  $$('#tab-content > .panel').forEach((p) => p.classList.toggle('active', p.dataset.tab === tab));
  loadActiveTab(false);
}

async function loadActiveTab(force) {
  const tab = state.activeTab;
  if (!state.selectedPath) { renderEmpty(tab); return; }
  if (state.cache[tab] && !force) { renderTab(tab, state.cache[tab]); return; }
  switch (tab) {
    case 'overview':  await loadOverview(); break;
    case 'checks':    /* checks run on demand */ renderChecks(state.cache.checks); break;
    case 'history':   await loadHistory(); break;
    case 'toolchain': await loadToolchain(); break;
    case 'scripts':   await loadScripts(); break;
    case 'logs':      await loadLogs(false); break;
  }
}

function renderEmpty(tab) {
  const id = bodyIdFor(tab);
  if (id) setBody(id, `<div class="empty"><div class="empty-icon">📁</div><div class="empty-title">No project selected</div><div class="empty-sub">Pick a project from the sidebar to see its ${tab}.</div></div>`);
}

function bodyIdFor(tab) {
  return ({
    overview: 'overview-body',
    checks: 'checks-body',
    history: 'history-body',
    toolchain: 'toolchain-body',
    scripts: 'scripts-body',
    logs: 'logs-body',
  })[tab];
}

function renderTab(tab, data) {
  switch (tab) {
    case 'overview':  return renderOverview(data);
    case 'checks':    return renderChecks(data);
    case 'history':   return renderHistory(data);
    case 'toolchain': return renderToolchain(data);
    case 'scripts':   return renderScripts(data);
    case 'logs':      return renderLogs(data);
  }
}

// ─── Overview ──────────────────────────────────────────────────────────

async function loadOverview() {
  setBody('overview-body', loadingHTML('Loading status…'));
  try {
    const data = await api.status(state.selectedPath, { lang: state.lang });
    state.cache.overview = data;
    renderOverview(data);
  } catch (err) { setBody('overview-body', errorHTML(err)); }
}

function renderOverview(d) {
  if (!d) return setBody('overview-body', '<div class="empty">No data.</div>');
  const ctx = d.context || {};
  const lc = d.lifecycle || {};
  const proj = d.project || {};
  const cards = `
    <div class="cards">
      <div class="card"><div class="card-label">Type</div><div class="card-value">${escapeHtml(ctx.projectType || '—')}</div></div>
      <div class="card"><div class="card-label">Location</div><div class="card-value">${escapeHtml(ctx.locationType || '—')}</div></div>
      <div class="card"><div class="card-label">Lifecycle</div><div class="card-value">${escapeHtml(lc.current || '—')}</div></div>
      <div class="card"><div class="card-label">Language</div><div class="card-value">${escapeHtml(ctx.language || '—')}</div></div>
    </div>
  `;
  setBody('overview-body', `
    ${cards}
    <div class="section-heading">Context</div>
    <dl class="kv">
      <dt>Workbench</dt><dd>${escapeHtml(ctx.workbenchRoot || '')}</dd>
      <dt>Project root</dt><dd>${escapeHtml(ctx.projectRoot || '')}</dd>
      <dt>Template</dt><dd>${escapeHtml(ctx.projectTemplate || '—')}</dd>
    </dl>
    <div class="section-heading">Project</div>
    <dl class="kv">
      <dt>ID</dt><dd>${escapeHtml(proj.id || '')}</dd>
      <dt>Name</dt><dd>${escapeHtml(proj.name || '')}</dd>
      <dt>Description</dt><dd>${escapeHtml(proj.description || '—')}</dd>
    </dl>
  `);
}

// ─── Checks ────────────────────────────────────────────────────────────

async function runCheck() {
  if (!state.selectedPath) return;
  setBody('checks-body', loadingHTML('Running checks…'));
  try {
    const data = await api.check(state.selectedPath, { lang: state.lang });
    state.cache.checks = data;
    renderChecks(data);
  } catch (err) { setBody('checks-body', errorHTML(err)); }
}

function renderChecks(d) {
  if (!d) {
    setBody('checks-body', `<div class="empty"><div class="empty-icon">✅</div><div class="empty-title">No data yet</div><div class="empty-sub">Click <b>Run check</b> to evaluate <code>checks.yaml</code>.</div></div>`);
    return;
  }
  const summary = d.checks || { total: 0, requiredFailed: 0, optionalFailed: 0 };
  const lc = d.lifecycle || {};
  const results = d.results || [];
  const grouped = groupBy(results, (r) => r.phase || '(default)');
  const lines = [];
  lines.push(`<div class="cards">
    <div class="card"><div class="card-label">Total</div><div class="card-value">${summary.total}</div></div>
    <div class="card"><div class="card-label">Required failed</div><div class="card-value" style="color: ${summary.requiredFailed ? 'var(--danger)' : 'var(--accent-2)'}">${summary.requiredFailed}</div></div>
    <div class="card"><div class="card-label">Optional failed</div><div class="card-value" style="color: ${summary.optionalFailed ? 'var(--warn)' : 'var(--accent-2)'}">${summary.optionalFailed}</div></div>
    <div class="card"><div class="card-label">Lifecycle</div><div class="card-value">${escapeHtml(lc.from || '—')} → ${escapeHtml(lc.to || lc.from || '—')}</div></div>
  </div>`);
  for (const [phase, rows] of Object.entries(grouped)) {
    lines.push(`<div class="section-heading">${escapeHtml(phase)}</div>`);
    for (const r of rows) {
      const cls = r.passed ? 'passed' : (r.required ? 'failed-required' : 'failed-optional');
      const mark = r.passed ? '✓' : (r.required ? '✗' : '!');
      const detail = [r.type, r.required ? 'required' : 'optional', r.path ? `path=${r.path}` : null]
        .filter(Boolean).join(' · ');
      lines.push(`<div class="check-row ${cls}">
        <div class="mark">${mark}</div>
        <div><div class="id">${escapeHtml(r.id)}</div>${r.message ? `<div class="muted small">${escapeHtml(r.message)}</div>` : ''}</div>
        <div class="meta">${escapeHtml(detail)}</div>
      </div>`);
    }
  }
  setBody('checks-body', lines.join(''));
}

// ─── History ───────────────────────────────────────────────────────────

async function loadHistory() {
  setBody('history-body', loadingHTML('Loading history…'));
  try {
    const events = await api.historyTail(state.selectedPath, 100);
    state.cache.history = events;
    renderHistory(events);
  } catch (err) { setBody('history-body', errorHTML(err)); }
}

function renderHistory(events) {
  if (!events || events.length === 0) {
    setBody('history-body', '<div class="empty"><div class="empty-icon">📜</div><div class="empty-title">No history yet</div></div>');
    return;
  }
  const out = [];
  for (const e of events.slice().reverse()) {
    const ts = (e.at || e.startedAt || '').replace('T', ' ').slice(0, 19);
    const type = e.type || 'event';
    const typeCls = type === 'lifecycle' ? 'type-lifecycle' : 'type-cmd';
    let body = '';
    if (type === 'lifecycle') {
      body = `${escapeHtml(e.from || '')} → ${escapeHtml(e.to || '')}` + (e.reason ? ` <span class="muted">(${escapeHtml(e.reason)})</span>` : '');
    } else {
      const status = e.exitCode !== undefined ? ` exit=${e.exitCode}` : '';
      body = `${escapeHtml(e.command || '?')}${status} <span class="muted">${escapeHtml(e.summary || '')}</span>`;
    }
    out.push(`<div class="history-event">
      <div class="ts">${escapeHtml(ts)}</div>
      <div class="${typeCls}">${escapeHtml(type)}</div>
      <div>${body}</div>
    </div>`);
  }
  setBody('history-body', out.join(''));
}

// ─── Toolchain ─────────────────────────────────────────────────────────

async function loadToolchain() {
  setBody('toolchain-body', loadingHTML('Loading toolchain…'));
  try {
    const data = await api.toolShow(state.selectedPath);
    state.cache.toolchain = data;
    renderToolchain(data);
  } catch (err) { setBody('toolchain-body', errorHTML(err)); }
}

function renderToolchain(d) {
  if (!d) return setBody('toolchain-body', '<div class="empty">No data.</div>');
  const sections = [
    ['Runtimes', d.runtimes || []],
    ['Package managers', d.packageManagers || []],
    ['System tools', d.systemTools || []],
  ];
  const out = [];
  out.push(`<div class="section-heading">Policy</div>
    <dl class="kv">
      <dt>Mode</dt><dd>${escapeHtml((d.policy && d.policy.mode) || '')}</dd>
      <dt>Fallback to system</dt><dd>${(d.policy && d.policy.fallbackToSystem) ? 'true' : 'false'}</dd>
    </dl>`);
  for (const [title, rows] of sections) {
    out.push(`<div class="section-heading">${title}</div>`);
    if (rows.length === 0) {
      out.push(`<div class="muted small">(none declared)</div>`);
      continue;
    }
    out.push(`<table class="toolchain-table">
      <thead><tr><th>Name</th><th>Source</th><th>Path</th><th>Version</th><th>Status</th></tr></thead>
      <tbody>${rows.map((r) => `
        <tr>
          <td>${escapeHtml(r.name)}</td>
          <td>${escapeHtml(r.source || '')}</td>
          <td>${escapeHtml(r.path || '—')}</td>
          <td>${escapeHtml(r.version || '—')}</td>
          <td class="${r.available ? 'ok' : 'missing'}">${r.available ? 'ok' : 'missing'}</td>
        </tr>`).join('')}
      </tbody>
    </table>`);
  }
  setBody('toolchain-body', out.join(''));
}

// ─── Scripts ───────────────────────────────────────────────────────────

async function loadScripts() {
  setBody('scripts-body', loadingHTML('Loading scripts…'));
  try {
    const data = await api.scripts(state.selectedPath);
    state.cache.scripts = data;
    renderScripts(data);
  } catch (err) { setBody('scripts-body', errorHTML(err)); }
}

function renderScripts(d) {
  if (!d) return setBody('scripts-body', '<div class="empty">No data.</div>');
  const scripts = d.scripts || [];
  if (!scripts.length) {
    setBody('scripts-body', `<div class="empty"><div class="empty-icon">🧰</div><div class="empty-title">No scripts</div><div class="empty-sub">Add entries to <code>.forge/scripts.yaml</code>.</div></div>`);
    return;
  }
  const out = [`<div class="section-heading">Available scripts</div>`];
  for (const name of scripts) {
    const safe = encodeURIComponent(name);
    out.push(`<div class="script-row">
      <div>
        <div class="sname">${escapeHtml(name)}</div>
        <div class="sdesc">forge run ${escapeHtml(name)}</div>
      </div>
      <button class="primary small" data-run="${safe}">▶ Run</button>
    </div>`);
  }
  setBody('scripts-body', out.join(''));
  $$('#scripts-body [data-run]').forEach((btn) => {
    btn.addEventListener('click', () => startRun(decodeURIComponent(btn.dataset.run)));
  });
}

async function startRun(name) {
  if (!state.selectedPath) return;
  if (state.run) {
    showToast(`Already running ${state.run.name}. Stop it first.`, true);
    return;
  }
  $('#run-output').innerHTML = '';
  $('#run-name').textContent = name;
  $('#run-pid').textContent = '';
  $('#run-console').classList.remove('hidden');
  try {
    const { runId, pid } = await api.runStart(state.selectedPath, name);
    state.run = { runId, name };
    $('#run-pid').textContent = `pid ${pid}`;
  } catch (err) {
    appendRunLine(err.message || String(err), 'stderr');
  }
}

async function stopRun() {
  if (!state.run) return;
  try { await api.runStop(state.run.runId); } catch { /* ignore */ }
}

function resetRun() {
  if (state.run) { try { api.runStop(state.run.runId); } catch { /* ignore */ } }
  state.run = null;
  const out = $('#run-output');
  if (out) out.innerHTML = '';
  $('#run-console').classList.add('hidden');
}

function handleRunEvent(evt) {
  if (!evt) return;
  if (state.run && evt.runId !== state.run.runId) return;
  if (evt.kind === 'stdout') {
    appendRunLine(evt.data, 'stdout');
  } else if (evt.kind === 'stderr') {
    appendRunLine(evt.data, 'stderr');
  } else if (evt.kind === 'exit') {
    const code = evt.data && evt.data.code;
    const cls = code === 0 ? 'exit-line' : 'exit-line fail';
    appendRunLine(`\n— exited (code=${code})\n`, cls);
    state.run = null;
  } else if (evt.kind === 'error') {
    appendRunLine(`\n[error] ${(evt.data && evt.data.message) || ''}\n`, 'stderr');
    state.run = null;
  }
}

function appendRunLine(text, cls) {
  const out = $('#run-output');
  const span = document.createElement('span');
  if (cls) span.className = cls;
  span.textContent = text;
  out.appendChild(span);
  out.scrollTop = out.scrollHeight;
}

// ─── Logs ──────────────────────────────────────────────────────────────

async function loadLogs(force) {
  if (state.cache.logs && !force) { renderLogs(state.cache.logs); return; }
  setBody('logs-body', loadingHTML('Loading logs…'));
  try {
    const data = await api.logs(state.selectedPath, 200);
    state.cache.logs = data;
    renderLogs(data);
  } catch (err) { setBody('logs-body', errorHTML(err)); }
}

function renderLogs(d) {
  if (!d) return setBody('logs-body', '<div class="empty">No data.</div>');
  const logs = d.logs || [];
  if (!logs.length) {
    setBody('logs-body', `<div class="empty"><div class="empty-icon">📓</div><div class="empty-title">No log entries</div><div class="empty-sub">Run <code>forge run &lt;script&gt;</code> to populate logs.</div></div>`);
    return;
  }
  const out = [];
  for (const l of logs.slice().reverse()) {
    const ts = (l.timestamp || l.at || '').replace('T', ' ').slice(0, 19);
    const cls = (l.exitCode === 0 || l.status === 'success') ? 'ok' : 'err';
    const summary = l.command || l.script || l.message || JSON.stringify(l);
    const exit = l.exitCode !== undefined ? `exit=${l.exitCode}` : (l.status || '');
    out.push(`<div class="log-row">
      <div class="ts">${escapeHtml(ts)}</div>
      <div>${escapeHtml(summary)}</div>
      <div class="${cls}">${escapeHtml(exit)}</div>
    </div>`);
  }
  setBody('logs-body', out.join(''));
}

// ─── Workbench / project init / language / doctor ─────────────────────

async function onPickWorkbench() {
  try {
    const dir = await api.pickWorkbench();
    if (!dir) return;
    try { await api.initWorkbench(dir); }
    catch (e) { /* may already exist; refresh anyway */ }
    showToast(`Workbench: ${dir}`);
    await refresh();
  } catch (err) { showError(err.message || String(err)); }
}

async function onRegisterProject() {
  try {
    const dir = await api.pickProject();
    if (!dir) return;
    await api.projectInit(dir);
    showToast(`Registered: ${dir}`);
    await refresh();
    selectProject(dir);
  } catch (err) { showError(err.message || String(err)); }
}

async function setLanguage(lang) {
  if (!['en', 'ko'].includes(lang)) return;
  state.lang = lang;
  $$('.lang-btn').forEach((b) => b.classList.toggle('active', b.dataset.lang === lang));
  try {
    const r = await api.setLanguage(lang);
    if (!r.ok) throw new Error(r.stderr || `exit ${r.exitCode}`);
    showToast(`Language: ${lang.toUpperCase()}`);
    state.cache = emptyCache();
    if (state.selectedPath) loadActiveTab(true);
  } catch (err) { showError(err.message || String(err)); }
}

async function openDoctor() {
  $('#doctor-modal').classList.remove('hidden');
  setBody('doctor-body', loadingHTML('Running doctor…'));
  try {
    const d = await api.doctor({ lang: state.lang });
    if (!d) { setBody('doctor-body', '<div class="muted">No data.</div>'); return; }
    const ok = (v) => /^(ok|\/.*)$/.test(v) || (typeof v === 'string' && v.startsWith('/'));
    const rows = Object.entries(d).map(([k, v]) => {
      const val = String(v == null ? '—' : v);
      const cls = (val === 'ok' || val.startsWith('/')) ? 'ok'
                : (val === 'not found' || val === 'missing') ? 'err' : '';
      return `<dt>${escapeHtml(k)}</dt><dd class="${cls}">${escapeHtml(val)}</dd>`;
    }).join('');
    setBody('doctor-body', `<dl class="kv">${rows}</dl>`);
  } catch (err) { setBody('doctor-body', errorHTML(err)); }
}

// ─── Helpers ───────────────────────────────────────────────────────────

function groupBy(arr, fn) {
  const m = {};
  for (const item of arr) {
    const k = fn(item);
    (m[k] = m[k] || []).push(item);
  }
  return m;
}

function initial(s) {
  return String(s || '?').trim().charAt(0).toUpperCase() || '?';
}

function escapeHtml(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function setBody(id, html) {
  const el = document.getElementById(id);
  if (el) el.innerHTML = html;
}

function loadingHTML(msg) {
  return `<div class="muted small">${escapeHtml(msg || 'Loading…')}</div>`;
}

function errorHTML(err) {
  return `<div class="error">${escapeHtml(err && err.message ? err.message : String(err))}</div>`;
}

let toastTimer = null;
function showToast(msg, isError = false) {
  const t = $('#toast');
  t.textContent = msg;
  t.classList.toggle('error', isError);
  t.classList.remove('hidden');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => t.classList.add('hidden'), 3000);
}
function showError(msg) { showToast(msg, true); }
