import type {
  AdminUser,
  CategoryNode,
  Company,
  CompanyDetails,
  CompanyInvitation,
  CompanyMember,
  CompanyRole,
  Me,
  DeliveryOptions,
  MyInvitation,
  Offer,
  ProductDetail,
  ProductPage,
  ProductSort,
  SessionInfo,
} from "@/types/api";

/** Thrown for any failed API call. status 0 means a network error. */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

/**
 * Base URL differs between server and browser:
 * - server (SSR) uses API_INTERNAL_URL, read at runtime (http://api:8080/api/v1 in Docker);
 * - browser uses NEXT_PUBLIC_API_BASE_URL, baked in at build time ("/api/v1" behind Nginx).
 */
function baseUrl(): string {
  if (typeof window === "undefined") {
    return process.env.API_INTERNAL_URL || process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8081/api/v1";
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL || "/api/v1";
}

type RequestOptions = {
  signal?: AbortSignal;
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  /** Server-side rendering only: the visitor's session cookie. */
  cookie?: string;
};

async function request<T>(path: string, init: RequestOptions = {}): Promise<T> {
  const { signal, method = "GET", body: payload, cookie } = init;
  const headers: Record<string, string> = { Accept: "application/json" };
  // Required by the API for state-changing requests (CSRF protection).
  if (method !== "GET") headers["X-Qurylys-Csrf"] = "1";
  if (payload !== undefined) headers["Content-Type"] = "application/json";
  if (cookie) headers.Cookie = cookie;
  let res: Response;
  try {
    res = await fetch(baseUrl() + path, {
      method,
      headers,
      body: payload === undefined ? undefined : JSON.stringify(payload),
      // Needed in development, where the API runs on another port.
      credentials: "include",
      cache: "no-store",
      signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err;
    throw new ApiError(0, "network_error", err instanceof Error ? err.message : "Network error");
  }

  if (res.status === 204) return undefined as T;
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const e = (body as { error?: { code?: string; message?: string } } | null)?.error;
    throw new ApiError(res.status, e?.code ?? "http_error", e?.message ?? res.statusText);
  }
  return (body as { data: T }).data;
}

export type ProductQuery = {
  q?: string;
  category?: string;
  sort?: ProductSort;
  page?: number;
  pageSize?: number;
};

export function productQueryString(q: ProductQuery): string {
  const p = new URLSearchParams();
  if (q.q) p.set("q", q.q);
  if (q.category) p.set("category", q.category);
  if (q.sort) p.set("sort", q.sort);
  if (q.page && q.page > 1) p.set("page", String(q.page));
  if (q.pageSize) p.set("page_size", String(q.pageSize));
  const s = p.toString();
  return s ? `?${s}` : "";
}

export const api = {
  categoryTree: (signal?: AbortSignal) => request<CategoryNode[]>("/categories/tree", { signal }),
  products: (q: ProductQuery, signal?: AbortSignal) =>
    request<ProductPage>("/products" + productQueryString(q), { signal }),
  product: (id: number) => request<ProductDetail>(`/products/${id}`),
  offers: (productId: number, signal?: AbortSignal) =>
    request<Offer[]>(`/products/${productId}/offers`, { signal }),
};

const post = <T = void>(path: string, body?: unknown) => request<T>(path, { method: "POST", body });
type Challenge = { challenge: string };

export const authApi = {
  options: (signal?: AbortSignal) => request<DeliveryOptions>("/auth/options", { signal }),
  register: (b: { full_name: string; contact: string; password: string }) => post<Challenge>("/auth/register", b),
  confirmRegistration: (b: Challenge & { code: string }) => post("/auth/register/confirm", b),
  login: (b: { contact: string; password: string }) => post("/auth/login", b),
  logout: () => post("/auth/logout"),
  forgot: (contact: string) => post<Challenge>("/auth/password/forgot", { contact }),
  reset: (b: Challenge & { code: string; password: string }) => post("/auth/password/reset", b),
};

export const accountApi = {
  me: (cookie: string, signal?: AbortSignal) => request<Me>("/me", { cookie, signal }),
  updateProfile: (full_name: string) => request<void>("/me", { method: "PATCH", body: { full_name } }),
  changePassword: (current_password: string, password: string) => post("/me/password", { current_password, password }),
  startContact: (contact: string, current_password: string) =>
    post<Challenge>("/me/contacts", { contact, current_password }),
  confirmContact: (b: Challenge & { code: string }) => post("/me/contacts/confirm", b),
  sessions: (signal?: AbortSignal) => request<SessionInfo[]>("/me/sessions", { signal }),
  endSession: (id: number) => request<void>(`/me/sessions/${id}`, { method: "DELETE" }),
  endOtherSessions: () => post("/me/sessions/revoke-others"),
  setActiveCompany: (company_id: number) =>
    request<void>("/me/active-company", { method: "PUT", body: { company_id } }),
  invitations: (signal?: AbortSignal) => request<MyInvitation[]>("/me/invitations", { signal }),
  answerInvitation: (id: number, accept: boolean) => post(`/me/invitations/${id}/${accept ? "accept" : "decline"}`),
};

export const companyApi = {
  create: (d: CompanyDetails) => post<{ id: number }>("/companies", d),
  get: (id: number, signal?: AbortSignal) => request<Company>(`/companies/${id}`, { signal }),
  update: (id: number, body: CompanyDetails) => request<void>(`/companies/${id}`, { method: "PUT", body }),
  members: (id: number, signal?: AbortSignal) => request<CompanyMember[]>(`/companies/${id}/members`, { signal }),
  setRole: (id: number, userId: number, role: CompanyRole) =>
    request<void>(`/companies/${id}/members/${userId}`, { method: "PATCH", body: { role } }),
  removeMember: (id: number, userId: number) =>
    request<void>(`/companies/${id}/members/${userId}`, { method: "DELETE" }),
  invitations: (id: number, signal?: AbortSignal) =>
    request<CompanyInvitation[]>(`/companies/${id}/invitations`, { signal }),
  invite: (id: number, contact: string) => post(`/companies/${id}/invitations`, { contact }),
  cancelInvitation: (id: number, invitationId: number) =>
    request<void>(`/companies/${id}/invitations/${invitationId}`, { method: "DELETE" }),
};

export const adminApi = {
  users: (q: string, signal?: AbortSignal) =>
    request<AdminUser[]>(`/admin/users?q=${encodeURIComponent(q)}`, { signal }),
  setMrx: (id: number, enabled: boolean) =>
    request<void>(`/admin/users/${id}/mrx`, { method: "PUT", body: { enabled } }),
};
