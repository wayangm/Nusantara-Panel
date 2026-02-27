# API V1 - Nusantara Panel

## Prinsip
- Base path: `/v1`
- JSON request/response
- Auth model v1: `Authorization: Bearer <token>`
- Semua endpoint sensitif menghasilkan audit log

## Endpoint tersedia (implementasi saat ini)
### `GET /healthz`
Response:
```json
{
  "status": "ok",
  "service": "nusantarad",
  "timestamp": "2026-02-26T14:00:00Z"
}
```

### `GET /v1/system/compatibility`
Response:
```json
{
  "target_os": "ubuntu 22.04+",
  "single_server_only": true,
  "native_on_server": true,
  "compatibility_info": {
    "goos": "linux",
    "id": "ubuntu",
    "version_id": "22.04",
    "supported": true
  }
}
```

### `POST /v1/auth/login`
Request:
```json
{
  "username": "admin",
  "password": "<BOOTSTRAP_PASSWORD>"
}
```
Catatan:
- Endpoint ini memiliki rate limit basic (5 gagal per 5 menit per kombinasi IP+username).
- Jika terblokir sementara, response `429`.
Response:
```json
{
  "token": "....",
  "expires_at": "2026-02-27T14:00:00Z",
  "user": {
    "id": "usr_...",
    "username": "admin",
    "role": "admin"
  }
}
```

### `POST /v1/auth/logout`
- Auth: required

### `GET /v1/auth/me`
- Auth: required

### `POST /v1/auth/change-password`
- Auth: required
Request:
```json
{
  "current_password": "old",
  "new_password": "newStrongPassword123"
}
```

### `GET /v1/sites`
- Auth: admin

### `POST /v1/sites`
- Auth: admin
Request:
```json
{
  "domain": "example.com",
  "root_path": "/var/www/example",
  "runtime": "php"
}
```
Catatan:
- Endpoint ini mengembalikan `site` dengan status awal `provisioning`.
- Eksekusi provisioning berjalan async lewat job worker.
- Pantau progress via `GET /v1/jobs/{job_id}` dan `GET /v1/sites/{site_id}`.

### `GET /v1/sites/{site_id}`
- Auth: admin

### `DELETE /v1/sites/{site_id}`
- Auth: admin
Catatan:
- Endpoint ini asynchronous.
- Site akan masuk status `deleting`.
- Deprovision dijalankan worker job.

### `GET /v1/jobs`
- Auth: admin

### `GET /v1/jobs/{job_id}`
- Auth: admin

### `GET /v1/db/databases`
- Auth: admin

### `POST /v1/db/databases`
- Auth: admin
Request:
```json
{
  "name": "app_db"
}
```

### `POST /v1/db/users`
- Auth: admin
Request:
```json
{
  "database": "app_db",
  "username": "app_user",
  "password": "StrongPass123",
  "host": "localhost"
}
```

### `POST /v1/backup/run`
- Auth: admin

### `POST /v1/backup/restore`
- Auth: admin
Request:
```json
{
  "file": "/var/backups/nusantara-panel/nusantara_state_20260226_230101.json"
}
```

### `POST /v1/ssl/issue`
- Auth: admin
Request:
```json
{
  "domain": "example.com",
  "email": "admin@example.com"
}
```

### `POST /v1/ssl/renew`
- Auth: admin

### `GET /v1/audit/logs`
- Auth: admin

### `GET /v1/monitor/host`
- Auth: admin

### `GET /v1/monitor/services`
- Auth: admin
Response item status diambil dari `systemctl is-active`.

### `POST /v1/panel/update`
- Auth: admin
- Trigger update panel melalui transient unit systemd (`nusantara-panel-updater.service`).
- Operasi ini asynchronous; panel service dapat restart saat update selesai.
Response:
```json
{
  "unit": "nusantara-panel-updater.service",
  "repo_url": "https://github.com/wayangm/Nusantara-Panel.git",
  "branch": "main",
  "script_url": "https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/install.sh",
  "started_at": "2026-02-27T14:00:00Z"
}
```

### `GET /v1/panel/update/check`
- Auth: admin
- Cek commit remote branch updater dibanding commit panel yang sedang berjalan.
- `status`:
  - `up_to_date`
  - `update_available`
  - `unknown`

### `GET /v1/panel/update/status`
- Auth: admin
- Menampilkan state unit updater + potongan log terbaru dari journal.

### `GET /v1/panel/version`
- Auth: admin
- Menampilkan metadata build panel yang sedang berjalan (`version`, `commit`, `build_time`).

## Backlog endpoint berikutnya
- `GET /v1/backup/list`




