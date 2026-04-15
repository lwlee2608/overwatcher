import { Routes, Route, NavLink } from "react-router-dom";
import { AgentDashboard } from "./components/AgentDashboard";
import { MappingDashboard } from "./components/MappingDashboard";
import { EventLogDashboard } from "./components/EventLogDashboard";

function navLinkClass({ isActive }: { isActive: boolean }) {
  return `text-sm font-medium pb-0.5 ${
    isActive
      ? "text-blue-600 border-b-2 border-blue-600 dark:text-blue-400 dark:border-blue-400"
      : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
  }`;
}

function App() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <nav className="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
        <div className="max-w-4xl mx-auto px-6 flex items-center gap-6 h-14">
          <span className="text-lg font-bold text-gray-900 dark:text-gray-100">
            Overwatcher
          </span>
          <div className="flex gap-4">
            <NavLink to="/" end className={navLinkClass}>
              Agents
            </NavLink>
            <NavLink to="/mappings" className={navLinkClass}>
              Mappings
            </NavLink>
            <NavLink to="/events" className={navLinkClass}>
              Events
            </NavLink>
          </div>
        </div>
      </nav>

      <div className="p-6">
        <Routes>
          <Route path="/" element={<AgentDashboard />} />
          <Route path="/mappings" element={<MappingDashboard />} />
          <Route path="/events" element={<EventLogDashboard />} />
        </Routes>
      </div>
    </div>
  );
}

export default App;
