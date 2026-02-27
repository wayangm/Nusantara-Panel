# Changelog

## v0.1.0 - 2026-02-27
- Initial public baseline for Nusantara Panel API service.
- Native on-server deployment model for Ubuntu 22.04+.
- Core API modules:
  - auth (login, me, change-password),
  - site lifecycle (create/list/get/delete with async jobs),
  - SSL issue/renew (certbot integration),
  - database provisioning endpoints,
  - backup run/restore,
  - audit log query,
  - host and service monitoring.
- Ubuntu installer and systemd service integration (`nusantara-panel`).
- Rebrand complete to Nusantara namespace (`nusantarad`, `NUSANTARA_*`).
