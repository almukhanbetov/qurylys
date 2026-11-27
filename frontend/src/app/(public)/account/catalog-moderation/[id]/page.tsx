import { notFound } from "next/navigation";
import { ModerationReviewView } from "@/features/catalog-requests/ModerationReviewView";

export default async function Page({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^[1-9]\d{0,15}$/.test(id)) notFound();
  return <ModerationReviewView id={Number(id)} />;
}
