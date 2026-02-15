"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Button } from "@/components/ui/Button";
import { useToast } from "@/components/ui/Toast";
import { subscriptionSettings as subscriptionSettingsApi } from "@/lib/api";

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

  const [initialState, setInitialState] = useState({
    title: "",
    refreshHours: "12",
    infoURL: "",
    extraURL: "",
    extraStatus: "",
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
        };
        setTitle(next.title);
        setRefreshHours(next.refreshHours);
        setInfoURL(next.infoURL);
        setExtraURL(next.extraURL);
        setExtraStatus(next.extraStatus);
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

  const hasChanges = useMemo(() => {
    return (
      title !== initialState.title ||
      refreshHours !== initialState.refreshHours ||
      infoURL !== initialState.infoURL ||
      extraURL !== initialState.extraURL ||
      extraStatus !== initialState.extraStatus
    );
  }, [title, refreshHours, infoURL, extraURL, extraStatus, initialState]);

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
      });
      toast("Общие настройки подписки обновлены", "success");
      setInitialState({
        title,
        refreshHours,
        infoURL,
        extraURL,
        extraStatus,
      });
      onClose();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить настройки", "error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal open={open} onClose={onClose} title="Общий редактор подписки">
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
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
        <Input
          label="Описание/доп. статус"
          value={extraStatus}
          onChange={(e) => setExtraStatus(e.target.value)}
          placeholder="✅ Active, истекает через 85 дн"
        />
        <Button type="submit" loading={loading} disabled={!initialLoaded || !hasChanges}>
          Сохранить
        </Button>
      </form>
    </Modal>
  );
}
