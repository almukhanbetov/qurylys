# Публикация этапа 1 (аккаунты и email)

HTTPS настроен 24.11.2026 (снаружи проверено: `https://` — 200, `http://` — 301). Вход в production использует cookie `__Host-…; Secure`, поэтому без HTTPS войти нельзя. Порядок шагов важен.

## 1. HTTPS (выполнено; команды — для повторной настройки)

На VPS, администратор:

```bash
sudo ss -ltnp | grep -E ':(80|443) '
sudo ufw status                          # 443/tcp должен быть разрешён
sudo install -d /var/www/letsencrypt
sudo certbot certonly --webroot -w /var/www/letsencrypt -d qurylys-b2b.kz
```

Затем блок 443 и редирект с 80 (сохранив `/.well-known/acme-challenge/`) по шаблону `deploy/nginx/qurylys.host.conf.template`; в блок 443 копируются все три `location`, включая `X-Forwarded-For`.

```bash
sudo nginx -t && sudo systemctl reload nginx
sudo certbot renew --dry-run
```

Проверка с любой машины:

```bash
curl -sS -o /dev/null -w '%{http_code} %{ssl_verify_result}\n' https://qurylys-b2b.kz/             # 200 0
curl -sS -o /dev/null -w '%{http_code} %{redirect_url}\n' http://qurylys-b2b.kz/                   # 301 https://…
curl -sS https://qurylys-b2b.kz/api/v1/ready
echo | openssl s_client -connect qurylys-b2b.kz:443 -servername qurylys-b2b.kz 2>/dev/null | openssl x509 -noout -issuer -enddate
```

Адрес `www` добавляйте в сертификат, только если для него есть DNS-запись. HSTS — после проверки.

## 2. Переменные VPS

`/opt/qurylys/env/api.env` (владелец `qurylys`, права 600), **добавить** к имеющимся строкам, значения без кавычек:

| Переменная | Значение |
|---|---|
| `CONTACT_DELIVERY` | `smtp` |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_TLS` | `smtp.gmail.com` / `587` / `starttls` |
| `SMTP_USERNAME`, `SMTP_FROM` | адрес ящика |
| `SMTP_PASSWORD` | **отдельный** пароль приложения для VPS, секрет |
| `SMTP_FROM_NAME` | `QURYLYS B2B` |
| `EMAIL_DAILY_LIMIT` | `400` |

`migrate.env`, `backup.env`, GitHub secrets не меняются. `<…>` в файле остаться не должно: `deploy.sh` откажет. Старый контейнер вручную не перезапускать — прежний код режима `smtp` не знает.

```bash
chmod 600 /opt/qurylys/env/api.env && stat -c '%a %U %n' /opt/qurylys/env/*.env
timeout 5 bash -c '</dev/tcp/smtp.gmail.com/587' && echo open
```

## 3. Права в базе

Сравните роли: `grep -oP '://\K[^:]+' /opt/qurylys/env/api.env /opt/qurylys/env/migrate.env`. Если роль **одна**, ничего делать не нужно.

Если разные, до деплоя выполните в `qurylys_b2b` (новые таблицы создаёт роль миграций):

```sql
ALTER DEFAULT PRIVILEGES FOR ROLE <MIGRATE_ROLE> IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO <APP_ROLE>;
```

После миграций можно сузить: `REVOKE UPDATE, DELETE ON audit_events FROM <APP_ROLE>;`. Права на последовательности не нужны (столбцы IDENTITY).

Без прав деплой пройдёт успешно, потому что `/ready` новые таблицы не проверяет, но вход и регистрация ответят 500. Поэтому проверяйте вход отдельно (раздел 6).

## 4. Резервная копия

`deploy.sh` делает копию сам перед миграциями и останавливается, если она не удалась. Дополнительно сделайте копию вручную и **восстановите её в отдельную базу** (`deployment.md`, раздел 8), затем унесите файл с сервера.

До деплоя запишите счётчики, после — сравните:

```bash
psql -d qurylys_b2b -Atc "SELECT (SELECT count(*) FROM companies),(SELECT count(*) FROM products),(SELECT count(*) FROM offers),(SELECT max(version_id) FROM goose_db_version)"
```

## 5. Публикация

1. Commit без `.env` и файлов с паролями: `git status`, добавлять файлы поимённо или `git add -u` плюс нужные новые; `git diff --cached --stat`.
2. Push в `main`: CI проверяет и публикует образы в GHCR. VPS **не меняется**.
3. GitHub → Actions → `ci` → Run workflow → `main`, `deploy=true`.
4. `deploy.sh`: копия → миграции 00006 и 00007 (схема 5 → 7) → запуск → проверки готовности.

## 6. После деплоя

```bash
cd /opt/qurylys
/opt/qurylys/bin/deploy.sh status
curl -sS https://qurylys-b2b.kz/api/v1/auth/options        # email true, phone false
docker compose --env-file state/current.env -f compose.prod.yml exec -T api admin test-email -contact <АДРЕС>
docker compose --env-file state/current.env -f compose.prod.yml logs --tail 200 api | grep -c 'mail delivery failed'
```

Права БД, ожидается 401 (500 — нет прав, раздел 3):

```bash
curl -sS -o /dev/null -w '%{http_code}\n' -H 'X-Qurylys-Csrf: 1' -H 'Origin: https://qurylys-b2b.kz' -H 'Content-Type: application/json' -d '{"contact":"nobody@example.com","password":"not-a-real-password"}' https://qurylys-b2b.kz/api/v1/auth/login
```

Создайте администратора (`deployment.md`, раздел 10). В браузере: регистрация с кодом из письма, выход, вход, восстановление, создание компании, приглашение второго пользователя, каталог, KZ/RU. В DevTools cookie `__Host-qurylys_session` — `Secure`, `HttpOnly`, `Lax`.

## 7. Откат

- Метка схемы прежнего образа — 5, база после миграций — 7. Автовосстановление и обычный откат будут отклонены.
- `/opt/qurylys/bin/deploy.sh rollback --force-schema`: допустимо, миграции только добавляют (старый API на схеме 7 проверен локально).
- Миграции вниз удаляют пользователей и компании кабинета: не выполнять.

## Перед открытием регистрации

Согласие на обработку персональных данных (юридический текст), MFA администратора, контроль `mail delivery failed`.
