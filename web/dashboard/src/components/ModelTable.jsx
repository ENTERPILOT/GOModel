import { useState, useMemo } from 'preact/hooks';

export default function ModelTable({ models }) {
  const [search, setSearch] = useState('');
  const [sortKey, setSortKey] = useState('id');
  const [sortAsc, setSortAsc] = useState(true);

  const filtered = useMemo(() => {
    const q = search.toLowerCase();
    let list = models.filter(
      (m) =>
        m.id.toLowerCase().includes(q) ||
        m.provider.toLowerCase().includes(q) ||
        m.owned_by.toLowerCase().includes(q)
    );

    list.sort((a, b) => {
      const aVal = a[sortKey] || '';
      const bVal = b[sortKey] || '';
      const cmp = aVal.localeCompare(bVal);
      return sortAsc ? cmp : -cmp;
    });

    return list;
  }, [models, search, sortKey, sortAsc]);

  function handleSort(key) {
    if (sortKey === key) {
      setSortAsc(!sortAsc);
    } else {
      setSortKey(key);
      setSortAsc(true);
    }
  }

  function SortHeader({ label, field }) {
    const active = sortKey === field;
    return (
      <th
        class="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider cursor-pointer hover:text-gray-700 select-none"
        onClick={() => handleSort(field)}
      >
        {label}
        {active && (
          <span class="ml-1">{sortAsc ? '\u2191' : '\u2193'}</span>
        )}
      </th>
    );
  }

  return (
    <div>
      <div class="mb-4">
        <input
          type="text"
          placeholder="Search models..."
          value={search}
          onInput={(e) => setSearch(e.target.value)}
          class="w-full max-w-md px-4 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
        />
      </div>
      <div class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
        <table class="min-w-full divide-y divide-gray-200">
          <thead class="bg-gray-50">
            <tr>
              <SortHeader label="Model ID" field="id" />
              <SortHeader label="Provider" field="provider" />
              <SortHeader label="Owned By" field="owned_by" />
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200">
            {filtered.length === 0 ? (
              <tr>
                <td colspan="3" class="px-4 py-8 text-center text-sm text-gray-500">
                  {search ? 'No models match your search.' : 'No models available.'}
                </td>
              </tr>
            ) : (
              filtered.map((m) => (
                <tr key={m.id} class="hover:bg-gray-50">
                  <td class="px-4 py-3 text-sm font-mono text-gray-900">{m.id}</td>
                  <td class="px-4 py-3 text-sm">
                    <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800">
                      {m.provider}
                    </span>
                  </td>
                  <td class="px-4 py-3 text-sm text-gray-500">{m.owned_by}</td>
                </tr>
              ))
            )}
          </tbody>
        </table>
        <div class="px-4 py-3 bg-gray-50 text-xs text-gray-500 border-t border-gray-200">
          {filtered.length} of {models.length} models
        </div>
      </div>
    </div>
  );
}
