// ── Crossagent UI ─────────────────────────────────────────────────────────────

const PHASE_NAMES = ['', 'plan', 'review', 'implement', 'verify'];

let state = null;
let ws = null;
let term = null;
let fitAddon = null;
let sessionActive = false;
let activeArtifact = null;
let resizeTimer = null;
let resizeFrame = null;
let lastReportedCols = 0;
let lastReportedRows = 0;
let syncOutputActive = false;
let syncOutputBuffer = '';
let syncOutputRemainder = '';
let pendingOutputFile = null;   // Track expected output file for auto-advance
let pendingPhaseName = null;    // Track which phase is running
let outputPollTimer = null;     // Poll for output file while session runs
let retryLoopActive = false;    // Whether we're in an autonomous retry loop
let projectsData = null;       // Cached projects list
let selectedProjectFilter = ''; // Current project filter in topbar
let viewingChatHistory = false; // Whether terminal is showing historical chat replay

// ── API ─────────────────────────────────────────────────────────────────────

async function api(path, opts = {}) {
  const res = await fetch(`/api${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  return res.json();
}

async function fetchStatus() {
  try {
    const data = await api('/status');
    if (data.error) {
      state = null;
      renderNoWorkflow();
      return data;
    }
    state = data;
    renderStatus();
    renderArtifactList();
    renderPhaseTracker();
    renderDirectories();
    renderInfo();
    updateRunButton();
    return data;
  } catch {
    state = null;
    renderNoWorkflow();
    return { error: 'Failed to fetch status' };
  }
}

async function fetchList() {
  try {
    const data = await api('/list');
    renderWorkflowSelect(data);
    return data;
  } catch {
    renderWorkflowSelect({ workflows: [] });
    return { workflows: [] };
  }
}

async function fetchProjects() {
  try {
    const data = await api('/projects');
    projectsData = data;
    renderProjectSelect(data);
    return data;
  } catch {
    projectsData = { projects: [] };
    renderProjectSelect({ projects: [] });
    return { projects: [] };
  }
}

function renderProjectSelect(data) {
  const sel = document.getElementById('project-select');
  const oldVal = sel.value;
  sel.innerHTML = '<option value="">All Projects</option>';
  if (data.projects && data.projects.length > 0) {
    data.projects.forEach(p => {
      const opt = document.createElement('option');
      opt.value = p.name;
      opt.textContent = `${p.name} (${p.workflow_count})`;
      sel.appendChild(opt);
    });
  }
  sel.value = oldVal || '';

  // Also update the new-workflow project dropdown
  const newSel = document.getElementById('new-project');
  if (newSel) {
    newSel.innerHTML = '';
    if (data.projects) {
      data.projects.forEach(p => {
        const opt = document.createElement('option');
        opt.value = p.name;
        opt.textContent = p.name;
        if (p.name === 'default') opt.selected = true;
        newSel.appendChild(opt);
      });
    }
  }
}

async function createProject(name) {
  try {
    const data = await api('/projects/new', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
    if (data.error) {
      alert(data.error);
      return;
    }
    await fetchProjects();
    renderProjectManager();
  } catch (err) {
    alert(err.message || 'Failed to create project');
  }
}

async function deleteProject(name) {
  if (!confirm(`Delete project "${name}"? Workflows will be moved to "default".`)) return;
  try {
    const data = await api('/projects/delete', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
    if (data.error) {
      alert(data.error);
      return;
    }
    await fetchProjects();
    await fetchList();
    await fetchStatus();
    renderProjectManager();
  } catch (err) {
    alert(err.message || 'Failed to delete project');
  }
}

async function renameProject(oldName) {
  const newName = prompt(`Rename project "${oldName}" to:`);
  if (!newName) return;
  try {
    const data = await api('/projects/rename', {
      method: 'POST',
      body: JSON.stringify({ old_name: oldName, new_name: newName }),
    });
    if (data.error) {
      alert(data.error);
      return;
    }
    await fetchProjects();
    await fetchList();
    await fetchStatus();
    renderProjectManager();
  } catch (err) {
    alert(err.message || 'Failed to rename project');
  }
}

async function moveWorkflow(workflow, project) {
  try {
    const data = await api('/move', {
      method: 'POST',
      body: JSON.stringify({ workflow, project }),
    });
    if (data.error) {
      alert(data.error);
      return;
    }
    await fetchProjects();
    await fetchList();
    await fetchStatus();
  } catch (err) {
    alert(err.message || 'Failed to move workflow');
  }
}

function renderProjectManager() {
  const el = document.getElementById('projects-list');
  if (!projectsData || !projectsData.projects || projectsData.projects.length === 0) {
    el.innerHTML = '<p class="muted">No projects</p>';
    return;
  }
  let html = '';
  projectsData.projects.forEach(p => {
    const isDefault = p.name === 'default';
    html += `<div class="project-row">
      <span class="project-name">${esc(p.name)}</span>
      <span class="project-count">${p.workflow_count} workflow(s)</span>
      <span class="project-actions">
        ${isDefault ? '' : `<button class="btn-icon project-rename" data-name="${esc(p.name)}" title="Rename">R</button>`}
        ${isDefault ? '' : `<button class="btn-icon project-delete" data-name="${esc(p.name)}" title="Delete">\u00d7</button>`}
      </span>
    </div>`;
  });
  el.innerHTML = html;

  // Bind events
  el.querySelectorAll('.project-rename').forEach(btn => {
    btn.addEventListener('click', () => renameProject(btn.dataset.name));
  });
  el.querySelectorAll('.project-delete').forEach(btn => {
    btn.addEventListener('click', () => deleteProject(btn.dataset.name));
  });
}

async function suggestProject(workflowName, description) {
  try {
    const data = await api('/suggest-project', {
      method: 'POST',
      body: JSON.stringify({ description }),
    });
    if (data.suggested_project) {
      // Return a promise that resolves when user clicks Move, Keep, or dismisses
      return new Promise((resolve) => {
        document.getElementById('suggest-project-name').textContent = data.suggested_project;
        document.getElementById('suggest-matched').textContent = 'Matched: ' + (data.matched_terms || '');
        document.getElementById('suggest-project-label').textContent = data.suggested_project;
        const modal = document.getElementById('suggest-modal');
        modal.classList.remove('hidden');

        const moveBtn = document.getElementById('suggest-move');
        const keepBtn = document.getElementById('suggest-keep');
        const backdrop = modal.querySelector('.modal-backdrop');
        let settled = false;

        const cleanup = () => {
          if (settled) return;
          settled = true;
          modal.classList.add('hidden');
          moveBtn.replaceWith(moveBtn.cloneNode(true));
          keepBtn.replaceWith(keepBtn.cloneNode(true));
          backdrop.removeEventListener('click', onDismiss);
          document.removeEventListener('keydown', onEscape, { capture: true });
        };

        const onDismiss = () => { cleanup(); resolve(); };

        const onEscape = (e) => {
          if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
            e.stopImmediatePropagation();
            cleanup();
            resolve();
          }
        };

        document.getElementById('suggest-move').addEventListener('click', async () => {
          cleanup();
          await moveWorkflow(workflowName, data.suggested_project);
          term.writeln(`\x1b[32m  Moved to project "${data.suggested_project}"\x1b[0m\r\n`);
          resolve();
        }, { once: true });

        document.getElementById('suggest-keep').addEventListener('click', () => {
          cleanup();
          resolve();
        }, { once: true });

        backdrop.addEventListener('click', onDismiss);
        document.addEventListener('keydown', onEscape, { capture: true });
      });
    }
  } catch {
    // Silent — suggestion is non-blocking
  }
}

// ── Render ──────────────────────────────────────────────────────────────────

function renderNoWorkflow() {
  document.title = 'Crossagent';
  document.querySelectorAll('.phase-item').forEach(el => {
    el.classList.remove('completed', 'current');
    el.classList.add('pending');
  });
  document.querySelectorAll('.artifact-item').forEach(el => {
    el.classList.remove('exists', 'active');
    el.querySelector('.artifact-icon').textContent = '-';
  });
  document.getElementById('workflow-info').innerHTML = '<p class="muted">No active workflow</p>';
  document.getElementById('dir-list').innerHTML = '<p class="muted dir-empty">No active workflow</p>';
  document.getElementById('add-dir-btn').style.display = 'none';
  const btn = document.getElementById('run-phase-btn');
  btn.textContent = 'Run Next Phase';
  btn.disabled = true;
  setGuide('Create or select a workflow to get started.');
}

function renderWorkflowSelect(data) {
  const sel = document.getElementById('workflow-select');
  sel.innerHTML = '';
  if (!data.workflows || data.workflows.length === 0) {
    sel.innerHTML = '<option value="">No workflows</option>';
    return;
  }
  const filter = selectedProjectFilter;
  let filtered = data.workflows;
  if (filter) {
    filtered = data.workflows.filter(wf => wf.project === filter);
  }
  if (filtered.length === 0) {
    sel.innerHTML = '<option value="">No workflows</option>';
    return;
  }
  filtered.forEach(wf => {
    const opt = document.createElement('option');
    opt.value = wf.name;
    const projectLabel = wf.project && wf.project !== 'default' ? ` [${wf.project}]` : '';
    opt.textContent = `${wf.name} (${wf.phase_label})${projectLabel}`;
    opt.selected = wf.active;
    sel.appendChild(opt);
  });
}

function renderPhaseTracker() {
  if (!state) return;
  const pn = state.complete ? 5 : parseInt(state.phase, 10);
  const phaseKeys = ['', 'plan', 'review', 'implement', 'verify'];

  document.querySelectorAll('.phase-item').forEach(el => {
    const p = parseInt(el.dataset.phase, 10);
    el.classList.remove('completed', 'current', 'pending');
    if (p < pn) el.classList.add('completed');
    else if (p === pn) el.classList.add('current');
    else el.classList.add('pending');

    const key = phaseKeys[p];
    const toolEl = el.querySelector('.phase-tool');
    if (key && state.agents && state.agents[key]) {
      toolEl.textContent = state.agents[key].display_name || state.agents[key].name;
    }
  });
}

function renderArtifactList() {
  if (!state) return;
  document.querySelectorAll('.artifact-item').forEach(el => {
    const type = el.dataset.artifact;
    const art = state.artifacts[type];
    el.classList.toggle('exists', art && art.exists);
    const icon = el.querySelector('.artifact-icon');
    if (art && art.exists) {
      icon.textContent = '\u2713';
    } else {
      icon.textContent = '-';
    }
  });
}

function renderDirectories() {
  const el = document.getElementById('dir-list');
  const addBtn = document.getElementById('add-dir-btn');
  if (!state || state.error) {
    el.innerHTML = '<p class="muted dir-empty">No active workflow</p>';
    addBtn.style.display = 'none';
    return;
  }
  addBtn.style.display = '';

  let html = '';

  // Primary repo
  if (state.repo) {
    html += `<div class="dir-item">
      <span class="dir-label">repo</span>
      <span class="dir-path">${esc(state.repo)}</span>
    </div>`;
  }

  // Workflow dir
  if (state.workflow_dir) {
    html += `<div class="dir-item">
      <span class="dir-label">wf</span>
      <span class="dir-path">${esc(state.workflow_dir)}</span>
    </div>`;
  }

  // Additional directories
  const addDirs = state.add_dirs || [];
  addDirs.forEach(dir => {
    html += `<div class="dir-item">
      <span class="dir-label">add</span>
      <span class="dir-path">${esc(dir)}</span>
      <span class="dir-remove" title="Remove directory" data-dir="${esc(dir)}">\u00d7</span>
    </div>`;
  });

  if (!html) {
    html = '<p class="muted dir-empty">No directories configured</p>';
  }

  el.innerHTML = html;

  // Bind remove buttons
  el.querySelectorAll('.dir-remove').forEach(btn => {
    btn.addEventListener('click', () => removeDirectory(btn.dataset.dir));
  });
}

function renderInfo() {
  const el = document.getElementById('workflow-info');
  if (!state || state.error) {
    el.innerHTML = '<p class="muted">No active workflow</p>';
    return;
  }
  let retryInfo = '';
  if (state.retry_count > 0) {
    retryInfo = `<div class="info-row"><span class="info-label">Retry</span><span class="info-value">${state.retry_count}/${state.max_retries}</span></div>`;
  }
  const projectInfo = state.project ? `<div class="info-row"><span class="info-label">Project</span><span class="info-value">${esc(state.project)}</span></div>` : '';
  el.innerHTML = `
    ${projectInfo}
    <div class="info-row"><span class="info-label">Repo</span><span class="info-value">${esc(state.repo)}</span></div>
    <div class="info-row"><span class="info-label">Phase</span><span class="info-value">${esc(state.phase_label)}</span></div>
    <div class="info-row"><span class="info-label">Created</span><span class="info-value">${esc(state.created)}</span></div>
    ${retryInfo}
    ${state.description ? `<div class="info-row"><span class="info-label">Desc</span><span class="info-value">${esc(state.description)}</span></div>` : ''}
  `;
}

function renderStatus() {
  document.title = state ? `Crossagent - ${state.name}` : 'Crossagent';
}

function updateRunButton() {
  const btn = document.getElementById('run-phase-btn');
  if (!state || state.complete) {
    btn.textContent = 'Workflow Complete';
    btn.disabled = true;
    if (state && state.complete && !sessionActive) {
      setGuide('Workflow complete. Review artifacts in the sidebar.');
    }
    return;
  }
  const pn = parseInt(state.phase, 10);
  const name = PHASE_NAMES[pn] || 'phase';
  btn.textContent = `Run ${capitalize(name)}`;
  btn.disabled = sessionActive;

  // Update guide when not in a session and not in retry loop
  if (!sessionActive && !retryLoopActive) {
    setGuide(`Click "Run ${capitalize(name)}" to start the ${name} phase.`);
  }
}

// ── Guide Banner ────────────────────────────────────────────────────────────

function setGuide(text, type) {
  const el = document.getElementById('guide-banner');
  if (!el) return;
  el.textContent = text;
  el.classList.toggle('hidden', !text);
  el.classList.remove('guide-warning', 'guide-error', 'guide-success');
  if (type === 'warning') el.classList.add('guide-warning');
  else if (type === 'error') el.classList.add('guide-error');
  else if (type === 'success') el.classList.add('guide-success');
}

// ── Terminal ────────────────────────────────────────────────────────────────

const SYNC_OUTPUT_START = '\x1b[?2026h';
const SYNC_OUTPUT_END = '\x1b[?2026l';

function matchingTerminalMarkerSuffix(text) {
  const maxLen = Math.max(SYNC_OUTPUT_START.length, SYNC_OUTPUT_END.length) - 1;
  const limit = Math.min(maxLen, text.length);

  for (let len = limit; len > 0; len--) {
    const suffix = text.slice(-len);
    if (SYNC_OUTPUT_START.startsWith(suffix) || SYNC_OUTPUT_END.startsWith(suffix)) {
      return len;
    }
  }

  return 0;
}

function writeTerminalOutput(data) {
  if (!term || !data) return;

  let text = syncOutputRemainder + data;
  syncOutputRemainder = '';

  while (text) {
    if (!syncOutputActive) {
      const startIdx = text.indexOf(SYNC_OUTPUT_START);
      if (startIdx === -1) {
        const suffixLen = matchingTerminalMarkerSuffix(text);
        const safeText = suffixLen > 0 ? text.slice(0, -suffixLen) : text;
        if (safeText) term.write(safeText);
        syncOutputRemainder = suffixLen > 0 ? text.slice(-suffixLen) : '';
        return;
      }

      if (startIdx > 0) {
        term.write(text.slice(0, startIdx));
      }

      syncOutputActive = true;
      text = text.slice(startIdx + SYNC_OUTPUT_START.length);
      continue;
    }

    const endIdx = text.indexOf(SYNC_OUTPUT_END);
    if (endIdx === -1) {
      const suffixLen = matchingTerminalMarkerSuffix(text);
      const safeText = suffixLen > 0 ? text.slice(0, -suffixLen) : text;
      if (safeText) syncOutputBuffer += safeText;
      syncOutputRemainder = suffixLen > 0 ? text.slice(-suffixLen) : '';
      return;
    }

    syncOutputBuffer += text.slice(0, endIdx);
    if (syncOutputBuffer) {
      term.write(syncOutputBuffer);
      syncOutputBuffer = '';
    }
    syncOutputActive = false;
    text = text.slice(endIdx + SYNC_OUTPUT_END.length);
  }
}

function flushTerminalOutput() {
  if (!term) return;

  if (syncOutputActive && (syncOutputRemainder || syncOutputBuffer)) {
    term.write(syncOutputBuffer + syncOutputRemainder);
  } else if (syncOutputRemainder) {
    term.write(syncOutputRemainder);
  }

  syncOutputActive = false;
  syncOutputBuffer = '';
  syncOutputRemainder = '';
}

function fitTerminal(options = {}) {
  const { notifyPty = false, force = false } = options;
  if (!term || !fitAddon) return;

  const terminalEl = document.getElementById('terminal');
  if (!terminalEl || terminalEl.clientWidth === 0 || terminalEl.clientHeight === 0) {
    return;
  }

  fitAddon.fit();

  if (!notifyPty || !ws || ws.readyState !== WebSocket.OPEN || !sessionActive) {
    return;
  }

  if (term.cols <= 0 || term.rows <= 0) {
    return;
  }

  if (!force && term.cols === lastReportedCols && term.rows === lastReportedRows) {
    return;
  }

  lastReportedCols = term.cols;
  lastReportedRows = term.rows;
  ws.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
}

function scheduleTerminalFit(options = {}) {
  clearTimeout(resizeTimer);
  if (resizeFrame) {
    cancelAnimationFrame(resizeFrame);
    resizeFrame = null;
  }

  resizeTimer = setTimeout(() => {
    resizeFrame = requestAnimationFrame(() => {
      resizeFrame = null;
      fitTerminal(options);
    });
  }, 50);
}

function initTerminal() {
  term = new Terminal({
    theme: {
      background: '#0d1117',
      foreground: '#e6edf3',
      cursor: '#58a6ff',
      selectionBackground: '#264f78',
      black: '#0d1117',
      red: '#f85149',
      green: '#3fb950',
      yellow: '#d29922',
      blue: '#58a6ff',
      magenta: '#bc8cff',
      cyan: '#39d2c0',
      white: '#e6edf3',
    },
    fontFamily: "'SF Mono', 'Cascadia Code', 'Fira Code', Menlo, monospace",
    fontSize: 13,
    lineHeight: 1.3,
    cursorBlink: true,
    cursorStyle: 'bar',
    scrollback: 10000,
  });

  fitAddon = new FitAddon.FitAddon();
  term.loadAddon(fitAddon);
  term.open(document.getElementById('terminal'));
  fitTerminal({ force: true });

  term.writeln('\x1b[2m  Crossagent Terminal\x1b[0m');
  term.writeln('\x1b[2m  Select or create a workflow, then click the Run button to start.\x1b[0m');
  term.writeln('');

  term.onData(data => {
    if (ws && ws.readyState === WebSocket.OPEN && sessionActive) {
      ws.send(JSON.stringify({ type: 'input', data }));
    }
  });

  const resizeObserver = new ResizeObserver(() => {
    scheduleTerminalFit({ notifyPty: true });
  });
  resizeObserver.observe(document.getElementById('terminal'));

  window.addEventListener('resize', () => {
    scheduleTerminalFit({ notifyPty: true });
  });

  if (document.fonts && typeof document.fonts.ready?.then === 'function') {
    document.fonts.ready.then(() => {
      scheduleTerminalFit({ notifyPty: true, force: true });
    }).catch(() => {});
  }
}

// ── WebSocket ───────────────────────────────────────────────────────────────

function connectWS() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${protocol}//${location.host}/ws/terminal`);

  ws.onopen = () => {
    document.getElementById('connection-status').classList.add('connected');
    document.getElementById('connection-status').title = 'Connected to server';
  };

  ws.onmessage = (event) => {
    let msg;
    try {
      msg = JSON.parse(event.data);
    } catch {
      return;
    }
    switch (msg.type) {
      case 'output':
        writeTerminalOutput(msg.data);
        break;
      case 'spawned':
        sessionActive = true;
        setTerminalStatus('running', `Running — PID ${msg.pid}`);
        updateRunButton();
        scheduleTerminalFit({ notifyPty: true, force: true });
        break;
      case 'exit':
        flushTerminalOutput();
        sessionActive = false;
        stopOutputPolling();
        handleSessionExit(msg.code);
        break;
      case 'error':
        flushTerminalOutput();
        term.writeln(`\r\n\x1b[31m  Error: ${msg.message}\x1b[0m\r\n`);
        sessionActive = false;
        stopOutputPolling();
        updateRunButton();
        break;
    }
  };

  ws.onerror = () => {};

  ws.onclose = () => {
    flushTerminalOutput();
    document.getElementById('connection-status').classList.remove('connected');
    document.getElementById('connection-status').title = 'Disconnected — reconnecting...';
    sessionActive = false;
    stopOutputPolling();
    setTimeout(connectWS, 3000);
  };
}

// ── Output File Polling ─────────────────────────────────────────────────────
// While a session is running, poll for the expected output file.
// When the agent writes it (e.g. codex writes review.md), auto-advance
// immediately — don't wait for the session to exit.

function startOutputPolling() {
  stopOutputPolling();
  if (!pendingOutputFile) return;
  outputPollTimer = setInterval(pollForOutput, 3000);
}

function stopOutputPolling() {
  if (outputPollTimer) {
    clearInterval(outputPollTimer);
    outputPollTimer = null;
  }
}

async function pollForOutput() {
  if (!pendingOutputFile || !sessionActive) {
    stopOutputPolling();
    return;
  }
  try {
    const result = await api('/check-file', {
      method: 'POST',
      body: JSON.stringify({ path: pendingOutputFile }),
    });
    if (result.exists) {
      stopOutputPolling();
      await onOutputDetected();
    }
  } catch {
    // ignore, will retry
  }
}

async function onOutputDetected() {
  const phaseName = pendingPhaseName;
  const outputFile = pendingOutputFile;
  pendingOutputFile = null;
  pendingPhaseName = null;

  term.writeln(`\r\n\x1b[32m  Output detected — ${phaseName}.md written successfully.\x1b[0m`);

  // Phases that need supervisor evaluation before advancing
  const supervisedPhases = ['verify', 'review'];

  if (supervisedPhases.includes(phaseName)) {
    const label = phaseName === 'verify' ? 'verification' : 'review';
    term.writeln(`\x1b[2m  Evaluating ${label} result...\x1b[0m`);
    setGuide(`Evaluating ${label} result...`);

    // Kill session first
    if (sessionActive && ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: 'kill' }));
    }

    await sleep(500);

    // Advance phase so supervise sees correct state, then evaluate
    try {
      await api('/check-advance', {
        method: 'POST',
        body: JSON.stringify({ output_file: outputFile }),
      });
    } catch {}

    await handleSupervisorDecision(phaseName);
    return;
  }

  // Normal auto-advance for plan and implement phases
  term.writeln(`\x1b[2m  Advancing to next phase...\x1b[0m`);
  setGuide(`${capitalize(phaseName)} complete. Advancing...`);

  try {
    const result = await api('/check-advance', {
      method: 'POST',
      body: JSON.stringify({ output_file: outputFile }),
    });
    if (result.advanced) {
      term.writeln(`\x1b[32m  Advanced to next phase.\x1b[0m\r\n`);
    }
  } catch {}

  // Kill the session — agent is done, output is written
  if (sessionActive && ws && ws.readyState === WebSocket.OPEN) {
    term.writeln(`\x1b[2m  Closing agent session...\x1b[0m\r\n`);
    ws.send(JSON.stringify({ type: 'kill' }));
  }

  await fetchStatus();
  await fetchList();
  updateRunButton();
  if (phaseName) loadArtifact(phaseName);

  // If we're in a retry loop, auto-run the next phase
  if (retryLoopActive && state && !state.complete) {
    await sleep(1500);
    await autoRunNextPhase();
  }
}

// ── Supervisor / Auto-Revert ───────────────────────────────────────────────

async function handleSupervisorDecision(phaseName) {
  const label = phaseName === 'verify' ? 'verification' : 'review';

  try {
    const result = await api('/supervise', {
      method: 'POST',
      body: JSON.stringify({ phase: phaseName }),
    });

    await fetchStatus();
    await fetchList();
    loadArtifact(phaseName);

    // Pass — approved or verified successfully
    if (result.action === 'pass' || result.action === 'done') {
      if (result.action === 'done') {
        retryLoopActive = false;
        term.writeln(`\r\n\x1b[32m  Verification passed — workflow complete!\x1b[0m\r\n`);
        setGuide('Workflow complete! Verification passed.', 'success');
      } else {
        term.writeln(`\r\n\x1b[32m  ${capitalize(label)} approved. Continuing...\x1b[0m\r\n`);
        setGuide(`${capitalize(label)} approved. Advancing to next phase...`);
      }
      updateRunButton();

      // Continue retry loop or auto-run next phase
      if ((retryLoopActive || result.action === 'pass') && state && !state.complete) {
        await sleep(1500);
        await autoRunNextPhase();
      }
      return;
    }

    // Retry limit reached
    if (result.action === 'needs_human') {
      retryLoopActive = false;
      term.writeln(`\r\n\x1b[33m  Retry limit reached (${result.retry_count}/${result.max_retries}).\x1b[0m`);
      term.writeln(`\x1b[33m  Manual intervention required. Review ${phaseName}.md for details.\x1b[0m\r\n`);
      setGuide(`Retry limit reached (${result.retry_count}/${result.max_retries}). Review ${phaseName}.md and resolve issues manually.`, 'error');
      updateRunButton();
      return;
    }

    // Reverted — surgical fix required
    if (result.action === 'reverted') {
      retryLoopActive = true;
      const targetLabel = result.target_label || `phase ${result.target_phase}`;
      const source = phaseName === 'verify' ? 'Verifier' : 'Reviewer';
      term.writeln(`\r\n\x1b[33m  ${source} found issues. Reverting to ${targetLabel} for surgical fix (attempt ${result.attempt}/${result.max_retries}).\x1b[0m\r\n`);
      setGuide(`Retry ${result.attempt}/${result.max_retries}: ${source} requested changes. Reverting to ${targetLabel}...`, 'warning');

      await fetchStatus();
      await fetchList();
      updateRunButton();

      await sleep(2000);
      await autoRunNextPhase();
      return;
    }

    // Unknown verdict
    if (result.action === 'unknown') {
      retryLoopActive = false;
      term.writeln(`\r\n\x1b[33m  Could not parse ${label} verdict. Review ${phaseName}.md manually.\x1b[0m\r\n`);
      setGuide(`Could not determine ${label} result. Review ${phaseName}.md and advance manually.`, 'warning');
      updateRunButton();
      return;
    }

    // Fallback
    retryLoopActive = false;
    term.writeln(`\r\n\x1b[33m  Unexpected supervisor result: ${result.action || 'unknown'}\x1b[0m\r\n`);
    updateRunButton();

  } catch (err) {
    retryLoopActive = false;
    term.writeln(`\r\n\x1b[31m  Supervisor error: ${err.message}\x1b[0m\r\n`);
    setGuide(`Supervisor error. Review ${phaseName}.md manually.`, 'error');
    await fetchStatus();
    updateRunButton();
  }
}

async function autoRunNextPhase() {
  if (!state || state.complete || sessionActive) return;
  if (!ws || ws.readyState !== WebSocket.OPEN) return;

  const pn = parseInt(state.phase, 10);
  const phaseName = PHASE_NAMES[pn];
  if (!phaseName) return;

  term.writeln(`\x1b[2m  Auto-running ${capitalize(phaseName)} phase...\x1b[0m\r\n`);
  setGuide(`Auto-running ${phaseName} phase (retry)...`, 'warning');

  try {
    const config = await api(`/phase-cmd/${phaseName}?force=true`);
    if (config.error) {
      retryLoopActive = false;
      term.writeln(`\r\n\x1b[31m  Error: ${config.error}\x1b[0m\r\n`);
      return;
    }

    pendingOutputFile = config.output_file || null;
    pendingPhaseName = phaseName;

    term.clear();
    term.writeln(`\x1b[2m  Phase ${pn}: ${capitalize(phaseName)} (retry)\x1b[0m`);
    term.writeln(`\x1b[2m  The AI agent will run in this terminal.\x1b[0m\r\n`);

    ws.send(JSON.stringify({
      type: 'spawn',
      phaseName: phaseName,
      force: true,
      cols: term.cols,
      rows: term.rows,
    }));

    startOutputPolling();
  } catch (err) {
    retryLoopActive = false;
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

// ── Session Exit Handler ────────────────────────────────────────────────────

async function handleSessionExit(exitCode) {
  const phaseName = pendingPhaseName;
  const outputFile = pendingOutputFile;
  pendingOutputFile = null;
  pendingPhaseName = null;

  setTerminalStatus('exited', `Exited (${exitCode})`);
  updateRunButton();

  // If output was already detected by polling, pendingOutputFile is null
  // and we just need to refresh state.
  if (!phaseName && !outputFile) {
    await fetchStatus();
    await fetchList();
    return;
  }

  if (outputFile) {
    // Check one more time for the output file
    term.writeln(`\r\n\x1b[2m  Checking for ${phaseName || 'phase'} output...\x1b[0m`);

    // For supervised phases (verify/review), use supervisor flow on exit
    const supervisedPhases = ['verify', 'review'];
    if (supervisedPhases.includes(phaseName)) {
      const exists = await checkFileExists(outputFile);
      if (exists) {
        try {
          await api('/check-advance', {
            method: 'POST',
            body: JSON.stringify({ output_file: outputFile }),
          });
        } catch {}
        await handleSupervisorDecision(phaseName);
        return;
      }
    }

    const advanced = await tryAutoAdvance(outputFile, 3);
    if (advanced) {
      term.writeln(`\x1b[32m  ${capitalize(phaseName || 'Phase')} complete! Advancing...\x1b[0m\r\n`);
      await refreshAfterAdvance(phaseName);

      // Continue retry loop if active
      if (retryLoopActive && state && !state.complete) {
        await sleep(1500);
        await autoRunNextPhase();
      }
      return;
    }
    term.writeln(`\r\n\x1b[33m  Output not detected. Click "Advance" to proceed manually.\x1b[0m\r\n`);
  }

  await fetchStatus();
  await fetchList();
  if (activeArtifact) loadArtifact(activeArtifact);
}

async function checkFileExists(filePath) {
  try {
    const result = await api('/check-file', {
      method: 'POST',
      body: JSON.stringify({ path: filePath }),
    });
    return result.exists;
  } catch {
    return false;
  }
}

async function refreshAfterAdvance(phaseName) {
  await fetchStatus();
  await fetchList();
  updateRunButton();
  if (phaseName) loadArtifact(phaseName);
}

async function tryAutoAdvance(outputFile, retries) {
  for (let i = 0; i < retries; i++) {
    if (i > 0) await sleep(1500);
    try {
      const result = await api('/check-advance', {
        method: 'POST',
        body: JSON.stringify({ output_file: outputFile }),
      });
      if (result.advanced) {
        if (result.recovered) {
          const basename = result.recovered_from.split('/').pop();
          term.writeln(`\x1b[33m  Sandbox-fallback: relocated ${basename} to workflow directory.\x1b[0m`);
        }
        return true;
      }
    } catch {}
  }
  return false;
}

function sleep(ms) { return new Promise(r => setTimeout(r, ms)); }

function setTerminalStatus(cls, text) {
  const el = document.getElementById('terminal-status');
  el.className = 'terminal-status ' + cls;
  el.textContent = text;
}

// ── Actions ─────────────────────────────────────────────────────────────────

async function runPhase() {
  if (!state || state.complete || sessionActive) return;

  if (!ws || ws.readyState !== WebSocket.OPEN) {
    term.writeln('\r\n\x1b[31m  Error: Not connected to server. Retrying...\x1b[0m\r\n');
    return;
  }

  const btn = document.getElementById('run-phase-btn');
  btn.disabled = true;

  const pn = parseInt(state.phase, 10);
  const phaseName = PHASE_NAMES[pn];
  if (!phaseName) {
    btn.disabled = false;
    return;
  }

  try {
    const config = await api(`/phase-cmd/${phaseName}`);
    if (config.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${config.error}\x1b[0m\r\n`);
      btn.disabled = sessionActive;
      return;
    }

    pendingOutputFile = config.output_file || null;
    pendingPhaseName = phaseName;

    term.clear();
    term.writeln(`\x1b[2m  Phase ${pn}: ${capitalize(phaseName)}\x1b[0m`);
    term.writeln(`\x1b[2m  The AI agent will run in this terminal. Interact as needed.\x1b[0m\r\n`);
    setGuide(`${capitalize(phaseName)} phase running... The agent is working in the terminal below.`);

    ws.send(JSON.stringify({
      type: 'spawn',
      phaseName: phaseName,
      cols: term.cols,
      rows: term.rows,
    }));

    // Start polling for output file while session runs
    startOutputPolling();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
    btn.disabled = sessionActive;
  }
}

async function loadArtifact(type) {
  activeArtifact = type;
  document.querySelectorAll('.artifact-item').forEach(el => {
    el.classList.toggle('active', el.dataset.artifact === type);
  });

  const viewer = document.getElementById('artifact-viewer');
  const title = document.getElementById('artifact-title');
  title.textContent = `${type}.md`;

  try {
    const data = await api(`/artifact/${type}`);
    if (data.error) {
      viewer.innerHTML = `<p class="muted centered">${esc(data.error)}</p>`;
      return;
    }
    viewer.innerHTML = marked.parse(data.content);
  } catch {
    viewer.innerHTML = '<p class="muted centered">Failed to load artifact</p>';
  }
}

async function loadChatHistory(phase) {
  if (sessionActive) return; // Don't interrupt active sessions
  try {
    const data = await api(`/chat-history/${phase}`);
    if (!data.exists) {
      term.writeln(`\r\n\x1b[33m  No chat history available for ${phase} phase.\x1b[0m\r\n`);
      return;
    }
    viewingChatHistory = true;
    term.clear();
    term.writeln(`\x1b[2m  Viewing ${phase} phase chat history\x1b[0m`);
    term.writeln(`\x1b[2m  ${'─'.repeat(50)}\x1b[0m\r\n`);
    if (data.large) {
      // Stream large files
      const res = await fetch(`/api/chat-history/${phase}/stream`);
      const text = await res.text();
      term.write(text);
    } else {
      term.write(data.content);
    }
    term.writeln(`\r\n\r\n\x1b[2m  ${'─'.repeat(50)}\x1b[0m`);
    term.writeln(`\x1b[2m  End of ${phase} phase chat history\x1b[0m\r\n`);
    setGuide(`Viewing ${phase} phase chat history. Click Run to start a new session.`);
    // Also show the corresponding artifact
    loadArtifact(phase);
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error loading chat history: ${err.message}\x1b[0m\r\n`);
  }
}

async function addDirectory(dirPath) {
  if (!dirPath || !state) return;
  try {
    const data = await api('/repos/add', {
      method: 'POST',
      body: JSON.stringify({ path: dirPath }),
    });
    if (data.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${data.error}\x1b[0m\r\n`);
      return;
    }
    term.writeln(`\r\n\x1b[32m  Directory added: ${dirPath}\x1b[0m\r\n`);
    await fetchStatus();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

async function removeDirectory(dirPath) {
  if (!dirPath || !state) return;
  try {
    const data = await api('/repos/remove', {
      method: 'POST',
      body: JSON.stringify({ path: dirPath }),
    });
    if (data.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${data.error}\x1b[0m\r\n`);
      return;
    }
    term.writeln(`\r\n\x1b[32m  Directory removed: ${dirPath}\x1b[0m\r\n`);
    await fetchStatus();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

async function createWorkflow(name, repo, description, addDirs, project) {
  const errorBanner = document.getElementById('new-form-error');
  try {
    const data = await api('/new', {
      method: 'POST',
      body: JSON.stringify({ name, repo, description, addDirs, project }),
    });
    if (data.error) {
      errorBanner.textContent = data.error;
      errorBanner.classList.remove('hidden');
      return false;
    }
    errorBanner.classList.add('hidden');
    term.clear();
    term.writeln(`\x1b[32m  Workflow "${name}" created.\x1b[0m`);
    term.writeln(`\x1b[2m  Click "Run Plan" to start the planning phase.\x1b[0m\r\n`);
    await fetchProjects();
    await fetchList();
    await fetchStatus();

    // Auto-suggest project if created under "default"
    if (!project || project === 'default') {
      await suggestProject(name, description);
    }

    // Show elicitation modal
    await showElicitation(name);

    return true;
  } catch (err) {
    errorBanner.textContent = err.message || 'Failed to create workflow';
    errorBanner.classList.remove('hidden');
    return false;
  }
}

async function switchWorkflow(name) {
  if (!name) return;
  try {
    const data = await api(`/use/${name}`, { method: 'POST' });
    if (data.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${data.error}\x1b[0m\r\n`);
    }
    retryLoopActive = false;
    activeArtifact = null;
    document.querySelectorAll('.artifact-item').forEach(el => el.classList.remove('active'));
    document.getElementById('artifact-viewer').innerHTML = '<p class="muted centered">Select an artifact from the sidebar to view it</p>';
    document.getElementById('artifact-title').textContent = 'Artifact Viewer';
    await fetchStatus();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

async function advancePhase() {
  try {
    const data = await api('/advance', { method: 'POST' });
    if (data.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${data.error}\x1b[0m\r\n`);
    } else {
      term.writeln(`\r\n\x1b[32m  Phase advanced.\x1b[0m\r\n`);
    }
    await fetchStatus();
    await fetchList();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

async function markDone() {
  try {
    const data = await api('/done', { method: 'POST' });
    if (data.error) {
      term.writeln(`\r\n\x1b[31m  Error: ${data.error}\x1b[0m\r\n`);
    } else {
      term.writeln(`\r\n\x1b[32m  Workflow marked as complete.\x1b[0m\r\n`);
    }
    retryLoopActive = false;
    await fetchStatus();
    await fetchList();
  } catch (err) {
    term.writeln(`\r\n\x1b[31m  Error: ${err.message}\x1b[0m\r\n`);
  }
}

// ── Utilities ───────────────────────────────────────────────────────────────

function esc(str) {
  const d = document.createElement('div');
  d.textContent = str || '';
  return d.innerHTML;
}

function capitalize(s) { return s ? s.charAt(0).toUpperCase() + s.slice(1) : ''; }

function setFieldError(input, errorEl, message) {
  if (message) {
    errorEl.textContent = message;
    errorEl.classList.add('visible');
    input.classList.add('field-invalid');
  } else {
    errorEl.textContent = '';
    errorEl.classList.remove('visible');
    input.classList.remove('field-invalid');
  }
}

function clearNewFormErrors() {
  document.getElementById('new-form-error').classList.add('hidden');
  document.querySelectorAll('#new-form .field-error').forEach(el => {
    el.textContent = '';
    el.classList.remove('visible');
  });
  document.querySelectorAll('#new-form .field-invalid').forEach(el => {
    el.classList.remove('field-invalid');
  });
}

// ── Elicitation ─────────────────────────────────────────────────────────────

function showElicitation(workflowName) {
  return new Promise((resolve) => {
    const modal = document.getElementById('elicit-modal');
    const form = document.getElementById('elicit-form');
    const skipBtn = document.getElementById('elicit-skip');
    const backdrop = modal.querySelector('.modal-backdrop');
    let settled = false;

    modal.classList.remove('hidden');

    const cleanup = () => {
      if (settled) return;
      settled = true;
      modal.classList.add('hidden');
      form.reset();
      skipBtn.removeEventListener('click', onSkip);
      form.removeEventListener('submit', onSubmit);
      backdrop.removeEventListener('click', onDismiss);
      document.removeEventListener('keydown', onEscape, { capture: true });
    };

    const onDismiss = () => { cleanup(); resolve(); };

    const onEscape = (e) => {
      if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
        e.stopImmediatePropagation();
        cleanup();
        resolve();
      }
    };

    const onSkip = () => {
      cleanup();
      resolve();
    };

    const onSubmit = async (e) => {
      e.preventDefault();

      const scope = document.getElementById('elicit-scope').value.trim();
      const style = document.querySelector('input[name="elicit-style"]:checked')?.value || '';
      const constraints = document.getElementById('elicit-constraints').value.trim();
      const criteria = document.getElementById('elicit-criteria').value.trim();

      // Build addendum from non-empty fields
      const parts = ['## Implementation Guidance (from elicitation)'];
      if (scope) parts.push(`**Scope**: ${scope}`);
      if (style) parts.push(`**Style**: ${style === 'surgical' ? 'Surgical — minimal changes, follow existing code style' : 'Holistic — refactor and improve surrounding code as needed'}`);
      if (constraints) parts.push(`**Constraints**: ${constraints}`);
      if (criteria) parts.push(`**Acceptance Criteria**: ${criteria}`);

      if (parts.length > 1) {
        try {
          await api('/update-description', {
            method: 'POST',
            body: JSON.stringify({ workflow: workflowName, append: parts.join('\n') }),
          });
        } catch {
          // Non-fatal — elicitation is best-effort
        }
      }

      cleanup();
      resolve();
    };

    skipBtn.addEventListener('click', onSkip);
    form.addEventListener('submit', onSubmit);
    backdrop.addEventListener('click', onDismiss);
    document.addEventListener('keydown', onEscape, { capture: true });
  });
}

// ── Tour System ─────────────────────────────────────────────────────────────

const TOUR_STEPS = [
  { target: '#new-btn', text: 'Click here to create a new workflow. A workflow is a task or feature you want the AI to plan, review, implement, and verify.', position: 'bottom' },
  { target: '#workflow-select', text: 'Switch between your workflows here. Each workflow tracks its own progress through the four phases.', position: 'bottom' },
  { target: '#project-select', text: 'Filter workflows by project. Projects group related workflows together.', position: 'bottom' },
  { target: '#phase-tracker', text: 'Your workflow progresses through 4 phases: Plan → Review → Implement → Verify. Click a completed phase to replay its chat history.', position: 'right' },
  { target: '#run-phase-btn', text: 'Click this to launch the current phase. The AI agent will work in the terminal below.', position: 'right' },
  { target: '#artifact-list', text: 'View the output of each phase here — plans, reviews, and verification reports rendered as markdown.', position: 'right' },
  { target: '#terminal-panel', text: 'This is where AI agents execute. You\'ll see their progress in real time.', position: 'left' },
];

let tourStep = 0;

function startTour() {
  tourStep = 0;
  const overlay = document.getElementById('tour-overlay');
  overlay.classList.remove('hidden');
  showTourStep();
}

function showTourStep() {
  if (tourStep >= TOUR_STEPS.length) {
    endTour();
    return;
  }

  // Clear previous highlights
  document.querySelectorAll('.tour-highlight').forEach(el => el.classList.remove('tour-highlight'));

  const step = TOUR_STEPS[tourStep];
  const target = document.querySelector(step.target);

  if (!target || target.getBoundingClientRect().width === 0) {
    // Skip invisible elements
    tourStep++;
    showTourStep();
    return;
  }

  target.classList.add('tour-highlight');

  document.getElementById('tour-step-indicator').textContent = `Step ${tourStep + 1} of ${TOUR_STEPS.length}`;
  document.getElementById('tour-text').textContent = step.text;

  const nextBtn = document.getElementById('tour-next');
  nextBtn.textContent = tourStep === TOUR_STEPS.length - 1 ? 'Done' : 'Next';

  // Position tooltip near target
  const tooltip = document.getElementById('tour-tooltip');
  const rect = target.getBoundingClientRect();
  tooltip.style.position = 'fixed';

  switch (step.position) {
    case 'bottom':
      tooltip.style.top = (rect.bottom + 12) + 'px';
      tooltip.style.left = Math.max(8, rect.left) + 'px';
      break;
    case 'right':
      tooltip.style.top = rect.top + 'px';
      tooltip.style.left = (rect.right + 12) + 'px';
      break;
    case 'left':
      tooltip.style.top = rect.top + 'px';
      tooltip.style.left = Math.max(8, rect.left - 340) + 'px';
      break;
    default:
      tooltip.style.top = (rect.bottom + 12) + 'px';
      tooltip.style.left = rect.left + 'px';
  }
}

function nextTourStep() {
  tourStep++;
  showTourStep();
}

function endTour() {
  document.querySelectorAll('.tour-highlight').forEach(el => el.classList.remove('tour-highlight'));
  document.getElementById('tour-overlay').classList.add('hidden');
}

// ── Event Binding ───────────────────────────────────────────────────────────

function bindEvents() {
  document.getElementById('run-phase-btn').addEventListener('click', runPhase);
  document.getElementById('advance-btn').addEventListener('click', advancePhase);
  document.getElementById('done-btn').addEventListener('click', markDone);

  document.getElementById('workflow-select').addEventListener('change', (e) => {
    switchWorkflow(e.target.value);
  });

  document.querySelectorAll('.artifact-item').forEach(el => {
    el.addEventListener('click', () => loadArtifact(el.dataset.artifact));
  });

  // Clickable completed phases — load chat history replay
  document.querySelectorAll('.phase-item').forEach(el => {
    el.addEventListener('click', () => {
      if (!el.classList.contains('completed')) return;
      const phaseNum = parseInt(el.dataset.phase, 10);
      const phaseName = PHASE_NAMES[phaseNum];
      if (phaseName) loadChatHistory(phaseName);
    });
  });

  document.getElementById('new-btn').addEventListener('click', async () => {
    await fetchProjects();
    document.getElementById('new-modal').classList.remove('hidden');
    document.getElementById('new-name').focus();
  });
  document.getElementById('new-cancel').addEventListener('click', () => {
    document.getElementById('new-modal').classList.add('hidden');
    clearNewFormErrors();
  });

  // Real-time name validation
  const NAME_REGEX = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;
  document.getElementById('new-name').addEventListener('input', () => {
    const nameInput = document.getElementById('new-name');
    const nameError = document.getElementById('new-name-error');
    const val = nameInput.value.trim();
    if (!val) {
      setFieldError(nameInput, nameError, '');
    } else if (!NAME_REGEX.test(val)) {
      setFieldError(nameInput, nameError, 'Name can only contain letters, numbers, hyphens, underscores, and dots. Must start with a letter or number. No spaces.');
    } else {
      setFieldError(nameInput, nameError, '');
    }
  });

  document.getElementById('new-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const nameInput = document.getElementById('new-name');
    const descInput = document.getElementById('new-desc');
    const nameError = document.getElementById('new-name-error');
    const descError = document.getElementById('new-desc-error');
    const name = nameInput.value.trim();
    const repo = document.getElementById('new-repo').value.trim();
    const desc = descInput.value.trim();
    const dirsText = document.getElementById('new-dirs').value.trim();
    const project = document.getElementById('new-project').value;

    // Validate
    let valid = true;
    if (!name) {
      setFieldError(nameInput, nameError, 'Name is required.');
      valid = false;
    } else if (!NAME_REGEX.test(name)) {
      setFieldError(nameInput, nameError, 'Name can only contain letters, numbers, hyphens, underscores, and dots. Must start with a letter or number. No spaces.');
      valid = false;
    } else {
      setFieldError(nameInput, nameError, '');
    }
    if (!desc) {
      setFieldError(descInput, descError, 'Description is required.');
      valid = false;
    } else {
      setFieldError(descInput, descError, '');
    }
    if (!valid) return;

    // Parse additional directories (one per line, skip empties)
    const addDirs = dirsText
      ? dirsText.split('\n').map(d => d.trim()).filter(Boolean)
      : undefined;

    const success = await createWorkflow(name, repo || undefined, desc, addDirs, project || undefined);
    if (success) {
      document.getElementById('new-modal').classList.add('hidden');
      document.getElementById('new-form').reset();
      clearNewFormErrors();
    }
  });

  // Project filter
  document.getElementById('project-select').addEventListener('change', async (e) => {
    selectedProjectFilter = e.target.value;
    await fetchList();
  });

  // Manage Projects
  document.getElementById('manage-projects-btn').addEventListener('click', async () => {
    await fetchProjects();
    renderProjectManager();
    document.getElementById('projects-modal').classList.remove('hidden');
  });
  document.getElementById('projects-close').addEventListener('click', () => {
    document.getElementById('projects-modal').classList.add('hidden');
  });
  document.getElementById('projects-new-btn').addEventListener('click', async () => {
    const name = document.getElementById('projects-new-name').value.trim();
    if (!name) return;
    await createProject(name);
    document.getElementById('projects-new-name').value = '';
  });


  // Add Directory modal
  document.getElementById('add-dir-btn').addEventListener('click', () => {
    if (!state) return;
    document.getElementById('adddir-modal').classList.remove('hidden');
    document.getElementById('adddir-path').focus();
  });
  document.getElementById('adddir-cancel').addEventListener('click', () => {
    document.getElementById('adddir-modal').classList.add('hidden');
  });
  document.getElementById('adddir-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const dirPath = document.getElementById('adddir-path').value.trim();
    if (!dirPath) return;
    await addDirectory(dirPath);
    document.getElementById('adddir-modal').classList.add('hidden');
    document.getElementById('adddir-form').reset();
  });

  // Close modals with backdrop click (skip promise-backed modals — they manage their own dismiss)
  const promiseModals = new Set(['suggest-modal', 'elicit-modal']);
  document.querySelectorAll('.modal-backdrop').forEach(backdrop => {
    backdrop.addEventListener('click', () => {
      const modal = backdrop.closest('.modal');
      if (!promiseModals.has(modal.id)) {
        modal.classList.add('hidden');
      }
    });
  });

  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') {
      // Close only the topmost visible modal (skip promise-backed modals — they handle Escape themselves)
      const openModals = [...document.querySelectorAll('.modal:not(.hidden)')].filter(m => !promiseModals.has(m.id));
      if (openModals.length > 0) {
        openModals[openModals.length - 1].classList.add('hidden');
      }
      // Close tour if active
      const tourOverlay = document.getElementById('tour-overlay');
      if (tourOverlay && !tourOverlay.classList.contains('hidden')) {
        endTour();
      }
    }
  });

  // Tour controls
  document.getElementById('tour-next').addEventListener('click', nextTourStep);
  document.getElementById('tour-skip').addEventListener('click', endTour);
  document.getElementById('tour-btn').addEventListener('click', startTour);

  // Poll for state updates every 5s when idle
  setInterval(() => {
    if (!sessionActive) fetchStatus();
  }, 5000);
}

// ── Init ────────────────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', async () => {
  initTerminal();
  connectWS();
  bindEvents();
  await fetchProjects();
  await fetchList();
  await fetchStatus();

  // Start tour on every app launch (user requirement: "tour the user on every launched of the app")
  // Users can skip immediately via "Skip Tour" button; "?" button in topbar replays it anytime
  startTour();
});
