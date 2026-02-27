# Konsep V1 - Nusantara Panel

## 1. Tujuan
Membangun control panel yang berjalan langsung di atas Linux untuk mengelola web server, database, domain, SSL, file, backup, dan monitoring melalui antarmuka web.

## 2. Pertanyaan Kunci: Apakah berjalan langsung di atas Linux?
Ya. Untuk v1, model yang dipakai adalah **native on-server**:
- Panel di-install langsung di server Linux (Ubuntu/Debian/AlmaLinux).
- Berjalan sebagai service systemd.
- Mengelola service Linux secara langsung (Nginx, PHP-FPM, MariaDB/MySQL, Redis, firewall, cron, SSL).
- Membutuhkan hak akses root/sudo terkontrol.

## 3. Scope Produk V1 (MVP)
### In Scope
- Auth: login admin, session, optional 2FA.
- RBAC sederhana: admin penuh + user terbatas.
- Site management:
  - Tambah/hapus domain.
  - Generate vhost Nginx.
  - Routing ke PHP-FPM atau app port (reverse proxy).
- SSL management:
  - Issue/renew Let's Encrypt.
  - Pasang sertifikat ke vhost.
- Database management:
  - Buat database + user.
  - Reset password DB user.
- File tools:
  - Upload/download file.
  - Edit file teks ringan.
- Terminal web terbatas (audit log wajib).
- Monitoring dasar:
  - CPU, RAM, disk, load average.
  - Status service (nginx/php-fpm/mysql/redis).
- Backup & restore:
  - Backup site + db ke local path.
  - Restore manual dari snapshot.

### Out of Scope (V1)
- Multi-node orchestration.
- Cluster Kubernetes.
- Marketplace plugin kompleks.
- Billing/WHM-style reseller lengkap.

## 4. Arsitektur Tingkat Tinggi
### Komponen
- **Frontend Web UI**: dashboard operasional.
- **Backend API**: auth, validasi, orchestration job.
- **Job Runner**: eksekusi task async (install/renew/reload).
- **System Adapter Layer**: wrapper aman untuk command Linux.
- **Config Renderer**: template Nginx/PHP/service config.
- **State Store**: metadata panel (user, site, job, log).

### Alur Eksekusi
1. User submit aksi dari UI.
2. API validasi input + izin role.
3. API membuat job.
4. Job Runner menjalankan langkah sistem via adapter (allowlist command).
5. Hasil disimpan ke log + status job.
6. UI polling status dan menampilkan outcome.

## 5. Pilihan Teknologi (Rekomendasi)
- Backend: Go (stabil untuk service + concurrency).
- Frontend: Vue + Vite.
- Database metadata: PostgreSQL (atau SQLite untuk single-node minimal).
- Queue ringan: internal worker atau Redis queue.
- OS target awal: Ubuntu 22.04+.

Catatan: Ini fleksibel. Bila tim lebih nyaman Node.js, backend bisa pakai Node + TypeScript.

## 6. Desain Keamanan Minimal
- TLS wajib untuk panel UI.
- Password hashing: Argon2id/bcrypt.
- CSRF protection + secure cookies.
- Rate limit login + lockout policy.
- RBAC enforcement di semua endpoint.
- Command execution allowlist (tanpa shell injection bebas).
- Backup config sebelum perubahan (safe rollback).
- Audit log immutable-ish untuk aksi sensitif (login, sudo task, delete).

## 7. Struktur Modul Fitur
- `auth`: login, session, 2FA.
- `users`: RBAC admin/user.
- `sites`: domain, vhost, app binding.
- `ssl`: issue/renew/install cert.
- `db`: database & user provisioning.
- `files`: explorer + editor sederhana.
- `terminal`: command terbatas + audit.
- `monitor`: metrics host + status service.
- `backup`: snapshot & restore.
- `jobs`: scheduler, retry, log.

## 8. UX Operasional Utama
- Wizard onboarding:
  - Set password admin pertama.
  - Deteksi OS + dependency check.
  - Install stack dasar (nginx/php/db opsional).
- Dashboard:
  - Status server, alert sederhana, job terbaru.
- Site flow cepat:
  - Create site -> set runtime -> issue SSL -> deploy root path.

## 9. Roadmap Bertahap
### Fase 1 (2-4 minggu)
- Auth, RBAC basic, site CRUD, nginx template, SSL LE, service status.

### Fase 2 (2-4 minggu)
- DB manager, backup/restore, file manager, audit log lengkap.

### Fase 3 (3-6 minggu)
- Hardening security, observability lebih detail, plugin API awal.

## 10. Risiko Teknis dan Mitigasi
- Risiko salah konfigurasi service -> gunakan template tervalidasi + test config sebelum reload.
- Risiko privilege abuse -> strict RBAC + audit + allowlist command.
- Risiko downtime saat update config -> atomic write + backup + rollback otomatis.
- Risiko dependency OS berbeda -> batasi dukungan distro di awal.

## 11. Definisi Sukses V1
- Bisa create dan publish website ber-SSL end-to-end < 5 menit.
- Operasi dasar server bisa dilakukan tanpa SSH manual untuk 80% use case umum.
- Semua aksi sensitif tercatat di audit log.
- Tidak ada command injection dari input user.

## 12. Keputusan Final V1
1. Mode deployment: native on-server.
2. Backend stack: Go.
3. Target OS awal: Ubuntu 22.04+.
4. Arsitektur: single-server.

---
Dokumen ini adalah baseline konsep sebelum masuk desain detail API, skema database, dan implementasi modul.


