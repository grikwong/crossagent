// ArtifactInfoRail — "Description", "This run", and "Directories" sections.
// Reads: store.status, store.selectedRound.

import { store } from '../state.js';
import { setState } from '../state.js';
import { esc, hashKey } from '../util.js';
import { wfApi } from '../api.js';

let root = null;
let lastKey = '';

function row(key, value) {
  return `<div class="ir-row"><span class="ir-key">${esc(key)}</span><span class="ir-val">${esc(value)}</span></div>`;
}

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const s = store.status;
  const key = hashKey(
    s && s.name, s && s.repo, s && (s.add_dirs || []).join(','),
    s && s.workflow_dir, s && s.retry_count, s && s.followup_round,
    s && s.description, s && s.phase,
    s && s.artifacts && s.artifacts.plan && s.artifacts.plan.exists,
    store.selectedRound,
  );
  if (key === lastKey) return;
  lastKey = key;

  if (!s || !s.name) {
    root.innerHTML = '';
    return;
  }

  const round = store.selectedRound != null
    ? store.selectedRound
    : ((s.followup_round || 0) + 1);

  // Description is editable only when viewing the current round (not archived),
  // phase is "1", and no plan artifact exists yet (workflow hasn't been run).
  const planExists = s.artifacts && s.artifacts.plan && s.artifacts.plan.exists;
  const editable = store.selectedRound == null && s.phase === '1' && !planExists;
  const descHtml = `
    <section class="ir-section">
      <h4 class="ir-label">Description</h4>
      ${editable
        ? `<textarea class="ir-desc-edit" id="ir-desc-textarea" rows="5">${esc(s.description || '')}</textarea>
           <button class="ir-save-btn" id="ir-desc-save">Save</button>
           <span class="ir-save-status" id="ir-save-status"></span>`
        : `<p class="ir-desc-text">${esc(s.description || '—')}</p>`
      }
    </section>
  `;

  const thisRun = `
    <section class="ir-section">
      <h4 class="ir-label">This run</h4>
      ${row('Project', s.project || 'default')}
      ${row('Round', `R${round}`)}
      ${row('Retries', String(s.retry_count || 0))}
      ${row('Created', s.created || '—')}
    </section>
  `;

  const dirs = [
    { kind: 'REPO', path: s.repo },
    ...(s.add_dirs || []).map(p => ({ kind: 'AD', path: p })),
    { kind: 'WF', path: s.workflow_dir },
  ].filter(d => d.path);

  const dirsHtml = dirs.map(d => `
    <div class="ir-dir">
      <span class="ir-dir-kind">${esc(d.kind)}</span>
      <span class="ir-dir-path">${esc(d.path)}</span>
    </div>
  `).join('');

  root.innerHTML = `
    ${descHtml}
    ${thisRun}
    <section class="ir-section">
      <div class="ir-label-row">
        <h4 class="ir-label">Directories</h4>
        <button class="ir-add-btn" id="ir-add-dir" title="Add additional directory">+</button>
      </div>
      ${dirsHtml}
    </section>
  `;

  const saveBtn = root.querySelector('#ir-desc-save');
  if (saveBtn) {
    saveBtn.addEventListener('click', async () => {
      const ta = root.querySelector('#ir-desc-textarea');
      const statusEl = root.querySelector('#ir-save-status');
      if (!ta || !s.name) return;
      saveBtn.disabled = true;
      statusEl.textContent = 'Saving…';
      try {
        const res = await wfApi(s.name, '/description', {
          method: 'PUT',
          body: JSON.stringify({ description: ta.value }),
        });
        if (res && res.error) throw new Error(res.error);
        if (res && res.name && res.name === store.selectedWorkflowId) {
          setState({ status: res });
        }
        statusEl.textContent = 'Saved';
      } catch (err) {
        statusEl.textContent = 'Error: ' + (err.message || err);
      } finally {
        saveBtn.disabled = false;
      }
    });
  }

  const addBtn = root.querySelector('#ir-add-dir');
  if (addBtn) {
    addBtn.addEventListener('click', () => {
      const legacy = document.getElementById('add-dir-btn');
      if (legacy) legacy.click();
    });
  }
}
