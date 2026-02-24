"use client";

import { useState, useEffect, FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Card } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { PasswordInput } from "@/components/ui/PasswordInput";
import { Button } from "@/components/ui/Button";
import { auth, setCsrfToken } from "@/lib/api";
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

  const { settings } = usePanelSettings();

  useEffect(() => {
    document.title = settings.pageTitles.adminLogin;
  }, [settings.pageTitles.adminLogin]);

  const validateUsername = (value: string): boolean => {
    // Проверка на кириллицу
    if (/[а-яА-ЯёЁ]/.test(value)) {
      setUsernameError("Логин не может содержать кириллицу");
      return false;
    }
    // Проверка: только латиница и цифры
    if (!/^[a-zA-Z0-9]+$/.test(value)) {
      setUsernameError("Логин может содержать только буквы латиницы и цифры");
      return false;
    }
    // Проверка: цифра не должна быть первой
    if (/^\d/.test(value)) {
      setUsernameError("Логин не может начинаться с цифры");
      return false;
    }
    setUsernameError("");
    return true;
  };

  const validatePassword = (value: string): boolean => {
    // Пароль может содержать только латиницу, цифры и знаки (не кириллицу)
    if (/[а-яА-ЯёЁ]/.test(value)) {
      setPasswordError("Пароль не может содержать кириллицу");
      return false;
    }
    setPasswordError("");
    return true;
  };

  const handleUsernameChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setUsername(value);
    if (value) {
      validateUsername(value);
    } else {
      setUsernameError("");
    }
  };

  const handlePasswordChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setPassword(value);
    if (value) {
      validatePassword(value);
    } else {
      setPasswordError("");
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError("");

    // Валидация перед отправкой
    const isUsernameValid = validateUsername(username);
    const isPasswordValid = validatePassword(password);

    if (!isUsernameValid || !isPasswordValid) {
      return;
    }

    setLoading(true);
    try {
      const data = await auth.login(username, password);
      setCsrfToken(data.csrf_token);
      router.push("/admin");
    } catch {
      setError("Пользователь с такими данными не найден");
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center p-4">
      <Card className="w-full max-w-sm">
        <div className="flex items-center justify-center gap-2 mb-6">
          {settings.logoDataUrl && (
            // eslint-disable-next-line @next/next/no-img-element
            <img
              src={settings.logoDataUrl}
              alt="Логотип"
              className="h-5 w-auto object-contain"
              style={{ imageRendering: "auto" }}
            />
          )}
          <h1 className="text-xl font-semibold text-center">
            <EmojiText text={settings.panelTitle} />
          </h1>
        </div>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <Input
            label="Логин"
            type="text"
            value={username}
            onChange={handleUsernameChange}
            error={usernameError}
            autoFocus
            required
          />
          <PasswordInput
            label="Пароль"
            value={password}
            onChange={handlePasswordChange}
            error={passwordError}
            required
          />
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button type="submit" loading={loading}>
            Войти
          </Button>
        </form>
      </Card>
    </div>
  );
}
