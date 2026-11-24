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

export type AdminUser = Pick<Me, "id" | "full_name" | "email" | "phone" | "platform_roles">;

export type ApiErrorBody = { error: { code: string; message: string } };
