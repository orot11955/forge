// Forge GUI preload — exposes the safe IPC surface to the renderer via
// contextBridge. The renderer never sees `require`, `process`, or `fs`.

'use strict';

const { contextBridge, ipcRenderer } = require('electron');

const api = {
  version:       () => ipcRenderer.invoke('forge:version'),
  list:          (opts) => ipcRenderer.invoke('forge:list', opts || {}),
  status:        (p, opts) => ipcRenderer.invoke('forge:status', p, opts || {}),
  check:         (p, opts) => ipcRenderer.invoke('forge:check', p, opts || {}),
  doctor:        (opts) => ipcRenderer.invoke('forge:doctor', opts || {}),
  toolShow:      (p, opts) => ipcRenderer.invoke('forge:toolShow', p, opts || {}),
  workStatus:    (opts) => ipcRenderer.invoke('forge:workStatus', opts || {}),
  scripts:       (p) => ipcRenderer.invoke('forge:scripts', p),
  logs:          (p, tail) => ipcRenderer.invoke('forge:logs', p, tail || 200),
  projectInit:   (p) => ipcRenderer.invoke('forge:projectInit', p),
  configPath:    () => ipcRenderer.invoke('forge:configPath'),
  setLanguage:   (lang) => ipcRenderer.invoke('forge:setLanguage', lang),
  historyTail:   (p, lines) => ipcRenderer.invoke('history:tail', p, lines || 50),
  pickWorkbench: () => ipcRenderer.invoke('workbench:pick'),
  initWorkbench: (dir) => ipcRenderer.invoke('workbench:init', dir),
  pickProject:   () => ipcRenderer.invoke('project:pick'),
  openInShell:   (target) => ipcRenderer.invoke('shell:open', target),

  // Streaming script runs.
  runStart: (p, name) => ipcRenderer.invoke('run:start', p, name),
  runStop:  (runId) => ipcRenderer.invoke('run:stop', runId),
  onRunEvent: (cb) => {
    const listener = (_e, payload) => cb(payload);
    ipcRenderer.on('run:event', listener);
    return () => ipcRenderer.removeListener('run:event', listener);
  },
};

contextBridge.exposeInMainWorld('forgeAPI', api);
