# Skema Data V1 Draft - Nusantara Panel

Target skema relasional tahap produksi: PostgreSQL 15+.

Catatan implementasi saat ini: service masih memakai persistence file JSON lokal (`NUSANTARA_DB_PATH`) untuk mempercepat delivery fase awal.

## 1. users
```sql
create table users (
  id uuid primary key,
  username text not null unique,
  password_hash text not null,
  role text not null check (role in ('admin', 'user')),
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

## 2. sites
```sql
create table sites (
  id uuid primary key,
  domain text not null unique,
  root_path text not null,
  runtime text not null,
  status text not null check (status in ('provisioning', 'active', 'deleting', 'failed')),
  created_by uuid not null references users(id),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

## 3. ssl_certificates
```sql
create table ssl_certificates (
  id uuid primary key,
  site_id uuid not null references sites(id) on delete cascade,
  provider text not null default 'letsencrypt',
  status text not null check (status in ('issued', 'pending', 'failed', 'expired')),
  expires_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);
```

## 4. jobs
```sql
create table jobs (
  id uuid primary key,
  type text not null,
  status text not null check (status in ('queued', 'running', 'success', 'failed')),
  payload jsonb not null default '{}'::jsonb,
  error_message text,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz not null default now()
);

create index jobs_status_created_at_idx on jobs(status, created_at desc);
```

## 5. audit_logs
```sql
create table audit_logs (
  id bigserial primary key,
  actor_user_id uuid references users(id),
  action text not null,
  target_type text not null,
  target_id text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index audit_logs_created_at_idx on audit_logs(created_at desc);
```

## Catatan
- Table `sites` dan `jobs` jadi prioritas implementasi fase pertama.
- Trigger `updated_at` bisa ditambah saat migrasi SQL nyata dibuat.




