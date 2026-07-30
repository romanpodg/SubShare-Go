"use client";

import React, { useEffect, useRef, useState, useMemo } from "react";
import type { User, VLESSKey } from "@/lib/types";
import { users as usersApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { Select } from "@/components/ui/Select";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { AddUserModal } from "./AddUserModal";
import { BulkEditUsersModal } from "./BulkEditUsersModal";
import { EditSubscriptionModal } from "./EditSubscriptionModal";
import { KeyAssignerModal } from "./KeyAssignerModal";
import { HwidManager } from "./HwidManager";
import { copyToClipboard } from "@/lib/clipboard";
import { EmojiText } from "@/components/ui/EmojiText";
import { ChevronDown, ChevronUp, Copy, Edit, Fingerprint, Key, Plus, Search, UsersRound, X } from "lucide-react";

interface Props {
  users: User[];
  assignableKeys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

const formatDateTime = (value: string | null) => {
  if (!value) {
    return null;
  }

  const isoMatch = value.match(/^(\d{4}-\d{2}-\d{2})(?:[T\s](\d{2}:\d{2})(?::\d{2})?)?/);
  if (isoMatch) {
    const parsed = new Date(value.replace(/-/g, "/"));
    if (!Number.isNaN(parsed.getTime())) {
      return parsed
        .toLocaleString("en-GB", {
          day: "2-digit",
          month: "2-digit",
          year: "numeric",
          hour: isoMatch[2] ? "2-digit" : undefined,
          minute: isoMatch[2] ? "2-digit" : undefined,
        })
        .replace(",", "");
    }
  }

  const slashMatch = value.match(/^(\d{2})\/(\d{2})\/(\d{4})\s+(\d{2}):(\d{2})$/);
  if (slashMatch) {
    return `${slashMatch[1]}/${slashMatch[2]}/${slashMatch[3]} ${slashMatch[4]}:${slashMatch[5]}`;
  }

  return value;
};

export function UsersSection({ users, assignableKeys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [showAddUser, setShowAddUser] = useState(false);
  const [showBulkEdit, setShowBulkEdit] = useState(false);
  const [editSubUser, setEditSubUser] = useState<User | null>(null);
  const [editKeysUser, setEditKeysUser] = useState<User | null>(null);
  const [hwidUser, setHwidUser] = useState<User | null>(null);
  
  // Search, filtering, and row expansion states
  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "non-active" | "paused" | "blocked">("all");
  const [expandedUserIDs, setExpandedUserIDs] = useState<number[]>([]);
  
  const [selectedUserIDs, setSelectedUserIDs] = useState<number[]>([]);
  const [deleteTargetIDs, setDeleteTargetIDs] = useState<number[] | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [copyingEncryptedFor, setCopyingEncryptedFor] = useState<number | null>(null);
  const selectAllRef = useRef<HTMLInputElement | null>(null);

  // Filter users based on query and status filter
  const filteredUsers = useMemo(() => {
    return users.filter((user) => {
      const query = searchQuery.toLowerCase().trim();
      if (query) {
        const nameMatch = user.name?.toLowerCase().includes(query);
        const tgMatch = user.email?.toLowerCase().includes(query);
        const codeMatch = user.activation_code?.toLowerCase().includes(query);
        const tokenMatch = user.subscription_id?.toLowerCase().includes(query);
        if (!nameMatch && !tgMatch && !codeMatch && !tokenMatch) {
          return false;
        }
      }
      if (statusFilter !== "all") {
        if (user.status !== statusFilter) {
          return false;
        }
      }
      return true;
    });
  }, [users, searchQuery, statusFilter]);

  const selectedCount = selectedUserIDs.length;
  const filteredCount = filteredUsers.length;
  const allSelected = filteredCount > 0 && filteredUsers.every((user) => selectedUserIDs.includes(user.id));
  const partiallySelected = selectedCount > 0 && !allSelected;

  useEffect(() => {
    const syncSelectedUser = (selected: User | null, setSelected: (value: User | null) => void) => {
      if (!selected) {
        return;
      }
      const fresh = users.find((item) => item.id === selected.id);
      if (!fresh) {
        setSelected(null);
        return;
      }
      if (fresh !== selected) {
        setSelected(fresh);
      }
    };

    syncSelectedUser(editSubUser, setEditSubUser);
    syncSelectedUser(editKeysUser, setEditKeysUser);
    syncSelectedUser(hwidUser, setHwidUser);
  }, [users, editSubUser, editKeysUser, hwidUser]);

  useEffect(() => {
    setSelectedUserIDs((prev) => prev.filter((id) => users.some((user) => user.id === id)));
  }, [users]);

  useEffect(() => {
    if (!selectAllRef.current) {
      return;
    }
    selectAllRef.current.indeterminate = selectedCount > 0 && !allSelected;
  }, [selectedCount, allSelected]);

  const toggleUserSelection = (id: number) => {
    setSelectedUserIDs((prev) => (prev.includes(id) ? prev.filter((value) => value !== id) : [...prev, id]));
  };

  const toggleSelectAllUsers = () => {
    if (allSelected) {
      setSelectedUserIDs((prev) => prev.filter((id) => !filteredUsers.some((user) => user.id === id)));
    } else {
      setSelectedUserIDs((prev) => {
        const next = [...prev];
        filteredUsers.forEach((user) => {
          if (!next.includes(user.id)) {
            next.push(user.id);
          }
        });
        return next;
      });
    }
  };

  const toggleUserExpanded = (userId: number) => {
    setExpandedUserIDs((prev) =>
      prev.includes(userId) ? prev.filter((id) => id !== userId) : [...prev, userId]
    );
  };

  const openDeleteSelectedDialog = () => {
    if (selectedUserIDs.length === 0) {
      return;
    }
    setDeleteTargetIDs([...selectedUserIDs]);
  };

  const confirmDelete = async () => {
    if (!deleteTargetIDs || deleteTargetIDs.length === 0) return;
    setDeleting(true);
    try {
      const results = await Promise.allSettled(deleteTargetIDs.map((id) => usersApi.delete(id)));
      const failedIDs = results
        .map((result, index) => (result.status === "rejected" ? deleteTargetIDs[index] : null))
        .filter((id): id is number => id !== null);
      const successCount = deleteTargetIDs.length - failedIDs.length;

      if (successCount > 0 && failedIDs.length === 0) {
        toast(successCount === 1 ? "Пользователь удален" : `Удалено пользователей: ${successCount}`, "success");
      } else if (successCount > 0) {
        toast(`Удалено ${successCount} из ${deleteTargetIDs.length}. Проверьте оставшихся пользователей.`, "error");
      } else {
        toast("Не удалось удалить выбранных пользователей", "error");
      }

      setSelectedUserIDs(failedIDs);
      await onRefresh();
    } catch {
      toast("Не удалось удалить выбранных пользователей", "error");
    } finally {
      setDeleting(false);
      setDeleteTargetIDs(null);
    }
  };

  const resolveSubscriptionURL = (subscriptionID: string) => {
    if (!subscriptionID) return "";
    if (typeof window === "undefined") return `/sub/${subscriptionID}`;
    return `${window.location.origin}/sub/${subscriptionID}`;
  };

  const copyText = async (value: string, successMessage: string) => {
    try {
      const copied = await copyToClipboard(value);
      if (copied) {
        toast(successMessage, "success");
        return;
      }

      if (typeof window !== "undefined") {
        window.prompt("Скопируйте ссылку вручную:", value);
        toast("Буфер обмена недоступен: ссылка показана для ручного копирования", "info");
        return;
      }

      toast("Не удалось скопировать ссылку", "error");
    } catch {
      if (typeof window !== "undefined") {
        window.prompt("Скопируйте ссылку вручную:", value);
        toast("Буфер обмена недоступен: ссылка показана для ручного копирования", "info");
        return;
      }

      toast("Не удалось скопировать ссылку", "error");
    }
  };

  const handleCopySubscriptionURL = async (user: User) => {
    if (!user.subscription_id) {
      toast("У пользователя нет ссылки подписки", "error");
      return;
    }
    await copyText(resolveSubscriptionURL(user.subscription_id), "URL подписки скопирован");
  };

  const handleCopyEncryptedSubscriptionURL = async (user: User) => {
    if (!user.subscription_id) {
      toast("У пользователя нет ссылки подписки", "error");
      return;
    }

    setCopyingEncryptedFor(user.id);
    try {
      const result = await usersApi.getSubscriptionURLs(user.id);
      if (!result.encrypted_url) {
        toast("Зашифрованная ссылка недоступна: проверьте настройку HAPP API", "error");
        return;
      }
      await copyText(result.encrypted_url, "URL зашифрованной подписки скопирован");
    } catch (error: unknown) {
      toast(error instanceof Error ? error.message : "Не удалось получить зашифрованную ссылку", "error");
    } finally {
      setCopyingEncryptedFor(null);
    }
  };

  const deleteTargetUsers = deleteTargetIDs
    ? users.filter((user) => deleteTargetIDs.includes(user.id))
    : [];
  const selectedUsers = users.filter((user) => selectedUserIDs.includes(user.id));
  const deleteTargetNames = deleteTargetUsers.map((user) => user.name);
  const deleteTargetLabel = (() => {
    if (!deleteTargetIDs || deleteTargetIDs.length === 0) {
      return "";
    }
    if (deleteTargetIDs.length === 1) {
      return `Вы уверены, что хотите удалить пользователя "${deleteTargetNames[0] ?? "—"}"?`;
    }

    const preview = deleteTargetNames.slice(0, 3).map((name) => `"${name}"`).join(", ");
    const extraCount = deleteTargetIDs.length - Math.min(deleteTargetNames.length, 3);
    const extraSuffix = extraCount > 0 ? ` и еще ${extraCount}` : "";
    if (!preview) {
      return `Вы уверены, что хотите удалить ${deleteTargetIDs.length} пользователей?`;
    }
    return `Вы уверены, что хотите удалить ${deleteTargetIDs.length} пользователей (${preview}${extraSuffix})?`;
  })();

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="h-8 w-8 flex items-center justify-center rounded-lg border border-border bg-surface-2 transition-all hover:bg-surface-1"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Развернуть" : "Свернуть"}
          >
            <ChevronDown className={`h-4 w-4 text-zinc-400 transition-transform ${collapsed ? "-rotate-90" : ""}`} />
          </button>
          <h2 className="flex items-center gap-2 text-lg font-semibold">
            <UsersRound className="h-4 w-4 text-accent" aria-hidden="true" />
            <span>Пользователи ({users.length})</span>
          </h2>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={() => setShowBulkEdit(true)} className="text-xs" disabled={selectedCount === 0}>
            Изменить
          </Button>
          {selectedCount > 0 && (
            <Button variant="danger" onClick={openDeleteSelectedDialog} className="text-xs">
              Удалить ({selectedCount})
            </Button>
          )}
          <Button onClick={() => setShowAddUser(true)} className="text-xs">
            <Plus className="h-3.5 w-3.5" aria-hidden="true" />
            <span>Добавить</span>
          </Button>
        </div>
      </div>

      {/* Панель поиска и фильтров */}
      {!collapsed && (
        <div className="flex flex-col md:flex-row gap-3 mb-4 p-3 bg-surface-2/30 rounded-xl border border-border">
          <div className="flex-1 relative flex items-center">
            <Search className="w-4 h-4 text-zinc-500 absolute left-3 pointer-events-none" />
            <input
              type="text"
              placeholder="Поиск по имени, Telegram, коду..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full h-9 bg-surface-2 border border-border rounded-lg pl-9 pr-8 text-sm focus:outline-none focus:border-accent text-zinc-100 placeholder-zinc-500 transition-colors"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery("")}
                aria-label="Очистить поиск"
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 text-xs w-5 h-5 flex items-center justify-center rounded-full hover:bg-surface-1"
              >
                <X className="h-3.5 w-3.5" aria-hidden="true" />
              </button>
            )}
          </div>
          <div className="flex gap-2">
            <Select
              ariaLabel="Фильтр по статусу"
              value={statusFilter}
              onChange={(e) =>
                setStatusFilter(
                  e.target.value as "all" | "active" | "non-active" | "paused" | "blocked"
                )
              }
              className="w-48 min-w-48"
              options={[
                { value: "all", label: "Все статусы" },
                { value: "active", label: "Активен" },
                { value: "non-active", label: "Неактивен" },
                { value: "paused", label: "Приостановлен" },
                { value: "blocked", label: "Заблокирован" },
              ]}
            />
          </div>
        </div>
      )}

      {!collapsed && (
        <div className="overflow-x-auto">
          <table className="w-full table-fixed text-sm">
            <caption className="sr-only">Список пользователей</caption>
            <colgroup>
              <col style={{ width: "2.5rem" }} />
              <col style={{ width: "12rem" }} />
              <col style={{ width: "12rem" }} />
              <col style={{ width: "8rem" }} />
              <col style={{ width: "10rem" }} />
              <col style={{ width: "16rem" }} />
            </colgroup>
            <thead>
              <tr className="text-left text-zinc-400 border-b border-border">
                <th scope="col" className="pb-2 pr-2">
                  <label className="flex h-9 w-9 cursor-pointer items-center justify-center rounded-xl border border-transparent transition-colors hover:border-border/70 hover:bg-surface-2/60">
                    <input
                      ref={selectAllRef}
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAllUsers}
                      aria-label={allSelected ? "Снять выбор со всех пользователей" : "Выбрать всех пользователей"}
                      className="peer sr-only"
                    />
                    <span
                      className={`flex h-5 w-5 items-center justify-center rounded-md border shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] transition-all ${
                        allSelected || partiallySelected
                          ? "border-accent/80 bg-accent/20 text-accent"
                          : "border-border bg-surface-2 text-transparent"
                      } peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface-1`}
                    >
                      {partiallySelected ? (
                        <span className="h-0.5 w-2.5 rounded-full bg-current" />
                      ) : (
                        <svg
                          viewBox="0 0 16 16"
                          aria-hidden="true"
                          className={`h-3.5 w-3.5 transition-opacity ${allSelected ? "opacity-100" : "opacity-0"}`}
                        >
                          <path
                            d="M4 8.25 6.5 10.75 12 5.25"
                            fill="none"
                            stroke="currentColor"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth="2"
                          />
                        </svg>
                      )}
                    </span>
                  </label>
                </th>
                <th scope="col" className="pb-2 pr-4">Имя</th>
                <th scope="col" className="pb-2 pr-4">Telegram</th>
                <th scope="col" className="pb-2 pr-4">Статус</th>
                <th scope="col" className="pb-2 pr-4">Подписка</th>
                <th scope="col" className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {filteredUsers.map((user) => {
                const isSelected = selectedUserIDs.includes(user.id);
                const isExpanded = expandedUserIDs.includes(user.id);

                return (
                  <React.Fragment key={user.id}>
                    <tr className={`border-b border-border/50 hover:bg-surface-2/10 transition-colors ${isExpanded ? "bg-surface-2/10 border-b-transparent" : "last:border-0"}`}>
                      <td className="py-3 pr-2 align-middle">
                        <label className="flex h-10 w-full cursor-pointer items-center justify-center rounded-xl border border-transparent transition-colors hover:border-border/70 hover:bg-surface-2/40">
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={() => toggleUserSelection(user.id)}
                            aria-label={`Выбрать пользователя ${user.name}`}
                            className="peer sr-only"
                          />
                          <span
                            className={`flex h-5 w-5 items-center justify-center rounded-md border shadow-[inset_0_1px_0_rgba(255,255,255,0.04)] transition-all peer-focus-visible:ring-2 peer-focus-visible:ring-accent peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface-1 ${
                              isSelected
                                ? "border-accent/80 bg-accent/20 text-accent"
                                : "border-border bg-surface-2 text-transparent"
                            }`}
                          >
                            <svg
                              viewBox="0 0 16 16"
                              aria-hidden="true"
                              className={`h-3.5 w-3.5 transition-opacity ${isSelected ? "opacity-100" : "opacity-0"}`}
                            >
                              <path
                                d="M4 8.25 6.5 10.75 12 5.25"
                                fill="none"
                                stroke="currentColor"
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth="2"
                              />
                            </svg>
                          </span>
                        </label>
                      </td>
                      <td className="py-3 pr-4 font-medium text-zinc-100"><EmojiText text={user.name} /></td>
                      <td className="py-3 pr-4 text-zinc-400 font-mono text-xs">{user.email ? `@${user.email.replace(/^@+/, "")}` : "—"}</td>
                      <td className="py-3 pr-4"><StatusBadge status={user.status} /></td>
                      <td className="py-3 pr-4 align-middle">
                        {user.subscription_id ? (
                          <button
                            onClick={() => void handleCopySubscriptionURL(user)}
                            className="inline-flex items-center gap-1.5 h-8 px-2.5 rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/50 text-xs transition cursor-pointer font-medium text-zinc-200"
                            title="Скопировать ссылку подписки"
                          >
                            <Copy className="w-3.5 h-3.5 opacity-70" />
                            <span>Копировать URL</span>
                          </button>
                        ) : (
                          <span className="text-zinc-500 text-xs">Нет ссылки</span>
                        )}
                      </td>
                      <td className="py-3 align-middle">
                        <div className="flex items-center gap-1.5">
                          <button
                            onClick={() => setEditSubUser(user)}
                            className="h-8 px-2.5 flex items-center gap-1.5 rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/40 text-xs transition cursor-pointer font-medium text-zinc-200"
                            title="Редактировать пользователя"
                          >
                            <Edit className="w-3.5 h-3.5 text-zinc-400" />
                            <span>Изменить</span>
                          </button>
                          <button
                            onClick={() => setEditKeysUser(user)}
                            className="h-8 px-2.5 flex items-center gap-1.5 rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/40 text-xs transition cursor-pointer font-medium text-zinc-200"
                            title="Назначить ключи"
                          >
                            <Key className="w-3.5 h-3.5 text-zinc-400" />
                            <span>Ключи</span>
                          </button>
                          <button
                            onClick={() => setHwidUser(user)}
                            className="h-8 px-2.5 flex items-center gap-1.5 rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/40 text-xs transition cursor-pointer font-medium text-zinc-200"
                            title="Управление HWID"
                          >
                            <Fingerprint className="w-3.5 h-3.5 text-zinc-400" />
                            <span>HWID</span>
                          </button>
                          <button
                            onClick={() => toggleUserExpanded(user.id)}
                            className="h-8 w-8 flex items-center justify-center rounded-lg border border-border bg-surface-2 hover:bg-surface-1 hover:border-accent/40 transition cursor-pointer"
                            title={isExpanded ? "Скрыть детали" : "Показать детали"}
                          >
                            {isExpanded ? (
                              <ChevronUp className="w-4 h-4 text-zinc-400" />
                            ) : (
                              <ChevronDown className="w-4 h-4 text-zinc-400" />
                            )}
                          </button>
                        </div>
                      </td>
                    </tr>

                    {/* Разворачиваемая подстрока с подробной информацией */}
                    {isExpanded && (
                      <tr className="bg-surface-2/20 border-b border-border/50">
                        <td />
                        <td colSpan={5} className="py-4 px-6 text-xs text-zinc-400">
                          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                            {/* Активация */}
                            <div className="flex flex-col gap-1.5 bg-surface-1/50 p-3.5 rounded-xl border border-border/50">
                              <span className="font-semibold text-zinc-300 uppercase tracking-wider text-[10px]">Активация</span>
                              <div className="flex justify-between border-b border-border/30 pb-1 mt-1">
                                <span>Код:</span>
                                <span className="font-mono text-zinc-200 font-semibold">{user.activation_code}</span>
                              </div>
                              <div className="flex justify-between border-b border-border/30 pb-1">
                                <span>Активирован:</span>
                                <span className="text-zinc-200">{formatDateTime(user.activation_used_at) ?? "—"}</span>
                              </div>
                              <div className="flex flex-col gap-1 mt-1">
                                <span>Токен подписки:</span>
                                <span className="font-mono text-zinc-300 break-all bg-surface-2 p-1.5 rounded border border-border/40 text-[10px] select-all">{user.subscription_id || "—"}</span>
                              </div>
                            </div>

                            {/* Сроки подписки */}
                            <div className="flex flex-col gap-1.5 bg-surface-1/50 p-3.5 rounded-xl border border-border/50 justify-between">
                              <div className="flex flex-col gap-1.5">
                                <span className="font-semibold text-zinc-300 uppercase tracking-wider text-[10px]">Сроки подписки</span>
                                <div className="flex justify-between border-b border-border/30 pb-1 mt-1">
                                  <span>Начало:</span>
                                  <span className="text-zinc-200 font-medium">{user.starts_at || "—"}</span>
                                </div>
                                <div className="flex justify-between border-b border-border/30 pb-1">
                                  <span>Истекает:</span>
                                  <span className="text-zinc-200 font-medium">{user.expires_at || "—"}</span>
                                </div>
                              </div>
                            </div>

                            {/* Зашифрованная ссылка */}
                            <div className="flex flex-col gap-1.5 bg-surface-1/50 p-3.5 rounded-xl border border-border/50 justify-between">
                              <div className="flex flex-col gap-1">
                                <span className="font-semibold text-zinc-300 uppercase tracking-wider text-[10px]">Зашифрованная ссылка</span>
                                <p className="text-[11px] text-zinc-500 mt-1">
                                  Используется для клиентов с поддержкой HAPP API (шифрование ссылок).
                                </p>
                              </div>
                              <Button
                                variant="ghost"
                                className="w-full text-xs mt-3 h-8 border border-border hover:bg-surface-1"
                                onClick={() => void handleCopyEncryptedSubscriptionURL(user)}
                                loading={copyingEncryptedFor === user.id}
                                disabled={!user.subscription_id || (copyingEncryptedFor !== null && copyingEncryptedFor !== user.id)}
                              >
                                Скопировать зашифрованный URL
                              </Button>
                            </div>
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })}
              {filteredUsers.length === 0 && (
                <tr>
                  <td colSpan={6} className="py-8 text-center text-zinc-400">
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="mx-auto mb-2 h-8 w-8 text-zinc-500"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                      strokeWidth={1.5}
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z"
                      />
                    </svg>
                    Пользователи не найдены
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTargetIDs}
        title={deleteTargetIDs && deleteTargetIDs.length > 1 ? "Удаление пользователей" : "Удаление пользователя"}
        message={deleteTargetLabel}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTargetIDs(null)}
        loading={deleting}
      />

      <AddUserModal open={showAddUser} onClose={() => setShowAddUser(false)} onRefresh={onRefresh} />
      {showBulkEdit && selectedUsers.length > 0 && (
        <BulkEditUsersModal
          users={selectedUsers}
          onClose={() => setShowBulkEdit(false)}
          onRefresh={onRefresh}
        />
      )}
      {editSubUser && (
        <EditSubscriptionModal
          user={editSubUser}
          onClose={() => setEditSubUser(null)}
          onRefresh={onRefresh}
        />
      )}
      {editKeysUser && (
        <KeyAssignerModal
          user={editKeysUser}
          assignableKeys={assignableKeys}
          onClose={() => setEditKeysUser(null)}
          onRefresh={onRefresh}
        />
      )}
      {hwidUser && (
        <HwidManager user={hwidUser} onClose={() => setHwidUser(null)} onRefresh={onRefresh} />
      )}
    </Card>
  );
}
