-- QURYLYS demo catalogue. Everything here is fictional demonstration data:
-- companies are named "Демо жеткізуші", have no BIN, and every row has is_demo = TRUE.
-- Prices are in tiyn (1 KZT = 100 tiyn) and are illustrative only.
-- Idempotent: upserts keyed by slug and (product_id, supplier_company_id).

-- Companies ------------------------------------------------------------------
INSERT INTO companies (slug, name, city, is_buyer, is_supplier, is_demo) VALUES
    ('demo-supplier-1', 'Демо жеткізуші №1', 'Алматы',  FALSE, TRUE, TRUE),
    ('demo-supplier-2', 'Демо жеткізуші №2', 'Астана',  FALSE, TRUE, TRUE),
    ('demo-supplier-3', 'Демо жеткізуші №3', 'Шымкент', TRUE,  TRUE, TRUE)
ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name, city = EXCLUDED.city,
    is_buyer = EXCLUDED.is_buyer, is_supplier = EXCLUDED.is_supplier,
    status = 'active', is_demo = TRUE;

-- Categories (three levels) -------------------------------------------------
CREATE TEMP TABLE seed_categories (
    lvl INT, slug TEXT, parent_slug TEXT, name_kk TEXT, name_ru TEXT, sort_order INT
) ON COMMIT DROP;

INSERT INTO seed_categories VALUES
    (0, 'cement-dry-mixes', NULL, 'Цемент және құрғақ қоспалар', 'Цемент и сухие смеси', 10),
    (0, 'wall-materials',   NULL, 'Қабырға материалдары',        'Стеновые материалы',   20),
    (0, 'metal',            NULL, 'Металл прокаты',              'Металлопрокат',        30),
    (0, 'roofing',          NULL, 'Шатыр материалдары',          'Кровельные материалы', 40),
    (0, 'insulation',       NULL, 'Жылу оқшаулау',               'Утеплители',           50),
    (0, 'drywall',          NULL, 'Гипсокартон',                 'Гипсокартон',          60),
    (1, 'cement',         'cement-dry-mixes', 'Цемент',            'Цемент',              10),
    (1, 'plaster',        'cement-dry-mixes', 'Сылақ қоспалары',   'Штукатурные смеси',   20),
    (1, 'tile-adhesive',  'cement-dry-mixes', 'Плитка желімі',     'Плиточный клей',      30),
    (1, 'bricks',         'wall-materials',   'Кірпіш',            'Кирпич',              10),
    (1, 'blocks',         'wall-materials',   'Блоктар',           'Блоки',               20),
    (1, 'rebar',          'metal',            'Арматура',          'Арматура',            10),
    (1, 'profile-pipe',   'metal',            'Профиль құбыр',     'Профильная труба',    20),
    (1, 'metal-tile',     'roofing',          'Металл черепица',   'Металлочерепица',     10),
    (1, 'profiled-sheet', 'roofing',          'Профнастил',        'Профнастил',          20),
    (2, 'aerated-blocks', 'blocks',           'Газоблоктар',       'Газоблоки',           10);

INSERT INTO categories (slug, parent_id, name_kk, name_ru, sort_order, is_active)
SELECT s.slug, NULL, s.name_kk, s.name_ru, s.sort_order, TRUE
FROM seed_categories s WHERE s.lvl = 0
ON CONFLICT (slug) DO UPDATE SET
    parent_id = EXCLUDED.parent_id, name_kk = EXCLUDED.name_kk, name_ru = EXCLUDED.name_ru,
    sort_order = EXCLUDED.sort_order, is_active = TRUE;

INSERT INTO categories (slug, parent_id, name_kk, name_ru, sort_order, is_active)
SELECT s.slug, p.id, s.name_kk, s.name_ru, s.sort_order, TRUE
FROM seed_categories s JOIN categories p ON p.slug = s.parent_slug WHERE s.lvl = 1
ON CONFLICT (slug) DO UPDATE SET
    parent_id = EXCLUDED.parent_id, name_kk = EXCLUDED.name_kk, name_ru = EXCLUDED.name_ru,
    sort_order = EXCLUDED.sort_order, is_active = TRUE;

INSERT INTO categories (slug, parent_id, name_kk, name_ru, sort_order, is_active)
SELECT s.slug, p.id, s.name_kk, s.name_ru, s.sort_order, TRUE
FROM seed_categories s JOIN categories p ON p.slug = s.parent_slug WHERE s.lvl = 2
ON CONFLICT (slug) DO UPDATE SET
    parent_id = EXCLUDED.parent_id, name_kk = EXCLUDED.name_kk, name_ru = EXCLUDED.name_ru,
    sort_order = EXCLUDED.sort_order, is_active = TRUE;

-- Products ---------------------------------------------------------------------
CREATE TEMP TABLE seed_products (
    slug TEXT, category_slug TEXT, name_kk TEXT, name_ru TEXT, unit TEXT, attributes JSONB
) ON COMMIT DROP;

INSERT INTO seed_products VALUES
    ('portland-cement-m400-50kg', 'cement', 'Портландцемент М400, 50 кг', 'Портландцемент М400, 50 кг', 'bag',
     '[{"key":"grade","name_kk":"Маркасы","name_ru":"Марка","value":"М400"},{"key":"weight","name_kk":"Салмағы","name_ru":"Вес","value":"50 кг"}]'),
    ('portland-cement-m500-50kg', 'cement', 'Портландцемент М500, 50 кг', 'Портландцемент М500, 50 кг', 'bag',
     '[{"key":"grade","name_kk":"Маркасы","name_ru":"Марка","value":"М500"},{"key":"weight","name_kk":"Салмағы","name_ru":"Вес","value":"50 кг"}]'),
    ('cement-plaster-25kg', 'plaster', 'Цементті сылақ қоспасы, 25 кг', 'Цементная штукатурка, 25 кг', 'bag',
     '[{"key":"weight","name_kk":"Салмағы","name_ru":"Вес","value":"25 кг"},{"key":"base","name_kk":"Негізі","name_ru":"Основа","value":"цемент"}]'),
    ('gypsum-plaster-30kg', 'plaster', 'Гипсті сылақ қоспасы, 30 кг', 'Гипсовая штукатурка, 30 кг', 'bag',
     '[{"key":"weight","name_kk":"Салмағы","name_ru":"Вес","value":"30 кг"},{"key":"base","name_kk":"Негізі","name_ru":"Основа","value":"гипс"}]'),
    ('tile-adhesive-25kg', 'tile-adhesive', 'Плитка желімі, 25 кг', 'Плиточный клей, 25 кг', 'bag',
     '[{"key":"weight","name_kk":"Салмағы","name_ru":"Вес","value":"25 кг"}]'),
    ('ceramic-brick-red', 'bricks', 'Қызыл керамикалық кірпіш', 'Кирпич керамический красный', 'piece',
     '[{"key":"size","name_kk":"Өлшемі","name_ru":"Размер","value":"250×120×65 мм"},{"key":"grade","name_kk":"Беріктік маркасы","name_ru":"Марка прочности","value":"М150"}]'),
    ('silicate-brick', 'bricks', 'Силикат кірпіш', 'Кирпич силикатный', 'piece',
     '[{"key":"size","name_kk":"Өлшемі","name_ru":"Размер","value":"250×120×88 мм"}]'),
    ('aerated-block-d500', 'aerated-blocks', 'Газоблок D500, 600×300×200', 'Газоблок D500, 600×300×200', 'm3',
     '[{"key":"density","name_kk":"Тығыздығы","name_ru":"Плотность","value":"D500"},{"key":"size","name_kk":"Өлшемі","name_ru":"Размер","value":"600×300×200 мм"}]'),
    ('aerated-block-d600', 'aerated-blocks', 'Газоблок D600, 600×300×250', 'Газоблок D600, 600×300×250', 'm3',
     '[{"key":"density","name_kk":"Тығыздығы","name_ru":"Плотность","value":"D600"},{"key":"size","name_kk":"Өлшемі","name_ru":"Размер","value":"600×300×250 мм"}]'),
    ('rebar-a500c-12mm', 'rebar', 'Арматура А500С, Ø12 мм', 'Арматура А500С, Ø12 мм', 'tonne',
     '[{"key":"diameter","name_kk":"Диаметрі","name_ru":"Диаметр","value":"12 мм"},{"key":"length","name_kk":"Ұзындығы","name_ru":"Длина","value":"11,7 м"}]'),
    ('rebar-a500c-10mm', 'rebar', 'Арматура А500С, Ø10 мм', 'Арматура А500С, Ø10 мм', 'tonne',
     '[{"key":"diameter","name_kk":"Диаметрі","name_ru":"Диаметр","value":"10 мм"}]'),
    ('profile-pipe-40x20', 'profile-pipe', 'Профиль құбыр 40×20×2 мм', 'Труба профильная 40×20×2 мм', 'm',
     '[{"key":"size","name_kk":"Қимасы","name_ru":"Сечение","value":"40×20 мм"},{"key":"wall","name_kk":"Қабырға қалыңдығы","name_ru":"Толщина стенки","value":"2 мм"}]'),
    ('metal-tile-045', 'metal-tile', 'Металл черепица, 0,45 мм', 'Металлочерепица, 0,45 мм', 'm2',
     '[{"key":"thickness","name_kk":"Қалыңдығы","name_ru":"Толщина","value":"0,45 мм"}]'),
    ('profiled-sheet-c8', 'profiled-sheet', 'Профнастил С8, 0,4 мм', 'Профнастил С8, 0,4 мм', 'm2',
     '[{"key":"thickness","name_kk":"Қалыңдығы","name_ru":"Толщина","value":"0,4 мм"}]'),
    ('mineral-wool-50mm', 'insulation', 'Минералды мақта, 50 мм', 'Минеральная вата, 50 мм', 'pack',
     '[{"key":"thickness","name_kk":"Қалыңдығы","name_ru":"Толщина","value":"50 мм"}]'),
    ('eps-board-50mm', 'insulation', 'Пенопласт ППС, 50 мм', 'Пенопласт ППС, 50 мм', 'm3',
     '[{"key":"thickness","name_kk":"Қалыңдығы","name_ru":"Толщина","value":"50 мм"}]'),
    ('drywall-12-5mm', 'drywall', 'Гипсокартон парағы, 12,5 мм', 'Гипсокартонный лист, 12,5 мм', 'sheet',
     '[{"key":"thickness","name_kk":"Қалыңдығы","name_ru":"Толщина","value":"12,5 мм"},{"key":"size","name_kk":"Өлшемі","name_ru":"Размер","value":"2500×1200 мм"}]');

INSERT INTO products (category_id, slug, name_kk, name_ru, description_kk, description_ru, brand, unit, attributes, is_active, is_demo)
SELECT c.id, s.slug, s.name_kk, s.name_ru,
       'Демонстрациялық тауар карточкасы. Сипаттамалар мысал ретінде берілген.',
       'Демонстрационная карточка товара. Характеристики приведены для примера.',
       NULL, s.unit, s.attributes, TRUE, TRUE
FROM seed_products s JOIN categories c ON c.slug = s.category_slug
ON CONFLICT (slug) DO UPDATE SET
    category_id = EXCLUDED.category_id, name_kk = EXCLUDED.name_kk, name_ru = EXCLUDED.name_ru,
    description_kk = EXCLUDED.description_kk, description_ru = EXCLUDED.description_ru,
    brand = EXCLUDED.brand, unit = EXCLUDED.unit, attributes = EXCLUDED.attributes,
    is_active = TRUE, is_demo = TRUE;

-- Offers -----------------------------------------------------------------------
-- price in tiyn; stock and MOQ in the product's unit.
CREATE TEMP TABLE seed_offers (
    product_slug TEXT, supplier_slug TEXT, price_tiyn BIGINT, stock NUMERIC, moq NUMERIC,
    delivery BOOLEAN, days_min INT, days_max INT
) ON COMMIT DROP;

INSERT INTO seed_offers VALUES
    ('portland-cement-m400-50kg', 'demo-supplier-1', 275000, 1200, 20,  TRUE, 1, 2),
    ('portland-cement-m400-50kg', 'demo-supplier-2', 289000, 800,  10,  TRUE, 2, 4),
    -- Out of stock: shown on the product page but excluded from min price.
    ('portland-cement-m400-50kg', 'demo-supplier-3', 269000, 0,    10,  FALSE, NULL, NULL),
    ('portland-cement-m500-50kg', 'demo-supplier-1', 320000, 600,  20,  TRUE, 1, 2),
    ('portland-cement-m500-50kg', 'demo-supplier-2', 315000, 450,  10,  TRUE, 2, 4),
    ('cement-plaster-25kg',       'demo-supplier-2', 185000, 300,  5,   TRUE, 2, 3),
    ('gypsum-plaster-30kg',       'demo-supplier-1', 240000, 250,  5,   TRUE, 1, 2),
    ('gypsum-plaster-30kg',       'demo-supplier-3', 235000, 180,  10,  FALSE, NULL, NULL),
    ('tile-adhesive-25kg',        'demo-supplier-3', 210000, 400,  5,   TRUE, 1, 3),
    ('ceramic-brick-red',         'demo-supplier-1', 9500,   20000, 500, TRUE, 2, 5),
    ('ceramic-brick-red',         'demo-supplier-2', 8900,   15000, 1000, TRUE, 3, 6),
    ('silicate-brick',            'demo-supplier-3', 7500,   12000, 500, TRUE, 2, 4),
    ('aerated-block-d500',        'demo-supplier-1', 2850000, 80,  2,   TRUE, 2, 4),
    ('aerated-block-d500',        'demo-supplier-2', 2790000, 120, 5,   TRUE, 3, 5),
    ('aerated-block-d500',        'demo-supplier-3', 2900000, 40.5, 1.5, FALSE, NULL, NULL),
    ('aerated-block-d600',        'demo-supplier-2', 3050000, 60,  5,   TRUE, 3, 5),
    ('rebar-a500c-12mm',          'demo-supplier-1', 34500000, 25, 1,   TRUE, 1, 3),
    ('rebar-a500c-12mm',          'demo-supplier-3', 33900000, 12.75, 0.5, TRUE, 2, 4),
    -- rebar-a500c-10mm intentionally has no offers.
    ('profile-pipe-40x20',        'demo-supplier-2', 78000,  3000, 6,   TRUE, 1, 2),
    ('metal-tile-045',            'demo-supplier-1', 320000, 900,  20,  TRUE, 3, 7),
    ('metal-tile-045',            'demo-supplier-3', 335000, 500,  10,  TRUE, 2, 5),
    ('profiled-sheet-c8',         'demo-supplier-2', 245000, 1500, 10,  TRUE, 1, 3),
    ('mineral-wool-50mm',         'demo-supplier-1', 890000, 200,  2,   TRUE, 1, 2),
    ('mineral-wool-50mm',         'demo-supplier-2', 920000, 150,  1,   TRUE, 2, 3),
    ('eps-board-50mm',            'demo-supplier-3', 2100000, 30,  1,   TRUE, 2, 4),
    ('drywall-12-5mm',            'demo-supplier-1', 265000, 700,  10,  TRUE, 1, 2),
    ('drywall-12-5mm',            'demo-supplier-2', 259000, 500,  20,  TRUE, 2, 3),
    ('drywall-12-5mm',            'demo-supplier-3', 270000, 350,  10,  TRUE, 1, 3);

INSERT INTO offers (product_id, supplier_company_id, price_tiyn, currency, stock_quantity, min_order_quantity,
                    delivery_available, delivery_days_min, delivery_days_max,
                    delivery_terms_kk, delivery_terms_ru, city, is_active, is_demo)
SELECT p.id, c.id, s.price_tiyn, 'KZT', s.stock, s.moq,
       s.delivery, s.days_min, s.days_max,
       CASE WHEN s.delivery THEN 'Қала ішінде жеткізу (демо шарттар)' ELSE 'Тек өзі алып кету (демо шарттар)' END,
       CASE WHEN s.delivery THEN 'Доставка по городу (демо-условия)' ELSE 'Только самовывоз (демо-условия)' END,
       c.city, TRUE, TRUE
FROM seed_offers s
JOIN products p  ON p.slug = s.product_slug
JOIN companies c ON c.slug = s.supplier_slug
ON CONFLICT (product_id, supplier_company_id) DO UPDATE SET
    price_tiyn = EXCLUDED.price_tiyn, stock_quantity = EXCLUDED.stock_quantity,
    min_order_quantity = EXCLUDED.min_order_quantity, delivery_available = EXCLUDED.delivery_available,
    delivery_days_min = EXCLUDED.delivery_days_min, delivery_days_max = EXCLUDED.delivery_days_max,
    delivery_terms_kk = EXCLUDED.delivery_terms_kk, delivery_terms_ru = EXCLUDED.delivery_terms_ru,
    city = EXCLUDED.city, is_active = TRUE, is_demo = TRUE;
