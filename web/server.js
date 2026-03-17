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
app.use('/assets', express.static(path.join(__dirname, '..', 'assets')));

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
  const { name, repo, addDirs, description, project } = req.body;
  if (!name || !description) {
    return res.status(400).json({ error: 'name and description required' });
  }
  if (!validateName(name)) {
    return res.status(400).json({ error: 'Invalid workflow name. Use alphanumeric characters, hyphens, underscores, and dots.' });
  }
  try {
    const args = ['new', name];
    if (repo) args.push('--repo', repo);
    if (project && validateName(project)) args.push('--project', project);
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

// ── Project API ──────────────────────────────────────────────────────────────

// API: List projects
app.get('/api/projects', (_req, res) => {
  try {
    res.json(crossagentJSON('projects', 'list', '--json'));
  } catch (err) {
    res.json({ error: err.message, projects: [] });
  }
});

// API: Create project
app.post('/api/projects/new', (req, res) => {
  const { name } = req.body;
  if (!name || !validateName(name)) {
    return res.status(400).json({ error: 'Invalid project name' });
  }
  try {
    crossagent('projects', 'new', name);
    res.json(crossagentJSON('projects', 'list', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Delete project
app.post('/api/projects/delete', (req, res) => {
  const { name } = req.body;
  if (!name || !validateName(name)) {
    return res.status(400).json({ error: 'Invalid project name' });
  }
  try {
    crossagent('projects', 'delete', name);
    res.json(crossagentJSON('projects', 'list', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Show project details
app.get('/api/projects/:name', (req, res) => {
  const name = req.params.name;
  if (!validateName(name)) {
    return res.status(400).json({ error: 'Invalid project name' });
  }
  try {
    res.json(crossagentJSON('projects', 'show', name, '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Rename project
app.post('/api/projects/rename', (req, res) => {
  const { old_name, new_name } = req.body;
  if (!old_name || !validateName(old_name) || !new_name || !validateName(new_name)) {
    return res.status(400).json({ error: 'Invalid project name(s)' });
  }
  try {
    crossagent('projects', 'rename', old_name, new_name);
    res.json(crossagentJSON('projects', 'list', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Move workflow to project
app.post('/api/move', (req, res) => {
  const { workflow, project } = req.body;
  if (!workflow || !validateName(workflow) || !project || !validateName(project)) {
    return res.status(400).json({ error: 'Invalid workflow or project name' });
  }
  try {
    crossagent('move', workflow, '--project', project);
    res.json(crossagentJSON('status', '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
  }
});

// API: Suggest project for a description
app.post('/api/suggest-project', (req, res) => {
  const { description } = req.body;
  if (!description || typeof description !== 'string') {
    return res.status(400).json({ error: 'description required' });
  }
  try {
    res.json(crossagentJSON('projects', 'suggest', '--description', description, '--json'));
  } catch (err) {
    res.status(400).json({ error: err.message });
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

// ── Chat History Capture ────────────────────────────────────────────────────

const CHAT_HISTORY_BUFFER_CAP = 50 * 1024 * 1024; // 50MB cap per session

// API: Read chat history for a phase
app.get('/api/chat-history/:phase', (req, res) => {
  const phase = req.params.phase;
  if (!VALID_PHASES.has(phase)) {
    return res.status(400).json({ error: `Invalid phase: ${phase}` });
  }
  try {
    const status = crossagentJSON('status', '--json');
    if (status.error) return res.status(404).json({ error: status.error });
    const logPath = path.join(status.workflow_dir, 'chat-history', `${phase}.log`);
    if (!fs.existsSync(logPath)) {
      return res.json({ exists: false });
    }
    const stat = fs.statSync(logPath);
    // Stream large files instead of loading into memory
    if (stat.size > 5 * 1024 * 1024) {
      res.setHeader('Content-Type', 'application/json');
      res.json({ exists: true, path: logPath, size: stat.size, large: true });
      return;
    }
    const content = fs.readFileSync(logPath, 'utf-8');
    res.json({ exists: true, content, path: logPath, size: stat.size });
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// Stream endpoint for large chat history files
app.get('/api/chat-history/:phase/stream', (req, res) => {
  const phase = req.params.phase;
  if (!VALID_PHASES.has(phase)) {
    return res.status(400).json({ error: `Invalid phase: ${phase}` });
  }
  try {
    const status = crossagentJSON('status', '--json');
    if (status.error) return res.status(404).json({ error: status.error });
    const logPath = path.join(status.workflow_dir, 'chat-history', `${phase}.log`);
    if (!fs.existsSync(logPath)) {
      return res.status(404).json({ error: 'Chat history not found' });
    }
    res.setHeader('Content-Type', 'text/plain; charset=utf-8');
    fs.createReadStream(logPath).pipe(res);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

// ── HTTP + WebSocket Server ─────────────────────────────────────────────────

const server = http.createServer(app);
const wss = new WebSocketServer({ server, path: '/ws/terminal' });

wss.on('connection', (ws) => {
  let ptyProcess = null;
  let chatBuffer = [];          // Buffered PTY output chunks
  let chatBufferSize = 0;       // Total bytes buffered
  let chatWorkflowDir = null;   // Workflow dir for this session
  let chatPhaseName = null;     // Phase name for this session
  let chatFlushed = false;      // Whether we already flushed to disk

  // Flush chat history buffer to disk. Called on exit, kill, or disconnect.
  // Safe to call multiple times — only the first call writes.
  function flushChatHistory() {
    if (chatFlushed) return;
    chatFlushed = true;

    if (!chatWorkflowDir || !chatPhaseName || chatBuffer.length === 0) return;

    const dir = path.join(chatWorkflowDir, 'chat-history');
    const logFile = path.join(dir, `${chatPhaseName}.log`);
    const tmpFile = logFile + '.tmp';

    try {
      fs.mkdirSync(dir, { recursive: true });
      fs.writeFileSync(tmpFile, chatBuffer.join(''), 'utf-8');
      fs.renameSync(tmpFile, logFile);
    } catch (err) {
      console.error(`Failed to write chat history: ${err.message}`);
      // Clean up temp file if rename failed
      try { fs.unlinkSync(tmpFile); } catch {}
    }

    // Release buffer memory
    chatBuffer = [];
    chatBufferSize = 0;
  }

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
        // Flush any previous session's chat history before replacing
        flushChatHistory();

        if (ptyProcess) {
          ptyProcess.kill();
          ptyProcess = null;
        }

        const { command, args, cwd } = msg;
        if (!command) {
          ws.send(JSON.stringify({ type: 'error', message: 'Missing command' }));
          return;
        }

        // Reset chat history state for new session
        chatBuffer = [];
        chatBufferSize = 0;
        chatFlushed = false;

        // Derive workflowDir from CLI status (never trust client payload)
        // and validate phaseName against the whitelist.
        chatWorkflowDir = null;
        chatPhaseName = null;
        if (msg.phaseName && VALID_PHASES.has(msg.phaseName)) {
          try {
            const status = crossagentJSON('status', '--json');
            if (status.workflow_dir) {
              chatWorkflowDir = status.workflow_dir;
              chatPhaseName = msg.phaseName;
            }
          } catch {
            // If status lookup fails, skip chat history capture (non-blocking)
          }
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
            // Buffer output for chat history (PTY echo includes user input)
            if (chatWorkflowDir && chatPhaseName && chatBufferSize < CHAT_HISTORY_BUFFER_CAP) {
              chatBuffer.push(data);
              chatBufferSize += Buffer.byteLength(data, 'utf-8');
            }
          });

          ptyProcess.onExit(({ exitCode }) => {
            flushChatHistory();
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
          // Also capture input for non-echoed cases (e.g. password prompts).
          // For echoed input this creates minor duplication in the log, but
          // ensures the full session transcript is preserved regardless of
          // PTY echo mode.
          if (chatWorkflowDir && chatPhaseName && chatBufferSize < CHAT_HISTORY_BUFFER_CAP) {
            chatBuffer.push(msg.data);
            chatBufferSize += Buffer.byteLength(msg.data, 'utf-8');
          }
        }
        break;
      }

      case 'kill': {
        if (ptyProcess) {
          flushChatHistory();
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
    flushChatHistory();
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
