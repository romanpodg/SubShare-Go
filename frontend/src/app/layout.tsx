import type { Metadata } from "next";
import { ToastProvider } from "@/components/ui/Toast";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { PanelSettingsProvider } from "@/context/PanelSettingsContext";
import "./globals.css";

export const metadata: Metadata = {
  title: {
    default: "SubShare",
    template: "%s — SubShare",
  },
  applicationName: "SubShare",
  description: "Self-hosted subscription delivery and configuration operations hub.",
};

// Runs synchronously before React hydrates — eliminates title/favicon flash on every page.
const STORAGE_KEY = "subshare_panel_settings";
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
    <html lang="ru" className="dark" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: earlyInitScript }} />
      </head>
      <body className="bg-bg min-h-screen">
        <a
          href="#main-content"
          className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[100] focus:bg-accent focus:px-4 focus:py-3 focus:font-mono focus:text-xs focus:font-semibold focus:uppercase focus:tracking-wider focus:text-accent-fg"
        >
          Перейти к содержимому
        </a>
        <ToastProvider>
          <PanelSettingsProvider>
            <ErrorBoundary>{children}</ErrorBoundary>
          </PanelSettingsProvider>
        </ToastProvider>
      </body>
    </html>
  );
}
