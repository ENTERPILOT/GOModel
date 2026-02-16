const BASE = '/admin/api/v1';

async function fetchJSON(path) {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    throw new Error(`API error: ${res.status} ${res.statusText}`);
  }
  return res.json();
}

export function getOverview() {
  return fetchJSON('/overview');
}

export function getModels() {
  return fetchJSON('/models');
}
