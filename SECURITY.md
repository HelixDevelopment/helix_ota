# Helix OTA — Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | Active development |
| 0.x     | Pre-release (MVP)  |

## Vulnerability Disclosure

If you discover a security vulnerability in Helix OTA, please report it
responsibly:

1. **DO NOT** open a public GitHub issue.
2. Email the report to **security@helixota.dev**.
3. Include a detailed description, steps to reproduce, and any proof-of-concept
   code or payloads.
4. We aim to acknowledge receipt within 72 hours and provide an initial
   assessment within 5 business days.
5. We request a 90-day embargo from the date of acknowledgement before any
   public disclosure.

## PGP Key

```
-----BEGIN PGP PUBLIC KEY BLOCK-----

mQINBG... (placeholder — replace with the project's published key)
-----END PGP PUBLIC KEY BLOCK-----
```

The project's published PGP key fingerprint:
`XXXX XXXX XXXX XXXX XXXX  XXXX XXXX XXXX XXXX XXXX`

Sensitive communications should be encrypted to this key.

## Scope

- The Helix OTA control-plane server (`server/`)
- The Helix OTA operator dashboard (`dashboard/`)
- The Helix OTA desktop manager client (`clients/ota-manager/`)
- The OTA protocol wire specification (`submodules/ota-protocol/`)
- The OTA artifact validator (`submodules/ota-artifact-validator/`)
- The OTA rollout engine (`submodules/ota-rollout-engine/`)

## Out of Scope

- Third-party dependencies (report upstream)
- Demo / example deployments
- Social engineering attacks
- Physical device access

## Preferred Languages

English.

## Acknowledgments

We maintain a [hall of fame](https://helixota.dev/security/thanks) for
responsible disclosures. Reporters are acknowledged with their consent.

## Policy

This policy is published at `SECURITY.md` in the repository root, mirrored at
`/.well-known/security.txt` via the HTTP server, and at
`https://helixota.dev/.well-known/security.txt`.

Last updated: 2026-07-26
