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
}

function buildSuggestedSubscriptionBody(params: {
  title: string;
  refreshHours: string;
  extraURL: string;
  extraStatus: string;
  providerID: string;
  happNoLimitMode: boolean;
  happNoLimitModeXHTTPOnly: boolean;
  happMandatoryHWID: boolean;
  happNotifyExpiration: boolean;
  happHideServerSettings: boolean;
}) {
  const lines: string[] = [];
  const title = params.title.trim();
  const refreshHours = params.refreshHours.trim();
  const extraURL = params.extraURL.trim();
  const extraStatus = params.extraStatus.trim();
  const providerID = params.providerID.trim();

  if (title) {
    lines.push(`#profile-title: ${title}`);
  }
  if (refreshHours) {
    lines.push(`#profile-update-interval: ${refreshHours}`);
  }
  lines.push("#profile-web-page-url: {subscription_url}");
  if (extraURL) {
    lines.push(`#support-url: ${extraURL}`);
  }
  if (extraStatus) {
    lines.push(`#announce: base64:${encodeBase64Text(extraStatus)}`);
  }
  if (providerID) {
    lines.push("");
    lines.push(`#providerid ${providerID}`);
    if (params.happNoLimitMode) lines.push("#no-limit-enabled: 1");
    if (params.happNoLimitModeXHTTPOnly) lines.push("#no-limit-xhttp-enabled: 1");
    if (params.happMandatoryHWID) lines.push("#subscription-always-hwid-enable: 1");
    if (params.happNotifyExpiration) lines.push("#notification-subs-expire: 1");
    if (params.happHideServerSettings) lines.push("#hide-settings: 1");
  }
  lines.push("");
  lines.push("# Далее сервер автоматически добавит ключи подписки ниже");
  return lines.join("\n").trim();
}

function encodeBase64Text(value: string) {
  if (!value) return "";
  const bytes = new TextEncoder().encode(value);
  let binary = "";
  bytes.forEach((byte) => {
    binary += String.fromCharCode(byte);
  });
  return btoa(binary);
}

function hasLegacySubscriptionBodyMarkers(body: string) {
  return [
    "#profile-desc:",
    "#profile-status:",
    "#description:",
    "#happ-provider-id:",
    "#happ-no-limit-mode:",
    "#happ-no-limit-mode-xhttp-only:",
    "#happ-mandatory-hwid:",
    "#happ-notify-expiration:",
    "#happ-hide-server-settings:",
  ].some((marker) => body.includes(marker));
}

function decodeBase64Text(value: string) {
  try {
    const binary = atob(value);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch {
    return value;
  }
}

function parseBoolDirectiveValue(value: string) {
  const normalized = value.trim().toLowerCase();
  return normalized === "1" || normalized === "true" || normalized === "yes" || normalized === "on";
}

function parseSubscriptionBody(body: string) {
  const parsed = {
    title: "",
    refreshHours: "",
    extraURL: "",
    extraStatus: "",
    providerID: "",
    happNoLimitMode: false,
    happNoLimitModeXHTTPOnly: false,
    happMandatoryHWID: false,
    happNotifyExpiration: false,
    happHideServerSettings: false,
  };

  const lines = body.replace(/\r\n/g, "\n").split("\n");
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line.startsWith("#")) continue;

    if (line.toLowerCase().startsWith("#profile-title:")) {
      parsed.title = line.slice("#profile-title:".length).trim();
      continue;
    }
    if (line.toLowerCase().startsWith("#profile-update-interval:")) {
      parsed.refreshHours = line.slice("#profile-update-interval:".length).trim();
      continue;
    }
    if (line.toLowerCase().startsWith("#support-url:")) {
      parsed.extraURL = line.slice("#support-url:".length).trim();
      continue;
    }
    if (line.toLowerCase().startsWith("#announce:")) {
      const announceValue = line.slice("#announce:".length).trim();
      if (announceValue.toLowerCase().startsWith("base64:")) {
        parsed.extraStatus = decodeBase64Text(announceValue.slice("base64:".length));
      } else {
        parsed.extraStatus = announceValue;
      }
      continue;
    }
    if (line.toLowerCase().startsWith("#providerid ")) {
      parsed.providerID = line.slice("#providerid ".length).trim();
      continue;
    }
    if (line.toLowerCase().startsWith("#no-limit-enabled:")) {
      parsed.happNoLimitMode = parseBoolDirectiveValue(line.slice("#no-limit-enabled:".length));
      continue;
    }
    if (line.toLowerCase().startsWith("#no-limit-xhttp-enabled:")) {
      parsed.happNoLimitModeXHTTPOnly = parseBoolDirectiveValue(line.slice("#no-limit-xhttp-enabled:".length));
      continue;
    }
    if (line.toLowerCase().startsWith("#subscription-always-hwid-enable:")) {
      parsed.happMandatoryHWID = parseBoolDirectiveValue(line.slice("#subscription-always-hwid-enable:".length));
      continue;
    }
    if (line.toLowerCase().startsWith("#notification-subs-expire:")) {
      parsed.happNotifyExpiration = parseBoolDirectiveValue(line.slice("#notification-subs-expire:".length));
      continue;
    }
    if (line.toLowerCase().startsWith("#hide-settings:")) {
      parsed.happHideServerSettings = parseBoolDirectiveValue(line.slice("#hide-settings:".length));
      continue;
    }
  }

  if (parsed.happNoLimitMode && parsed.happNoLimitModeXHTTPOnly) {
    parsed.happNoLimitMode = false;
  }
  if (!parsed.providerID) {
    parsed.happNoLimitMode = false;
    parsed.happNoLimitModeXHTTPOnly = false;
    parsed.happMandatoryHWID = false;
    parsed.happNotifyExpiration = false;
    parsed.happHideServerSettings = false;
  }

  return parsed;
}

function HappToggle({
  label,
  checked,
  onChange,
  disabled,
}: {
  label: string;
  checked: boolean;
  onChange: (next: boolean) => void;
  disabled: boolean;
}) {
  return (
    <button
      type="button"
      onClick={() => !disabled && onChange(!checked)}
      disabled={disabled}
      className={`flex items-center justify-between rounded-xl border px-4 py-2.5 text-left transition-colors ${
        disabled
          ? "cursor-not-allowed border-border bg-surface-2/40 text-zinc-500 opacity-60"
          : "border-border bg-surface-2 hover:bg-surface-1"
      }`}
    >
      <span className="text-sm font-medium text-inherit">{label}</span>
      <span
        className={`relative inline-flex h-5.5 w-10 shrink-0 items-center rounded-full transition-colors ${
          checked && !disabled ? "bg-accent" : "bg-zinc-700"
        }`}
      >
        <span
          className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
            checked ? "translate-x-5" : "translate-x-1"
          }`}
        />
      </span>
    </button>
  );
}

function HappModeGroup({
  noLimitMode,
  noLimitModeXHTTPOnly,
  onChangeNoLimitMode,
  onChangeNoLimitModeXHTTPOnly,
  disabled,
}: {
  noLimitMode: boolean;
  noLimitModeXHTTPOnly: boolean;
  onChangeNoLimitMode: (next: boolean) => void;
  onChangeNoLimitModeXHTTPOnly: (next: boolean) => void;
  disabled: boolean;
}) {
  return (
    <div
      className={`rounded-xl border p-2.5 ${disabled ? "border-border bg-surface-2/40 opacity-60" : "border-border bg-surface-2"}`}
    >
      <div className={`mb-2 text-sm font-medium ${disabled ? "text-zinc-500" : "text-zinc-200"}`}>
        No Limit Mode
      </div>
      <div className="grid gap-2.5">
        <HappToggle
          label="No Limit Mode"
          checked={noLimitMode}
          onChange={onChangeNoLimitMode}
          disabled={disabled}
        />
        <HappToggle
          label="No Limit Mode XHTTP Only"
          checked={noLimitModeXHTTPOnly}
          onChange={onChangeNoLimitModeXHTTPOnly}
          disabled={disabled}
        />
      </div>
    </div>
  );
}

export function GlobalSubscriptionSettingsModal({ open, onClose }: Props) {
  const { toast } = useToast();
  const extraStatusInputRef = useRef<AppleEmojiInputHandle>(null);
  const syncSourceRef = useRef<"init" | "fields" | "body">("init");
  const [loading, setLoading] = useState(false);
  const [initialLoaded, setInitialLoaded] = useState(false);

  const [title, setTitle] = useState("");
  const [refreshHours, setRefreshHours] = useState("12");
  const [extraURL, setExtraURL] = useState("");
  const [extraStatus, setExtraStatus] = useState("");
  const [timeZoneChoice, setTimeZoneChoice] = useState("Europe/Moscow");
  const [customTimeZone, setCustomTimeZone] = useState("");
  const [language, setLanguage] = useState("ru");
  const [providerID, setProviderID] = useState("");
  const [happNoLimitMode, setHappNoLimitMode] = useState(false);
  const [happNoLimitModeXHTTPOnly, setHappNoLimitModeXHTTPOnly] = useState(false);
  const [happMandatoryHWID, setHappMandatoryHWID] = useState(false);
  const [happNotifyExpiration, setHappNotifyExpiration] = useState(false);
  const [happHideServerSettings, setHappHideServerSettings] = useState(false);
  const [happSubscriptionBody, setHappSubscriptionBody] = useState("");

  const [initialState, setInitialState] = useState({
    title: "",
    refreshHours: "12",
    extraURL: "",
    extraStatus: "",
    timeZone: "Europe/Moscow",
    language: "ru",
    providerID: "",
    happNoLimitMode: false,
    happNoLimitModeXHTTPOnly: false,
    happMandatoryHWID: false,
    happNotifyExpiration: false,
    happHideServerSettings: false,
    happSubscriptionBody: "",
  });

  const markFieldChange = () => {
    syncSourceRef.current = "fields";
  };

  const applyParsedBodyToSettings = (nextBody: string) => {
    const parsed = parseSubscriptionBody(nextBody);
    setTitle(parsed.title);
    setRefreshHours(parsed.refreshHours || "12");
    setExtraURL(parsed.extraURL);
    setExtraStatus(parsed.extraStatus);
    setProviderID(parsed.providerID);
    setHappNoLimitMode(parsed.happNoLimitMode);
    setHappNoLimitModeXHTTPOnly(parsed.happNoLimitModeXHTTPOnly);
    setHappMandatoryHWID(parsed.happMandatoryHWID);
    setHappNotifyExpiration(parsed.happNotifyExpiration);
    setHappHideServerSettings(parsed.happHideServerSettings);
  };

  const handleBodyChange = (nextBody: string) => {
    syncSourceRef.current = "body";
    setHappSubscriptionBody(nextBody);
    applyParsedBodyToSettings(nextBody);
  };

  useEffect(() => {
    if (!open) return;

    const load = async () => {
      setLoading(true);
      try {
        const data = await subscriptionSettingsApi.get();
        const next = {
          title: data.title || "",
          refreshHours: String(data.refresh_hours || 12),
          extraURL: data.extra_url || "",
          extraStatus: data.extra_status || "",
          timeZone: data.time_zone || "Europe/Moscow",
          language: data.language || "ru",
          providerID: data.provider_id || "",
          happNoLimitMode: Boolean(data.happ_no_limit_mode),
          happNoLimitModeXHTTPOnly: Boolean(data.happ_no_limit_mode_xhttp_only),
          happMandatoryHWID: Boolean(data.happ_mandatory_hwid),
          happNotifyExpiration: Boolean(data.happ_notify_expiration),
          happHideServerSettings: Boolean(data.happ_hide_server_settings),
          happSubscriptionBody: data.happ_subscription_body || "",
        };
        const normalizedTimeZone = next.timeZone.trim() || "Europe/Moscow";
        const hasPresetTimeZone = POPULAR_TIMEZONES.includes(normalizedTimeZone);
        const generatedBody = buildSuggestedSubscriptionBody(next);
        const effectiveBody =
          next.happSubscriptionBody.trim() === "" || hasLegacySubscriptionBodyMarkers(next.happSubscriptionBody)
            ? generatedBody
            : next.happSubscriptionBody;
        syncSourceRef.current = "init";
        setTitle(next.title);
        setRefreshHours(next.refreshHours);
        setExtraURL(next.extraURL);
        setExtraStatus(next.extraStatus);
        setTimeZoneChoice(hasPresetTimeZone ? normalizedTimeZone : CUSTOM_TIMEZONE_VALUE);
        setCustomTimeZone(hasPresetTimeZone ? "" : normalizedTimeZone);
        setLanguage(next.language);
        setProviderID(next.providerID);
        setHappNoLimitMode(next.happNoLimitMode);
        setHappNoLimitModeXHTTPOnly(next.happNoLimitModeXHTTPOnly);
        setHappMandatoryHWID(next.happMandatoryHWID);
        setHappNotifyExpiration(next.happNotifyExpiration);
        setHappHideServerSettings(next.happHideServerSettings);
        setHappSubscriptionBody(effectiveBody);
        setInitialState({ ...next, happSubscriptionBody: effectiveBody });
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

  const suggestedSubscriptionBody = useMemo(
    () =>
      buildSuggestedSubscriptionBody({
        title,
        refreshHours,
        extraURL,
        extraStatus,
        providerID,
        happNoLimitMode,
        happNoLimitModeXHTTPOnly,
        happMandatoryHWID,
        happNotifyExpiration,
        happHideServerSettings,
      }),
    [
      title,
      refreshHours,
      extraURL,
      extraStatus,
      providerID,
      happNoLimitMode,
      happNoLimitModeXHTTPOnly,
      happMandatoryHWID,
      happNotifyExpiration,
      happHideServerSettings,
    ]
  );

  const hasProviderID = providerID.trim() !== "";

  useEffect(() => {
    if (syncSourceRef.current === "body") {
      return;
    }
    setHappSubscriptionBody((prev) => (prev === suggestedSubscriptionBody ? prev : suggestedSubscriptionBody));
  }, [suggestedSubscriptionBody]);

  const hasChanges = useMemo(() => {
    return (
      title !== initialState.title ||
      refreshHours !== initialState.refreshHours ||
      extraURL !== initialState.extraURL ||
      extraStatus !== initialState.extraStatus ||
      effectiveTimeZone !== initialState.timeZone ||
      language !== initialState.language ||
      providerID !== initialState.providerID ||
      happNoLimitMode !== initialState.happNoLimitMode ||
      happNoLimitModeXHTTPOnly !== initialState.happNoLimitModeXHTTPOnly ||
      happMandatoryHWID !== initialState.happMandatoryHWID ||
      happNotifyExpiration !== initialState.happNotifyExpiration ||
      happHideServerSettings !== initialState.happHideServerSettings ||
      happSubscriptionBody !== initialState.happSubscriptionBody
    );
  }, [
    title,
    refreshHours,
    extraURL,
    extraStatus,
    effectiveTimeZone,
    language,
    providerID,
    happNoLimitMode,
    happNoLimitModeXHTTPOnly,
    happMandatoryHWID,
    happNotifyExpiration,
    happHideServerSettings,
    happSubscriptionBody,
    initialState,
  ]);

  const canSubmit =
    initialLoaded &&
    hasChanges &&
    !(timeZoneChoice === CUSTOM_TIMEZONE_VALUE && effectiveTimeZone === "");

  const handleChangeNoLimitMode = (next: boolean) => {
    setHappNoLimitMode(next);
    if (next) {
      setHappNoLimitModeXHTTPOnly(false);
    }
  };

  const handleChangeNoLimitModeXHTTPOnly = (next: boolean) => {
    setHappNoLimitModeXHTTPOnly(next);
    if (next) {
      setHappNoLimitMode(false);
    }
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!hasChanges) return;

    setLoading(true);
    try {
      await subscriptionSettingsApi.update({
        title,
        refresh_hours: parsedRefreshHours,
        info_url: "",
        extra_url: extraURL,
        extra_status: extraStatus,
        time_zone: effectiveTimeZone,
        language,
        provider_id: providerID.trim(),
        happ_no_limit_mode: hasProviderID ? happNoLimitMode : false,
        happ_no_limit_mode_xhttp_only: hasProviderID ? happNoLimitModeXHTTPOnly : false,
        happ_mandatory_hwid: hasProviderID ? happMandatoryHWID : false,
        happ_notify_expiration: hasProviderID ? happNotifyExpiration : false,
        happ_hide_server_settings: hasProviderID ? happHideServerSettings : false,
        happ_subscription_body: happSubscriptionBody.trim() || suggestedSubscriptionBody,
      });
      toast("Общие настройки подписки обновлены", "success");
      setInitialState({
        title,
        refreshHours,
        extraURL,
        extraStatus,
        timeZone: effectiveTimeZone,
        language,
        providerID,
        happNoLimitMode: hasProviderID ? happNoLimitMode : false,
        happNoLimitModeXHTTPOnly: hasProviderID ? happNoLimitModeXHTTPOnly : false,
        happMandatoryHWID: hasProviderID ? happMandatoryHWID : false,
        happNotifyExpiration: hasProviderID ? happNotifyExpiration : false,
        happHideServerSettings: hasProviderID ? happHideServerSettings : false,
        happSubscriptionBody: happSubscriptionBody.trim() || suggestedSubscriptionBody,
      });
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
      className="w-[96vw] max-w-[96rem] max-h-[92vh] overflow-y-auto"
    >
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_minmax(26rem,1fr)]">
          <div className="grid content-start gap-3 rounded-xl border border-border bg-surface-2/20 p-4">
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
            />
            <div className="-mt-1">
              <EmojiPickerButton
                inline
                onSelect={(emoji) => {
                  markFieldChange();
                  extraStatusInputRef.current?.insertEmoji(emoji);
                }}
              />
            </div>
            <div className="border-t border-border" />
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
              <div className="bg-surface-2 border border-border rounded-lg px-3 py-2 text-sm text-zinc-500 cursor-not-allowed">
                Недоступно
              </div>
            </div>
          </div>

          <div className="grid content-start gap-4 rounded-xl border border-border bg-surface-2/20 p-4">
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
              placeholder="Например, happ-provider-main"
            />
            {!hasProviderID && (
              <div className="rounded-lg border border-border bg-surface-2/50 px-3 py-2 text-xs text-zinc-500">
                Пока Provider ID не заполнен, Happ-тогглы ниже неактивны и не будут применяться к подписке.
              </div>
            )}
            <div className="grid gap-2.5">
              <HappModeGroup
                noLimitMode={happNoLimitMode}
                noLimitModeXHTTPOnly={happNoLimitModeXHTTPOnly}
                onChangeNoLimitMode={(next) => {
                  markFieldChange();
                  handleChangeNoLimitMode(next);
                }}
                onChangeNoLimitModeXHTTPOnly={(next) => {
                  markFieldChange();
                  handleChangeNoLimitModeXHTTPOnly(next);
                }}
                disabled={!hasProviderID}
              />
              <HappToggle
                label="Неотключаемый HWID"
                checked={happMandatoryHWID}
                onChange={(next) => {
                  markFieldChange();
                  setHappMandatoryHWID(next);
                }}
                disabled={!hasProviderID}
              />
              <HappToggle
                label="Уведомление об окончании подписки"
                checked={happNotifyExpiration}
                onChange={(next) => {
                  markFieldChange();
                  setHappNotifyExpiration(next);
                }}
                disabled={!hasProviderID}
              />
              <HappToggle
                label="Скрыть настройки сервера в подписке"
                checked={happHideServerSettings}
                onChange={(next) => {
                  markFieldChange();
                  setHappHideServerSettings(next);
                }}
                disabled={!hasProviderID}
              />
              <div className="grid gap-1.5 rounded-xl border border-border bg-surface-2 p-3">
                <label htmlFor="happ-subscription-body" className="text-sm font-medium text-zinc-200">
                  Редактор тела подписки
                </label>
                <textarea
                  id="happ-subscription-body"
                  value={happSubscriptionBody}
                  onChange={(e) => handleBodyChange(e.target.value)}
                  placeholder="Здесь можно вручную задать тело подписки для Happ"
                  className="min-h-32 rounded-lg border border-border bg-surface-1 px-3 py-2 text-sm text-zinc-200 placeholder:text-zinc-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg"
                />
                <div className="text-right text-xs text-zinc-500">
                  {happSubscriptionBody.length}/10000
                </div>
              </div>
            </div>
          </div>
        </div>
        <Button type="submit" loading={loading} disabled={!canSubmit}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
