#!/usr/bin/env bash
#
# Helix OTA — scaffold a new reusable submodule repo and push to all hosts.
# Creates README (with metadata table), LICENSE (Apache-2.0), .gitignore,
# docs/ and tests/ placeholders, then configures multi-upstream remotes
# (GitHub + GitLab) and pushes the initial commit to both (§2.1).
#
# Usage: scripts/scaffold_submodule.sh <name> <kind:go|kotlin> <purpose> <boundary>
# Env:   GH_ORG (default HelixDevelopment), GL_GROUP (default helixdevelopment1)
set -euo pipefail

NAME="${1:?name}"; KIND="${2:?kind}"; PURPOSE="${3:?purpose}"; BOUNDARY="${4:?boundary}"
GH_ORG="${GH_ORG:-HelixDevelopment}"; GL_GROUP="${GL_GROUP:-helixdevelopment1}"
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WORK="$(mktemp -d)/$NAME"; mkdir -p "$WORK"; cd "$WORK"

git init -q -b main
cp "$SELF_DIR/LICENSE" LICENSE

if [ "$KIND" = "kotlin" ]; then
  cat > .gitignore <<'EOF'
*.class
.gradle/
build/
local.properties
.idea/
*.iml
_exports/
EOF
else
  cat > .gitignore <<'EOF'
*.exe
*.dll
*.so
*.dylib
*.test
*.out
coverage.*
go.work
go.work.sum
.env
_exports/
EOF
fi

cat > README.md <<EOF
# $NAME

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-06-07 |
| Status | scaffold |
| Part of | [Helix OTA](https://github.com/HelixDevelopment/helix_ota) |
| Language | $KIND |
| License | Apache-2.0 |

## Purpose

$PURPOSE

## Boundary (decoupling)

$BOUNDARY

This is a **reusable, independently versioned** building brick (HelixConstitution
§11.4.28 submodules-as-equal-codebase). It is consumed by Helix OTA and is designed
to be reusable by other projects. It must ship in-depth documentation, user guides,
and full test coverage (§1 four-layer) before leaving \`scaffold\` status.

## Status

Scaffold. Implementation tracked in the Helix OTA spec corpus
(\`docs/research/main_specs/\`). See the master design and the submodule reuse map.

## Mirrors

- GitHub: https://github.com/$GH_ORG/$NAME
- GitLab: https://gitlab.com/$GL_GROUP/$NAME
EOF

mkdir -p docs tests
cat > docs/README.md <<EOF
# $NAME — Documentation

In-depth documentation, user guides, manuals, diagrams and schemes land here
(HelixConstitution §6 / §11.4.65 multi-format export). Currently scaffold.
EOF
cat > tests/README.md <<EOF
# $NAME — Tests

Full test coverage required before release (HelixConstitution §1 four-layer:
source-presence gate, artifact gate, runtime/integration, mutation meta-test).
Currently scaffold.
EOF

git add -A
git -c user.name="Milos Vasic" -c user.email="milos85vasic.3rd@gmail.com" \
    commit -q -m "chore: scaffold $NAME reusable submodule

Initial scaffold: README (purpose+boundary), Apache-2.0 LICENSE, .gitignore,
docs/ and tests/ placeholders. Part of Helix OTA.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>"

git remote add origin "git@github.com:$GH_ORG/$NAME.git"
git remote set-url --add --push origin "git@github.com:$GH_ORG/$NAME.git"
git remote set-url --add --push origin "git@gitlab.com:$GL_GROUP/$NAME.git"
git push -q origin main 2>&1 | tail -2 || true
echo "scaffolded+pushed: $NAME -> github:$GH_ORG/$NAME + gitlab:$GL_GROUP/$NAME"
