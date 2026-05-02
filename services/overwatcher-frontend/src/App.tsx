import { Routes, Route, NavLink, Outlet } from "react-router-dom";
import { AgentDashboard } from "./components/AgentDashboard";
import { UsersDashboard } from "./components/UsersDashboard";
import { ProjectsDashboard } from "./components/ProjectsDashboard";
import { ProjectDetail } from "./components/ProjectDetail";
import { EventLogDashboard } from "./components/EventLogDashboard";
import { DeploymentDashboard } from "./components/DeploymentDashboard";
import { LoginPage } from "./components/LoginPage";
import { ProtectedRoute } from "./components/ProtectedRoute";
import { UserMenu } from "./components/UserMenu";
import { BootstrapPasswordBanner } from "./components/BootstrapPasswordBanner";

function navLinkClass({ isActive }: { isActive: boolean }) {
  return `text-sm font-medium pb-0.5 ${
    isActive
      ? "text-blue-600 border-b-2 border-blue-600 dark:text-blue-400 dark:border-blue-400"
      : "text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
  }`;
}

function AppShell() {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900">
      <nav className="border-b border-gray-200 bg-white dark:border-gray-700 dark:bg-gray-800">
        <div className="max-w-4xl mx-auto px-6 flex items-center gap-6 h-14">
          <span className="text-lg font-bold text-gray-900 dark:text-gray-100">
            Overwatcher
          </span>
          <div className="flex gap-4 flex-1">
            <NavLink to="/" end className={navLinkClass}>
              Agents
            </NavLink>
            <NavLink to="/users" className={navLinkClass}>
              Users
            </NavLink>
            <NavLink to="/projects" className={navLinkClass}>
              Projects
            </NavLink>
            <NavLink to="/deployments" className={navLinkClass}>
              Deployments
            </NavLink>
            <NavLink to="/events" className={navLinkClass}>
              Events
            </NavLink>
          </div>
          <UserMenu />
        </div>
      </nav>

      <BootstrapPasswordBanner />

      <div className="p-6">
        <Outlet />
      </div>
    </div>
  );
}

function App() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route element={<ProtectedRoute />}>
        <Route element={<AppShell />}>
          <Route index element={<AgentDashboard />} />
          <Route path="users" element={<UsersDashboard />} />
          <Route path="projects" element={<ProjectsDashboard />} />
          <Route path="projects/:id" element={<ProjectDetail />} />
          <Route path="deployments" element={<DeploymentDashboard />} />
          <Route path="events" element={<EventLogDashboard />} />
        </Route>
      </Route>
    </Routes>
  );
}

export default App;
