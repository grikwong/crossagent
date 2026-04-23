// PipelineHeader — eyebrow (project · round), workflow id, state pill,
// Run + Follow Up + Advance buttons.
//
// Action buttons target store.selectedWorkflowId (not store.status.name) so
// clicks immediately after a workflow switch always target the workflow the
// user just selected, even if /api/status hasn't responded yet. Buttons are
// disabled until the status fetch for the selected workflow completes.

import { store } from '../state.js';
import { esc, hashKey } from '../util.js';
import { wfApi } from '../api.js';

let root = null;
let lastKey = '';

function stateLabel() {
  if (!store.status) return { label: 'No workflow', cls: 'idle' };
  if (store.status.phase === 'done') return { label: 'Complete', cls: 'ok' };
  if (store.session.active) return { label: 'Running', cls: 'run' };
  return { label: store.status.phase_label || store.status.phase || 'Idle', cls: 'idle' };
}

function statusIsFresh() {
  return store.status && store.selectedWorkflowId === store.status.name;
}

async function safeClick(label, fn) {
  try {
    await fn();
  } catch (err) {
    window.alert(`${label} failed: ${(err && err.message) || err}`);
  }
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const s = store.status;
  const key = hashKey(
    s && s.name, s && s.phase, s && s.followup_round,
    store.session.active ? 1 : 0,
    store.selectedWorkflowId,
    statusIsFresh() ? 1 : 0,
  );
  if (key === lastKey) return;
  lastKey = key;

  if (!store.selectedWorkflowId) {
    root.innerHTML = `<div class="pl-empty">Select or create a workflow.</div>`;
    return;
  }

  const fresh = statusIsFresh();
  const round = s ? (s.followup_round || 0) + 1 : 1;
  const st = stateLabel();
  const done = s && s.phase === 'done';
  const running = store.session.active;
  const wfName = fresh ? s.name : store.selectedWorkflowId;
  const project = fresh ? (s.project || 'default') : '';

  const disabled = fresh ? '' : 'disabled';
  const runDisabled = (!fresh || done || running) ? 'disabled' : '';

  root.innerHTML = `
    <div class="pl-left">
      <div class="pl-eyebrow">${esc(project.toUpperCase())} · ROUND ${round}</div>
      <div class="pl-title">${esc(wfName)}</div>
    </div>
    <div class="pl-right">
      <span class="pl-state-pill pl-state-pill--${st.cls}">
        <span class="pl-state-dot"></span>${esc(st.label)}
      </span>
      <button class="pl-btn pl-btn--primary" id="pl-run" ${runDisabled}>Run ${esc(st.label === 'Complete' ? '✓' : (s && s.phase_label) ? ('' + s.phase_label[0].toUpperCase() + s.phase_label.slice(1)) : 'Phase')}</button>
      <button class="pl-btn" id="pl-followup" ${disabled}>Follow Up</button>
      <button class="pl-btn" id="pl-advance" ${disabled || (done ? 'disabled' : '')}>Advance →</button>
    </div>
  `;

  root.querySelector('#pl-run').addEventListener('click', () => {
    // Delegate to the legacy Run Next Phase handler, which knows how to pick
    // the current phase, build spawn params, and start the PTY.
    const btn = document.getElementById('run-phase-btn');
    if (btn) btn.click();
  });

  root.querySelector('#pl-advance').addEventListener('click', () => {
    safeClick('Advance', async () => {
      const name = store.selectedWorkflowId;
      if (!name) return;
      const res = await wfApi(name, '/advance', { method: 'POST' });
      if (res && res.error) throw new Error(res.error);
      if (typeof window.__crossagentRefreshStatus === 'function') {
        await window.__crossagentRefreshStatus();
      }
    });
  });

  root.querySelector('#pl-followup').addEventListener('click', () => {
    const btn = document.getElementById('followup-btn');
    if (btn) btn.click();
  });
}
