import type { ReactNode } from "react";
import { SectionGate } from "@/features/admin/SectionGate";

export default function Layout({ children }: { children: ReactNode }) {
  return <SectionGate section="staff">{children}</SectionGate>;
}
