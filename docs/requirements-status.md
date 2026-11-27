# Статус требований ТЗ

Источник — текст [ТЗ 1.0](specifications/README.md), отступления — [spec-deviations.md](spec-deviations.md), роли — [roles-and-permissions.md](roles-and-permissions.md).

Статусы определены по коду ревизии `179f45c` (25.11.2026) и незакоммиченным шагам 3.1–3.3. Развёртывание на VPS не оценивается. **Реализовано** — есть код и тест либо однозначное подтверждение в коде; **частично** — выполнена часть; **требует проверки** — по репозиторию установить нельзя. Отступление и принятое решение не делают требование реализованным.

| реализовано | частично | не реализовано | требует проверки | всего |
|---|---|---|---|---|
| 14 | 29 | 41 | 4 | 88 |

## Основа и доступ

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| Пользователь в нескольких организациях, контексты раздельны | §01, с. 3 | частично | `TestMembershipAndActiveCompany`, `TestCompanyIsolation` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); ролей ТЗ внутри компании нет |
| Регистрация доступна каждому | §01, с. 3 | частично | `TestRegistrationLoginLogout` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); на практике только email, [smtp.go](../backend/internal/delivery/smtp.go) |
| Платформа не принимает оплату и не организует доставку | §01, с. 3 | реализовано | платёжных и логистических модулей нет: [router.go](../backend/internal/router/router.go) |
| Backend проверяет членство и роль на каждый запрос | §03, с. 5 | реализовано | [companies.Authorize](../backend/internal/companies/repository.go); `TestCompanyIsolation` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |
| Доступ к филиалу, `branch_id` | §03, с. 5; BR-14, с. 15 | не реализовано | таблицы филиалов в [00007_company_accounts.sql](../backend/migrations/00007_company_accounts.sql) и других миграциях нет |
| 19 ролей и разрешения (RBAC) | §02, с. 3–4; §03, с. 5 | не реализовано | только `owner`/`employee` ([00007_company_accounts.sql](../backend/migrations/00007_company_accounts.sql)) и `mrx`/`admin` ([00006_users_auth.sql](../backend/migrations/00006_users_auth.sql)) |
| Смена организации: контекст меняется полностью, кэш очищается | §17, с. 16 | частично | `TestMembershipAndActiveCompany` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); кэш — [hardNavigate](../frontend/src/lib/redirect.ts), автотеста на кэш нет |
| JWT/refresh | §03, с. 5 | не реализовано | сессии в cookie — [B1](../docs/spec-deviations.md); безопасность входа: `TestLoginBruteForce`, `TestCSRF` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |
| Аудит всех важных действий | §17, с. 16 | частично | [audit.go](../backend/internal/audit/audit.go) — аккаунты, компании, роли; `TestPlatformRoles` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); просмотра журнала нет |
| `GET /audit/events` по разрешению | §15, с. 15 | не реализовано | маршрута нет: [handler.go](../backend/internal/auth/handler.go) |
| Ограничение доступа к персональным данным | §17, с. 16 | частично | контакты сотрудников скрыты от `employee`: `TestMembershipAndActiveCompany` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); администратор видит контакты всех |
| Текст предупреждения безопасности | §13, с. 14 | частично | показан только при регистрации: [RegisterForm.tsx](../frontend/src/features/auth/RegisterForm.tsx); профиль продавца, предложение, заказ, справка — нет |
| Факт ознакомления при заказе — в журнале | §13, с. 14 | не реализовано | заказов нет |
| Push и in-app уведомления | §03, с. 5 | не реализовано | только email: [templates.go](../backend/internal/delivery/templates.go) |

## MP — покупатель

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| MP-01: поиск по названию, SKU и GTIN | §05, с. 7 | частично | название и бренд: `TestSearch` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)); SKU и GTIN нет |
| MP-01: порядок предложений по цене | §05, с. 7 | реализовано | `TestOffersForProduct` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)) |
| MP-02: фильтры категория, бренд, город, цена, наличие | §05, с. 7 | частично | только категория: `TestFilterByCategoryIncludesDescendants` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)) |
| MP-02: общая карточка, без дублей ([S3](decisions/supplier-stage-3.md)) | §05, с. 7; §01, с. 3 | частично | карточка отделена от предложения: [00005_offers.sql](../backend/migrations/00005_offers.sql); защиты от дублей товара, кроме slug, нет |
| MP-03: фото, SKU, GTIN, хранение, партия | §05, с. 7 | не реализовано | полей нет: [00004_products.sql](../backend/migrations/00004_products.sql) (есть описание и характеристики) |
| MP-04: цена, НДС, объём, минимум, остаток, компания | §05, с. 7 | частично | нет НДС и уровней: [OffersList.tsx](../frontend/src/features/products/OffersList.tsx) |
| MP-05: корзина одного продавца | §05, с. 7 | не реализовано | в репозитории нет |
| MP-06: подтверждение заказа | §05, с. 7 | не реализовано | в репозитории нет |
| MP-07: история заказов | §05, с. 7 | не реализовано | в репозитории нет |
| MP-08: профиль, данные компании, заявка на проверку | §05, с. 7 | реализовано | [CompanyView.tsx](../frontend/src/features/account/CompanyView.tsx), [VerificationView.tsx](../frontend/src/features/account/VerificationView.tsx); `TestVerificationFullFlow` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)) |
| MP-08: РРЦ зарегистрированным, опт только одобренным | §05, с. 7; §01, с. 3 | не реализовано | уровней цен нет; РРЦ видна и гостям ([A7](../docs/spec-deviations.md)) |

## SL — продавец

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| SL-01: дашборд | §06, с. 9 | не реализовано | в репозитории нет |
| SL-02: свои предложения, публикация, цена | §06, с. 9 | частично | кабинет и выбор владельца: `TestSellerOfferReferenceExample` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); публичного показа нет |
| SL-03: форма товара, 23 поля | §06, с. 9 | не реализовано | в репозитории нет |
| SL-04: РРЦ/Опт/Крупный опт и минимумы | §06, с. 9 | реализовано | `TestSellerOfferValidation`, `TestSellerOfferReferenceExample` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)) |
| SL-05: простой остаток | §06, с. 9 | реализовано | изменение владельцем, проверка единиц: `TestSellerOfferValidation` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); синхронизация с WMS — ERP |
| SL-06: заказы и статусы | §06, с. 9 | не реализовано | в репозитории нет |
| SL-07: БИН, адреса, директор, телефон, город, часы работы | §06, с. 9 | частично | нет часов работы; представитель вместо директора: `TestCompanyDetails` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |

## MRX — верификация

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| MRX-01: список и статусы | §07, с. 11 | частично | новая и проверяется объединены: `TestVerificationFullFlow` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)) |
| MRX-02: название, БИН/ИИН, данные, документы, заявитель | §07, с. 11 | частично | ИИН отдельно не поддержан; заявитель виден в истории: [MrxReviewView.tsx](../frontend/src/features/account/MrxReviewView.tsx) |
| MRX-03: одобрить, отказать с обоснованием, вернуть | §07, с. 11 | реализовано | `TestVerificationFullFlow`, `TestVerificationRejectAndReapply`, `TestDecisionRace` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)) |
| MRX-04: реестр со статусом и датой решения | §07, с. 11 | частично | статус в списке, дата решения только в карточке: [MrxListView.tsx](../frontend/src/features/account/MrxListView.tsx) |
| MRX-05: неизменяемый журнал | §07, с. 11 | реализовано | `TestVerificationDatabaseGuards` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)); [00008_verification.sql](../backend/migrations/00008_verification.sql) |
| MRX не меняет цены, товары, заказы, подписки, финансы | §07, с. 11 | частично | данные компании — 404: `TestVerificationPermissions` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)); API изменения цен и заказов ещё нет, проверять нечего |
| Знак проверки — только статус, не гарантия | §07, с. 11 | частично | пояснение в кабинете: [ru.ts](../frontend/src/lib/i18n/dictionaries/ru.ts); публичного знака нет |
| Закрытые документы: антивирус, срок доступа | §03, с. 5; §17, с. 16 | частично | доступ по правам и проверка формата: `TestDocumentUploadRules`, `TestVerificationPermissions` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)); антивируса и срока нет |

## Бизнес-правила BR

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| BR-01: новый пользователь видит РРЦ, опт закрыт | §14, с. 15 | частично | показывается единственная цена: `TestOffersForProduct` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)); опта в БД нет |
| BR-02: одобрение открывает опт | §14, с. 15 | не реализовано | одобрение только фиксирует результат: [verification.md](../docs/verification.md) |
| BR-03: розница 50 000 ₸ не принимается | §14, с. 15 | не реализовано | в репозитории нет |
| BR-04: розница 50 001 ₸ принимается | §14, с. 15 | не реализовано | в репозитории нет |
| BR-05: два продавца — раздельные корзины | §14, с. 15 | не реализовано | в репозитории нет |
| BR-06: опт ниже минимума не принимается | §14, с. 15 | не реализовано | в репозитории нет |
| BR-07: 5 коробок по 12 = 60 штук | §14, с. 15 | частично | кратность и пример 5 × 12 = 60: `TestSaleRule` ([quantity_test.go](../backend/internal/quantity/quantity_test.go)); заказов нет |
| BR-08: одна карточка, 5 предложений | §14, с. 15 | требует проверки | тест на 3 предложения: `TestOffersForProduct` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)); ограничения числа в запросе нет, теста на 5 нет |
| BR-09: снижение цены меняет порядок | §14, с. 15 | частично | порядок по цене при каждом запросе: [repository.go](../backend/internal/offers/repository.go); изменить цену и выбрать уровень нельзя |
| BR-10: SKU и GTIN — два обязательных поля | §14, с. 15 | реализовано | GTIN обязателен карточке, артикул — публикации: `TestPublicationConditions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)) |
| BR-11: срок годности только для применимых категорий | §14, с. 15 | не реализовано | в репозитории нет |
| BR-12: признак НДС у предложения | §14, с. 15 | частично | поле и обязательность для публикации: `TestPublicationConditions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); покупателю не показывается |
| BR-13: MRX меняет цену — 403 | §14, с. 15 | реализовано | `TestSellerOfferPermissions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)) |
| BR-14: BD не видит чужой филиал | §14, с. 15 | не реализовано | в репозитории нет |
| BR-15: при заказе предупреждение и условия продавца | §14, с. 15 | не реализовано | в репозитории нет |

## ERP, CRM, WMS

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| GD: KPI, назначение директора, цели, отчёты | §08, с. 13 | не реализовано | в репозитории нет |
| BD: филиал | §08, с. 13 | не реализовано | в репозитории нет |
| ACC: счета, акты, дебиторка | §08, с. 13 | не реализовано | в репозитории нет |
| HR: кадры | §08, с. 13 | не реализовано | в репозитории нет |
| WH, AU: склад, инвентаризация | §09, с. 13 | не реализовано | в репозитории нет |
| SR, OP, SV: CRM | §10, с. 13 | не реализовано | в репозитории нет |
| LG, DR: логистика, QR | §11, с. 14 | не реализовано | в репозитории нет |
| PR: закупки | §11, с. 14 | не реализовано | в репозитории нет |
| CA: сотрудники, приглашения, роли, филиалы, безопасность | §08, с. 13 | не реализовано | роли CA нет; близкие функции `owner`: `TestMembershipAndActiveCompany` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |
| Смена роли в компании — запись в аудите | §08, с. 13 | частично | событие `company.member_role`: [repository.go](../backend/internal/companies/repository.go); автотеста на запись нет |

## Платформа

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| TS: тикеты, FAQ | §12, с. 14 | не реализовано | в репозитории нет |
| CC: лиды, onboarding | §12, с. 14 | не реализовано | в репозитории нет |
| SA: тарифы, подписки, состояние системы, модерация, аудит | §12, с. 14 | не реализовано | `admin` выдаёт только MRX: [handler.go](../backend/internal/auth/handler.go); `TestPlatformRoles` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |
| SA: усиленная MFA | §12, с. 14 | не реализовано | в репозитории нет |
| SA: все действия в журнале | §12, с. 14 | не реализовано | роли SA нет; у `admin` журналируется выдача ролей, просмотр списка — нет: [repository.go](../backend/internal/auth/repository.go) |
| Тарифы и подписки | §12, с. 14 | не реализовано | отложено: [A3](../docs/spec-deviations.md) |

## API и качество

| Требование | Источник | Статус | Подтверждение |
|---|---|---|---|
| Серверная валидация, структурированные ошибки | §15, с. 15 | реализовано | [app_error.go](../backend/internal/httpx/app_error.go); `TestInvalidParams` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)), `TestCompanyDetails` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)) |
| Пагинация | §15, с. 15 | частично | `TestPagination` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)); список пользователей администратора обрезан до 50: [repository.go](../backend/internal/auth/repository.go) |
| Ограничение частоты | §15, с. 15 | частично | вход, коды, загрузки, компании: [ratelimit.go](../backend/internal/ratelimit/ratelimit.go); `TestLoginBruteForce` ([accounts_integration_test.go](../backend/internal/router/accounts_integration_test.go)); каталог без лимита |
| Idempotency key для заказа | §15, с. 15 | не реализовано | в репозитории нет |
| Оптимистичная блокировка остатка и цены | §15, с. 15 | не реализовано | для цен и остатка нет; у решений MRX есть `submission_no` |
| Snapshot заказа | §17, с. 16 | не реализовано | в репозитории нет |
| Каталог отделён от предложений | §17, с. 16 | реализовано | [00005_offers.sql](../backend/migrations/00005_offers.sql); `TestDatabaseConstraints` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)) |
| Unit, integration, e2e; проверка прав | §16, с. 16 | частично | unit и integration есть; e2e в репозитории нет: [ci.yml](../.github/workflows/ci.yml) |
| CI/CD, миграции | §16, с. 16 | реализовано | [ci.yml](../.github/workflows/ci.yml); `TestMigrationLock` ([router_integration_test.go](../backend/internal/router/router_integration_test.go)), `TestVerificationMigrationKeepsData` ([verification_integration_test.go](../backend/internal/router/verification_integration_test.go)) |
| Backup/restore | §16, с. 16 | требует проверки | копия в [deploy.sh](../deploy/bin/deploy.sh); восстановление описано в [deployment.md](../docs/deployment.md), в production не подтверждено репозиторием |
| README, API, deploy/runbook, руководство администратора | §16, с. 16 | частично | [README.md](../README.md), [deployment.md](../docs/deployment.md); формальной спецификации API и отдельного руководства нет |
| Пустые, ошибочные, загрузочные состояния экранов | §04, с. 5; §16, с. 16 | требует проверки | [QueryState.tsx](../frontend/src/features/account/QueryState.tsx); автотестов интерфейса нет |
| Казахский и русский | §04, с. 5 | реализовано | тип `Dictionary` требует одинаковых ключей: [types.ts](../frontend/src/lib/i18n/types.ts) |
| Архитектура для английского и китайского | §04, с. 5 | частично | словари отдельны, но поля БД и `pick` — только kk/ru: [index.ts](../frontend/src/lib/i18n/index.ts) |
| Android/iOS, Flutter | §01, с. 3; §16, с. 16 | не реализовано | веб на Next.js — [A1](../docs/spec-deviations.md); приложений нет |
| Crash/error logging | §16, с. 16 | частично | журнал API и обработка паник: [logging.go](../backend/internal/middleware/logging.go); сбора ошибок интерфейса нет |
| Утверждённые макеты экранов, акт приёмки | §16, с. 16 | требует проверки | в репозитории отсутствуют; могут храниться вне его |

## Согласованные дополнения этапа 3

Источник — [decisions/supplier-stage-3.md](decisions/supplier-stage-3.md), связанное требование ТЗ указано для справки. В счётчик выше не входят.

| Решение | Связано с ТЗ | Статус | Подтверждение |
|---|---|---|---|
| S1.1 черновики непроверенной компании | SL-02 | реализовано (в `main`, на production не выкачено) | `TestPublicationConditions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)) |
| S1.2 публикация только проверенным поставщиком | SL-02 | реализовано (в `main`, на production не выкачено) | `TestPublicationConditions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); публичный показ — 3.4 |
| S1.3, C8 снятие верификации исключает из каталога, возврат после одобрения | — | не реализовано | то же: [model.go](../backend/internal/offers/model.go) |
| S1.4 история заказов сохраняется | — | не реализовано | заказов в репозитории нет |
| S2, N2 управляет только владелец, сотрудник смотрит; назначаемое право — позже | SL-02 | реализовано (в `main`, на production не выкачено) | `TestSellerOfferPermissions` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); назначаемого права нет |
| S3, C1, C2, R2–R5 заявки на карточки, GTIN в заявке, `catalog.moderate` | MP-02 | реализовано (в `main`, на production не выкачено) | [catalog](../backend/internal/catalog/repository.go); `TestCatalogRequestFullFlow`, `TestCatalogPermissionGrant`, `TestModeratorConflictOfInterest` ([catalog_integration_test.go](../backend/internal/router/catalog_integration_test.go)) |
| R6, R11, N3 правка и блокировка карточек | MP-02 | не реализовано | API нет: [handler.go](../backend/internal/catalog/handler.go) |
| S3.6 централизованные наборы характеристик | MP-02 | не реализовано | характеристики — JSON карточки: [00004_products.sql](../backend/migrations/00004_products.sql) |
| S4, C3, R1 артикул каталога, `seller_sku`, подтверждённый GTIN, поиск | BR-10 | частично | `TestSellerOfferDuplicates` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)), `TestCatalogNumbers` ([catalog_integration_test.go](../backend/internal/router/catalog_integration_test.go)); публичного поиска нет |
| S5, C6, C7, R8–R10, N4, N6 конечная цена, НДС, уровни, остаток, упаковка и кратность | SL-04; BR-06, BR-07, BR-12 | реализовано (в `main`, на production не выкачено) | `TestSellerOfferValidation`, `TestMoneyRuleInSQL` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); сравнение продавцов и заказы — нет |
| C4 старые карточки помечены, отчёт, настоящие предложения только с GTIN | BR-10 | частично | `needs_review` и `admin catalog-report`: `TestCatalogReport` ([catalog_integration_test.go](../backend/internal/router/catalog_integration_test.go)); правило показа — шаг 3.4 |
| C5, R7 демо вне реальных цен и заказов | — | частично | смешать демо и настоящее нельзя: `TestSellerOfferDatabaseGuards` ([selleroffers_integration_test.go](../backend/internal/router/selleroffers_integration_test.go)); расчёт цен каталога — 3.4 |
| C9 доставка не входит в цену, это показано покупателю | §01 | частично | показано продавцу: [OfferForm.tsx](../frontend/src/features/seller-offers/OfferForm.tsx); покупателю — 3.4 |
| C10 снятие направления при наличии предложений | — | не реализовано | код делает обратное (409 `supplier_has_offers`): [repository.go](../backend/internal/companies/repository.go) |
| N1, N7 опт проверенной компании-покупателю | MP-08, BR-02 | не реализовано | маршрута и оптовых цен нет: [handler.go](../backend/internal/offers/handler.go); правило N7 соответствует коду: [00008_verification.sql](../backend/migrations/00008_verification.sql) |
| N5 фото и документы товара | §06, поля 06 и 21 | не реализовано | решение не принято: [stage-3-product-files.md](plans/stage-3-product-files.md) |

При изменении поведения обновляйте строку, ссылку на код или тест и ревизию.
