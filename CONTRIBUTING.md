# Contributing to Nusantara Panel

## Prasyarat
- Go 1.24+
- Linux environment untuk uji provisioning nyata (Ubuntu 22.04+)

## Setup cepat
```bash
go test ./...
go build ./cmd/nusantarad
```

Untuk dev non-Linux:
```bash
NUSANTARA_ALLOW_NON_UBUNTU=true NUSANTARA_PROVISION_APPLY=false go test ./...
```

## Standar kontribusi
- Gunakan naming dan istilah sesuai `docs/BRANDING.md`.
- Jaga kompatibilitas target utama Ubuntu 22.04+.
- Jangan memperluas scope tanpa decision log baru di `docs/DECISIONS.md`.
- Setiap perubahan behavior API harus diperbarui di `docs/API_V1.md`.

## Alur pull request
1. Buat branch fitur/perbaikan.
2. Implementasi perubahan + update dokumen.
3. Jalankan `go test ./...`.
4. Kirim pull request memakai template yang tersedia.

## Proses release
1. Perbarui `VERSION` dan `CHANGELOG.md`.
2. Validasi artifact lokal dengan `make package`.
3. Buat tag yang sama dengan `VERSION` (format: `vX.Y.Z`).
4. Push tag ke GitHub untuk memicu workflow release.
