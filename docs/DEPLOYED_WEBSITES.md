# Helix OTA — Deployed Website Targets & Test Links

**Revision:** 1
**Last modified:** 2026-07-13T01:10:00Z

Complete inventory of all Firebase-hosted web surfaces for the `helix-ota` project.
All three targets are live, verified HTTP 200, and listed below.

---

## Firebase Project

| Field | Value |
|---|---|
| Project ID | `helix-ota` |
| Console | https://console.firebase.google.com/project/helix-ota/overview |
| Default URL | https://helix-ota.web.app |

---

## 1. Dashboard (Operator Console)

| Field | Value |
|---|---|
| **Test link** | **[https://helix-ota.web.app](https://helix-ota.web.app)** |
| Firebase target | `dashboard` |
| Firebase site | `helix-ota` |
| Source directory | `dashboard/dist` |
| Purpose | Fleet management dashboard — releases, deployments, device groups, telemetry |
| SEO | `noindex` (admin SPA — not a public surface) |

---

## 2. OTA Manager (Admin SPA)

| Field | Value |
|---|---|
| **Test link** | **[https://helix-ota-manager.web.app](https://helix-ota-manager.web.app)** |
| Firebase target | `ota-manager` |
| Firebase site | `helix-ota-manager` |
| Source directory | `clients/ota-manager/dist` |
| Purpose | OTA administration console — artifact upload, signing, release management |
| SEO | `noindex` (admin SPA) |

---

## 3. Public Marketing Website

| Field | Value |
|---|---|
| **Test link** | **[https://helix-ota-website.web.app](https://helix-ota-website.web.app)** |
| Firebase target | `website` |
| Firebase site | `helix-ota-website` |
| Source directory | `submodules/website/dist/helix_ota_website/browser` |
| Source repo | `github.com/HelixDevelopment/helix_ota_website` |
| Framework | Angular 22 SSR/SSG |
| Design | 9-section single-page home — OpenDesign Helix-green brand |
| SEO | crawlable/indexable (§11.4.190) |

---

## Quick Reference — All Links

| # | Site | URL |
|---|---|---|
| 1 | Dashboard | **https://helix-ota.web.app** |
| 2 | OTA Manager | **https://helix-ota-manager.web.app** |
| 3 | Marketing Website | **https://helix-ota-website.web.app** |

---

## Configuration

Three named Firebase hosting targets defined in `firebase.json`:

```json
"dashboard"   → public: "dashboard/dist"
"ota-manager" → public: "clients/ota-manager/dist"
"website"     → public: "submodules/website/dist/helix_ota_website/browser"
```

Site ID to target mapping in `.firebaserc`:

```json
"dashboard"   → site: "helix-ota"
"ota-manager" → site: "helix-ota-manager"
"website"     → site: "helix-ota-website"
```

---

## Deploy Commands

```bash
firebase deploy --only hosting:dashboard
firebase deploy --only hosting:ota-manager
firebase deploy --only hosting:website
```

No custom domain is configured. All sites serve on `*.web.app` subdomains.
The design spec (`WEBSITE_DESIGN_PROPOSAL.md`) references a future production
domain — this document will be updated when that domain is provisioned.
