// WorkflowList region — grouped list of workflows with search + state dots.
//
// Reads: store.workflows, store.projects, store.selectedWorkflowId,
//        store.selectedProjectFilter, store.workflowSearch, store.session.
// Writes: store.selectedWorkflowId, store.workflowSearch.

import { store, setState } from '../state.js';
import { esc, hashKey } from '../util.js';
import { api } from '../api.js';

let root = null;
let lastKey = '';

// Workflow-level state dot: derived from phase + session.
function workflowState(wf) {
  if (wf.phase === 'done') return 'complete';
  if (store.session.active && store.selectedWorkflowId === wf.name) return 'running';
  return 'idle';
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const key = hashKey(
    (store.workflows || []).map(w => `${w.name}:${w.phase}:${w.project}`).join(','),
    store.selectedWorkflowId,
    store.selectedProjectFilter,
    store.workflowSearch,
    store.session.active ? 1 : 0,
  );
  if (key === lastKey) return;
  lastKey = key;

  const search = (store.workflowSearch || '').toLowerCase();
  const filter = store.selectedProjectFilter || '';
  const workflows = (store.workflows || []).filter(w => {
    if (filter && w.project !== filter) return false;
    if (search && !w.name.toLowerCase().includes(search)) return false;
    return true;
  });

  // Group by project
  const groups = new Map();
  for (const w of workflows) {
    const key = w.project || 'default';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(w);
  }

  const groupsHtml = [...groups.entries()].map(([proj, list]) => {
    const rows = list.map(w => {
      const isSel = store.selectedWorkflowId === w.name;
      const st = workflowState(w);
      return `
        <div class="wfl-row ${isSel ? 'wfl-row--selected' : ''}" data-name="${esc(w.name)}">
          <span class="wfl-dot wfl-dot--${st}"></span>
          <span class="wfl-body">
            <span class="wfl-id">${esc(w.name)}</span>
            <span class="wfl-meta">${esc(w.project)} · ${esc(w.phase_label || w.phase)}</span>
          </span>
        </div>
      `;
    }).join('');
    return `
      <div class="wfl-group">
        <div class="wfl-group-head">${esc(proj)}</div>
        ${rows}
      </div>
    `;
  }).join('');

  root.innerHTML = `
    <div class="wfl-search">
      <input type="text" id="wfl-search-input" placeholder="Search workflows…" value="${esc(store.workflowSearch || '')}"/>
    </div>
    <div class="wfl-groups">
      ${groupsHtml || '<div class="wfl-empty">No workflows match.</div>'}
    </div>
  `;

  root.querySelector('#wfl-search-input').addEventListener('input', (e) => {
    setState({ workflowSearch: e.target.value });
  });

  root.querySelectorAll('.wfl-row').forEach(el => {
    el.addEventListener('click', async () => {
      const name = el.dataset.name;
      if (!name || name === store.selectedWorkflowId) return;
      setState({ selectedWorkflowId: name, selectedRound: null, selectedPhase: null });
      // Set server-side active workflow so polling picks up the right status.
      try {
        await api(`/use/${encodeURIComponent(name)}`, { method: 'POST' });
      } catch { /* best effort */ }
      // Trigger a status refetch through the legacy fetchStatus if exposed.
      if (typeof window.__crossagentRefreshStatus === 'function') {
        window.__crossagentRefreshStatus();
      }
    });
  });
}
