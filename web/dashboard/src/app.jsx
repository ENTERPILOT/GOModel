import Router from 'preact-router';
import Sidebar from './components/Sidebar';
import Overview from './pages/Overview';
import Models from './pages/Models';

export default function App() {
  return (
    <div class="flex min-h-screen">
      <Sidebar />
      <main class="flex-1 p-6 ml-64">
        <Router>
          <Overview path="/admin/" />
          <Overview path="/admin" />
          <Models path="/admin/models" />
        </Router>
      </main>
    </div>
  );
}
