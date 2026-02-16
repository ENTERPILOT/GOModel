export default function StatsCard({ title, value, subtitle }) {
  return (
    <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6">
      <p class="text-sm font-medium text-gray-500">{title}</p>
      <p class="mt-2 text-3xl font-semibold text-gray-900">{value}</p>
      {subtitle && (
        <p class="mt-1 text-sm text-gray-400">{subtitle}</p>
      )}
    </div>
  );
}
