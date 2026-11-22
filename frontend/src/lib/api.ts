import type {
  CategoryNode,
  Offer,
  ProductDetail,
  ProductPage,
  ProductSort,
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

async function request<T>(path: string, init?: { signal?: AbortSignal }): Promise<T> {
  let res: Response;
  try {
    res = await fetch(baseUrl() + path, {
      headers: { Accept: "application/json" },
      cache: "no-store",
      signal: init?.signal,
    });
  } catch (err) {
    if (err instanceof DOMException && err.name === "AbortError") throw err;
    throw new ApiError(0, "network_error", err instanceof Error ? err.message : "Network error");
  }

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
