# QURYLYS — правила проекта

B2B-маркетплейс строительных материалов для Казахстана. Несколько поставщиков, основной язык интерфейса — казахский, второй — русский.

## Стек (не менять без отдельной задачи)

- Backend: Go 1.25 + Gin. БД через `database/sql` + `github.com/jackc/pgx/v5/stdlib`. **Без GORM и lib/pq.**
- Миграции: Goose, только SQL, файлы `backend/migrations/NNNNN_name.sql`, встроены в бинарник `migrate` через `embed`.
- PostgreSQL 17 (builtin-локаль `C.UTF-8`) локально; production — системный PostgreSQL 16 на VPS (libc `C.UTF-8`, БД `qurylys_b2b`, не пересоздавать). CI проверяет обе конфигурации.
- Frontend: Next.js 16 (App Router) + TypeScript (strict) + Tailwind CSS 4.
- Инфраструктура: Docker Compose + Nginx (единый домен: `/api/` → API, остальное → Next.js). Production: образы из GHCR, `deploy/compose.prod.yml`, системный Nginx, деплой — `docs/deployment.md`.
- Платные сервисы не подключать без отдельной задачи. Ничего не деплоить без отдельной задачи.

## Backend

- Модули по предметной области: `backend/internal/<module>/` с файлами `model.go`, `handler.go`, `service.go`, `repository.go` — только те, что реально нужны. Пустых файлов и абстракций «для структуры» не создавать.
- Зависимости строго: `router → handler → service → repository → PostgreSQL`. Handler не пишет SQL, repository не знает про HTTP.
- `cmd/api/main.go` только собирает конфигурацию, пул и сервер.
- SQL — только параметры `$n`. Пользовательский ввод никогда не попадает в текст запроса. Имена полей сортировки — только из фиксированной карты (`products.sortClauses`).
- Ошибки клиенту: `{"error": {"code", "message"}}` через `internal/httpx`. Внутренние ошибки логируются, клиенту — общее сообщение.
- Конфигурация — только переменные окружения (`internal/config`). Секреты не коммитить; `.env.example` без реальных значений.
- Межмодульные зависимости — без циклов (сейчас `products` использует `offers.AvailableCondition`, `companies` использует `auth.Principal`).
- Доступ: сессии — `internal/auth` (HttpOnly-cookie), CSRF — `middleware.CSRF` на всём `/api/v1`. Любой запрос к данным компании проходит через `companies.authorize` (членство → активная компания → роль). См. `docs/accounts.md`.
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
- Нереализованные функции (корзина, бренды, поставщики) либо скрыты, либо явно отключены с меткой «Жақында/Скоро». Никаких заглушек, имитирующих работу.
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
