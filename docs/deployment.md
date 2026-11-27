# Production: CI/CD и VPS

> **Статус:** конфигурация подготовлена и проверена локально. Сайт работает на https://qurylys-b2b.kz; публикация этапа 1 — [release-stage1.md](release-stage1.md).

## Схема

```
PR ──► ci: backend (PG16 libc C.UTF-8 + PG17 builtin) · frontend · сборка образов без push
push main ──► ci: проверки ──► publish: ghcr.io/almukhanbetov/qurylys-{backend,frontend}:<sha>
Run workflow (main, deploy=true) ──► проверки ──► publish ──► deploy (environment production)
                                                                │ ssh (ключ, закреплённый host key)
                                                                ▼
VPS 83.141.104.203:22122, пользователь qurylys, /opt/qurylys
  bin/ssh-entry.sh (forced command) ─► bin/deploy.sh deploy <backend-ref> <frontend-ref>
    flock ─► проверка ссылок ─► docker pull ─► pg_dump + pg_restore --list ─► migrate up
    ─► compose up -d (без down) ─► /api/v1/ready + / ─► state/current.env
  системный Nginx :80 ─► 127.0.0.1:18080 (api) / 127.0.0.1:13000 (web)
  системный PostgreSQL 16 ◄─ Unix socket /var/run/postgresql (TCP 5432 наружу не открывается)
```

Ссылка на образ всегда содержит полный SHA коммита и digest:
`ghcr.io/almukhanbetov/qurylys-backend:<40 hex>@sha256:<64 hex>`. `latest` не используется. `deploy.sh` принимает только такие ссылки на эти два репозитория и требует, чтобы backend и frontend были собраны из одного коммита, а метка `org.opencontainers.image.revision` совпадала с SHA.

## 0. Чего ещё не хватает

Перед первым деплоем администратор должен выяснить и вписать на сервере:

| Данные | Где используются |
|---|---|
| Роль PostgreSQL для API (`<APP_ROLE>`) и её пароль | `env/api.env`, `pg_hba.conf` |
| Роль для миграций (`<MIGRATE_ROLE>`): владелец `qurylys_b2b` или её схемы `public`; может совпадать с `<APP_ROLE>` | `env/migrate.env`, `pg_hba.conf` |
| Роль для `pg_dump` (`<BACKUP_ROLE>`), обычно та же, что для миграций | `env/backup.env`, `~/.pgpass` |
| Что уже есть в `qurylys_b2b`: таблицы, установлен ли `pg_trgm`, есть ли `goose_db_version` | раздел 3.4 |
| Хранятся ли пароли ролей как `SCRAM-SHA-256` | раздел 3.4 |
| Свободны ли порты `127.0.0.1:18080` и `127.0.0.1:13000` | раздел 3.1 |
| Домен и способ выпуска TLS-сертификата | раздел 3.10 |
| Пакеты GHCR приватные или публичные, учётная запись для чтения | раздел 3.7 |

Пароли и роли в этом документе не выдуманы: везде стоят заполнители `<…>`. `deploy.sh` откажется работать, пока в `env/*.env` остаются `<…>`.

## 1. Почему именно так

- **Сборка только на GitHub-hosted runner.** На VPS нет исходников и `build:`, сервер только скачивает готовые образы.
- **PR не получает ничего лишнего.** Job'ы `publish` и `deploy` для `pull_request` не запускаются, `GITHUB_TOKEN` по умолчанию `contents: read`, `packages: write` есть только у `publish`. Production-секреты лежат в environment `production`, доступном только из `main`.
- **Первый деплой только вручную** (`workflow_dispatch`, `deploy=true`), и он использует digest'ы, собранные и проверенные в этом же запуске.
- **Unix socket вместо TCP.** PostgreSQL продолжает слушать только `localhost`, 5432 не публикуется и не открывается в UFW. Контейнер `migrate` вообще без сети (`network_mode: none`).
- **Docker и UFW.** Опубликованные порты Docker обходят правила UFW, поэтому оба порта публикуются только на `127.0.0.1` и снаружи недоступны.

## 2. Ограничения, о которых нужно знать

- **Группа `docker` = права root.** Кто может управлять Docker, тот может смонтировать `/` хоста в контейнер. Пользователь `qurylys` состоит в группе `docker`, поэтому:
  - CI-ключ ограничен forced command (`restrict,command=…`): с ним можно выполнить только `deploy <ref> <ref>` и `status`;
  - личные ключи администратора и ключ деплоя — разные ключи;
  - `sudo` в workflow не используется и не нужен. **`NOPASSWD:ALL` не выдавать.** Административные шаги ниже администратор выполняет сам, интерактивно.
- **Токен GHCR на диске.** `docker login` сохраняет токен в `~qurylys/.docker/config.json` в base64, без шифрования. Поэтому токен только `read:packages`, от отдельной машинной учётной записи, права файла `600`.
- **Откат схемы не автоматический.** Откат приложения допустим, только если старый код совместим с текущей схемой (раздел 7).
- **Версии тулчейна** (28.11.2026): Go 1.27.4 и Node.js 26.13.1 — одинаково в Dockerfile (обновлены Dependabot), CI (`GO_VERSION`, `frontend/.nvmrc`). Минимальная версия языка в `go.mod` осталась 1.25. Меняются вместе и только отдельной задачей.

## 3. Однократная настройка VPS (администратор, интерактивно)

### 3.1 Docker Engine и Compose

Установка из официального apt-репозитория Docker для Ubuntu 24.04 (`noble`) по инструкции <https://docs.docker.com/engine/install/ubuntu/>: пакеты `docker-ce`, `docker-ce-cli`, `containerd.io`, `docker-buildx-plugin`, `docker-compose-plugin`. Ключ репозитория проверяйте по той же инструкции.

```bash
docker --version && docker compose version
```

Ограничение логов по умолчанию для всех контейнеров (у сервисов QURYLYS оно задано и в `compose.prod.yml`):

```bash
sudo tee /etc/docker/daemon.json >/dev/null <<'EOF'
{ "log-driver": "json-file", "log-opts": { "max-size": "10m", "max-file": "5" } }
EOF
sudo systemctl restart docker
```

Проверить, что порты свободны и Docker ничего не опубликовал наружу:

```bash
ss -ltnp | grep -E ':(18080|13000)\b' || echo "порты свободны"
```

Если порт занят, поменяйте его одновременно в `compose.prod.yml`, в `deploy.sh` (`API_READY_URL`, `WEB_URL`) и в конфиге Nginx.

### 3.2 Пользователь `qurylys` и группа `docker`

```bash
sudo usermod -aG docker qurylys    # см. раздел 2: это права уровня root
```

После этого выйдите из сессии `qurylys` и войдите снова. Утилиты на хосте: `curl`, `flock` (util-linux), `pg_dump`/`pg_restore`/`psql` (`postgresql-client-16` уже стоит вместе с сервером), `sha256sum`.

### 3.3 Каталог `/opt/qurylys`

Код и конфигурация принадлежат root и непривилегированный пользователь не может их менять. Данные (`env`, `state`, `backups`) принадлежат `qurylys`.

```bash
sudo install -d -o root    -g qurylys -m 0750 /opt/qurylys
sudo install -d -o root    -g root    -m 0755 /opt/qurylys/bin
sudo install -d -o qurylys -g qurylys -m 0700 /opt/qurylys/env /opt/qurylys/state /opt/qurylys/backups
```

Файлы из репозитория копируются администратором (scp своим ключом во временный каталог, затем `install`):

```bash
sudo install -o root -g root -m 0755 deploy/bin/deploy.sh deploy/bin/ssh-entry.sh /opt/qurylys/bin/
sudo install -o root -g root -m 0644 deploy/compose.prod.yml /opt/qurylys/compose.prod.yml
```

При изменении этих файлов в репозитории повторите копирование. CI их не обновляет: deploy-ключ не может писать файлы.

### 3.4 PostgreSQL 16: роли, socket, `pg_hba.conf`

**1. Посмотреть, что есть** (ничего не меняет):

```bash
sudo -u postgres psql -d qurylys_b2b -c '\l qurylys_b2b' -c '\du' -c '\dx' -c '\dt'
sudo -u postgres psql -Atc "SHOW password_encryption"
sudo -u postgres psql -Atc "SELECT rolname, left(rolpassword, 14) FROM pg_authid WHERE rolname = '<APP_ROLE>'"
```

- Если в `qurylys_b2b` уже есть таблицы `companies`, `categories`, `products`, `offers` не от этого проекта, миграции упадут. Это нужно решить до деплоя: базу не удалять и не пересоздавать.
- Пароль роли должен начинаться с `SCRAM-SHA-256$`. Если там `md5…`, при `password_encryption = scram-sha-256` задайте пароль заново (`\password <APP_ROLE>`) и согласуйте это с остальными клиентами этой роли.
- Если роль миграций не владелец базы и не может выполнить `CREATE EXTENSION`, создайте расширение заранее:
  `sudo -u postgres psql -d qurylys_b2b -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm'`.

**2. Права на socket.** Контейнер работает с uid 10001, которого на хосте нет, поэтому для него действуют права «прочие» (other).

```bash
stat -c '%A %U:%G %n' /var/run/postgresql /var/run/postgresql/.s.PGSQL.5432
# ожидается: drwxrwsr-x postgres:postgres /var/run/postgresql
#            srwxrwxrwx postgres:postgres /var/run/postgresql/.s.PGSQL.5432
```

Нужны `x` на каталоге и `w` на сокете для «прочих». Это стандартные права Ubuntu (`unix_socket_permissions = 0777`), менять ничего не нужно. Каталог монтируется в контейнер только для чтения, этого достаточно для подключения (проверено локально).

**3. `pg_hba.conf`.** Подключение из контейнера приходит по `local`, но `peer` для uid 10001 не сработает. Добавьте узкие правила **выше** строки `local all all peer` в `/etc/postgresql/16/main/pg_hba.conf`:

```
# QURYLYS: API/migrate containers and host pg_dump, Unix socket only, password (SCRAM)
local   qurylys_b2b   <APP_ROLE>       scram-sha-256
local   qurylys_b2b   <MIGRATE_ROLE>   scram-sha-256
```

Вторая строка нужна, только если роль отличается. Строк `host` для QURYLYS не добавляется, `listen_addresses` не меняется.

```bash
sudo -u postgres psql -Atc "SELECT pg_reload_conf()"
sudo -u postgres psql -c "SELECT line_number, type, database, user_name, auth_method, error FROM pg_hba_file_rules WHERE 'qurylys_b2b' = ANY(database)"
```

Столбец `error` должен быть пустым. UFW для 5432 не трогать.

### 3.5 `~qurylys/.pgpass` (для `pg_dump` на хосте)

```bash
sudo -iu qurylys
umask 077
printf '%s\n' '/var/run/postgresql:5432:qurylys_b2b:<BACKUP_ROLE>:<PASSWORD>' > ~/.pgpass
chmod 600 ~/.pgpass
```

### 3.6 Production-переменные

Шаблоны лежат в `deploy/env/*.env.example`. На сервере, от имени `qurylys`:

```bash
cd /opt/qurylys/env
# скопируйте содержимое шаблонов в api.env, migrate.env, backup.env и замените <…>
chmod 600 api.env migrate.env backup.env
```

- `api.env`: `DATABASE_URL=postgres://<APP_ROLE>:<APP_PASSWORD>@/qurylys_b2b?host=/var/run/postgresql&sslmode=disable` плюс размер пула. Сумма пулов должна укладываться в `max_connections` вместе с другими клиентами.
- `migrate.env`: тот же формат для `<MIGRATE_ROLE>`, плюс `MIGRATE_LOCK_WAIT=2m`.
- `backup.env`: только `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`; пароль берётся из `.pgpass`.

Пароли лучше генерировать как `openssl rand -hex 32`: такие пароли не нужно экранировать в URL. Драйвер pgx принимает и URL с `?host=/var/run/postgresql`, и форму `host=/var/run/postgresql dbname=… user=… password=…`; обе проверены.

Ссылки на образы в эти файлы не пишутся: их ведёт `deploy.sh` в `state/` (`releases/<sha>.env`, `current.env`, `previous.env`, `history.log`).

### 3.7 Доступ VPS к GHCR (только чтение)

Первая публикация создаст пакеты `qurylys-backend` и `qurylys-frontend`. Проверьте их видимость в GitHub → Packages → Package settings. Дальше один из двух вариантов:

- **Публичные пакеты** (Change visibility → Public). Тогда `docker login` на сервере не нужен. В образах нет секретов (проверено), но код фронтенда и бинарники станут доступны всем.
- **Приватные пакеты** (рекомендуется). Нужна отдельная машинная учётная запись GitHub:
  1. в Package settings → Manage access выдать ей **Read** на оба пакета;
  2. создать для неё **classic** PAT только со scope `read:packages` и ограниченным сроком действия. На момент написания GHCR не работает с fine-grained токенами; проверьте актуальное состояние в документации GitHub;
  3. на сервере от имени `qurylys`:
     ```bash
     read -rs GHCR_TOKEN && printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u <MACHINE_ACCOUNT> --password-stdin; unset GHCR_TOKEN
     chmod 600 ~/.docker/config.json
     ```

`GITHUB_TOKEN` из workflow на сервер **никогда** не передаётся: он живёт только во время job'а.

### 3.8 Отдельный SSH-ключ для деплоя

Ключ генерируется на машине администратора, а не на сервере и не в CI:

```bash
ssh-keygen -t ed25519 -N '' -C qurylys-gha-deploy -f ./qurylys-gha-deploy
```

Публичную часть добавьте в `~qurylys/.ssh/authorized_keys` одной строкой:

```
restrict,command="/opt/qurylys/bin/ssh-entry.sh" ssh-ed25519 AAAA… qurylys-gha-deploy
```

`restrict` отключает pty и все виды проброса, `command=` разрешает только `deploy` и `status`. Проверка с машины администратора:

```bash
ssh -i ./qurylys-gha-deploy -p 22122 qurylys@83.141.104.203 status     # работает
ssh -i ./qurylys-gha-deploy -p 22122 qurylys@83.141.104.203 id         # отклоняется
```

Приватный ключ поместите в секрет `DEPLOY_SSH_KEY` (раздел 4), после чего удалите локальную копию.

### 3.9 `known_hosts` — проверка ключа хоста

Отпечаток берётся **по независимому каналу**, например из консоли провайдера или из уже проверенной сессии администратора:

```bash
# на сервере
ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub
# на своей машине
ssh-keyscan -p 22122 -t ed25519 83.141.104.203 > deploy_known_hosts
ssh-keygen -lf deploy_known_hosts        # отпечатки должны совпасть
```

Содержимое `deploy_known_hosts` имеет вид `[83.141.104.203]:22122 ssh-ed25519 AAAA…`; его нужно сохранить в секрет `DEPLOY_KNOWN_HOSTS`. Workflow использует `StrictHostKeyChecking yes` и падает, если ключ хоста не совпал. `StrictHostKeyChecking=no` нигде не используется.

### 3.10 Системный Nginx

Шаблон — `deploy/nginx/qurylys.host.conf.template` (проверен `nginx -t` на 1.24, как в Ubuntu 24.04, и на 1.28):

```bash
sudo cp qurylys.host.conf.template /etc/nginx/sites-available/qurylys.conf
sudo sed -i 's/QURYLYS_DOMAIN_PLACEHOLDER/<ДОМЕН>/g' /etc/nginx/sites-available/qurylys.conf
sudo ln -s ../sites-available/qurylys.conf /etc/nginx/sites-enabled/qurylys.conf
sudo nginx -t && sudo systemctl reload nginx
```

Проверьте, что шаблон не конфликтует с существующими сайтами (`server_name`, `default_server`).

**HTTPS** для `qurylys-b2b.kz` настроен 24.11.2026. Повторная настройка:
1. `sudo install -d /var/www/letsencrypt`;
2. `sudo certbot certonly --webroot -w /var/www/letsencrypt -d <ДОМЕН>`;
3. раскомментировать блок 443 в шаблоне и включить редирект с 80;
4. HSTS включать только после проверки HTTPS в браузере.

До этого сайт работает только по HTTP, вход в кабинет невозможен.

## 4. GitHub: Environment, Secrets, Variables

**Settings → Actions → General**
- Workflow permissions: *Read repository contents and packages permissions*. Workflow сам повышает права только для `publish`.
- Fork pull request workflows: требовать одобрения для внешних участников.

**Settings → Branches**: правило для `main`: изменения только через PR, обязательные проверки `backend (pg16-libc-c-utf8)`, `backend (pg17-builtin)`, `frontend`, `images-build`.

**Settings → Environments → `production`**
- Deployment branches and tags: *Selected branches* → `main`.
- Required reviewers: вы сами. Тогда деплой ждёт подтверждения даже после «Run workflow».

| Тип | Имя | Назначение |
|---|---|---|
| Secret (environment `production`) | `DEPLOY_SSH_KEY` | приватный ключ из 3.8 |
| Secret (environment `production`) | `DEPLOY_KNOWN_HOSTS` | строка host key из 3.9 |
| Variable (environment `production`) | `DEPLOY_HOST` | `83.141.104.203` |
| Variable (environment `production`) | `DEPLOY_PORT` | `22122` |
| Variable (environment `production`) | `DEPLOY_USER` | `qurylys` |
| Variable (repository, необязательно) | `NEXT_PUBLIC_API_BASE_URL` | адрес API для браузера; по умолчанию `/api/v1` |
| автоматически | `GITHUB_TOKEN` | только push в GHCR из job'а `publish` |

Пароли БД, `DATABASE_URL` и токен GHCR в GitHub **не хранятся**.

## 5. Первый ручной деплой

1. Все пункты раздела 3 выполнены; `ssh … status` работает, а другие команды отклоняются.
2. Изменения в `main`; push в `main` прошёл `ci` с job'ом `publish`, пакеты созданы, доступ VPS к ним настроен (3.7).
3. Пробный pull на сервере. Ссылку взять из Summary запуска:
   ```bash
   docker pull 'ghcr.io/almukhanbetov/qurylys-backend:<sha>@sha256:<digest>'
   docker run --rm --network none -v /var/run/postgresql:/var/run/postgresql:ro \
     --env-file /opt/qurylys/env/migrate.env '<та же ссылка>' migrate status
   ```
   `migrate status` показывает список миграций (`pending` для новой базы) и ничего не меняет.
4. GitHub → Actions → **ci** → **Run workflow** → ветка `main`, `deploy` = ✓. Подтвердите деплой в environment, если включены reviewers.
5. Workflow заново проверяет и публикует эту ревизию, затем вызывает `deploy.sh`. Деплой успешен только при зелёном job'е `deploy`. Если readiness не прошёл, job падает и `state/current.env` не создаётся.
6. Проверка на сервере:
   ```bash
   /opt/qurylys/bin/deploy.sh status
   curl -fsS http://127.0.0.1:18080/api/v1/ready
   curl -fsS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:13000/
   curl -fsS -H 'Host: <ДОМЕН>' http://127.0.0.1/api/v1/health      # через Nginx
   ```

Демо-данные в production **не загружаются**. `seed` при `APP_ENV=production` отказывается работать, а в `compose.prod.yml` сервиса seed нет.

## 6. Что делает `deploy.sh deploy`

1. `flock` на `state/deploy.lock` (ждёт до 5 мин); параллельно в GitHub работает `concurrency: production-deploy` без отмены текущего запуска.
2. Проверка ссылок (регулярное выражение по двум разрешённым репозиториям, SHA и digest), env-файлов (владелец, права `600`, нет `<…>`) и наличия утилит.
3. `docker pull` обоих образов и сверка метки ревизии с SHA.
4. `pg_dump -Fc` в `backups/qurylys_b2b-<UTC>-pre-<sha12>.dump` (`umask 077`, файлы `600`, каталог `700`), с проверкой свободного места и версии `pg_dump`.
5. `pg_restore --list`: архив читается, оглавление разбирается; рядом записывается `.sha256`. **Это проверка читаемости, а не восстановления** (раздел 8).
6. `docker compose run --rm migrate` (`migrate up` под advisory lock Goose). Ошибка останавливает деплой, контейнеры приложения не трогаются.
7. `docker compose up -d --wait api web`, без `down`, без удаления volumes, без `prune`.
8. Проверки `GET /api/v1/ready` (с БД) и `GET /` фронтенда: 24 попытки по 5 с, таймаут запроса 5 с.
9. При успехе: `current.env` → `previous.env`, новый релиз → `current.env`, запись в `history.log`.
10. При провале старта: логи, запись `FAILED`. Если есть `current.env` и схема совместима, автоматически поднимается прошлый релиз, но деплой **всё равно завершается с ошибкой**.

Старые образы не удаляются: они нужны для отката. Место на диске освобождайте вручную, только для образов, которых нет в `state/releases/`.

## 7. Откат приложения

```bash
/opt/qurylys/bin/deploy.sh rollback
```

- После неудачного деплоя возвращает `current.env`, то есть последний здоровый релиз; после удачного — `previous.env`.
- Миграции вниз **не выполняются**. Образ backend несёт метку `kz.qurylys.schema-version` (номер последней миграции). Откат отклоняется, если версия схемы в БД больше этого номера.
- `rollback --force-schema` выполняйте только после ручной проверки, что старый код работает с новой схемой.

Чтобы откат оставался возможным, миграции пишутся «расширяющими»: новые таблицы и nullable/default-столбцы. Удаления и переименования выносятся в отдельный последующий релиз (expand → contract). Откат схемы — ручная операция из резервной копии, с остановкой приложения; автоматически безопасным он не считается.

## 8. Резервные копии

- Хранятся в `/opt/qurylys/backups` (`700`, файлы `600`), по одной перед каждым деплоем. Автоматически не удаляются: следите за местом и переносите копии за пределы сервера.
- `pg_restore --list` доказывает только, что архив читается. **Полноценная проверка — восстановление в изолированную базу**, лучше на другой машине:
  ```bash
  createdb qurylys_restore_check
  pg_restore --no-owner --exit-on-error -d qurylys_restore_check qurylys_b2b-<…>.dump
  psql -d qurylys_restore_check -c 'SELECT max(version_id) FROM goose_db_version' -c 'SELECT count(*) FROM products'
  dropdb qurylys_restore_check
  ```
  Никогда не восстанавливайте поверх `qurylys_b2b`, кроме аварии, и только с остановленным `api`.

## 9. Диагностика

```bash
/opt/qurylys/bin/deploy.sh status
docker compose --project-directory /opt/qurylys -f /opt/qurylys/compose.prod.yml \
  --env-file /opt/qurylys/state/current.env logs --tail 100 api web
tail -n 20 /opt/qurylys/state/history.log
```

| Симптом | Причина |
|---|---|
| `failed SASL auth` | пароль в `env/*.env` или `.pgpass`, либо роль не имеет SCRAM-пароля |
| `no pg_hba.conf entry` / `Peer authentication failed` | правило из 3.4 стоит ниже `local all all peer` или не перечитано |
| `failed to acquire lock` | параллельно идёт другая миграция (ждали `MIGRATE_LOCK_WAIT`) |
| `pull failed` / `denied` | доступ к GHCR (3.7) |
| `Host key verification failed` в Actions | `DEPLOY_KNOWN_HOSTS` не совпадает с сервером: проверьте отпечаток, а не отключайте проверку |

## 10. Аккаунты и первый администратор

- Пошаговая публикация: [release-stage1.md](release-stage1.md).
- Миграции 00006–00007 только добавляют таблицы и необязательные столбцы: предыдущая версия с новой схемой работает (проверено локально: `ready`, каталог). Но метка схемы у старого образа — 5, поэтому автовосстановление и обычный `rollback` будут **отклонены**; нужен `rollback --force-schema`.
- `CONTACT_DELIVERY=disabled` (по умолчанию): вход работает, регистрация и восстановление отвечают 503, пока не настроен SMTP ([email.md](email.md)). `local` в production — API не запустится.
- Если роль API отличается от роли миграций, ей нужны права на новые таблицы (`SELECT, INSERT, UPDATE, DELETE`; на `audit_events` достаточно `SELECT, INSERT`).
- Cookie сессии в production — `__Host-…; Secure`, поэтому **вход работает только по HTTPS**. До первого входа проверьте HTTPS (раздел 3.10).
- Лимиты входа считаются по адресу клиента: системный Nginx обязан передавать `X-Forwarded-For`, как в шаблоне.

Создание администратора (интерактивно на сервере, пароль не попадает в историю и аргументы):

```bash
cd /opt/qurylys
read -rsp 'Password: ' P; echo
printf '%s' "$P" | docker compose --env-file state/current.env -f compose.prod.yml exec -T api admin create -contact <EMAIL> -name "<ФИО>"; unset P
```
