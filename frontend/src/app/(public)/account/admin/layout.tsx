import type { ReactNode } from "react";
import { AdminNav } from "@/features/admin/AdminNav";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <div className="space-y-4">
      <AdminNav />
      {children}
    </div>
  );
}
