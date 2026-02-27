# Nusantara Panel

Kelola server Linux lebih cepat, aman, dan terarah.

Baseline implementasi control panel Linux model `native on-server` dengan backend Go.

![Nusantara Panel Logo](assets/brand/nusantara-panel-logo.svg)

Keputusan tetap untuk v1:
- Deployment: native di host Linux.
- Backend: Go.
- Target OS: Ubuntu 22.04+.
- Arsitektur: single-server.

## Struktur
- `cmd/nusantarad`: entrypoint service.
- `internal/`: modul aplikasi inti.
- `deploy/systemd/`: unit file systemd.
- `scripts/`: utilitas instalasi host dan release installer.
- `docs/`: dokumen teknis lanjutan.

## Jalankan lokal (dev)
```bash
go build -o bin/nusantarad ./cmd/nusantarad
NUSANTARA_ALLOW_NON_UBUNTU=true NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=DevStrongPass123 ./bin/nusantarad
```

Untuk dev non-Linux, disarankan mode dry-run provisioning:
```bash
NUSANTARA_ALLOW_NON_UBUNTU=true NUSANTARA_PROVISION_APPLY=false NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD=DevStrongPass123 ./bin/nusantarad
```

Saat startup pertama, service akan membuat akun admin bootstrap:
- `username`: `admin`
- `password`: sesuai `NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD`

Pada installer Ubuntu, password bootstrap di-generate otomatis saat setup awal (atau saat env password masih default placeholder) dan ditampilkan di output install.
Jika service sudah punya user admin di state DB, reinstall tidak mereset password login akun admin yang sudah ada.

Ganti lewat env:
- `NUSANTARA_BOOTSTRAP_ADMIN_USERNAME`
- `NUSANTARA_BOOTSTRAP_ADMIN_PASSWORD`
- `NUSANTARA_PROVISION_APPLY`
- `NUSANTARA_NGINX_SITES_AVAILABLE_DIR`
- `NUSANTARA_NGINX_SITES_ENABLED_DIR`
- `NUSANTARA_NGINX_TEST_COMMAND`
- `NUSANTARA_NGINX_RELOAD_COMMAND`
- `NUSANTARA_CERTBOT_COMMAND`
- `NUSANTARA_MYSQL_COMMAND`
- `NUSANTARA_BACKUP_DIR`
- `NUSANTARA_UPDATE_REPO_URL`
- `NUSANTARA_UPDATE_BRANCH`
- `NUSANTARA_UPDATE_SCRIPT_URL`
- `NUSANTARA_UPDATE_UNIT_NAME`
- `NUSANTARA_UPDATE_LOG_LINES`
- `NUSANTARA_UPDATE_COOLDOWN_SECS`

Endpoint utama:
- `GET /healthz`
- `GET /` (Web UI preview)
- `GET /ui` (Web UI preview)
- `GET /v1/system/compatibility`
- `POST /v1/auth/login`
- `POST /v1/auth/change-password`
- `GET /v1/auth/me`
- `GET /v1/sites`
- `POST /v1/sites`
- `GET /v1/sites/{site_id}/content`
- `PUT /v1/sites/{site_id}/content`
- `GET /v1/sites/{site_id}/files`
- `POST /v1/sites/{site_id}/files/upload`
- `DELETE /v1/sites/{site_id}/files`
- `GET /v1/jobs`
- `GET /v1/db/databases`
- `POST /v1/db/databases`
- `POST /v1/db/users`
- `POST /v1/backup/run`
- `POST /v1/backup/restore`
- `POST /v1/ssl/issue`
- `POST /v1/ssl/renew`
- `GET /v1/audit/logs`
- `POST /v1/panel/update`
- `GET /v1/panel/update/status`
- `GET /v1/panel/version`
- `GET /v1/panel/update/check`

## Build cepat
```bash
go test ./...
go build ./cmd/nusantarad
```

## Packaging release
```bash
make package
```
Output artifact akan dibuat di direktori `dist/`.

## Deploy Ubuntu 22.04+
Cara paling simpel (binary dari GitHub Release, tanpa compile Go):
```bash
curl -fsSL https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/scripts/install_release.sh -o /tmp/nusantara-install.sh
sudo bash /tmp/nusantara-install.sh
```

Install rilis versi tertentu:
```bash
sudo bash /tmp/nusantara-install.sh --tag v0.1.0
```

Cara simpel alternatif (build dari source dengan satu script):
```bash
sudo bash install.sh
```

Atau jika installer di-run dari URL raw script:
```bash
curl -fsSL https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh --repo https://github.com/wayangm/Nusantara-Panel.git --branch main
```

Cara manual:
1. Build binary `nusantarad`.
2. Jalankan installer:
```bash
sudo ./scripts/install_ubuntu_22.sh ./bin/nusantarad
```
3. Cek service:
```bash
systemctl status nusantara-panel
curl http://127.0.0.1:8080/healthz
```
4. Simpan bootstrap password yang muncul di output installer, login ke API, lalu segera ganti password.
5. Buka UI preview di browser:
```bash
http://<IP_VPS>:8080/
```

Installer akan menyiapkan paket host:
- nginx
- php-fpm
- mariadb-server
- redis-server
- certbot + plugin nginx

## Dokumen
- [Konsep V1](CONCEPT_V1.md)
- [Arsitektur Teknis V1](docs/ARCHITECTURE_V1.md)
- [API V1 Draft](docs/API_V1.md)
- [Skema Data V1 Draft](docs/DB_SCHEMA_V1.md)
- [Decision Log](docs/DECISIONS.md)
- [Branding Guide](docs/BRANDING.md)
- [Brand Kit](docs/BRAND_KIT.md)
- [Quickstart API](docs/QUICKSTART_API.md)
- [Operations Runbook](docs/OPERATIONS.md)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)

## Publikasi GitHub
Rekomendasi metadata repo:
- Repository name: `Nusantara-Panel`
- Description: `Control panel Linux native on-server berbasis Go untuk operasi server yang cepat, aman, dan terarah.`
- Topics: `linux`, `control-panel`, `golang`, `devops`, `nginx`, `systemd`, `server-management`

## Status Implementasi
Sudah tersedia:
- auth token (`login`, `logout`, `me`) dengan middleware RBAC.
- endpoint `change-password`.
- login rate limiting (anti brute-force basic).
- CRUD site dasar (create/list/get/delete async deprovision).
- File editor dasar site (`GET/PUT /v1/sites/{site_id}/content`) untuk file `index.html`, `index.htm`, `index.php`.
- Upload/list/delete file dasar per-site via API/UI (`/v1/sites/{site_id}/files*`) dengan validasi relative path.
- job worker async (queued -> running -> success/failed).
- provisioning Nginx untuk `create site`:
  - render `server` config ke `sites-available`,
  - link ke `sites-enabled`,
  - `nginx -t`,
  - `systemctl reload nginx`,
  - auto bootstrap `index` default untuk runtime `php`/`static` jika root path masih kosong (mencegah 403 saat awal deploy).
- deprovision Nginx untuk `delete site`:
  - unlink dari `sites-enabled`,
  - hapus config `sites-available`,
  - `nginx -t`,
  - `systemctl reload nginx`.
- issue/renew SSL Let's Encrypt via `certbot` endpoint admin.
- database management endpoint:
  - list database,
  - create database,
  - create user + grant privilege database level.
- backup management endpoint:
  - run backup state snapshot,
  - restore state dari file backup terverifikasi path.
- audit log query.
- monitoring host + probe status service Linux via `systemctl is-active`.
- persistence lokal berbasis file JSON di `NUSANTARA_DB_PATH`.
- UI preview mendukung alur SSL dasar (`/v1/ssl/issue` dan `/v1/ssl/renew`) tanpa SSH.

Catatan keamanan:
- hashing password menggunakan bcrypt.
- service berjalan sebagai root pada default unit agar bisa write config Nginx dan reload service.
- setelah login pertama, segera ganti password admin bootstrap.




