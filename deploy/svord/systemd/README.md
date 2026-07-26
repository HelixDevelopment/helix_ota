# HelixOTA (Svord) — systemd units

**Revision:** 1
**Last modified:** 2026-07-14T00:00:00Z
**Classification:** project-specific (§11.4.17) — consumer-layer deploy scaffolding.

Rootless (**user**) systemd units (§11.4.161 — NO sudo/root) that run the HelixOTA
stack + its TLS-cert renewal **on the deploy host**, operating the bundle the
deploy scripts rsync to `~/hxota-stack`.

| Unit | Purpose |
|---|---|
| `hxota-stack.service` | Bring the container stack up at boot (rootless `podman compose up -d`). |
| `hxota-certs-renew.service` | Renew TLS certs via the lets_encrypt submodule + reload the proxy. |
| `hxota-certs-renew.timer` | Fire the renew service twice a day (randomized). |

## Install (as the `hxota` user on the remote host)

```sh
mkdir -p ~/.config/systemd/user
cp hxota-stack.service hxota-certs-renew.service hxota-certs-renew.timer \
   ~/.config/systemd/user/
systemctl --user daemon-reload

# Persist across logout / start at host boot (no active login needed):
loginctl enable-linger hxota

systemctl --user enable --now hxota-stack.service
systemctl --user enable --now hxota-certs-renew.timer
```

## Notes

- **Boots with the system OR distributed per config**: with `enable-linger` the
  stack service starts at host boot; without it, on first login. The cert timer is
  independent and can be disabled per install.
- The units call **rootless** `podman-compose` / `podman compose` only — never
  `sudo`, never a rootful daemon.
- `hxota-certs-renew.service` honestly no-ops (SKIP, §11.4.6) until the
  `lets_encrypt` submodule is incorporated and `renew.env` provides
  `LETS_ENCRYPT_HOME` / `HXOTA_LE_ENTRYPOINT` (confirm the CLI vs the submodule
  README — do NOT guess, §11.4.6 / §11.4.99).
- Secrets (`stack.env`, `renew.env`) are `chmod 600`, never committed (§11.4.10).
