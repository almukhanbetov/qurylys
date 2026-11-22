"use server";

import { cookies } from "next/headers";
import { isLocale, LOCALE_COOKIE } from "./types";

/**
 * Saves the visitor's language. Setting a cookie in a Server Action makes Next.js
 * re-render the current route, so all server-rendered texts switch immediately.
 */
export async function setLocale(locale: string): Promise<void> {
  if (!isLocale(locale)) return;
  (await cookies()).set(LOCALE_COOKIE, locale, {
    path: "/",
    maxAge: 60 * 60 * 24 * 365,
    sameSite: "lax",
    httpOnly: false,
  });
}
