# Helix OTA API Changelog

## 1.0.0-MVP

### ADDED

| Route | Method | Description |
|-------|--------|-------------|
| `/auth/login` | POST | OAuth2 ROPC login — exchange username/password for token pair |
| `/auth/refresh` | POST | Rotate refresh token into new access/refresh pair |
| `/auth/select-account` | POST | Select target account; returns account-scoped token pair (M4) |
| `/admin/accounts` | GET | List all accounts (super-admin only) |
| `/admin/accounts` | POST | Create a new account (super-admin only) |
| `/admin/accounts/:id` | GET | Get account by ID (super-admin only) |
| `/admin/accounts/:id` | PATCH | Update account (super-admin only) |
| `/admin/accounts/:id` | DELETE | Delete account (super-admin only) |
| `/admin/accounts/:id/suspend` | POST | Suspend an account (super-admin only) |
| `/admin/accounts/:id/unsuspend` | POST | Unsuspend an account (super-admin only) |
| `/admin/accounts/:id/archive` | POST | Archive an account (super-admin only) |
| `/admin/accounts/:accountId/members` | POST | Set account membership (super-admin only) |
| `/accounts/:accountId/projects` | GET | List projects scoped to an account |
| `/devices/register` | POST | Provision a device, mint device-scoped bearer token |
| `/devices` | GET | List registered devices |
| `/devices/by-hardware/:hardwareId` | GET | Look up device by hardware ID |
| `/devices/:deviceId/status` | GET | Current registry + last-known runtime status |
| `/devices/:deviceId/telemetry` | GET | Device telemetry event history, cursor-paginated |
| `/artifacts/upload` | POST | Upload OTA artifact with server-side SHA-256 + signature validation |
| `/artifacts/:artifactId` | GET | Artifact metadata |
| `/deltas` | POST | Register a base→target delta artifact |
| `/deltas` | GET | Look up delta for base+target artifact pair |
| `/deltas/generate` | POST | On-demand delta generation |
| `/releases` | POST | Publish a validated artifact as a deployable version |
| `/releases` | GET | List releases (paginated, filterable) |
| `/releases/:releaseId` | GET | Read a single release |
| `/deployments` | POST | Create deployment (strategy=all-targets for MVP) |
| `/deployments` | GET | List active deployments |
| `/deployments/:deploymentId` | GET | Read deployment with aggregate progress |
| `/deployments/:deploymentId/rollout` | POST | Create + start staged rollout |
| `/deployments/:deploymentId/rollout` | GET | Read rollout state |
| `/deployments/:deploymentId/rollout/evaluate` | POST | Submit health verdict, receive engine decision |
| `/deployments/:deploymentId/recall` | POST | Server-driven recall (forward-fix rollback) |
| `/deployments/:deploymentId/rollbacks` | GET | Rollback/abort history for a deployment |
| `/client/update` | GET | Device update check — 204 up-to-date, 200 with apply instructions |
| `/client/telemetry` | POST | Device reports lifecycle events + health (async ingest) |
| `/telemetry/overview` | GET | Fleet-wide telemetry counts + failure rate |
| `/groups` | POST | Create device group |
| `/groups` | GET | List device groups, cursor-paginated |
| `/groups/:groupId` | GET | Read a single group |
| `/groups/:groupId` | PATCH | Update group name/description |
| `/groups/:groupId` | DELETE | Delete group (admin only) |
| `/groups/:groupId/members` | GET | List group members with join times |
| `/groups/:groupId/members` | POST | Batch-add devices to group |
| `/groups/:groupId/members/:deviceId` | DELETE | Remove device from group |
| `/projects` | POST | Create project (account-scoped) |
| `/projects` | GET | List projects (account-scoped) |
| `/projects/:projectId` | GET | Get project |
| `/projects/:projectId` | PATCH | Update project (admin only) |
| `/projects/:projectId` | DELETE | Delete project (admin only) |
| `/projects/:projectId/members` | GET | List project members |
| `/projects/:projectId/members` | POST | Add project member (admin only) |
| `/projects/:projectId/members/:userId` | PATCH | Update project member role (admin only) |
| `/projects/:projectId/members/:userId` | DELETE | Remove project member |
| `/webhooks` | POST | Create webhook (project-scoped callback) |
| `/webhooks` | GET | List webhooks |
| `/webhooks/:id` | DELETE | Delete webhook |
| `/branches` | POST | Create branch (release channel) |
| `/branches` | GET | List branches |
| `/branches/:id` | GET | Get branch |
| `/branches/:id` | PATCH | Update branch |
| `/branches/:id` | DELETE | Delete branch |
| `/fabric/nodes` | POST | Register fabric node (admin only) |
| `/fabric/nodes/:nodeId` | GET | Get fabric node (admin only) |
| `/fabric/targets` | POST | Register fabric target (admin only) |
| `/fabric/targets` | GET | List fabric targets (admin only) |
| `/audit` | GET | Read audit log (admin only, paginated) |
| `/healthz` | GET | Health probe (unauthenticated) |
| `/readyz` | GET | Readiness probe (unauthenticated) |
| `/metrics` | GET | Prometheus metrics (when metrics enabled) |

### MODIFIED

None (initial release).

### DEPRECATED

None.
