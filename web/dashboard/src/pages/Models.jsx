import { useState, useEffect } from 'preact/hooks';
import { getModels } from '../api';
import ModelTable from '../components/ModelTable';

export default function Models() {
  const [data, setData] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    getModels()
      .then(setData)
      .catch((err) => setError(err.message));
  }, []);

  if (error) {
    return (
      <div class="p-4 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">
        Failed to load models: {error}
      </div>
    );
  }

  if (!data) {
    return <p class="text-gray-500 text-sm">Loading...</p>;
  }

  return (
    <div>
      <h2 class="text-2xl font-bold text-gray-900 mb-6">Models</h2>
      <ModelTable models={data.models} />
    </div>
  );
}
