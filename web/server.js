const express = require('express');
const http = require('http');
const { WebSocketServer } = require('ws');
const path = require('path');
const fs = require('fs');
const { execFileSync } = require('child_process');

let pty;
try {
  pty = require('node-pty');
} catch {
  console.error('Error: node-pty not installed. Run: cd web && npm install');
  process.exit(1);
}

// ── Config ──────────────────────────────────────────────────────────────────

const PORT = parseInt(process.env.CROSSAGENT_PORT || '3456', 10);
const CROSSAGENT_BIN = process.env.CROSSAGENT_BIN || path.resolve(__dirname, '..', 'crossagent');
const CROSSAGENT_HOME = process.env.CROSSAGENT_HOME || path.join(require('os').homedir(), '.crossagent');

// ── Validation ──────────────────────────────────────────────────────────────

const VALID_PHASES = new Set(['plan', 'review', 'implement', 'verify']);
const VALID_ARTIFACTS = new Set(['plan', 'review', 'implement', 'verify', 'memory']);
const NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

function validateName(name) {
  if (!name || typeof name !== 'string') return false;
  if (name.length > 128) return false;
  return NAME_RE.test(name);
}

// ── Helpers ─────────────────────────────────────────────────────────────────

function crossagent(...args) {
  try {
    const result = execFileSync(CROSSAGENT_BIN, args, {
      encoding: 'utf-8',
      timeout: 10000,
      env: { ...process.env, CROSSAGENT_HOME },
    });
    return result.trim();
  } catch (err) {
    throw new Error(err.stderr || err.message);
  }
}

function crossagentJSON(...args) {
  const raw = crossagent(...args);
  try {
    return JSON.parse(raw);
  } catch {
    throw new Error(`Invalid JSON from crossagent: ${raw.slice(0, 200)}`);
  }
}

function readArtifact(workflowDir, name) {
  const filePath = path.join(workflowDir, `${name}.md`);
  if (!fs.existsSync(filePath)) return null;
  return fs.readFileSync(filePath, 'utf-8');
}

// ── Express App ─────────────────────────────────────────────────────────────

const app = express();
app.use(express.json());
app.use(express.static(path.join(__dirname, 'public')));

// API: Get workflow status
app.get('/api/status', (_req, res) => {
  try {
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.json({ error: err.message });
  }
});

// API: List workflows
app.get('/api/list', (_req, res) => {
  try {
    res.json(crossagentJSON('list', '--json'));
  } catch (err) {
    res.json({ error: err.message, workflows: [], active: '' });
  }
});

// API: Get launch params for a phase
app.get('/api/phase-cmd/:phase', (req, res) => {
  const phase = req.params.phase;
  if (!VALID_PHASES.has(phase)) {
    return res.status(400).json({ error: `Invalid phase: ${phase}` });
  }
  try {
    const args = ['phase-cmd', phase, '--json'];
    if (req.query.subphase) {
      const sub = String(req.query.subphase);
      if (!/^\d+$/.test(sub)) {
        return res.status(400).json({ error: 'subphase must be a number' });
      }
      args.push('--phase', sub);
    }
    if (req.query.force === 'true') {
      args.push('--force');
    }
    res.json(crossagentJSON(...args));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Read an artifact
app.get('/api/artifact/:type', (req, res) => {
  const type = req.params.type;
  if (!VALID_ARTIFACTS.has(type)) {
    return res.status(400).json({ error: `Invalid artifact type: ${type}` });
  }
  try {
    const status = crossagentJSON('status', '--json');
    if (status.error) return res.status(404).json({ error: status.error });
    const content = readArtifact(status.workflow_dir, type);
    if (content === null) return res.status(404).json({ error: 'Artifact not found' });
    res.json({ content, path: path.join(status.workflow_dir, `${type}.md`) });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// API: Switch workflow
app.post('/api/use/:name', (req, res) => {
  const name = req.params.name;
  if (!validateName(name)) {
    return res.status(400).json({ error: 'Invalid workflow name' });
  }
  try {
    crossagent('use', name);
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Advance phase
app.post('/api/advance', (_req, res) => {
  try {
    crossagent('advance');
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Check if a file exists (used for live output polling)
app.post('/api/check-file', (req, res) => {
  const { path: filePath } = req.body;
  if (!filePath || typeof filePath !== 'string') {
    return res.status(400).json({ error: 'path required' });
  }
  res.json({ exists: fs.existsSync(filePath) });
});

// API: Check if phase output exists and auto-advance
app.post('/api/check-advance', (req, res) => {
  const { output_file } = req.body;
  if (!output_file || typeof output_file !== 'string') {
    return res.status(400).json({ error: 'output_file required' });
  }
  try {
    const exists = fs.existsSync(output_file);
    if (exists) {
      crossagent('advance');
      const status = crossagentJSON('status', '--json');
      res.json({ advanced: true, status });
    } else {
      res.json({ advanced: false });
    }
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Mark done
app.post('/api/done', (_req, res) => {
  try {
    crossagent('done');
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Create new workflow
app.post('/api/new', (req, res) => {
  const { name, repo, addDirs, description } = req.body;
  if (!name || !description) {
    return res.status(400).json({ error: 'name and description required' });
  }
  if (!validateName(name)) {
    return res.status(400).json({ error: 'Invalid workflow name. Use alphanumeric characters, hyphens, underscores, and dots.' });
  }
  try {
    const args = ['new', name];
    if (repo) args.push('--repo', repo);
    if (addDirs && Array.isArray(addDirs)) {
      addDirs.forEach(d => args.push('--add-dir', d));
    }
    execFileSync(CROSSAGENT_BIN, args, {
      encoding: 'utf-8',
      timeout: 10000,
      input: description,
      env: { ...process.env, CROSSAGENT_HOME },
    });
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.stderr || err.message });
  }
});

// API: Add a directory to the workflow
app.post('/api/repos/add', (req, res) => {
  const { path: dirPath } = req.body;
  if (!dirPath || typeof dirPath !== 'string') {
    return res.status(400).json({ error: 'path required' });
  }
  try {
    crossagent('repos', 'add', dirPath);
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Remove a directory from the workflow
app.post('/api/repos/remove', (req, res) => {
  const { path: dirPath } = req.body;
  if (!dirPath || typeof dirPath !== 'string') {
    return res.status(400).json({ error: 'path required' });
  }
  try {
    crossagent('repos', 'remove', dirPath);
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Supervise — evaluate phase output and decide next action
app.post('/api/supervise', (req, res) => {
  try {
    const args = ['supervise', '--json'];
    if (req.body && req.body.phase) {
      args.push('--phase', String(req.body.phase));
    }
    res.json(crossagentJSON(...args));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Revert to a previous phase
app.post('/api/revert', (req, res) => {
  const { target_phase, reason } = req.body;
  if (!target_phase || !/^[1-4]$/.test(String(target_phase))) {
    return res.status(400).json({ error: 'target_phase must be 1-4' });
  }
  try {
    const args = ['revert', String(target_phase), '--json'];
    if (reason) args.push('--reason', String(reason));
    res.json(crossagentJSON(...args));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// ── HTTP + WebSocket Server ─────────────────────────────────────────────────

const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: '/ws/terminal' });

wss.on('connection', (ws) => {
  let ptyProcess = null;

  ws.on('error', (err) => {
    console.error('WebSocket error:', err.message);
  });

  ws.on('message', (raw) => {
    let msg;
    try {
      msg = JSON.parse(raw);
    } catch {
      ws.send(JSON.stringify({ type: 'error', message: 'Invalid JSON' }));
      return;
    }

    switch (msg.type) {
      case 'spawn': {
        if (ptyProcess) {
          ptyProcess.kill();
          ptyProcess = null;
        }

        const { command, args, cwd } = msg;
        if (!command) {
          ws.send(JSON.stringify({ type: 'error', message: 'Missing command' }));
          return;
        }

        try {
          // Build a clean environment for spawned agents.
          // Remove Claude Code internal vars so spawned claude sessions
          // don't detect themselves as nested and refuse to start or
          // skip the initial prompt.
          const spawnEnv = { ...process.env, TERM: 'xterm-256color' };
          delete spawnEnv.CLAUDECODE;
          delete spawnEnv.CLAUDE_CODE_ENTRYPOINT;

          ptyProcess = pty.spawn(command, args || [], {
            name: 'xterm-256color',
            cols: Math.min(Math.max(msg.cols || 120, 10), 500),
            rows: Math.min(Math.max(msg.rows || 30, 5), 200),
            cwd: cwd || process.env.HOME,
            env: spawnEnv,
          });

          ws.send(JSON.stringify({ type: 'spawned', pid: ptyProcess.pid }));

          ptyProcess.onData((data) => {
            if (ws.readyState === ws.OPEN) {
              ws.send(JSON.stringify({ type: 'output', data }));
            }
          });

          ptyProcess.onExit(({ exitCode }) => {
            if (ws.readyState === ws.OPEN) {
              ws.send(JSON.stringify({ type: 'exit', code: exitCode }));
            }
            ptyProcess = null;
          });
        } catch (err) {
          ws.send(JSON.stringify({ type: 'error', message: err.message }));
        }
        break;
      }

      case 'input': {
        if (ptyProcess) {
          ptyProcess.write(msg.data);
        }
        break;
      }

      case 'kill': {
        if (ptyProcess) {
          ptyProcess.kill();
          ptyProcess = null;
        }
        break;
      }

      case 'resize': {
        if (ptyProcess && msg.cols && msg.rows) {
          const cols = Math.min(Math.max(msg.cols, 10), 500);
          const rows = Math.min(Math.max(msg.rows, 5), 200);
          ptyProcess.resize(cols, rows);
        }
        break;
      }

      default:
        ws.send(JSON.stringify({ type: 'error', message: `Unknown type: ${msg.type}` }));
    }
  });

  ws.on('close', () => {
    if (ptyProcess) {
      ptyProcess.kill();
      ptyProcess = null;
    }
  });
});

// ── Graceful Shutdown ────────────────────────────────────────────────────────

function shutdown() {
  console.log('\n  Shutting down...');
  wss.clients.forEach((ws) => ws.close());
  server.close(() => process.exit(0));
  setTimeout(() => process.exit(1), 3000);
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

// ── Start ───────────────────────────────────────────────────────────────────

server.listen(PORT, () => {
  console.log(`\n  Crossagent UI running at http://localhost:${PORT}\n`);
});
