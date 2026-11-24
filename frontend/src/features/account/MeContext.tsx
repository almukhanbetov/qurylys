"use client";

import { createContext, useContext, type ReactNode } from "react";
import type { Me, Membership } from "@/types/api";

const MeContext = createContext<Me | null>(null);

export function MeProvider({ me, children }: { me: Me; children: ReactNode }) {
  return <MeContext.Provider value={me}>{children}</MeContext.Provider>;
}

export function useMe(): Me {
  const me = useContext(MeContext);
  if (!me) throw new Error("useMe must be used inside MeProvider");
  return me;
}

/** The company selected for this session; every company screen works in its context. */
export function useActiveCompany(): Membership | undefined {
  const me = useMe();
  return me.companies.find((c) => c.id === me.active_company_id);
}
