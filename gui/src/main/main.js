// Forge GUI — Electron main process.
//
// Design (Stage 11):
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

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1180,
    height: 760,
    minWidth: 900,
    minHeight: 600,
    title: 'Forge',
    backgroundColor: '#101216',
    webPreferences: {
      preload: path.join(__dirname, '..', 'preload', 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  });
  mainWindow.loadFile(path.join(__dirname, '..', 'renderer', 'index.html'));
  mainWindow.on('closed', () => { mainWindow = null; });
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
  ipcMain.handle('forge:list',     async (_e, opts)               => forge.list(opts || {}));
  ipcMain.handle('forge:status',   async (_e, projectPath, opts)  => forge.status(projectPath, opts || {}));
  ipcMain.handle('forge:check',    async (_e, projectPath, opts)  => forge.check(projectPath, opts || {}));
  ipcMain.handle('forge:doctor',   async (_e, opts)               => forge.doctor(opts || {}));
  ipcMain.handle('forge:toolShow', async (_e, projectPath, opts)  => forge.toolShow(projectPath, opts || {}));
  ipcMain.handle('forge:workStatus', async (_e, opts)             => forge.workStatus(opts || {}));
  ipcMain.handle('forge:version',  async ()                       => forge.version());

  // Filesystem-side reads (history.jsonl) so the renderer never touches disk.
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
