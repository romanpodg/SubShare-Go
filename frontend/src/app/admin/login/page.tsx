"use client";

import { FormEvent, useEffect, useState } from "react";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { Activity, ArrowRight, LockKeyhole } from "lucide-react";
import { Input } from "@/components/ui/Input";
import { PasswordInput } from "@/components/ui/PasswordInput";
import { Button } from "@/components/ui/Button";
import {
  OperationalStatus,
  SignalCoreDiagram,
  SystemLabel,
  TechnicalFrame,
} from "@/components/ui/Technical";
import { useAuth } from "@/hooks/useAuth";
import { usePanelSettings } from "@/context/PanelSettingsContext";
import { EmojiText } from "@/components/ui/EmojiText";

export default function LoginPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [usernameError, setUsernameError] = useState("");
  const [passwordError, setPasswordError] = useState("");
  const [loading, setLoading] = useState(false);
  const { login } = useAuth();
  const { settings } = usePanelSettings();

  useEffect(() => {
    document.title = settings.pageTitles.adminLogin;
  }, [settings.pageTitles.adminLogin]);

  const validateUsername = (value: string) => {
    if (/[а-яА-ЯёЁ]/.test(value)) {
      setUsernameError("Логин не может содержать кириллицу");
      return false;
    }
    if (!/^[a-zA-Z0-9]+$/.test(value)) {
      setUsernameError("Используйте только латинские буквы и цифры");
      return false;
    }
    if (/^\d/.test(value)) {
      setUsernameError("Логин не может начинаться с цифры");
      return false;
    }
    setUsernameError("");
    return true;
  };

  const validatePassword = (value: string) => {
    if (/[а-яА-ЯёЁ]/.test(value)) {
      setPasswordError("Пароль не может содержать кириллицу");
      return false;
    }
    setPasswordError("");
    return true;
  };

  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    if (!validateUsername(username) || !validatePassword(password)) return;

    setLoading(true);
    try {
      await login(username, password);
      router.replace("/admin/overview");
    } catch {
      setError("Пользователь с такими данными не найден");
    } finally {
      setLoading(false);
    }
  };

  return (
    <main id="main-content" className="technical-grid min-h-screen bg-bg p-3 sm:p-6">
      <div className="mx-auto grid min-h-[calc(100vh-1.5rem)] max-w-[1280px] border border-border bg-[#070808] sm:min-h-[calc(100vh-3rem)] lg:grid-cols-[1.15fr_0.85fr]">
        <section className="relative hidden min-h-[680px] overflow-hidden border-r border-border p-8 lg:flex lg:flex-col lg:justify-between xl:p-12">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {settings.logoDataUrl ? (
                <Image src={settings.logoDataUrl} alt="" width={40} height={40} unoptimized className="h-10 w-10 object-contain" />
              ) : (
                <div className="flex h-10 w-10 items-center justify-center border border-accent/30 bg-accent/5 text-accent">
                  <Activity className="h-5 w-5" />
                </div>
              )}
              <div>
                <div data-testid="login-display-en" className="font-display text-lg font-medium text-zinc-100">
                  <EmojiText text={settings.panelTitle || "SubShare"} />
                </div>
                <SystemLabel>Infrastructure access layer</SystemLabel>
              </div>
            </div>
            <OperationalStatus label="SYSTEM OPERATIONAL" />
          </div>

          <div className="relative z-10 max-w-[650px]">
            <SystemLabel className="text-accent/80">CONTROL PLANE / AUTH GATE</SystemLabel>
            <h1
              data-testid="login-display-ru"
              className="mt-5 max-w-2xl font-display text-[2.75rem] font-medium leading-[0.98] tracking-[-0.045em] text-zinc-100 xl:text-[3.5rem]"
            >
              <span className="block">Конфигурации</span>
              <span className="block">под контролем.</span>
              <span className="block text-zinc-300">Доставка без шума.</span>
            </h1>
            <p className="mt-6 max-w-xl text-base leading-7 text-zinc-500">
              Единый операционный контур для пользователей, ключей, источников и персональных VPN-подписок.
            </p>
          </div>

          <TechnicalFrame className="technical-grid mt-8 h-[290px] overflow-hidden bg-[#080909]">
            <SignalCoreDiagram className="h-full w-full text-zinc-500" />
            <div className="absolute bottom-4 left-4 flex gap-6">
              <div><SystemLabel>Channel</SystemLabel><div className="mt-1 font-mono text-xs text-zinc-300">TLS / VLESS</div></div>
              <div><SystemLabel>State</SystemLabel><div className="mt-1 font-mono text-xs text-accent">SYNCHRONIZED</div></div>
              <div><SystemLabel>Routes</SystemLabel><div className="mt-1 font-mono text-xs text-zinc-300">04 / ACTIVE</div></div>
            </div>
          </TechnicalFrame>
        </section>

        <section className="flex min-h-[640px] items-center justify-center p-5 sm:p-10 lg:min-h-0 xl:p-14">
          <div className="w-full max-w-[420px]">
            <div className="mb-9 flex items-center justify-between lg:hidden">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center border border-accent/30 bg-accent/5 text-accent">
                  <Activity className="h-5 w-5" />
                </div>
                <div className="font-display text-lg font-medium text-zinc-100">
                  <EmojiText text={settings.panelTitle || "SubShare"} />
                </div>
              </div>
              <OperationalStatus label="ONLINE" />
            </div>

            <SystemLabel className="text-accent/80">SECURE SESSION / 01</SystemLabel>
            <h2 className="mt-4 font-display text-4xl font-medium tracking-[-0.045em] text-zinc-100">
              Вход в SubShare
            </h2>
            <p className="mt-3 text-sm leading-6 text-zinc-500">
              Авторизуйтесь для доступа к операционному контуру.
            </p>

            <TechnicalFrame className="mt-8 bg-surface-1 p-5 sm:p-6">
              <div className="mb-6 flex items-center gap-3 border-b border-border pb-4">
                <div className="flex h-9 w-9 items-center justify-center border border-border bg-surface-2 text-zinc-400">
                  <LockKeyhole className="h-4 w-4" />
                </div>
                <div>
                  <SystemLabel>Authentication channel</SystemLabel>
                  <div className="mt-1 font-mono text-[10px] text-accent">ENCRYPTED / READY</div>
                </div>
              </div>

              <form onSubmit={handleSubmit} className="flex flex-col gap-5">
                <Input
                  label="Логин администратора"
                  type="text"
                  value={username}
                  onChange={(event) => {
                    setUsername(event.target.value);
                    if (event.target.value) validateUsername(event.target.value);
                    else setUsernameError("");
                  }}
                  error={usernameError}
                  autoComplete="username"
                  autoFocus
                  required
                />
                <PasswordInput
                  label="Пароль"
                  value={password}
                  onChange={(event) => {
                    setPassword(event.target.value);
                    if (event.target.value) validatePassword(event.target.value);
                    else setPasswordError("");
                  }}
                  error={passwordError}
                  autoComplete="current-password"
                  required
                />
                {error && (
                  <div role="alert" className="border border-danger/25 bg-danger/5 px-3 py-3 font-mono text-[10px] uppercase tracking-wide text-danger">
                    {error}
                  </div>
                )}
                <Button type="submit" loading={loading} className="mt-1 w-full justify-between">
                  Войти в систему
                  <ArrowRight className="h-4 w-4" />
                </Button>
              </form>
            </TechnicalFrame>

            <div className="mt-5 flex items-center justify-between">
              <SystemLabel>SubShare / Admin</SystemLabel>
              <span className="font-mono text-[9px] text-zinc-700">ACCESS NODE 01</span>
            </div>
          </div>
        </section>
      </div>
    </main>
  );
}
