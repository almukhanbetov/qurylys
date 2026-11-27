# QURYLYS B2B

B2B-маркетплейс строительных материалов для Казахстана. **Этап 1:** фундамент проекта и каталог из PostgreSQL — категории, товары, предложения нескольких поставщиков, поиск, фильтр, сортировка по цене, интерфейс на казахском и русском языках.

> Все товары, цены и поставщики в базе — **демонстрационные** данные (`is_demo`), а не реальные предложения.

Подробности: [docs/catalog.md](docs/catalog.md) (модель, API, правило минимальной цены), [docs/accounts.md](docs/accounts.md) (пользователи, компании, права), [docs/verification.md](docs/verification.md) (проверка компаний), [docs/roadmap.md](docs/roadmap.md) (этапы), [docs/decisions/](docs/decisions/) (решения), [docs/plans/](docs/plans/) (планы), [docs/testing/](docs/testing/) (приёмка), [docs/requirements-status.md](docs/requirements-status.md) (статус ТЗ), [docs/money.md](docs/money.md) (представление денег), [docs/deployment.md](docs/deployment.md) (CI/CD и production на VPS), [docs/release-stage1.md](docs/release-stage1.md) (публикация этапа 1), [CLAUDE.md](CLAUDE.md) (правила архитектуры).

## Требования (Ubuntu)

- Docker Engine + Docker Compose v2 (`docker compose version`)
- Для разработки без Docker: Go 1.27 (минимальная версия языка в `go.mod` — 1.25), Node.js 26 (версия — `frontend/.nvmrc`, как в CI и Dockerfile), GNU make
- VS Code: расширения *Go* (golang.go), *ESLint*, *Tailwind CSS IntelliSense*

Порты по умолчанию (не пересекаются с типичными 3000/8080/5432):

| Что | Адрес |
|---|---|
| Весь сайт через Nginx (Docker) | http://localhost:8088 |
| API на хосте (`make api`) | http://localhost:8081 |
| Next.js dev (`make web`) | http://localhost:3100 |
| PostgreSQL для разработки | 127.0.0.1:5442 (только с `compose.dev.yml`) |

## Вариант A. Весь проект в Docker

```bash
cd ~/my-projects/QURYLYS
cp .env.example .env
# задайте пароль БД:
sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$(openssl rand -hex 16)/" .env

docker compose up -d --build        # postgres → migrate (up) → api → web → nginx
docker compose run --rm seed        # демо-данные (вручную, идемпотентно)
```

Откройте http://localhost:8088. Миграции применяются автоматически сервисом `migrate` при каждом `up`; демо-данные — **только** вручную.

Проверка:

```bash
docker compose ps                                   # api: healthy, migrate: exited (0)
curl -s localhost:8088/api/v1/health
curl -s localhost:8088/api/v1/ready                 # {"checks":{"database":"up"},...}
curl -s 'localhost:8088/api/v1/products?category=wall-materials&sort=price_asc' | jq '.data.total'   # 4
```

Наружу публикуется только Nginx (по умолчанию на `127.0.0.1`; `HTTP_BIND=0.0.0.0` в `.env` — открыть в сеть). PostgreSQL и API доступны только внутри сети Docker.

Остановить: `docker compose down` (данные в томе `pgdata` сохраняются; `docker compose down -v` — удалить и их).

## Вариант B. Разработка в VS Code (API и Next.js на хосте)

```bash
cd ~/my-projects/QURYLYS
cp .env.example .env                       # пароль БД, как в варианте A
cp backend/.env.example backend/.env       # пароль в DATABASE_URL и TEST_DATABASE_URL = POSTGRES_PASSWORD из .env
cp frontend/.env.example frontend/.env.local
cd frontend && npm ci && cd ..

make db-up            # только PostgreSQL на 127.0.0.1:5442
make migrate-up       # применить миграции
make seed             # демо-данные
```

Затем в двух терминалах VS Code (`Ctrl+Shift+``):

```bash
make api              # терминал 1: API на :8081
make web              # терминал 2: Next.js на http://localhost:3100
```

В этом режиме браузер обращается к API напрямую, поэтому в `backend/.env` разрешены только origins `http://localhost:3100` и `http://127.0.0.1:3100` (`CORS_ALLOWED_ORIGINS`). За Nginx CORS отключён — фронтенд и API на одном домене.

`make help` — список всех команд.

## Регистрация локально и первый администратор

Реальная отправка email — [docs/email.md](docs/email.md). Локально коды пишутся в закрытый файл-«исходящие» (`CONTACT_DELIVERY=local`), в production этот режим запрещён:

```bash
docker compose exec api tail -n 5 /tmp/qurylys-outbox/outbox.jsonl   # Docker
make outbox                                                           # API на хосте
```

Администратор создаётся только служебной командой, пароль читается из stdin:

```bash
read -rsp 'Password: ' P; echo
printf '%s' "$P" | docker compose exec -T api admin create -contact admin@example.kz -name "Имя Фамилия"; unset P
```

## Миграции

```bash
make migrate-up        # применить все
make migrate-status    # статус
make migrate-down      # откатить последнюю
# в Docker:
docker compose run --rm migrate migrate status
```

Новая миграция: файл `backend/migrations/0000N_описание.sql` с секциями `-- +goose Up` / `-- +goose Down` (функции оборачивать в `-- +goose StatementBegin/End`). Команда `migrate reset` запрещена при `APP_ENV=production`.

## Проверки

```bash
make check-backend      # gofmt, go vet, go build
make test               # пересоздаёт БД qurylys_test, unit + интеграционные тесты
make check-frontend     # eslint, tsc --noEmit, next build
cd frontend && npm test # тесты форматирования денег/количеств
```

Интеграционные тесты (`backend/internal/router/router_integration_test.go`) работают **только** с базой, имя которой оканчивается на `_test`: очищают схему, прогоняют все миграции up → down → up, дважды загружают seed и проверяют HTTP-цепочку целиком — дерево категорий, фильтр с подкатегориями, поиск (в т.ч. регистр казахских букв и экранирование `%`/`_`), правило минимальной цены, сортировку, пагинацию, ошибки параметров, предложения и ограничения БД. Без `TEST_DATABASE_URL` они пропускаются.

## Структура

```
backend/
  cmd/api, cmd/migrate, cmd/seed, cmd/admin   # точки входа
  internal/
    config, database, middleware, router, httpx, health
    selleroffers, quantity         # кабинет предложений, точные расчёты
    catalog                        # заявки на новые карточки, GTIN, модерация
    verification                   # заявки, документы, решения MRX
    auth, companies                # пользователи, сессии, компании, членство
    audit, delivery, ratelimit, validate
    categories, products, offers   # model / handler / service / repository
    seed                           # demo.sql
  migrations/                      # Goose SQL
frontend/src/
  app/(public)/                    # главная, catalog, categories, products/[id], login, register, forgot, account/*
  components/{ui,layout}/  features/{catalog,products,auth,account}/
  lib/{api.ts,format.ts,i18n/}  hooks/  types/
deploy/nginx/default.conf          # Nginx локального Docker-стека
deploy/compose.prod.yml            # production: только образы из GHCR
deploy/bin/deploy.sh, ssh-entry.sh # деплой на VPS (docs/deployment.md)
deploy/env/*.env.example           # шаблоны production-переменных
deploy/nginx/qurylys.host.conf.template  # системный Nginx на VPS (шаблон)
compose.yml, compose.dev.yml, Makefile
```
