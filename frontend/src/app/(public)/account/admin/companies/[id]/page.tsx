import { notFound } from "next/navigation";
import { CompanyMembersView } from "@/features/admin/CompaniesView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <CompanyMembersView id={Number(id)} />;
}
