// Forge GUI preload — exposes the safe IPC surface to the renderer via
// contextBridge. The renderer never sees `require`, `process`, or `fs`.

'use strict';

const { contextBridge, ipcRenderer } = require('electron');

const api = {
  version:      () => ipcRenderer.invoke('forge:version'),
  list:         (opts) => ipcRenderer.invoke('forge:list', opts || {}),
  status:       (projectPath, opts) => ipcRenderer.invoke('forge:status', projectPath, opts || {}),
  check:        (projectPath, opts) => ipcRenderer.invoke('forge:check', projectPath, opts || {}),
  doctor:       (opts) => ipcRenderer.invoke('forge:doctor', opts || {}),
  toolShow:     (projectPath, opts) => ipcRenderer.invoke('forge:toolShow', projectPath, opts || {}),
  workStatus:   (opts) => ipcRenderer.invoke('forge:workStatus', opts || {}),
  historyTail:  (projectPath, lines) => ipcRenderer.invoke('history:tail', projectPath, lines || 50),
  pickWorkbench: () => ipcRenderer.invoke('workbench:pick'),
  initWorkbench: (dir) => ipcRenderer.invoke('workbench:init', dir),
  openInShell:   (target) => ipcRenderer.invoke('shell:open', target),
};

contextBridge.exposeInMainWorld('forgeAPI', api);
