import { notFound } from "next/navigation";
import { UserView } from "@/features/admin/UserView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <UserView id={Number(id)} />;
}
