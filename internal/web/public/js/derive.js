// Client-side rollup of /api/status into the UI's RoundSummary[].
// Reuses existing server fields only — does not invent new shapes.
//
// Shape produced:
//   [
//     { number: 1, current: false, phases: [{phase,state,retries,file,source}, …4] },
//     …
//     { number: N, current: true,  phases: [{phase,state,retries,file,source}, …4] },
//   ]
//
// Phase states: 'done' | 'retried' | 'pending' | 'running' | 'missing' | 'failed'.

import { PHASE_IDS } from './util.js';

const PHASES = ['plan', 'review', 'implement', 'verify'];

function phaseIdFromName(name) {
  if (!name || name === 'done') return 5;
  return PHASE_IDS[name] || 1;
}

export function deriveRoundSummaries(status, session) {
  if (!status) return [];
  const out = [];

  // Archived rounds (status.rounds[])
  for (const r of (status.rounds || [])) {
    out.push({
      number: r.number,
      current: false,
      phases: PHASES.map(p => {
        const exists = !!(r.artifacts && r.artifacts[p] && r.artifacts[p].exists);
        const retries = (r.attempt_artifacts || []).filter(a => a.phase === p).length;
        const state = exists ? (retries > 0 ? 'retried' : 'done') : 'missing';
        return {
          phase: p,
          state,
          retries,
          file: `${p}.md`,
          source: { kind: 'round', round: r.number, phase: p },
        };
      }),
    });
  }

  // Current round
  const currentNum = (status.followup_round || 0) + 1;
  const currentPhaseId = phaseIdFromName(status.phase);

  out.push({
    number: currentNum,
    current: true,
    phases: PHASES.map((p, i) => {
      const id = i + 1;
      const exists = !!(status.artifacts && status.artifacts[p] && status.artifacts[p].exists);
      const retries = (status.attempt_artifacts || []).filter(a => a.phase === p).length;
      let state;
      if (status.phase === 'done') {
        state = 'done';
      } else if (id < currentPhaseId) {
        state = 'done';
      } else if (id === currentPhaseId) {
        if (session && session.active && session.phase === p) state = 'running';
        else state = exists ? 'done' : 'pending';
      } else {
        state = 'pending';
      }
      if (state === 'done' && retries > 0) state = 'retried';
      return {
        phase: p,
        state,
        retries,
        file: `${p}.md`,
        source: { kind: 'current', round: currentNum, phase: p },
      };
    }),
  });

  return out;
}
