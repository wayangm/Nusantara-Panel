# Quickstart API - Nusantara Panel

## 1. Login
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<BOOTSTRAP_PASSWORD>"}'
```

Ambil nilai `token` dari response.

## 2. Ganti password admin
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/auth/change-password \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"current_password":"<BOOTSTRAP_PASSWORD>","new_password":"StrongAdminPass123"}'
```

## 3. Cek profil
```bash
curl -sS http://127.0.0.1:8080/v1/auth/me \
  -H "Authorization: Bearer <TOKEN>"
```

## 4. Buat site
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sites \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com","root_path":"/var/www/example","runtime":"php"}'
```
Catatan:
- Untuk runtime `php`/`static`, jika root masih kosong maka panel akan membuat file index bootstrap otomatis agar site tidak langsung 403.

## 5. Hapus site (async deprovision)
```bash
curl -sS -X DELETE http://127.0.0.1:8080/v1/sites/<SITE_ID> \
  -H "Authorization: Bearer <TOKEN>"
```

## 6. Edit konten index site tanpa SSH
Load file:
```bash
curl -sS "http://127.0.0.1:8080/v1/sites/<SITE_ID>/content?file=index.html" \
  -H "Authorization: Bearer <TOKEN>"
```

Simpan file:
```bash
curl -sS -X PUT http://127.0.0.1:8080/v1/sites/<SITE_ID>/content \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"file":"index.html","content":"<h1>Updated from API</h1>"}'
```

## 7. Upload/list/delete file site
List file:
```bash
curl -sS "http://127.0.0.1:8080/v1/sites/<SITE_ID>/files?dir=assets" \
  -H "Authorization: Bearer <TOKEN>"
```

Upload file (contoh payload base64 dipersingkat):
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sites/<SITE_ID>/files/upload \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"path":"assets/logo.txt","content_base64":"SGVsbG8gTnVzYW50YXJhCg=="}'
```

Delete file:
```bash
curl -sS -X DELETE "http://127.0.0.1:8080/v1/sites/<SITE_ID>/files?path=assets/logo.txt" \
  -H "Authorization: Bearer <TOKEN>"
```

Download file:
```bash
curl -sS "http://127.0.0.1:8080/v1/sites/<SITE_ID>/files/download?path=assets/logo.txt" \
  -H "Authorization: Bearer <TOKEN>" \
  -o logo.txt
```

Create directory:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sites/<SITE_ID>/dirs \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"path":"assets/images"}'
```

Delete directory (recursive):
```bash
curl -sS -X DELETE "http://127.0.0.1:8080/v1/sites/<SITE_ID>/dirs?path=assets/images&recursive=true" \
  -H "Authorization: Bearer <TOKEN>"
```

Backup site content:
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/sites/<SITE_ID>/backup \
  -H "Authorization: Bearer <TOKEN>"
```

## 8. Lihat job, audit, dan monitor
```bash
curl -sS http://127.0.0.1:8080/v1/jobs -H "Authorization: Bearer <TOKEN>"
curl -sS http://127.0.0.1:8080/v1/audit/logs -H "Authorization: Bearer <TOKEN>"
curl -sS http://127.0.0.1:8080/v1/monitor/services -H "Authorization: Bearer <TOKEN>"
```

## 9. Issue SSL
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/ssl/issue \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"domain":"example.com","email":"admin@example.com"}'
```

## 10. Database create + user grant
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/db/databases \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"name":"app_db"}'

curl -sS -X POST http://127.0.0.1:8080/v1/db/users \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"database":"app_db","username":"app_user","password":"StrongPass123","host":"localhost"}'
```

## 11. Run backup snapshot
```bash
curl -sS -X POST http://127.0.0.1:8080/v1/backup/run \
  -H "Authorization: Bearer <TOKEN>"
```



