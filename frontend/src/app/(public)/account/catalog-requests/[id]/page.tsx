import { notFound } from "next/navigation";
import { RequestView } from "@/features/catalog-requests/RequestView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <RequestView id={Number(id)} />;
}
