# Arsitektur Teknis V1 - Nusantara Panel

## 1. Batas Sistem
Panel berjalan sebagai daemon `nusantarad` pada satu host Ubuntu 22.04+.

Tanggung jawab v1:
- mengelola metadata panel (user, site, job, audit),
- mengeksekusi operasi sistem melalui layer adapter terkontrol,
- menyediakan API HTTP untuk UI web.

Non-target v1:
- multi-node orchestration,
- high-availability cluster controller.

## 2. Komponen Runtime
- `API Layer`: HTTP endpoint, auth, validasi input.
- `Orchestrator`: membangun workflow task dan state transisi job.
- `System Adapter`: wrapper command Linux allowlist.
- `Config Renderer`: templating Nginx/PHP/service file.
- `Job Worker`: eksekusi async untuk operasi berat.
- `Audit Logger`: catat aksi sensitif.

## 3. Boundary Keamanan
- Default unit saat ini menjalankan service sebagai `root` agar bisa write config Nginx + reload service.
- Roadmap hardening: pindahkan eksekusi privileged ke adapter terkontrol (sudo granular) dan turunkan privilege proses utama.
- Semua perubahan config menjalani:
  1. render ke temp file,
  2. syntax check (`nginx -t`),
  3. atomic move,
  4. reload service.
- Audit wajib untuk login, create/delete site, issue cert, terminal action.

## 4. State & Data
- Metadata store implementasi saat ini: file JSON lokal (`NUSANTARA_DB_PATH`).
- Target evolusi berikutnya: PostgreSQL.
- Direktori data host:
  - `/var/lib/nusantara-panel`: data runtime panel.
  - `/var/log/nusantara-panel`: log panel.
  - `/etc/nusantara-panel`: konfigurasi service panel.

## 5. Request Flow (Contoh: create site)
1. `POST /v1/sites` diterima API.
2. Validasi domain, path, runtime.
3. Simpan metadata `site` status `provisioning`.
4. Buat job provisioning.
5. Worker render vhost, test config, reload nginx.
6. Status site/job diupdate.
7. Audit log ditulis.

## 6. Modul Kode Awal
- `cmd/nusantarad`: bootstrap process.
- `internal/config`: env configuration.
- `internal/httpserver`: router dan handler.
- `internal/platform/oscheck`: validasi Ubuntu 22.04+.
- `internal/jobs`: service enqueue/list/get job.
- `internal/store`: kontrak persistence.
- `internal/store/filedb`: persistence lokal berbasis JSON.
- `internal/service/auth`: login, session token, bootstrap admin.
- `internal/service/sites`: validasi + CRUD site.
- `internal/db`: database manager (list/create db, create user grant).
- `internal/backup`: backup/restore state snapshot.
- `internal/provision`: adapter provisioning Nginx (render, test, reload, rollback).
- `internal/monitor`: probe status service via `systemctl is-active`.
- `internal/ssl`: issue/renew cert via certbot.
- `internal/audit`: audit log service.

## 7. Milestone Teknis Berikutnya
1. Migrasi persistence ke PostgreSQL untuk produksi.
2. Turunkan privilege process + sudo allowlist terbatas.
3. Tambah backup retention policy + scheduler terjadwal.
4. Tambah scheduler renew SSL otomatis + notifikasi kegagalan.
5. Tambah policy granular RBAC per modul.




