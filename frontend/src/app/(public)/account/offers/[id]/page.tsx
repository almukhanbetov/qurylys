import { notFound } from "next/navigation";
import { OfferView } from "@/features/seller-offers/OfferView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <OfferView id={Number(id)} />;
}
