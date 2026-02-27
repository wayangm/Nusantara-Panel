# Decision Log V1 - Nusantara Panel

## D-001 Deployment model
- Status: accepted
- Decision: native on-server.
- Rationale: latency rendah, model operasi sederhana, dan akses sistem langsung untuk manajemen service.

## D-002 Backend stack
- Status: accepted
- Decision: Go.
- Rationale: binary tunggal, stabil untuk daemon systemd, concurrency baik untuk worker/job.

## D-003 Target OS awal
- Status: accepted
- Decision: Ubuntu 22.04+.
- Rationale: memperkecil matriks kompatibilitas pada fase awal.

## D-004 Topologi
- Status: accepted
- Decision: single-server architecture.
- Rationale: minim kompleksitas distribusi; fokus validasi use case inti dulu.

## D-005 Persistence fase awal
- Status: accepted
- Decision: file-based JSON store (`NUSANTARA_DB_PATH`) untuk v1 awal.
- Rationale: menghindari dependency eksternal agar bootstrap cepat; tetap diposisikan sebagai langkah transisi sebelum PostgreSQL.

## D-006 Password hashing
- Status: accepted
- Decision: bcrypt (`golang.org/x/crypto/bcrypt`).
- Rationale: baseline keamanan login lebih kuat untuk penggunaan riil.

## D-007 Provisioning Nginx v1
- Status: accepted
- Decision: provisioning site dieksekusi async oleh job worker dengan adapter Nginx (`render -> test -> reload`).
- Rationale: operasi file/service Linux tidak boleh blocking request API.
- Catatan: mode `NUSANTARA_PROVISION_APPLY=false` dipakai untuk dev non-Linux (dry-run).

## D-008 Deprovision site v1
- Status: accepted
- Decision: delete site dilakukan async (`deprovision_site` job) sebelum metadata dihapus.
- Rationale: menghindari orphan config Nginx dan memastikan status operasi bisa diaudit.

## D-009 Runtime privilege v1
- Status: accepted
- Decision: unit service default dijalankan sebagai `root` untuk fase ini.
- Rationale: provisioning membutuhkan write `/etc/nginx` + reload system service.
- Follow-up wajib: migrasi ke model privilege terpisah (daemon non-root + helper terbatas).

## D-010 SSL automation v1
- Status: accepted
- Decision: endpoint admin memanggil `certbot` langsung untuk issue/renew.
- Rationale: mempercepat go-live situs HTTPS pada arsitektur single-server.

## D-011 Database management v1
- Status: accepted
- Decision: endpoint admin memanggil CLI `mysql` untuk list/create database dan create user + grant.
- Rationale: pendekatan pragmatis untuk single-server tanpa dependency tambahan di fase awal.

## D-012 Backup state v1
- Status: accepted
- Decision: backup/restore awal menggunakan snapshot file `nusantara_state.json` ke direktori backup.
- Rationale: recovery cepat dengan kompleksitas rendah sebelum migrasi ke database relasional.




