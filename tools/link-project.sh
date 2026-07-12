#!/bin/bash
# Purpose:  Link an existing OTA project to a multi-tenant account by calling
#           PATCH /api/v1/projects/:projectId with an account_id. This is the
#           M5 project-side integration step — an account admin runs this once
#           per project to bring the project under the account's tenant scope.
# Usage:    link-project.sh <api-url> <token> <project-id> <account-id>
# Inputs:   api-url     — base URL of the OTA server (e.g. http://localhost:8080)
#           token       — an admin-scoped Bearer token (must have admin on the project)
#           project-id  — the project to link
#           account-id  — the target account (must exist, token-holder must be a member)
# Outputs:  JSON response from the PATCH endpoint confirming the link
# Side-effects: Updates the project's account_id in the store
# Dependencies: curl
# Cross-references: server/internal/api/handlers_project.go (handleUpdateProject),
#                   server/internal/store/store.go (Project.AccountID)

set -euo pipefail

if [ $# -ne 4 ]; then
    echo "Usage: $0 <api-url> <token> <project-id> <account-id>" >&2
    exit 1
fi

API_URL="${1%/}"
TOKEN="$2"
PROJECT_ID="$3"
ACCOUNT_ID="$4"

curl -s -w "\n%{http_code}" \
    -X PATCH "${API_URL}/api/v1/projects/${PROJECT_ID}" \
    -H "Authorization: Bearer ${TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"account_id\":\"${ACCOUNT_ID}\"}"
