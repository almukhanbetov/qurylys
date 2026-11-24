import "server-only";
import { cache } from "react";
import { cookies } from "next/headers";
import { accountApi, ApiError } from "./api";
import type { Me } from "@/types/api";

const COOKIES = ["__Host-qurylys_session", "qurylys_session"];

/**
 * The signed-in user, or null without a valid session. Only the session cookie is
 * forwarded. Other failures are thrown, so the cabinet shows an error, not a login form.
 */
export const getMe = cache(async (): Promise<Me | null> => {
  const jar = await cookies();
  const cookie = COOKIES.flatMap((name) => {
    const value = jar.get(name)?.value;
    return value ? [`${name}=${value}`] : [];
  }).join("; ");
  if (!cookie) return null;
  try {
    // The header is on every page: a hanging API must not hold them back.
    return await accountApi.me(cookie, AbortSignal.timeout(3000));
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) return null;
    throw err;
  }
});

/** For the public header: an API failure must not take the page down. */
export async function getMeOrNull(): Promise<Me | null> {
  return getMe().catch(() => null);
}
