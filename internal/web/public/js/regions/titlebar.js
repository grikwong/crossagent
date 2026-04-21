// TitleBar region — app name, status dot, project filter, terminal toggle.
//
// Scope for commit 4:
//   - Logo + status dot (WS connection) + version
//   - Project filter dropdown (reads store.projects, writes
//     store.selectedProjectFilter). A trailing "Manage projects…" item opens
//     the existing manage-projects modal.
//   - Terminal toggle button (wired in commit 5)
//
// Deferred to commit 6:
//   - Model picker (per-workflow agent chooser)
//   - Ctrl+, / Cmd+, density toggle
//   - "New workflow" + "Tour" buttons (users can keep using the legacy topbar
//     buttons until commit 7 removes them)

import { store, setState } from '../state.js';
import { esc, hashKey } from '../util.js';

let root = null;
let lastKey = '';

export function mount(el) {
  root = el;
  render();
}

export function render() {
  if (!root) return;
  const key = hashKey(
    store.projects.length,
    store.selectedProjectFilter,
    store.terminalDrawerOpen,
    store.session.active,
  );
  if (key === lastKey) return;
  lastKey = key;

  const projects = store.projects || [];
  const filter = store.selectedProjectFilter || '';
  const projectOptions = projects
    .map(p => `<option value="${esc(p.name)}" ${p.name === filter ? 'selected' : ''}>${esc(p.name)} (${p.workflow_count})</option>`)
    .join('');

  root.innerHTML = `
    <div class="tb-left">
      <span class="tb-logo">Crossagent</span>
      <span class="tb-version" id="tb-version"></span>
    </div>
    <div class="tb-center">
      <label class="tb-filter" aria-label="Filter by project">
        <select id="tb-project-filter">
          <option value="">All projects</option>
          ${projectOptions}
          <option disabled>──────</option>
          <option value="__manage__">Manage projects…</option>
        </select>
      </label>
    </div>
    <div class="tb-right">
      <span id="tb-status-dot" class="tb-status-dot ${store.session.active ? 'is-active' : ''}"
        title="${store.session.active ? 'Session running' : 'Idle'}"></span>
      <button class="tb-btn" id="tb-terminal-toggle" aria-pressed="${store.terminalDrawerOpen ? 'true' : 'false'}">
        Terminal${store.terminalDrawerOpen ? ' ▾' : ' ▴'}
      </button>
    </div>
  `;

  const versionEl = document.getElementById('app-version');
  const tbVersion = root.querySelector('#tb-version');
  if (versionEl && tbVersion) tbVersion.textContent = versionEl.textContent;

  root.querySelector('#tb-project-filter').addEventListener('change', (e) => {
    const v = e.target.value;
    if (v === '__manage__') {
      // Open the existing manage-projects modal and revert the dropdown.
      e.target.value = store.selectedProjectFilter;
      const manageBtn = document.getElementById('manage-projects-btn');
      if (manageBtn) manageBtn.click();
      return;
    }
    setState({ selectedProjectFilter: v });
  });

  root.querySelector('#tb-terminal-toggle').addEventListener('click', () => {
    setState({ terminalDrawerOpen: !store.terminalDrawerOpen });
  });
}
