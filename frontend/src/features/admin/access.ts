import type { Me, PlatformPermission } from "@/types/api";

export const ADMIN_SECTIONS = ["users", "staff", "companies", "roles", "categories", "brands", "products"] as const;
export type AdminSection = (typeof ADMIN_SECTIONS)[number];

/** The permission each catalogue section needs; the other sections need the admin role. */
export const SECTION_PERMISSION: Partial<Record<AdminSection, PlatformPermission>> = {
  categories: "catalog.categories",
  brands: "catalog.brands",
  products: "catalog.moderate",
};

export type SectionAccess = { section: AdminSection; allowed: boolean };

/**
 * Sections shown in "Administration". An administrator sees every catalogue
 * section, marked when the permission is missing (it is granted explicitly on
 * the user page, never on opening). Others see only what they hold. This only
 * shapes the menu: the API checks every request on its own.
 */
export function adminSections(me: Me): SectionAccess[] {
  const admin = me.platform_roles.includes("admin");
  const out: SectionAccess[] = [];
  for (const section of ADMIN_SECTIONS) {
    const perm = SECTION_PERMISSION[section];
    if (!perm) {
      if (admin) out.push({ section, allowed: true });
      continue;
    }
    const allowed = me.platform_permissions.includes(perm);
    if (allowed || admin) out.push({ section, allowed });
  }
  return out;
}
