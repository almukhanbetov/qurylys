// Types mirroring the backend JSON (backend/internal/*/model.go).

/** Exact money: integer tiyn, 1 KZT = 100 tiyn. See docs/money.md. */
export type Money = { amount_tiyn: number; currency: "KZT" };

export const UNITS = [
  "piece", "kg", "tonne", "bag", "m", "m2", "m3", "litre", "pack", "roll", "sheet", "pallet",
] as const;

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
  /** Platform article of the shared card; not a seller's SKU. */
  catalog_no: string;
  /** Confirmed GTIN, 14 digits. */
  gtin: string | null;
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

export type VerificationStatus =
  | "none" | "draft" | "in_review" | "needs_changes" | "approved" | "rejected" | "reverification";
export type ReviewStatus = "in_review" | "needs_changes" | "approved" | "rejected";
export const REVIEW_STATUSES: ReviewStatus[] = ["in_review", "needs_changes", "approved", "rejected"];

export type DocumentKind = "registration" | "authority" | "other";
export const DOCUMENT_KINDS: DocumentKind[] = ["registration", "authority", "other"];

export type VerificationDocument = {
  id: number;
  kind: DocumentKind;
  filename: string;
  content_type: string;
  size: number;
  created_at: string;
};

export type VerificationEvent = {
  id: number;
  submission_no: number;
  action: "created" | "submitted" | "approved" | "changes_requested" | "rejected" | "verification_lost";
  /** Empty for reviewers in the company's view. */
  actor: string;
  comment: string;
  changed: { field: string; old: string; new: string }[];
  created_at: string;
};

export type CompanyVerification = {
  status: VerificationStatus;
  comment: string;
  submitted_at: string | null;
  decided_at: string | null;
  can_edit: boolean;
  requisites: CompanyDetails;
  documents: VerificationDocument[];
  events: VerificationEvent[];
  max_file_size: number;
  max_documents: number;
};

export type ReviewListItem = {
  id: number;
  company_name: string;
  bin: string;
  city: string;
  status: ReviewStatus;
  submission_no: number;
  submitted_at: string | null;
  decided_at: string | null;
  conflict: boolean;
};

export type ReviewPage = { items: ReviewListItem[]; page: number; total: number; total_pages: number };

export type Review = {
  id: number;
  status: ReviewStatus;
  submission_no: number;
  comment: string;
  submitted_at: string | null;
  decided_at: string | null;
  requisites: CompanyDetails;
  documents: VerificationDocument[];
  events: VerificationEvent[];
};

export type ReviewDecision = "approve" | "request_changes" | "reject";

export type CatalogRequestStatus = "draft" | ReviewStatus;
export type GtinSourceKind = "manufacturer" | "packaging" | "official_catalog";
export const GTIN_SOURCE_KINDS: GtinSourceKind[] = ["manufacturer", "packaging", "official_catalog"];

export type CatalogRequestFields = {
  category_id: number | null;
  name_kk: string;
  name_ru: string;
  brand: string;
  unit: Unit | "";
  description_kk: string;
  description_ru: string;
  gtin: string | null;
  gtin_source_kind: GtinSourceKind | null;
  gtin_source: string;
};

export type CatalogRequest = CatalogRequestFields & {
  id: number;
  status: CatalogRequestStatus;
  submission_no: number;
  category_kk: string;
  category_ru: string;
  gtin_confirmed: boolean;
  comment: string;
  product_id: number | null;
  catalog_no: string | null;
  submitted_at: string | null;
  decided_at: string | null;
  updated_at: string;
};

export type CatalogEvent = {
  id: number;
  submission_no: number;
  action: "created" | "submitted" | "gtin_confirmed" | "changes_requested" | "approved" | "rejected";
  actor: string;
  comment: string;
  created_at: string;
};

export type CompanyCatalogRequest = CatalogRequest & { events: CatalogEvent[] };

export type ModerationItem = CatalogRequest & { company_name: string; conflict: boolean };
export type ModerationPage = { items: ModerationItem[]; page: number; total: number; total_pages: number };

export type ModerationReview = CatalogRequest & {
  company_name: string;
  confirmed_source_kind: GtinSourceKind | null;
  confirmed_source: string | null;
  similar: {
    id: number;
    catalog_no: string;
    name_kk: string;
    name_ru: string;
    brand: string | null;
    gtin: string | null;
    same_gtin: boolean;
    is_demo: boolean;
  }[];
  same_gtin_requests: number;
  events: CatalogEvent[];
};

export type OfferState = "draft" | "published" | "hidden";
export type SaleMode = "piece" | "pack";
export type PriceLevel = "retail" | "wholesale" | "large_wholesale";
export const PRICE_LEVELS: PriceLevel[] = ["retail", "wholesale", "large_wholesale"];

/** Quantities are exact decimal strings in base units; prices are integer tiyn. */
export type OfferLevel = { price_tiyn: number | null; sale_mode: SaleMode; min_quantity: string; step_quantity: string };

export type OfferLevelView = OfferLevel & {
  pack_price_tiyn: number | null;
  min_total_tiyn: number | null;
  in_stock: boolean;
};

export type OfferReason =
  | "company_suspended" | "company_not_supplier" | "company_not_verified" | "card_inactive" | "card_demo"
  | "card_needs_review" | "card_gtin_unconfirmed" | "price_missing" | "seller_sku_missing" | "vat_missing";

export type OfferCard = {
  id: number;
  catalog_no: string;
  name_kk: string;
  name_ru: string;
  unit: Unit;
  gtin: string;
  has_offer: boolean;
};

export type VatStatus = "" | "with_vat" | "without_vat";

export type SellerOfferInput = {
  seller_sku: string;
  vat_status: VatStatus;
  pack_size: string;
  stock_quantity: string;
  city: string;
  retail: OfferLevel;
  wholesale: OfferLevel | null;
  large_wholesale: OfferLevel | null;
};

export type SellerOffer = Omit<SellerOfferInput, PriceLevel> & {
  id: number;
  card: OfferCard;
  owner_state: OfferState;
  retail: OfferLevelView;
  wholesale: OfferLevelView | null;
  large_wholesale: OfferLevelView | null;
  version: number;
  updated_at: string;
  reasons: OfferReason[];
};

export type SellerOfferView = SellerOffer & { can_manage: boolean };
export type SellerOfferList = { items: SellerOffer[]; can_manage: boolean; can_create: boolean };

export type CompanyRole = "owner" | "employee";

export type Membership = {
  id: number;
  name: string;
  role: CompanyRole;
  is_buyer: boolean;
  is_supplier: boolean;
  status: "active" | "suspended";
  verification: VerificationStatus;
};

export type Me = {
  id: number;
  full_name: string;
  email: string | null;
  phone: string | null;
  platform_roles: ("mrx" | "admin")[];
  platform_permissions: "catalog.moderate"[];
  companies: Membership[];
  active_company_id: number | null;
  pending_invitations: number;
};

/** Channels that can currently receive confirmation codes. */
export type DeliveryOptions = { email: boolean; phone: boolean };

export type SessionInfo = {
  id: number;
  user_agent: string;
  created_at: string;
  last_seen_at: string;
  current: boolean;
};

export type CompanyDetails = {
  name: string;
  bin: string;
  city: string;
  legal_address: string;
  actual_address: string;
  work_phone: string;
  representative_name: string;
  is_buyer: boolean;
  is_supplier: boolean;
};

export type Company = CompanyDetails & { id: number; status: "active" | "suspended"; verification: VerificationStatus; role: CompanyRole };

export type CompanyMember = {
  user_id: number;
  full_name: string;
  role: CompanyRole;
  /** Present for owners only. */
  email?: string | null;
  phone?: string | null;
  is_me: boolean;
};

export type CompanyInvitation = { id: number; contact: string; expires_at: string };

export type MyInvitation = {
  id: number;
  company_name: string;
  bin: string | null;
  invited_by: string | null;
  expires_at: string;
};

export type AdminUser = Pick<Me, "id" | "full_name" | "email" | "phone" | "platform_roles" | "platform_permissions">;

export type ApiErrorBody = { error: { code: string; message: string } };
