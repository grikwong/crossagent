// Pipeline-timeline (Variation B) bootstrap. This is the active layout.
// The legacy .app shell is kept hidden as a compat layer that exposes modal
// buttons (Follow Up, Manage Projects, Manage Agents) that v2 regions
// delegate to via element.click().

import { store, setState, subscribe } from './state.js';
import * as TitleBar from './regions/titlebar.js';
import * as WorkflowList from './regions/workflow-list.js';
import * as PipelineHeader from './regions/pipeline-header.js';
import * as PipelineBoard from './regions/pipeline-board.js';
import * as ArtifactReader from './regions/artifact-reader.js';
import * as ArtifactInfoRail from './regions/artifact-info-rail.js';
import * as TerminalDrawer from './regions/terminal-drawer.js';

export function initV2() {
  const legacy = document.querySelector('.app');
  const v2 = document.querySelector('.app-v2');
  if (!v2) return;

  // Ensure legacy stays hidden; v2 is the default.
  if (legacy) legacy.classList.add('hidden');
  document.body.dataset.v2 = 'true';
  v2.dataset.density = store.density;

  // Hidden density toggle: Ctrl+, (Win/Linux) / Cmd+, (macOS). Not visible in
  // the TitleBar by design — power users discover it, most won't need it.
  document.addEventListener('keydown', (e) => {
    if (e.key !== ',' || e.target.matches('input, textarea, select')) return;
    const mod = navigator.platform.includes('Mac') ? e.metaKey : e.ctrlKey;
    if (!mod) return;
    e.preventDefault();
    const next = store.density === 'compact' ? 'comfortable' : 'compact';
    setState({ density: next });
  });
  subscribe((s, patch) => {
    if ('density' in patch) v2.dataset.density = s.density;
  });

  // Mount regions.
  TitleBar.mount(document.getElementById('v2-titlebar'));
  WorkflowList.mount(document.getElementById('v2-workflow-list'));
  PipelineHeader.mount(document.getElementById('v2-pipeline-header'));
  PipelineBoard.mount(document.getElementById('v2-pipeline-board'));
  ArtifactReader.mount(document.getElementById('v2-artifact-reader'));
  ArtifactInfoRail.mount(document.getElementById('v2-artifact-info-rail'));
  TerminalDrawer.mount(document.getElementById('v2-terminal-drawer'));

  // Single subscription re-renders every region on store change.
  subscribe(() => {
    TitleBar.render();
    WorkflowList.render();
    PipelineHeader.render();
    PipelineBoard.render();
    ArtifactReader.render();
    ArtifactInfoRail.render();
  });

  // Initial paint (the subscribe above fires only on subsequent setState).
  TitleBar.render();
  WorkflowList.render();
  PipelineHeader.render();
  PipelineBoard.render();
  ArtifactReader.render();
  ArtifactInfoRail.render();
}
