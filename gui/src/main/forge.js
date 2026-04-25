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
const { execFile } = require('node:child_process');

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
// itself cannot be spawned (ENOENT etc.).
function run(args, opts = {}) {
  const env = { ...process.env, ...(opts.env || {}) };
  const cwd = opts.cwd;
  const exec = (binary) => new Promise((resolve, reject) => {
    execFile(binary, args, { env, cwd, maxBuffer: 8 * 1024 * 1024 }, (err, stdout, stderr) => {
      if (err && err.code === 'ENOENT') return reject(err);
      resolve({
        stdout: String(stdout || ''),
        stderr: String(stderr || ''),
        exitCode: err && typeof err.code === 'number' ? err.code : (err ? -1 : 0),
      });
    });
  });
  return exec(resolveBinary()).catch((err) => {
    if (err && err.code === 'ENOENT') return exec('forge'); // PATH fallback
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

module.exports = {
  resolveBinary,
  version:    () => callJSON(['version']),
  list:       (opts) => callJSON(['list'], opts),
  doctor:     (opts) => callJSON(['doctor'], opts),
  workStatus: (opts) => callJSON(['work', 'status'], opts),
  workInit:   (dir)  => callJSON(['work', 'init', dir]),
  status:     (projectPath) => callIn(projectPath, ['status']),
  check:      (projectPath) => callIn(projectPath, ['check', '--no-update']),
  toolShow:   (projectPath) => callIn(projectPath, ['tool', 'show']),
};
