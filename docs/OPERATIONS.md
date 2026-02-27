# Operations Runbook - Nusantara Panel

## 1. Install service
Satu script dari GitHub Release (disarankan):
```bash
curl -fsSL https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/scripts/install_release.sh -o /tmp/nusantara-install.sh
sudo bash /tmp/nusantara-install.sh
```

Install rilis versi tertentu:
```bash
sudo bash /tmp/nusantara-install.sh --tag v0.1.0
```

Alternatif build dari source:
```bash
sudo bash install.sh
```

Atau via repo URL:
```bash
curl -fsSL https://raw.githubusercontent.com/wayangm/Nusantara-Panel/main/install.sh -o /tmp/install.sh
sudo bash /tmp/install.sh --repo https://github.com/wayangm/Nusantara-Panel.git --branch main
```

Metode manual (tanpa installer wrapper):
```bash
go build -o bin/nusantarad ./cmd/nusantarad
sudo ./scripts/install_ubuntu_22.sh ./bin/nusantarad
```

## 2. Verify platform
```bash
systemctl status nusantara-panel
systemctl status nginx
curl -sS http://127.0.0.1:8080/healthz
curl -sS http://127.0.0.1:8080/v1/system/compatibility
```

UI preview:
- `http://<IP_VPS>:8080/`
- `http://<IP_VPS>:8080/ui`

## 3. First login
- Bootstrap username: `admin`
- Bootstrap password: output installer
- Catatan: bootstrap password hanya dipakai untuk seed admin awal saat user belum ada di state DB.
- Wajib segera panggil endpoint `POST /v1/auth/change-password`.

## 4. Create first site
1. Login -> ambil bearer token.
2. `POST /v1/sites`
3. Poll `GET /v1/jobs/{job_id}` sampai `success`.
4. Verifikasi config:
```bash
sudo nginx -t
ls -l /etc/nginx/sites-available
ls -l /etc/nginx/sites-enabled
```

## 5. Delete site
1. `DELETE /v1/sites/{site_id}`
2. Poll job sampai `success`.
3. Verifikasi file config sudah terhapus dari Nginx directories.

## 5b. Edit file index site tanpa SSH
Load file:
```bash
curl -sS "http://127.0.0.1:8080/v1/sites/<SITE_ID>/content?file=index.html" \
  -H "Authorization: Bearer <TOKEN>"
```

Save file:
```bash
curl -sS -X PUT http://127.0.0.1:8080/v1/sites/<SITE_ID>/content \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"file":"index.html","content":"<h1>Updated content</h1>"}'
```

## 5c. Upload/list/delete file site tanpa SSH
List:
```bash
curl -sS "http://127.0.0.1:8080/v1/sites/<SITE_ID>/files?dir=assets" \
  -H "Authorization: Bearer <TOKEN>"
```

Upload:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sites/<SITE_ID>/files/upload \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"path":"assets/readme.txt","content_base64":"SGVsbG8="}'
```

Delete:
```bash
curl -sS -X DELETE "http://127.0.0.1:8080/v1/sites/<SITE_ID>/files?path=assets/readme.txt" \
  -H "Authorization: Bearer <TOKEN>"
```

## 6. Database provisioning
List database:
```bash
curl -sS http://127.0.0.1:8080/v1/db/databases \
  -H "Authorization: Bearer <TOKEN>"
```

Create database:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/db/databases \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"app_db"}'
```

Create user and grant:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/db/users \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"database":"app_db","username":"app_user","password":"StrongPass123","host":"localhost"}'
```

## 7. Backup state
Jalankan backup via API:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/backup/run \
  -H "Authorization: Bearer <TOKEN>"
```

Restore:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/backup/restore \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"file":"/var/backups/nusantara-panel/nusantara_state_20260226_230101.json"}'
```

Setelah restore, restart service direkomendasikan:
```bash
sudo systemctl restart nusantara-panel
```

## 8. SSL issue/renew
Issue cert:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/ssl/issue \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com","email":"admin@example.com"}'
```
Catatan:
- Panel mencoba alur `certbot certonly` lalu `certbot install --nginx --redirect`.
- Jika path webroot standar ada (`/var/www/<domain>/public`), challenge akan menggunakan mode webroot agar lebih stabil.

Renew all cert:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/ssl/renew \
  -H "Authorization: Bearer <TOKEN>"
```

## 9. Update panel tanpa SSH manual build
Via UI:
- Login admin ke `http://<IP_VPS>:8080/`
- Klik `GET /v1/panel/update/check` untuk cek update baru
- Klik `POST /v1/panel/update`
- Pantau `GET /v1/panel/update/status`

Via API:
```bash
curl -sS http://127.0.0.1:8080/v1/panel/update/check \
  -H "Authorization: Bearer <TOKEN>"
```

Trigger update:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/panel/update \
  -H "Authorization: Bearer <TOKEN>"
```

Status update:
```bash
curl -sS http://127.0.0.1:8080/v1/panel/update/status \
  -H "Authorization: Bearer <TOKEN>"
```

Cek versi panel terpasang:
```bash
curl -sS http://127.0.0.1:8080/v1/panel/version \
  -H "Authorization: Bearer <TOKEN>"
```




