import { useState, useEffect } from 'preact/hooks';
import { getOverview } from '../api';
import StatsCard from '../components/StatsCard';

export default function Overview() {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    getOverview()
      .then(setData)
      .catch((err) => setError(err.message));
  }, []);

  if (error) {
    return (
      <div class="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">
        Failed to load overview: {error}
      </div>
    );
  }

  if (!data) {
    return <p class="text-gray-500 text-sm">Loading...</p>;
  }

  return (
    <div>
      <h2 class="text-2xl font-bold text-gray-900 mb-6">Overview</h2>
      <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard title="Models" value={data.model_count} />
        <StatsCard title="Providers" value={data.provider_count} />
        <StatsCard title="Uptime" value={data.uptime} />
        <StatsCard title="Version" value={data.version} subtitle={data.go_version} />
      </div>
    </div>
  );
}
