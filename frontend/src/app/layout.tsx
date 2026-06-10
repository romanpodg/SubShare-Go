import type { Metadata } from "next";
import { Inter } from "next/font/google";
import { ToastProvider } from "@/components/ui/Toast";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { PanelSettingsProvider } from "@/context/PanelSettingsContext";
import { ThemeProvider } from "@/components/ThemeProvider";
import "./globals.css";

const inter = Inter({ subsets: ["latin", "cyrillic"] });

export const metadata: Metadata = {
  description: "VLESS subscription management",
};

// Runs synchronously before React hydrates — eliminates title/favicon flash on every page.
const STORAGE_KEY = "xray_sub_panel_settings";
const earlyInitScript = `(function(){try{
  var s=JSON.parse(localStorage.getItem('${STORAGE_KEY}')||'{}');
  if(s.faviconDataUrl){
    var links=document.querySelectorAll("link[rel~='icon']");
    if(!links.length){
      var icon=document.createElement('link');
      icon.rel='icon';
      icon.href=s.faviconDataUrl;
      document.head.appendChild(icon);
      var shortcut=document.createElement('link');
      shortcut.rel='shortcut icon';
      shortcut.href=s.faviconDataUrl;
      document.head.appendChild(shortcut);
    }else{
      for(var i=0;i<links.length;i++){links[i].href=s.faviconDataUrl;}
      var hasShortcut=false;
      for(var j=0;j<links.length;j++){if((links[j].rel||'').toLowerCase()==='shortcut icon'){hasShortcut=true;break;}}
      if(!hasShortcut){
        var sc=document.createElement('link');
        sc.rel='shortcut icon';
        sc.href=s.faviconDataUrl;
        document.head.appendChild(sc);
      }
    }
  }
  var p=window.location.pathname;
  var t=s.pageTitles||{};
  var title;
  if(p==='/admin/login'||p==='/admin/login/')title=t.adminLogin;
  else if(p.indexOf('/subscription')===0)title=t.subscription;
  else if(p.indexOf('/admin')===0)title=t.admin;
  if(title)document.title=title;
}catch(e){}})();`;

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="ru" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: earlyInitScript }} />
      </head>
      <body className={`${inter.className} bg-bg min-h-screen`}>
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-50 focus:bg-accent focus:text-white focus:px-4 focus:py-2 focus:rounded-lg"
        >
          Перейти к содержимому
        </a>
        <ToastProvider>
          <ThemeProvider attribute="class" defaultTheme="system" enableSystem>
            <PanelSettingsProvider>
              <ErrorBoundary>{children}</ErrorBoundary>
            </PanelSettingsProvider>
          </ThemeProvider>
        </ToastProvider>
      </body>
    </html>
  );
}
