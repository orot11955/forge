// Forge GUI — Electron main process.
//
// Design:
//   - The CLI is the source of truth. The GUI never re-implements logic.
//   - Every read goes through `forge --json`; mutating actions also delegate
//     to the CLI so behavior stays identical to the terminal.
//   - The renderer never spawns processes directly; all child_process calls
//     are gated through ipcMain handlers exposed by preload.js.

'use strict';

const path = require('node:path');
const fs = require('node:fs');
const os = require('node:os');
const { app, BrowserWindow, dialog, ipcMain, shell } = require('electron');
const forge = require('./forge');

if (require('electron-squirrel-startup')) {
  app.quit();
}

let mainWindow = null;
const runningScripts = new Map(); // runId -> controller

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1280,
    height: 820,
    minWidth: 980,
    minHeight: 640,
    title: 'Forge',
    backgroundColor: '#0b0d12',
    webPreferences: {
      preload: path.join(__dirname, '..', 'preload', 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  mainWindow.loadFile(path.join(__dirname, '..', 'renderer', 'index.html'));
  mainWindow.on('closed', () => {
    mainWindow = null;
    for (const ctrl of runningScripts.values()) ctrl.kill();
    runningScripts.clear();
  });
}

app.whenReady().then(() => {
  registerIpc();
  createWindow();
  app.on('activate', () => {
    if (BrowserWindow.getAllWindows().length === 0) createWindow();
  });
});

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit();
});

function registerIpc() {
  // Read-only forge calls — all return parsed JSON.
  ipcMain.handle('forge:list',       async (_e, opts)              => forge.list(opts || {}));
  ipcMain.handle('forge:status',     async (_e, p, opts)           => forge.status(p, opts || {}));
  ipcMain.handle('forge:check',      async (_e, p, opts)           => forge.check(p, opts || {}));
  ipcMain.handle('forge:doctor',     async (_e, opts)              => forge.doctor(opts || {}));
  ipcMain.handle('forge:toolShow',   async (_e, p, opts)           => forge.toolShow(p, opts || {}));
  ipcMain.handle('forge:workStatus', async (_e, opts)              => forge.workStatus(opts || {}));
  ipcMain.handle('forge:scripts',    async (_e, p)                 => forge.scripts(p));
  ipcMain.handle('forge:logs',       async (_e, p, tail)           => forge.logs(p, tail));
  ipcMain.handle('forge:projectInit', async (_e, p)                => forge.projectInit(p));
  ipcMain.handle('forge:configPath', async ()                      => forge.configPath());
  ipcMain.handle('forge:setLanguage', async (_e, lang)             => {
    const { stdout, stderr, exitCode } = await forge.setLanguage(lang);
    return { ok: exitCode === 0, stdout, stderr, exitCode };
  });
  ipcMain.handle('forge:version',    async ()                      => forge.version());

  // history.jsonl tail (filesystem read in main so renderer can't touch disk).
  ipcMain.handle('history:tail', async (_e, projectPath, lines = 50) => {
    return readHistoryTail(projectPath, lines);
  });

  // Workbench picking / init.
  ipcMain.handle('workbench:pick', async () => {
    const result = await dialog.showOpenDialog(mainWindow, {
      title: 'Select Workbench Root',
      properties: ['openDirectory', 'createDirectory'],
      defaultPath: os.homedir(),
    });
    if (result.canceled || !result.filePaths.length) return null;
    return result.filePaths[0];
  });
  ipcMain.handle('workbench:init', async (_e, dir) => forge.workInit(dir));

  // Project picking (for `forge init <existing project>`).
  ipcMain.handle('project:pick', async () => {
    const result = await dialog.showOpenDialog(mainWindow, {
      title: 'Select Project Directory',
      properties: ['openDirectory'],
      defaultPath: os.homedir(),
    });
    if (result.canceled || !result.filePaths.length) return null;
    return result.filePaths[0];
  });

  // Streaming run for `forge run <script>`.
  ipcMain.handle('run:start', async (event, projectPath, scriptName) => {
    const runId = `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
    const sender = event.sender;
    const ctrl = forge.runScript(projectPath, scriptName, (evt) => {
      if (sender.isDestroyed()) return;
      sender.send('run:event', { runId, ...evt });
    });
    runningScripts.set(runId, ctrl);
    ctrl.promise.finally(() => runningScripts.delete(runId));
    return { runId, pid: ctrl.pid };
  });
  ipcMain.handle('run:stop', async (_e, runId) => {
    const ctrl = runningScripts.get(runId);
    if (!ctrl) return false;
    ctrl.kill();
    return true;
  });

  // Open external links/paths in the system shell (Finder / Explorer / xdg).
  ipcMain.handle('shell:open', async (_e, target) => {
    if (typeof target !== 'string' || !target) return false;
    await shell.openPath(target);
    return true;
  });
}

function readHistoryTail(projectPath, lines) {
  if (!projectPath) return [];
  const file = path.join(projectPath, '.forge', 'history.jsonl');
  let raw;
  try {
    raw = fs.readFileSync(file, 'utf8');
  } catch (err) {
    if (err.code === 'ENOENT') return [];
    throw err;
  }
  const all = raw.split('\n').filter(Boolean);
  const tail = all.slice(-lines);
  return tail.map((line) => {
    try { return JSON.parse(line); } catch { return { raw: line, malformed: true }; }
  });
}
