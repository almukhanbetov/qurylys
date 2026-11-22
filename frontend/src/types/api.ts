// Types mirroring the backend JSON (backend/internal/*/model.go).

/** Exact money: integer tiyn, 1 KZT = 100 tiyn. See docs/money.md. */
export type Money = { amount_tiyn: number; currency: "KZT" };

export type Unit =
  | "piece" | "kg" | "tonne" | "bag" | "m" | "m2" | "m3" | "litre" | "pack" | "roll" | "sheet" | "pallet";

export type CategoryNode = {
  id: number;
  parent_id: number | null;
  slug: string;
  name_kk: string;
  name_ru: string;
  depth: number;
  /** Includes products of all descendant categories. */
  product_count: number;
  children: CategoryNode[];
};

export type CategoryRef = { id: number; slug: string; name_kk: string; name_ru: string };

export type ProductListItem = {
  id: number;
  slug: string;
  name_kk: string;
  name_ru: string;
  brand: string | null;
  unit: Unit;
  is_demo: boolean;
  category: CategoryRef;
  /** Lowest price among available offers, null if none. */
  min_price: Money | null;
  offer_count: number;
};

export type ProductAttribute = { key: string; name_kk: string; name_ru: string; value: string };

export type ProductDetail = ProductListItem & {
  description_kk: string;
  description_ru: string;
  attributes: ProductAttribute[];
  category_path: CategoryRef[];
  updated_at: string;
};

export type ProductPage = {
  items: ProductListItem[];
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
};

export type ProductSort = "newest" | "price_asc" | "price_desc" | "offers";
export const PRODUCT_SORTS: ProductSort[] = ["newest", "price_asc", "price_desc", "offers"];

export type Offer = {
  id: number;
  supplier: { id: number; slug: string; name: string; city: string | null; is_demo: boolean };
  price: Money;
  /** Decimal strings, exact (PostgreSQL NUMERIC). */
  stock_quantity: string;
  min_order_quantity: string;
  is_available: boolean;
  delivery_available: boolean;
  delivery_days_min: number | null;
  delivery_days_max: number | null;
  delivery_terms_kk: string;
  delivery_terms_ru: string;
  city: string | null;
  is_demo: boolean;
  updated_at: string;
};

export type ApiErrorBody = { error: { code: string; message: string } };
