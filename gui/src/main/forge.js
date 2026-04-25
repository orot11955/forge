// Forge CLI invocation helper.
//
// `forge` reports failures via exit codes (e.g. exit 4 when required checks
// fail) but still emits its `--json` payload on stdout. We must therefore
// preserve the exit code rather than treating it as a thrown error — the
// renderer needs the JSON to render the failures, and `requiredPassed=false`
// is part of normal product behavior, not a crash.

'use strict';

const path = require('node:path');
const fs = require('node:fs');
const { execFile, spawn } = require('node:child_process');

// Resolve the forge binary:
//   1. FORGE_BIN env var
//   2. packaged extraResource: resources/bin/forge(.exe)
//   3. gui/resources/bin/forge(.exe) staged by `make gui-stage-core`
//   4. repo ./bin/forge(.exe) from local `make build`
//   5. fall back to PATH lookup ("forge")
function resolveBinary() {
  if (process.env.FORGE_BIN) return process.env.FORGE_BIN;
  const ext = process.platform === 'win32' ? '.exe' : '';
  const candidates = [
    process.resourcesPath && path.join(process.resourcesPath, 'bin', `forge${ext}`),
    path.join(__dirname, '..', '..', 'resources', 'bin', `forge${ext}`),
    path.join(__dirname, '..', '..', '..', 'bin', `forge${ext}`),
    path.join(__dirname, '..', '..', '..', 'bin', 'forge'),
  ].filter(Boolean);
  for (const candidate of candidates) {
    if (fs.existsSync(candidate)) return candidate;
  }
  return 'forge';
}

// run returns { stdout, stderr, exitCode }. It only rejects when the binary
// cannot be spawned (ENOENT / ENOEXEC / EACCES), triggering a PATH fallback.
function run(args, opts = {}) {
  const env = { ...process.env, ...(opts.env || {}) };
  const cwd = opts.cwd;
  const isSpawnFailure = (e) => e && (e.code === 'ENOENT' || e.code === 'ENOEXEC' || e.code === 'EACCES');
  const exec = (binary) => new Promise((resolve, reject) => {
    execFile(binary, args, { env, cwd, maxBuffer: 8 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (isSpawnFailure(err)) return reject(err);
      resolve({
        stdout: String(stdout || ''),
        stderr: String(stderr || ''),
        exitCode: err && typeof err.code === 'number' ? err.code : (err ? -1 : 0),
      });
    });
  });
  return exec(resolveBinary()).catch((err) => {
    if (isSpawnFailure(err)) return exec('forge'); // PATH fallback
    throw err;
  });
}

// callJSON runs `args + --json` and returns the parsed payload. If the CLI
// emitted a non-empty stdout we attempt to parse it regardless of exit code,
// because Forge is designed to emit JSON even on failure (e.g. exit 4 with
// `requiredPassed: false`). The caller can inspect `__exitCode`/`__stderr`
// added to the parsed object when present.
async function callJSON(args, opts = {}) {
  const fullArgs = [...args, '--json'];
  if (opts.workbench) fullArgs.push('--workbench', opts.workbench);
  if (opts.lang)      fullArgs.push('--lang', opts.lang);

  const { stdout, stderr, exitCode } = await run(fullArgs, opts);
  if (!stdout.trim()) {
    if (exitCode === 0) return null;
    const err = new Error((stderr || `forge exited ${exitCode}`).trim());
    err.exitCode = exitCode;
    err.stderr = stderr;
    throw err;
  }
  let data;
  try { data = JSON.parse(stdout); }
  catch (parseErr) {
    const err = new Error(`forge ${args.join(' ')} returned non-JSON: ${parseErr.message}\n${stdout.slice(0, 200)}`);
    err.exitCode = exitCode;
    err.stderr = stderr;
    throw err;
  }
  if (data && typeof data === 'object') {
    Object.defineProperty(data, '__exitCode', { value: exitCode, enumerable: false });
    if (stderr) Object.defineProperty(data, '__stderr', { value: stderr, enumerable: false });
  }
  return data;
}

const callIn = (cwd, args) => callJSON(args, { cwd });

// callPlain runs without --json (some commands produce only human output,
// e.g. `forge config language <code>`). Returns { stdout, stderr, exitCode }.
async function callPlain(args, opts = {}) {
  return run(args, opts);
}

// streamRun spawns the forge binary and streams stdout/stderr to the
// onEvent callback. Returns a controller with { kill, promise }. Used by
// `forge run <script>` so the renderer can show live output.
function streamRun(args, opts = {}, onEvent = () => {}) {
  const env = { ...process.env, ...(opts.env || {}) };
  const cwd = opts.cwd;

  const emit = (kind, data) => {
    try { onEvent({ kind, data }); } catch { /* swallow */ }
  };

  const isSpawnErr = (e) => e && (e.code === 'ENOENT' || e.code === 'ENOEXEC' || e.code === 'EACCES');

  function spawnBinary(binary) {
    const child = spawn(binary, args, { env, cwd });
    child.stdout.on('data', (chunk) => emit('stdout', chunk.toString('utf8')));
    child.stderr.on('data', (chunk) => emit('stderr', chunk.toString('utf8')));
    return child;
  }

  let child = spawnBinary(resolveBinary());

  const killRef = { fn: (sig) => { try { child.kill(sig); } catch { /* swallow */ } } };

  const promise = new Promise((resolve) => {
    const attach = (c, allowRetry) => {
      c.on('close', (code, signal) => {
        emit('exit', { code, signal });
        resolve({ code, signal });
      });
      c.on('error', (err) => {
        if (allowRetry && isSpawnErr(err)) {
          child = spawnBinary('forge');
          killRef.fn = (sig) => { try { child.kill(sig); } catch { /* swallow */ } };
          attach(child, false);
          return;
        }
        emit('error', { message: err.message, code: err.code });
        resolve({ code: -1, signal: null, error: err });
      });
    };
    attach(child, true);
  });

  return {
    get pid() { return child.pid; },
    kill: (sig = 'SIGTERM') => killRef.fn(sig),
    promise,
  };
}

module.exports = {
  resolveBinary,
  version:    () => callJSON(['version']),
  list:       (opts) => callJSON(['list'], opts),
  doctor:     (opts) => callJSON(['doctor'], opts),
  workStatus: (opts) => callJSON(['work', 'status'], opts),
  workInit:   (dir)  => callJSON(['work', 'init', dir]),
  status:     (projectPath, opts) => callIn(projectPath, ['status']),
  check:      (projectPath, opts) => callIn(projectPath, ['check', ...(opts && opts.update ? [] : ['--no-update'])]),
  toolShow:   (projectPath, opts) => callIn(projectPath, ['tool', 'show']),
  scripts:    (projectPath) => callIn(projectPath, ['run']),
  logs:       (projectPath, tail) => callIn(projectPath, ['logs', ...(tail ? ['--tail', String(tail)] : [])]),
  projectInit: (projectPath, opts) => callIn(projectPath, ['init']),
  setLanguage: (lang) => callPlain(['config', 'language', lang]),
  configPath:  () => callJSON(['config', 'path']),
  runScript:   (projectPath, name, onEvent) => streamRun(['run', name], { cwd: projectPath }, onEvent),
};
