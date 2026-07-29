"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { LoadingSpinner } from "@/components/ui/LoadingSpinner";

export default function AdminIndexPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/admin/overview");
  }, [router]);
  return (
    <div className="flex min-h-[50vh] items-center justify-center">
      <LoadingSpinner />
    </div>
  );
}
