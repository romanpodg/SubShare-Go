"use client";

import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { AppleEmojiInput, type AppleEmojiInputHandle } from "@/components/ui/AppleEmojiInput";
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
  onSaved?: () => Promise<void> | void;
}

type SubscriptionFormatOption = "links" | "xray-json";

interface SegmentedOption<T extends string> {
  value: T;
  label: string;
}

const SUBSCRIPTION_FORMAT_OPTIONS: SegmentedOption<SubscriptionFormatOption>[] = [
  { value: "links", label: "Ссылки-ключи" },
  { value: "xray-json", label: "XRAY-JSON" },
];

function SegmentedSelector<T extends string>({
  label,
  value,
  options,
  onChange,
  disabled,
}: {
  label: string;
  value: T;
  options: SegmentedOption<T>[];
  onChange: (next: T) => void;
  disabled: boolean;
}) {
  return (
    <div
      className={`rounded-xl border p-2.5 ${disabled ? "border-border bg-surface-2/40 opacity-60" : "border-border bg-surface-2"}`}
    >
      <div className={`mb-2 text-sm font-medium ${disabled ? "text-zinc-500" : "text-zinc-200"}`}>
        {label}
      </div>
      <div
        className="grid gap-1 rounded-lg border border-border bg-surface-1 p-1"
        style={{ gridTemplateColumns: `repeat(${Math.max(options.length, 1)}, minmax(0, 1fr))` }}
      >
        {options.map((option) => {
          const active = option.value === value;
          return (
            <button
              key={option.value}
              type="button"
              onClick={() => !disabled && onChange(option.value)}
              disabled={disabled}
              className={`rounded-md px-2 py-2 text-xs font-medium transition ${
                active
                  ? "bg-accent text-accent-fg shadow-sm"
                  : "text-zinc-300 hover:bg-surface-2/80 hover:text-zinc-100"
              }`}
            >
              {option.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

export function GlobalSubscriptionSettingsModal({ open, onClose, onSaved }: Props) {
  const { toast } = useToast();
  const extraStatusInputRef = useRef<AppleEmojiInputHandle>(null);
  const [loading, setLoading] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);

  const [title, setTitle] = useState("");
  const [refreshHours, setRefreshHours] = useState("12");
  const [extraURL, setExtraURL] = useState("");
  const [extraStatus, setExtraStatus] = useState("");
  const [subscriptionFormat, setSubscriptionFormat] = useState<SubscriptionFormatOption>("links");
  const [showSubscriptionExpiration, setShowSubscriptionExpiration] = useState(false);
  const [timeZoneChoice, setTimeZoneChoice] = useState("Europe/Moscow");
  const [customTimeZone, setCustomTimeZone] = useState("");
  const [language, setLanguage] = useState("ru");
  const [providerID, setProviderID] = useState("");
  const [infoURL, setInfoURL] = useState("");
  const [happSubscriptionBody, setHappSubscriptionBody] = useState("");
  const [happNoLimitMode, setHappNoLimitMode] = useState(false);
  const [happNoLimitModeXHTTPOnly, setHappNoLimitModeXHTTPOnly] = useState(false);
  const [happMandatoryHWID, setHappMandatoryHWID] = useState(false);
  const [happNotifyExpiration, setHappNotifyExpiration] = useState(false);
  const [happHideServerSettings, setHappHideServerSettings] = useState(false);

  const [initialState, setInitialState] = useState({
    title: "",
    refreshHours: "12",
    extraURL: "",
    extraStatus: "",
    subscriptionFormat: "links" as SubscriptionFormatOption,
    showSubscriptionExpiration: false,
    timeZone: "Europe/Moscow",
    language: "ru",
    providerID: "",
    happNoLimitMode: false,
    happNoLimitModeXHTTPOnly: false,
    happMandatoryHWID: false,
    happNotifyExpiration: false,
    happHideServerSettings: false,
  });

  const markFieldChange = () => void 0;

  useEffect(() => {
    if (!open) return;

    const load = async () => {
      setLoading(true);
      try {
        const data = await subscriptionSettingsApi.get();
        const normalizedSubscriptionFormat: SubscriptionFormatOption =
          data.subscription_format === "xray-json" ? "xray-json" : "links";
        const next = {
          title: data.title || "",
          refreshHours: String(data.refresh_hours || 12),
          extraURL: data.extra_url || "",
          extraStatus: data.extra_status || "",
          subscriptionFormat: normalizedSubscriptionFormat,
          showSubscriptionExpiration: Boolean(data.show_subscription_expiration),
          timeZone: data.time_zone || "Europe/Moscow",
          language: data.language || "ru",
          providerID: data.provider_id || "",
          happNoLimitMode: Boolean(data.happ_no_limit_mode),
          happNoLimitModeXHTTPOnly: Boolean(data.happ_no_limit_mode_xhttp_only),
          happMandatoryHWID: Boolean(data.happ_mandatory_hwid),
          happNotifyExpiration: Boolean(data.happ_notify_expiration),
          happHideServerSettings: Boolean(data.happ_hide_server_settings),
        };
        const normalizedTimeZone = next.timeZone.trim() || "Europe/Moscow";
        const hasPresetTimeZone = POPULAR_TIMEZONES.includes(normalizedTimeZone);
        setTitle(next.title);
        setRefreshHours(next.refreshHours);
        setExtraURL(next.extraURL);
        setExtraStatus(next.extraStatus);
        setSubscriptionFormat(next.subscriptionFormat);
        setShowSubscriptionExpiration(next.showSubscriptionExpiration);
        setTimeZoneChoice(hasPresetTimeZone ? normalizedTimeZone : CUSTOM_TIMEZONE_VALUE);
        setCustomTimeZone(hasPresetTimeZone ? "" : normalizedTimeZone);
        setLanguage(next.language);
        setProviderID(next.providerID);
        setInfoURL(data.info_url || "");
        setHappSubscriptionBody(data.happ_subscription_body || "");
        setHappNoLimitMode(next.happNoLimitMode);
        setHappNoLimitModeXHTTPOnly(next.happNoLimitModeXHTTPOnly);
        setHappMandatoryHWID(next.happMandatoryHWID);
        setHappNotifyExpiration(next.happNotifyExpiration);
        setHappHideServerSettings(next.happHideServerSettings);
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
      extraURL !== initialState.extraURL ||
      extraStatus !== initialState.extraStatus ||
      subscriptionFormat !== initialState.subscriptionFormat ||
      showSubscriptionExpiration !== initialState.showSubscriptionExpiration ||
      effectiveTimeZone !== initialState.timeZone ||
      language !== initialState.language ||
      providerID !== initialState.providerID ||
      happNoLimitMode !== initialState.happNoLimitMode ||
      happNoLimitModeXHTTPOnly !== initialState.happNoLimitModeXHTTPOnly ||
      happMandatoryHWID !== initialState.happMandatoryHWID ||
      happNotifyExpiration !== initialState.happNotifyExpiration ||
      happHideServerSettings !== initialState.happHideServerSettings
    );
  }, [
    title,
    refreshHours,
    extraURL,
    extraStatus,
    subscriptionFormat,
    showSubscriptionExpiration,
    effectiveTimeZone,
    language,
    providerID,
    happNoLimitMode,
    happNoLimitModeXHTTPOnly,
    happMandatoryHWID,
    happNotifyExpiration,
    happHideServerSettings,
    initialState,
  ]);

  const validProviderID = providerID.trim() === "" || /^[A-Za-z0-9]{8}$/.test(providerID.trim());
  const canSubmit =
    initialLoaded &&
    hasChanges &&
    validProviderID &&
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
        subscription_format: subscriptionFormat,
        show_subscription_expiration: showSubscriptionExpiration,
        time_zone: effectiveTimeZone,
        language,
        provider_id: providerID.trim(),
        happ_no_limit_mode: happNoLimitMode,
        happ_no_limit_mode_xhttp_only: happNoLimitModeXHTTPOnly,
        happ_mandatory_hwid: happMandatoryHWID,
        happ_notify_expiration: happNotifyExpiration,
        happ_hide_server_settings: happHideServerSettings,
        happ_subscription_body: happSubscriptionBody,
      });
      toast("Общие настройки подписки обновлены", "success");
      setInitialState({
        title,
        refreshHours,
        extraURL,
        extraStatus,
        subscriptionFormat,
        showSubscriptionExpiration,
        timeZone: effectiveTimeZone,
        language,
        providerID,
        happNoLimitMode,
        happNoLimitModeXHTTPOnly,
        happMandatoryHWID,
        happNotifyExpiration,
        happHideServerSettings,
      });
      if (onSaved) {
        await onSaved();
      }
      onClose();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить настройки", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Настройки сервиса"
      className="max-h-[calc(100dvh-1.5rem)] w-[96vw] max-w-[96rem]"
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="ui-joined-grid grid xl:grid-cols-[minmax(0,1fr)_minmax(26rem,1fr)]">
          <div data-testid="settings-primary-column" className="grid content-start gap-3 rounded-xl border border-border bg-surface-2/20 p-4">
            <div className="text-sm font-semibold text-zinc-200">Редактор подписки</div>
            <Input
              label="Название подписки"
              value={title}
              onChange={(e) => {
                markFieldChange();
                setTitle(e.target.value);
              }}
              placeholder="VPN Subscription"
            />
            <Input
              label="Автообновление (часы)"
              type="number"
              min="1"
              max="720"
              value={refreshHours}
              onChange={(e) => {
                markFieldChange();
                setRefreshHours(e.target.value);
              }}
            />
            <Input
              label="Ссылка на страницу поддержки"
              type="url"
              value={extraURL}
              onChange={(e) => {
                markFieldChange();
                setExtraURL(e.target.value);
              }}
              placeholder="https://example.com"
            />
            <AppleEmojiInput
              ref={extraStatusInputRef}
              label="Объявление"
              value={extraStatus}
              onChange={(e) => {
                markFieldChange();
                setExtraStatus(e.target.value);
              }}
              placeholder="Ваша подписка истекает через 100 дней"
              multiline
              maxLength={200}
              showCounter
              rightSlot={
                <EmojiPickerButton
                  iconOnly
                  onSelect={(emoji) => {
                    markFieldChange();
                    extraStatusInputRef.current?.insertEmoji(emoji);
                  }}
                />
              }
            />
            <div className="border-t border-border" />
            <SegmentedSelector
              label="Формат всей подписки"
              value={subscriptionFormat}
              options={SUBSCRIPTION_FORMAT_OPTIONS}
              onChange={(next) => {
                markFieldChange();
                setSubscriptionFormat(next);
              }}
              disabled={false}
            />
            <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-border bg-surface-2/50 px-3 py-3">
              <input
                type="checkbox"
                checked={showSubscriptionExpiration}
                onChange={(event) => {
                  markFieldChange();
                  setShowSubscriptionExpiration(event.target.checked);
                }}
                className="mt-0.5 h-4 w-4 accent-[var(--accent)]"
              />
              <span className="min-w-0">
                <span className="block text-sm font-medium text-zinc-200">Показывать дату окончания подписки</span>
                <span className="mt-1 block text-xs leading-5 text-zinc-500">
                  Передаёт дату окончания подписки совместимым клиентам.
                </span>
              </span>
            </label>
          </div>

          <div data-testid="settings-secondary-column" className="grid content-start gap-4 rounded-xl border border-border bg-surface-2/20 p-4">
            <div>
              <div className="text-sm font-semibold text-zinc-200">Настройки для Happ</div>
              <div className="mt-1 text-xs text-zinc-500">
                Дополнительные параметры работают только при заполненном Provider ID.
              </div>
            </div>
            <Input
              label="Provider ID"
              value={providerID}
              onChange={(e) => {
                markFieldChange();
                setProviderID(e.target.value);
              }}
              placeholder="Например, A1B2C3D4"
              maxLength={8}
            />
            <div className={`rounded-lg border px-3 py-2 text-xs ${validProviderID ? "border-border bg-surface-2/50 text-zinc-500" : "border-red-500/40 bg-red-500/10 text-red-300"}`}>
              Provider ID должен состоять ровно из 8 латинских букв или цифр. Неподтверждённые Happ-параметры скрыты; их прежние значения сохраняются.
            </div>
            <section data-testid="settings-localization" className="grid gap-3 border-t border-border pt-4">
              <div className="text-sm font-semibold text-zinc-200">Локализация сервиса</div>
              <Select
                label="Часовой пояс"
                value={timeZoneChoice}
                onChange={(e) => {
                  markFieldChange();
                  setTimeZoneChoice(e.target.value);
                }}
                options={[
                  ...POPULAR_TIMEZONES.map((zone) => ({ value: zone, label: zone })),
                  { value: CUSTOM_TIMEZONE_VALUE, label: "Другой (ввести вручную)" },
                ]}
              />
              {timeZoneChoice === CUSTOM_TIMEZONE_VALUE && (
                <Input
                  label="Часовой пояс вручную (IANA)"
                  value={customTimeZone}
                  onChange={(e) => {
                    markFieldChange();
                    setCustomTimeZone(e.target.value);
                  }}
                  placeholder="Europe/Moscow"
                />
              )}
              <div className="flex flex-col gap-1.5">
                <label className="text-sm text-zinc-400">Язык интерфейса</label>
                <div className="cursor-not-allowed rounded-lg border border-border bg-surface-2 px-3 py-2 text-sm text-zinc-500">
                  Недоступно
                </div>
              </div>
            </section>
          </div>
        </div>
        <Button type="submit" loading={loading} disabled={!canSubmit}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
