/** Allows only same-site paths, so ?next= cannot send the visitor to another site. */
export function safeNext(value: string | string[] | undefined): string {
  if (typeof value !== "string" || !/^\/(?![/\\])/.test(value)) return "/account";
  // Control characters are dropped by URL parsers and could reveal "//host".
  return /[\u0000-\u001f\\]/.test(value) ? "/account" : value;
}

/**
 * Full page load after the user or the active company changes: client-side
 * navigation would keep data cached for the previous context.
 */
export function hardNavigate(path: string): void {
  window.location.assign(path);
}
