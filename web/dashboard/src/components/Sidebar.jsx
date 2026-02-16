import { useRoute } from 'preact-router';

const navItems = [
  { href: '/admin/', label: 'Overview', icon: HomeIcon },
  { href: '/admin/models', label: 'Models', icon: ModelsIcon },
];

function HomeIcon() {
  return (
    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
        d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0h4" />
    </svg>
  );
}

function ModelsIcon() {
  return (
    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
        d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
    </svg>
  );
}

export default function Sidebar() {
  return (
    <aside class="fixed left-0 top-0 h-full w-64 bg-gray-900 text-white flex flex-col">
      <div class="p-5 border-b border-gray-700">
        <h1 class="text-xl font-bold tracking-tight">GOModel</h1>
        <p class="text-xs text-gray-400 mt-1">Admin Dashboard</p>
      </div>
      <nav class="flex-1 py-4">
        {navItems.map((item) => (
          <NavLink key={item.href} href={item.href} label={item.label} Icon={item.icon} />
        ))}
      </nav>
    </aside>
  );
}

function NavLink({ href, label, Icon }) {
  const [matches] = useRoute(href);
  const active = matches;

  return (
    <a
      href={href}
      class={`flex items-center gap-3 px-5 py-2.5 text-sm transition-colors ${
        active
          ? 'bg-gray-800 text-white border-r-2 border-blue-500'
          : 'text-gray-300 hover:bg-gray-800 hover:text-white'
      }`}
    >
      <Icon />
      {label}
    </a>
  );
}
