// Forge GUI renderer — vanilla JS. Talks to the main process via window.forgeAPI
// (set up in preload.js). All forge invocations route through that bridge.

'use strict';

const $  = (sel, root = document) => root.querySelector(sel);
const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));

const state = {
  workbench: null,        // { root, id, name, projects, default }
  projects: [],           // RegistryEntry[]
  selectedPath: null,     // currently selected project's absolute path
  activeTab: 'status',
  // per-tab caches (cleared on selection change / refresh)
  cache: { status: null, check: null, history: null, toolchain: null },
};

document.addEventListener('DOMContentLoaded', () => {
  bindUI();
  refresh();
});

function bindUI() {
  $('#refresh').addEventListener('click', refresh);
  $('#pick-workbench').addEventListener('click', onPickWorkbench);
  $('#run-check').addEventListener('click', () => runCheck(true));
  $$('#tabs .tab').forEach((btn) => {
    btn.addEventListener('click', () => switchTab(btn.dataset.tab));
  });
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'r') { e.preventDefault(); refresh(); }
  });
}

async function refresh() {
  try {
    const ws = await window.forgeAPI.workStatus();
    state.workbench = ws;
    renderWorkbench(ws);
    const list = await window.forgeAPI.list();
    state.projects = (list && list.projects) || [];
    renderProjectList();
    if (state.projects.length && !findSelected()) {
      selectProject(state.projects[0].path);
    } else if (state.selectedPath) {
      // Re-render currently-selected project
      await loadActiveTab(true);
    }
  } catch (err) {
    showError(err.message || String(err));
  }
}

function renderWorkbench(ws) {
  if (!ws) {
    $('#wb-name').textContent = '(no workbench)';
    $('#wb-path').textContent = '';
    return;
  }
  $('#wb-name').textContent = ws.name || ws.id || 'workbench';
  $('#wb-path').textContent = ws.root + (ws.default ? '  (default)' : '');
  $('#wb-path').title = ws.root;
}

function renderProjectList() {
  const ul = $('#project-list');
  ul.innerHTML = '';
  if (!state.projects.length) {
    $('#project-empty').classList.remove('hidden');
    return;
  }
  $('#project-empty').classList.add('hidden');
  for (const p of state.projects) {
    const li = document.createElement('li');
    li.dataset.path = p.path;
    if (p.path === state.selectedPath) li.classList.add('active');
    const stale = p.state === 'stale';
    li.innerHTML = `
      <div>
        <div class="pname">${escapeHtml(p.name)}</div>
        <div class="ptype">${escapeHtml(p.type)} · ${escapeHtml(p.location_type)}</div>
      </div>
      <span class="pstatus ${stale ? 'stale' : escapeHtml(p.status)}">${stale ? 'stale' : escapeHtml(p.status)}</span>
    `;
    li.addEventListener('click', () => selectProject(p.path));
    ul.appendChild(li);
  }
}

function findSelected() {
  return state.projects.find((p) => p.path === state.selectedPath) || null;
}

function selectProject(path) {
  state.selectedPath = path;
  state.cache = { status: null, check: null, history: null, toolchain: null };
  renderProjectList();
  loadActiveTab(true);
}

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
    case 'status':    await loadStatus(); break;
    case 'check':     /* on-demand */ renderTab('check', state.cache.check); break;
    case 'history':   await loadHistory(); break;
    case 'toolchain': await loadToolchain(); break;
  }
}

async function loadStatus() {
  setBody('status-body', '<div class="muted">Loading…</div>');
  try {
    const data = await window.forgeAPI.status(state.selectedPath);
    state.cache.status = data;
    renderTab('status', data);
  } catch (err) { setBody('status-body', errorHTML(err)); }
}

async function runCheck(force) {
  const path = state.selectedPath;
  if (!path) return;
  setBody('check-body', '<div class="muted">Running check…</div>');
  try {
    const data = await window.forgeAPI.check(path);
    state.cache.check = data;
    renderTab('check', data);
  } catch (err) { setBody('check-body', errorHTML(err)); }
}

async function loadHistory() {
  setBody('history-body', '<div class="muted">Loading…</div>');
  try {
    const events = await window.forgeAPI.historyTail(state.selectedPath, 50);
    state.cache.history = events;
    renderTab('history', events);
  } catch (err) { setBody('history-body', errorHTML(err)); }
}

async function loadToolchain() {
  setBody('toolchain-body', '<div class="muted">Loading…</div>');
  try {
    const data = await window.forgeAPI.toolShow(state.selectedPath);
    state.cache.toolchain = data;
    renderTab('toolchain', data);
  } catch (err) { setBody('toolchain-body', errorHTML(err)); }
}

function renderEmpty(tab) {
  setBody(`${tab}-body`, `<div class="empty">Select a project to see its ${tab}.</div>`);
}

function renderTab(tab, data) {
  switch (tab) {
    case 'status':    return renderStatus(data);
    case 'check':     return renderCheck(data);
    case 'history':   return renderHistory(data);
    case 'toolchain': return renderToolchain(data);
  }
}

function renderStatus(d) {
  if (!d) return setBody('status-body', '<div class="empty">No data.</div>');
  const ctx = d.context || {};
  const lc = d.lifecycle || {};
  const proj = d.project || {};
  setBody('status-body', `
    <div class="section-heading">Context</div>
    <dl class="kv">
      <dt>Workbench</dt><dd>${escapeHtml(ctx.workbenchRoot || '')}</dd>
      <dt>Project root</dt><dd>${escapeHtml(ctx.projectRoot || '')}</dd>
      <dt>Type</dt><dd>${escapeHtml(ctx.projectType || '')}</dd>
      <dt>Location</dt><dd>${escapeHtml(ctx.locationType || '')}</dd>
      <dt>Language</dt><dd>${escapeHtml(ctx.language || '')}</dd>
    </dl>
    <div class="section-heading">Project</div>
    <dl class="kv">
      <dt>ID</dt><dd>${escapeHtml(proj.id || '')}</dd>
      <dt>Name</dt><dd>${escapeHtml(proj.name || '')}</dd>
    </dl>
    <div class="section-heading">Lifecycle</div>
    <dl class="kv">
      <dt>Current</dt><dd>${escapeHtml(lc.current || '')}</dd>
    </dl>
  `);
}

function renderCheck(d) {
  if (!d) {
    setBody('check-body', `<div class="empty">Click <b>Run check</b> to evaluate <code>checks.yaml</code>.</div>`);
    return;
  }
  const summary = d.checks || { total: 0, requiredFailed: 0, optionalFailed: 0 };
  const lc = d.lifecycle || {};
  const results = (d.results || []);
  const grouped = groupBy(results, (r) => r.phase || '(default)');
  const lines = [];
  lines.push(`<div class="section-heading">Summary</div>`);
  lines.push(`<dl class="kv">
    <dt>Total</dt><dd>${summary.total}</dd>
    <dt>Required failed</dt><dd>${summary.requiredFailed}</dd>
    <dt>Optional failed</dt><dd>${summary.optionalFailed}</dd>
    <dt>Lifecycle</dt><dd>${escapeHtml(lc.from || '')} → ${escapeHtml(lc.to || '')}</dd>
  </dl>`);
  for (const [phase, rows] of Object.entries(grouped)) {
    lines.push(`<div class="section-heading">[${escapeHtml(phase)}]</div>`);
    for (const r of rows) {
      const cls = r.passed ? 'passed' : (r.required ? 'failed-required' : 'failed-optional');
      const mark = r.passed ? '✓' : (r.required ? '✗' : '!');
      lines.push(`<div class="check-row ${cls}">
        <div class="mark">${mark}</div>
        <div>${escapeHtml(r.id)}</div>
        <div class="meta">${escapeHtml(r.type)}${r.required ? ' · required' : ''}</div>
      </div>`);
    }
  }
  setBody('check-body', lines.join(''));
}

function renderHistory(events) {
  if (!events || events.length === 0) {
    setBody('history-body', '<div class="empty">No events yet.</div>');
    return;
  }
  const out = [];
  for (const e of events.slice().reverse()) {
    const ts = (e.at || e.startedAt || '').replace('T', ' ').slice(0, 19);
    const type = e.type || 'event';
    const typeCls = type === 'lifecycle' ? 'type-lifecycle' : 'type-cmd';
    let body = '';
    if (type === 'lifecycle') {
      body = `${escapeHtml(e.from)} → ${escapeHtml(e.to)}` + (e.reason ? ` <span class="meta">(${escapeHtml(e.reason)})</span>` : '');
    } else {
      const status = e.exitCode !== undefined ? ` exit=${e.exitCode}` : '';
      body = `${escapeHtml(e.command || '?')}${status} <span class="meta">${escapeHtml(e.summary || '')}</span>`;
    }
    out.push(`<div class="history-event">
      <div class="ts">${escapeHtml(ts)}</div>
      <div class="${typeCls}">${escapeHtml(type)}</div>
      <div>${body}</div>
    </div>`);
  }
  setBody('history-body', out.join(''));
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
      out.push(`<div class="muted">(none declared)</div>`);
      continue;
    }
    out.push(`<table class="toolchain-table">
      <thead><tr><th>Name</th><th>Source</th><th>Path</th><th>Version</th><th>Status</th></tr></thead>
      <tbody>${rows.map((r) => `
        <tr>
          <td>${escapeHtml(r.name)}</td>
          <td>${escapeHtml(r.source || '')}</td>
          <td>${escapeHtml(r.path || '')}</td>
          <td>${escapeHtml(r.version || '')}</td>
          <td class="${r.available ? 'ok' : 'missing'}">${r.available ? 'ok' : 'missing'}</td>
        </tr>`).join('')}
      </tbody>
    </table>`);
  }
  setBody('toolchain-body', out.join(''));
}

async function onPickWorkbench() {
  try {
    const dir = await window.forgeAPI.pickWorkbench();
    if (!dir) return;
    // Try work init; if it already exists this is a no-op + a notice.
    try { await window.forgeAPI.initWorkbench(dir); }
    catch (e) { /* probably already exists; refresh anyway */ }
    showToast(`Workbench: ${dir}`);
    await refresh();
  } catch (err) { showError(err.message || String(err)); }
}

// ---------- helpers ----------

function groupBy(arr, fn) {
  const m = {};
  for (const item of arr) {
    const k = fn(item);
    (m[k] = m[k] || []).push(item);
  }
  return m;
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
