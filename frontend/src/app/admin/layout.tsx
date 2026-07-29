import { ReactNode } from "react";
import { DashboardShell } from "@/components/admin/DashboardShell";
import { AuthProvider } from "@/context/AuthContext";

export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <AuthProvider>
      <DashboardShell>{children}</DashboardShell>
    </AuthProvider>
  );
}
