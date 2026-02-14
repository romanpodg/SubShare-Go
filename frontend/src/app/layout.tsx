import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { ToastProvider } from "@/components/ui/Toast";
import "./globals.css";

const inter = Inter({ subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  title: "Xray Sub",
  description: "VLESS subscription management",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" className="dark">
      <body className={`${inter.className} bg-bg text-zinc-200 min-h-screen`}>
        <ToastProvider>{children}</ToastProvider>
      </body>
    </html>
  );
}
