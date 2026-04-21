// REST helpers. All calls are workflow-scoped where a name is provided —
// /api/workflow/{name}/... paths avoid dependence on the server's active
// workflow file, which can lag behind UI selection.

export async function api(path, opts = {}) {
  const res = await fetch(`/api${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  });
  return res.json();
}

export function wfApi(workflowName, path, opts = {}) {
  if (!workflowName) return api(path, opts);
  return api(`/workflow/${encodeURIComponent(workflowName)}${path}`, opts);
}
