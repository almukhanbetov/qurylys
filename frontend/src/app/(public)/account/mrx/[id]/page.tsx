import { notFound } from "next/navigation";
import { MrxReviewView } from "@/features/account/MrxReviewView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <MrxReviewView id={Number(id)} />;
}
