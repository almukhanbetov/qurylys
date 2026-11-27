# QURYLYS — правила проекта

B2B-маркетплейс строительных материалов для Казахстана. Несколько поставщиков, основной язык интерфейса — казахский, второй — русский.

## Требования (читать перед изменением ролей, доступа и бизнес-логики)

- ТЗ (PDF): `docs/specifications/` (PDF не в git, репозиторий публичный; сверка по SHA-256 из README там же). Роли: `docs/roles-and-permissions.md`. Статус: `docs/requirements-status.md`. Отступления: `docs/spec-deviations.md`. Решения заказчика: `docs/decisions/` (этап 3 — `supplier-stage-3.md`, план — `docs/plans/stage-3-supplier.md`, приёмка — `docs/testing/supplier-stage-3-acceptance.md`; пункты «Доп.» — не требование PDF).
- Перед изменением ролей, прав, цен, заказов, проверки компаний прочитать эти документы. «Требует уточнения» не додумывать — спросить. `admin` ≠ SA, `owner` ≠ CA.
- Вместе с поведением обновлять `docs/requirements-status.md`.

## Стек (не менять без отдельной задачи)

- Backend: Go 1.27 (тулчейн CI и Dockerfile; `go.mod` — язык 1.25) + Gin. БД через `database/sql` + `github.com/jackc/pgx/v5/stdlib`. **Без GORM и lib/pq.**
- Миграции: Goose, только SQL, файлы `backend/migrations/NNNNN_name.sql`, встроены в бинарник `migrate` через `embed`.
- PostgreSQL 17 (builtin-локаль `C.UTF-8`) локально; production — системный PostgreSQL 16 на VPS (libc `C.UTF-8`, БД `qurylys_b2b`, не пересоздавать). CI проверяет обе конфигурации.
- Frontend: Node.js 26 (`frontend/.nvmrc` = Dockerfile = CI), Next.js 16 (App Router) + TypeScript (strict) + Tailwind CSS 4.
- Инфраструктура: Docker Compose + Nginx (единый домен: `/api/` → API, остальное → Next.js). Production: образы из GHCR, `deploy/compose.prod.yml`, системный Nginx, деплой — `docs/deployment.md`.
- Платные сервисы не подключать без отдельной задачи. Ничего не деплоить без отдельной задачи.

## Backend

- Модули по предметной области: `backend/internal/<module>/` с файлами `model.go`, `handler.go`, `service.go`, `repository.go` — только те, что реально нужны. Пустых файлов и абстракций «для структуры» не создавать.
- Зависимости строго: `router → handler → service → repository → PostgreSQL`. Handler не пишет SQL, repository не знает про HTTP.
- `cmd/api/main.go` только собирает конфигурацию, пул и сервер.
- SQL — только параметры `$n`. Пользовательский ввод никогда не попадает в текст запроса. Имена полей сортировки — только из фиксированной карты (`products.sortClauses`).
- Ошибки клиенту: `{"error": {"code", "message"}}` через `internal/httpx`. Внутренние ошибки логируются, клиенту — общее сообщение.
- Конфигурация — только переменные окружения (`internal/config`). Секреты не коммитить; `.env.example` без реальных значений.
- Межмодульные зависимости — без циклов (сейчас `products` использует `offers.AvailableCondition`, `companies` использует `auth.Principal`, `cards` — `catalog.NormalizeGTIN` и `companies.InvolvedSQL`).
- Доступ: сессии — `internal/auth` (HttpOnly-cookie), CSRF — `middleware.CSRF` на всём `/api/v1`. Любой запрос к данным компании проходит через `companies.Authorize` (членство → активная компания → роль). См. `docs/accounts.md`.
- Статус проверки компании меняется только через `internal/verification` (и триггеры миграции 00008); список ключевых реквизитов — в триггере, `companies.keyChange`, `verification.keyChanged` и `CompanyForm.KEY_FIELDS`, меняются вместе. Файлы — только через обработчики с проверкой прав.
- `catalog.moderate`, `catalog.categories`, `catalog.brands` — разрешения (`platform_permissions`, `auth.GrantablePermissions`), не роли; каждое проверяется в транзакции действия (`auth.HasPermission`), ни одно не следует из `admin`/`mrx` или из другого. Список меняется вместе с CHECK `platform_permissions_valid`.
- Карточка товара создаётся только одобрением заявки (`internal/catalog`), модератором с подтверждённым GTIN (`internal/cards`) или seed; GTIN — только подтверждённый. Правка карточки с настоящими предложениями — только косметика (N3: `cards.sameNumbers` и триггер `products_linked_guard`). Демо-карточки меняет только seed.
- `products.brand` — отображаемый текст; у привязанной карточки его ставит триггер из `brands`. Похожие тексты брендов не объединять автоматически.
- 19 ролей ТЗ — справочник `system_roles` (только чтение, `internal/roles`): он ничего не выдаёт. Действуют `owner`/`employee`, `admin`, `mrx` и разрешения каталога; `owner` ≠ CA/GD, `admin` ≠ SA. Будущие назначения — только по `system_roles.code`, у ERP-ролей с `organization_id`.
- Администратор (`internal/admin`) видит пользователей и состав компаний, но не данные компаний; паролей, хешей, токенов и кодов в ответах нет.
- Деньги и количества предложений — `internal/quantity`, `sale_rule_ok` и `frontend/src/lib/quantity.ts` меняются вместе. Опт (`offer_price_tiers`) не читать в публичных запросах. `owner_state = published` не означает публичный показ (docs/catalog.md).
- Персональные ответы — `Cache-Control: no-store`. Коды, пароли и токены не попадают в логи, аудит и ответы API.

## Предметная модель

- **Product** — общая карточка (название kk/ru, категория, характеристики, единица измерения). **Offer** — условия конкретного поставщика (цена, остаток, мин. заказ, доставка). Не смешивать. Корзина/заказы ссылаются на `offers.id`.
- Категории — дерево через `parent_id`; циклы запрещены триггером.
- Компания: покупатель и/или поставщик. Пользователь связан с компанией через членство (`company_members`: owner/employee). Платформенный администратор — отдельный уровень. Права никогда не выдаются по вводимому пользователем «коду роли».
- **Деньги — целые тиыны (`BIGINT`/`int64`/`amount_tiyn`), никогда float.** См. `docs/money.md`. Количества — `NUMERIC`, в API строкой.
- Правило «доступного предложения» и минимальной цены — `docs/catalog.md`, код — `offers.AvailableCondition`. Меняются вместе.

## Данные

- Демо-данные: только `backend/internal/seed/demo.sql`, идемпотентно (upsert по slug), `is_demo = TRUE`, компании «Демо жеткізуші №N» без БИН. Seed не запускается автоматически и отказывается работать при `APP_ENV=production` без `-allow-production`.
- Не выдумывать реальные компании, бренды, отзывы, статистику. Демо-данные в UI помечать как демо.

## Frontend

- Тексты интерфейса — только в словарях `src/lib/i18n/dictionaries/{kk,ru}.ts`. `kk.ts` — эталон, тип `Dictionary` заставляет `ru.ts` иметь те же ключи.
- Язык хранится в cookie `qurylys_locale`, меняется Server Action `setLocale`; серверные компоненты читают `getI18n()`, клиентские — `useI18n()`. Поля из API: `pick(locale, x_kk, x_ru)`.
- Каждый экран с данными имеет состояния: загрузка, пусто, ошибка с повтором.
- Нереализованные функции (корзина и т. п.) либо скрыты, либо явно отключены с меткой «Жақында/Скоро». Никаких заглушек, имитирующих работу.
- Публичная карточка — `products.PublicCardSQL`, публичный поставщик — `suppliers.PublicSupplierSQL`; публичные поля поставщика — только название, город, отметка проверки (решение A8). Новые публичные поля — только по решению.
- Браузер ходит в `NEXT_PUBLIC_API_BASE_URL` (`/api/v1` за Nginx), SSR — в `API_INTERNAL_URL`.
- Структура: `app/` (маршруты), `components/ui` и `components/layout` (общие), `features/<область>` (логика экранов), `lib`, `hooks`, `types`.

## Проверки перед сдачей

```bash
make check-backend   # gofmt, vet, build
make test            # unit + интеграционные тесты на qurylys_test
make check-frontend  # eslint, tsc, next build
cd frontend && npm test
```

Не заявлять об успешной проверке, если она не запускалась.
