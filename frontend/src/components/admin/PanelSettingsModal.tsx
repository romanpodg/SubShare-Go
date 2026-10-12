"use client";

import { useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { usePanelSettings } from "@/context/PanelSettingsContext";
import { FilePenLine, ImageIcon, Settings, SlidersHorizontal, Upload } from "lucide-react";

interface Props {
  open: boolean;
  onClose: () => void;
}

/** Resize & compress a raster image to at most maxPx on its longest side. SVGs are kept as-is. */
async function compressImage(file: File, maxPx: number): Promise<string> {
  // SVG — return raw data URL, no compression needed
  if (file.type === "image/svg+xml") {
    return new Promise((resolve) => {
      const reader = new FileReader();
      reader.onload = (e) => resolve(e.target?.result as string);
      reader.readAsDataURL(file);
    });
  }
  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = (e) => {
      const img = new Image();
      img.onload = () => {
        const scale = Math.min(1, maxPx / Math.max(img.width, img.height));
        const w = Math.round(img.width * scale);
        const h = Math.round(img.height * scale);
        const canvas = document.createElement("canvas");
        canvas.width = w;
        canvas.height = h;
        canvas.getContext("2d")!.drawImage(img, 0, 0, w, h);
        resolve(canvas.toDataURL("image/png", 0.9));
      };
      img.src = e.target?.result as string;
    };
    reader.readAsDataURL(file);
  });
}

export function PanelSettingsModal({ open, onClose }: Props) {
  const { loading } = usePanelSettings();
  if (!open) return null;
  if (loading) return <Modal open onClose={onClose} title="Настройки панели"><div role="status">Загрузка настроек…</div></Modal>;
  return <PanelSettingsEditor open onClose={onClose} />;
}

function PanelSettingsEditor({ open, onClose }: Props) {
  const { settings, updateSettings } = usePanelSettings();

  const [panelTitle, setPanelTitle] = useState(settings.panelTitle);
  const [logoDataUrl, setLogoDataUrl] = useState(settings.logoDataUrl);
  const [faviconDataUrl, setFaviconDataUrl] = useState(settings.faviconDataUrl);
  const [pageTitleAdmin, setPageTitleAdmin] = useState(settings.pageTitles.admin);
  const [pageTitleLogin, setPageTitleLogin] = useState(settings.pageTitles.adminLogin);
  const [pageTitleSub, setPageTitleSub] = useState(settings.pageTitles.subscription);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState("");
  const savingRef = useRef(false);

  const logoInputRef = useRef<HTMLInputElement>(null);
  const faviconInputRef = useRef<HTMLInputElement>(null);

  const handleLogoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const result = await compressImage(file, 256);
    setLogoDataUrl(result);
  };

  const handleFaviconUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    const result = await compressImage(file, 64);
    setFaviconDataUrl(result);
  };

  const handleSave = async () => {
    if (savingRef.current) return;
    savingRef.current = true;
    setSaving(true);
    setSaveError("");
    try {
      await updateSettings({
        panelTitle: panelTitle.trim() || "SubShare",
        logoDataUrl,
        faviconDataUrl,
        pageTitles: {
          admin: pageTitleAdmin.trim() || "Панель управления",
          adminLogin: pageTitleLogin.trim() || "Вход",
          subscription: pageTitleSub.trim() || "VPN-подписка",
        },
      });
      onClose();
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : "Не удалось сохранить настройки");
    } finally {
      savingRef.current = false;
      setSaving(false);
    }
  };

  const handleRemoveLogo = () => {
    setLogoDataUrl("");
    if (logoInputRef.current) logoInputRef.current.value = "";
  };

  const handleRemoveFavicon = () => {
    setFaviconDataUrl("");
    if (faviconInputRef.current) faviconInputRef.current.value = "";
  };

  return (
    <Modal open={open} onClose={() => { if (!savingRef.current) onClose(); }} title="Настройки панели" icon={<Settings className="h-4 w-4" aria-hidden="true" />}>
      <fieldset disabled={saving} className="flex min-w-0 flex-col gap-3">

        {/* Название панели */}
        <section className="flex flex-col gap-2">
          <h3 className="flex items-center gap-2 border-b border-border pb-1 text-xs font-semibold uppercase tracking-wide text-zinc-400">
            <SlidersHorizontal className="h-3.5 w-3.5 text-accent" aria-hidden="true" />
            <span>Название панели</span>
          </h3>
          <Input
            label="Заголовок панели"
            value={panelTitle}
            onChange={(e) => setPanelTitle(e.target.value)}
            placeholder="SubShare"
          />
        </section>

        {/* Логотип */}
        <section className="flex flex-col gap-2">
          <h3 className="flex items-center gap-2 border-b border-border pb-1 text-xs font-semibold uppercase tracking-wide text-zinc-400">
            <ImageIcon className="h-3.5 w-3.5 text-accent" aria-hidden="true" />
            <span>Логотип</span>
          </h3>
          <div className="flex items-center gap-3 flex-wrap">
            {logoDataUrl ? (
              <>
                <div className="w-7 h-7 rounded border border-border bg-surface-2 flex items-center justify-center overflow-hidden shrink-0">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={logoDataUrl} alt="Логотип" className="max-w-full max-h-full object-contain" />
                </div>
                <span className="text-xs text-zinc-400">Логотип загружен</span>
                <button
                  type="button"
                  onClick={handleRemoveLogo}
                  className="text-xs text-red-400 hover:text-red-300 transition-colors"
                >
                  Удалить
                </button>
              </>
            ) : (
              <span className="text-xs text-zinc-500">Логотип не установлен</span>
            )}
            <input
              ref={logoInputRef}
              type="file"
              accept="image/*"
              onChange={handleLogoUpload}
              className="hidden"
              id="logo-upload"
            />
            <label
              htmlFor="logo-upload"
              className="inline-flex items-center gap-1.5 cursor-pointer text-xs text-zinc-400 hover:text-zinc-200 transition-colors border border-border rounded-lg px-2.5 py-1.5"
            >
              <Upload className="h-3.5 w-3.5" aria-hidden="true" />
              {logoDataUrl ? "Заменить" : "Загрузить"}
            </label>
          </div>
          <p className="text-xs text-zinc-500">PNG, SVG, JPG — отображается левее названия.</p>
        </section>

        {/* Favicon */}
        <section className="flex flex-col gap-2">
          <h3 className="flex items-center gap-2 border-b border-border pb-1 text-xs font-semibold uppercase tracking-wide text-zinc-400">
            <FilePenLine className="h-3.5 w-3.5 text-accent" aria-hidden="true" />
            <span>Favicon и заголовки страниц</span>
          </h3>
          <div className="flex items-center gap-3 flex-wrap">
            {faviconDataUrl ? (
              <>
                <div className="w-5 h-5 rounded border border-border bg-surface-2 flex items-center justify-center overflow-hidden shrink-0">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img src={faviconDataUrl} alt="Favicon" className="max-w-full max-h-full object-contain" />
                </div>
                <span className="text-xs text-zinc-400">Favicon загружен</span>
                <button
                  type="button"
                  onClick={handleRemoveFavicon}
                  className="text-xs text-red-400 hover:text-red-300 transition-colors"
                >
                  Удалить
                </button>
              </>
            ) : (
              <span className="text-xs text-zinc-500">Favicon не установлен</span>
            )}
            <input
              ref={faviconInputRef}
              type="file"
              accept="image/*,.ico"
              onChange={handleFaviconUpload}
              className="hidden"
              id="favicon-upload"
            />
            <label
              htmlFor="favicon-upload"
              className="inline-flex items-center gap-1.5 cursor-pointer text-xs text-zinc-400 hover:text-zinc-200 transition-colors border border-border rounded-lg px-2.5 py-1.5"
            >
              <Upload className="h-3.5 w-3.5" aria-hidden="true" />
              {faviconDataUrl ? "Заменить" : "Загрузить"}
            </label>
          </div>
          <p className="text-xs text-zinc-500">ICO, PNG, SVG — отображается на вкладке браузера.</p>

          <div className="flex flex-col gap-2 mt-0.5">
            <Input
              label="Заголовок — Админ-панель"
              value={pageTitleAdmin}
              onChange={(e) => setPageTitleAdmin(e.target.value)}
              placeholder="Панель управления — SubShare"
            />
            <Input
              label="Заголовок — Страница входа"
              value={pageTitleLogin}
              onChange={(e) => setPageTitleLogin(e.target.value)}
              placeholder="Вход — SubShare"
            />
            <Input
              label="Заголовок — Клиентская страница"
              value={pageTitleSub}
              onChange={(e) => setPageTitleSub(e.target.value)}
              placeholder="VPN-подписка — SubShare"
            />
          </div>
        </section>

        {/* Кнопки */}
        {saveError ? <div role="alert" className="text-sm text-red-400">{saveError}</div> : null}
        <div className="flex justify-end gap-2 pt-1">
          <Button variant="ghost" onClick={onClose} disabled={saving}>
            Отмена
          </Button>
          <Button onClick={handleSave} disabled={saving}>
            Сохранить
          </Button>
        </div>
      </fieldset>
    </Modal>
  );
}
