"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput } from "@/components/ui/AppleEmojiInput";
import { Select } from "@/components/ui/Select";
import { Button } from "@/components/ui/Button";
import { EmojiPickerButton } from "@/components/ui/EmojiPickerButton";
import { useToast } from "@/components/ui/Toast";
import { subscriptionSettings as subscriptionSettingsApi } from "@/lib/api";

const CUSTOM_TIMEZONE_VALUE = "__custom_timezone__";

const POPULAR_TIMEZONES = [
  "Europe/Moscow",
  "Europe/Kaliningrad",
  "Europe/Samara",
  "Asia/Yekaterinburg",
  "Asia/Omsk",
  "Asia/Krasnoyarsk",
  "Asia/Irkutsk",
  "Asia/Yakutsk",
  "Asia/Vladivostok",
  "Asia/Tokyo",
  "Asia/Almaty",
  "Asia/Dubai",
  "Europe/Berlin",
  "Europe/Istanbul",
  "UTC",
];

interface Props {
  open: boolean;
  onClose: () => void;
}

export function GlobalSubscriptionSettingsModal({ open, onClose }: Props) {
  const { toast } = useToast();
  const [loading, setLoading] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);

  const [title, setTitle] = useState("");
  const [refreshHours, setRefreshHours] = useState("12");
  const [infoURL, setInfoURL] = useState("");
  const [extraURL, setExtraURL] = useState("");
  const [extraStatus, setExtraStatus] = useState("");
  const [timeZoneChoice, setTimeZoneChoice] = useState("Europe/Moscow");
  const [customTimeZone, setCustomTimeZone] = useState("");
  const [language, setLanguage] = useState("ru");

  const [initialState, setInitialState] = useState({
    title: "",
    refreshHours: "12",
    infoURL: "",
    extraURL: "",
    extraStatus: "",
    timeZone: "Europe/Moscow",
    language: "ru",
  });

  useEffect(() => {
    if (!open) return;

    const load = async () => {
      setLoading(true);
      try {
        const data = await subscriptionSettingsApi.get();
        const next = {
          title: data.title || "",
          refreshHours: String(data.refresh_hours || 12),
          infoURL: data.info_url || "",
          extraURL: data.extra_url || "",
          extraStatus: data.extra_status || "",
          timeZone: data.time_zone || "Europe/Moscow",
          language: data.language || "ru",
        };
        const normalizedTimeZone = next.timeZone.trim() || "Europe/Moscow";
        const hasPresetTimeZone = POPULAR_TIMEZONES.includes(normalizedTimeZone);
        setTitle(next.title);
        setRefreshHours(next.refreshHours);
        setInfoURL(next.infoURL);
        setExtraURL(next.extraURL);
        setExtraStatus(next.extraStatus);
        setTimeZoneChoice(hasPresetTimeZone ? normalizedTimeZone : CUSTOM_TIMEZONE_VALUE);
        setCustomTimeZone(hasPresetTimeZone ? "" : normalizedTimeZone);
        setLanguage(next.language);
        setInitialState(next);
        setInitialLoaded(true);
      } catch {
        toast("Не удалось загрузить общие настройки подписки", "error");
      } finally {
        setLoading(false);
      }
    };

    load();
  }, [open, toast]);

  const parsedRefreshHours = useMemo(() => {
    const parsed = parseInt(refreshHours, 10);
    if (Number.isNaN(parsed) || parsed <= 0) return 12;
    return parsed;
  }, [refreshHours]);

  const effectiveTimeZone = useMemo(() => {
    if (timeZoneChoice === CUSTOM_TIMEZONE_VALUE) {
      return customTimeZone.trim();
    }
    return timeZoneChoice;
  }, [timeZoneChoice, customTimeZone]);

  const hasChanges = useMemo(() => {
    return (
      title !== initialState.title ||
      refreshHours !== initialState.refreshHours ||
      infoURL !== initialState.infoURL ||
      extraURL !== initialState.extraURL ||
      extraStatus !== initialState.extraStatus ||
      effectiveTimeZone !== initialState.timeZone ||
      language !== initialState.language
    );
  }, [title, refreshHours, infoURL, extraURL, extraStatus, effectiveTimeZone, language, initialState]);

  const canSubmit =
    initialLoaded &&
    hasChanges &&
    !(timeZoneChoice === CUSTOM_TIMEZONE_VALUE && effectiveTimeZone === "");

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!hasChanges) return;

    setLoading(true);
    try {
      await subscriptionSettingsApi.update({
        title,
        refresh_hours: parsedRefreshHours,
        info_url: infoURL,
        extra_url: extraURL,
        extra_status: extraStatus,
        time_zone: effectiveTimeZone,
        language,
      });
      toast("Общие настройки подписки обновлены", "success");
      setInitialState({
        title,
        refreshHours,
        infoURL,
        extraURL,
        extraStatus,
        timeZone: effectiveTimeZone,
        language,
      });
      onClose();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить настройки", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Настройки сервиса">
      <form onSubmit={handleSubmit} className="flex flex-col gap-2.5">
        <div className="text-sm font-semibold text-zinc-200">Редактор подписки</div>
        <Input
          label="Название подписки"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="MetaFlex VPN"
        />
        <Input
          label="Автообновление (часы)"
          type="number"
          min="1"
          max="720"
          value={refreshHours}
          onChange={(e) => setRefreshHours(e.target.value)}
        />
        <Input
          label="Инфо-ссылка"
          type="url"
          value={infoURL}
          onChange={(e) => setInfoURL(e.target.value)}
          placeholder="https://example.com/subscription/info"
        />
        <Input
          label="Дополнительная ссылка"
          type="url"
          value={extraURL}
          onChange={(e) => setExtraURL(e.target.value)}
          placeholder="https://example.com"
        />
        <AppleEmojiInput
          label="Описание/доп. статус"
          value={extraStatus}
          onChange={(e) => setExtraStatus(e.target.value)}
          placeholder="✅ Active, истекает через 85 дн"
        />
        <div className="-mt-1">
          <EmojiPickerButton inline onSelect={(emoji) => setExtraStatus((prev) => `${prev}${emoji}`)} />
        </div>
        <div className="border-t border-border" />
        <div className="text-sm font-semibold text-zinc-200">Локализация сервиса</div>
        <Select
          label="Часовой пояс"
          value={timeZoneChoice}
          onChange={(e) => setTimeZoneChoice(e.target.value)}
          options={[
            ...POPULAR_TIMEZONES.map((zone) => ({ value: zone, label: zone })),
            { value: CUSTOM_TIMEZONE_VALUE, label: "Другой (ввести вручную)" },
          ]}
        />
        {timeZoneChoice === CUSTOM_TIMEZONE_VALUE && (
          <Input
            label="Часовой пояс вручную (IANA)"
            value={customTimeZone}
            onChange={(e) => setCustomTimeZone(e.target.value)}
            placeholder="Europe/Moscow"
          />
        )}
        <div className="flex flex-col gap-1.5">
          <label className="text-sm text-zinc-400">Язык интерфейса</label>
          <div className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-500 cursor-not-allowed">
            Недоступно
          </div>
        </div>
        <Button type="submit" loading={loading} disabled={!canSubmit}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
