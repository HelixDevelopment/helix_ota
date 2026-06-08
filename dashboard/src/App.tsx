// Helix OTA — application router (design §4 component architecture, §6 route map).
//
// AppRoot
//  └ AuthProvider (session/tokens/refresh)
//     └ Router
//        ├ /login (public)
//        └ ProtectedRoute -> AppShell -> feature screens

import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { AppShell, ProtectedRoute, PublicOnly } from "./components/AppShell";
import { LoginScreen } from "./screens/LoginScreen";
import { DashboardOverview } from "./screens/OverviewScreen";
import { ArtifactUploadScreen } from "./screens/ArtifactUploadScreen";
import {
  ReleaseCreateScreen,
  ReleaseDetail,
  ReleaseList,
} from "./screens/ReleasesScreen";
import {
  DeploymentCreateScreen,
  DeploymentDetail,
  DeploymentList,
} from "./screens/DeploymentsScreen";
import { DeviceDetail, FleetHealth } from "./screens/FleetScreen";
import { GroupCreateScreen, GroupDetail, GroupList } from "./screens/GroupsScreen";
import { AuditScreen } from "./screens/AuditScreen";

export function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route
            path="/login"
            element={
              <PublicOnly>
                <LoginScreen />
              </PublicOnly>
            }
          />

          <Route
            element={
              <ProtectedRoute>
                <AppShell />
              </ProtectedRoute>
            }
          >
            <Route index element={<DashboardOverview />} />
            <Route path="artifacts/upload" element={<ArtifactUploadScreen />} />
            <Route path="releases" element={<ReleaseList />} />
            <Route path="releases/new" element={<ReleaseCreateScreen />} />
            <Route path="releases/:releaseId" element={<ReleaseDetail />} />
            <Route path="deployments" element={<DeploymentList />} />
            <Route path="deployments/new" element={<DeploymentCreateScreen />} />
            <Route path="deployments/:deploymentId" element={<DeploymentDetail />} />
            <Route path="fleet" element={<FleetHealth />} />
            <Route path="fleet/:deviceId" element={<DeviceDetail />} />
            <Route path="groups" element={<GroupList />} />
            <Route path="groups/new" element={<GroupCreateScreen />} />
            <Route path="groups/:groupId" element={<GroupDetail />} />
            <Route path="audit" element={<AuditScreen />} />
          </Route>

          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
