"use client";

import { ShieldCheck } from "lucide-react";
import { useAuth } from "@/hooks/useAuth";
import { AdminsSection } from "@/components/admin/AdminsSection";
import { PageHeader } from "@/components/admin/PageHeader";

export default function AdminsPage() {
  const { role } = useAuth();
  return (
    <div>
      <PageHeader title="Администраторы" description="Учётные записи владельцев и операторов панели." icon={<ShieldCheck className="h-5 w-5" />} />
      <AdminsSection currentRole={role} />
    </div>
  );
}
