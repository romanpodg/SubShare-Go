"use client";

import { useState } from "react";
import type { User, VLESSKey } from "@/lib/types";
import { users as usersApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { StatusBadge } from "@/components/ui/StatusBadge";
import { AddUserModal } from "./AddUserModal";
import { EditSubscriptionModal } from "./EditSubscriptionModal";
import { KeyAssignerModal } from "./KeyAssignerModal";
import { HwidManager } from "./HwidManager";

interface Props {
  users: User[];
  assignableKeys: VLESSKey[];
  onRefresh: () => Promise<void>;
}

export function UsersSection({ users, assignableKeys, onRefresh }: Props) {
  const { toast } = useToast();
  const [collapsed, setCollapsed] = useState(false);
  const [showAddUser, setShowAddUser] = useState(false);
  const [editSubUser, setEditSubUser] = useState<User | null>(null);
  const [editKeysUser, setEditKeysUser] = useState<User | null>(null);
  const [hwidUser, setHwidUser] = useState<User | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{id: number, name: string} | null>(null);
  const [deleting, setDeleting] = useState(false);

  const handleDelete = (id: number, name: string) => {
    setDeleteTarget({ id, name });
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await usersApi.delete(deleteTarget.id);
      toast("Пользователь удален", "success");
      await onRefresh();
    } catch {
      toast("Не удалось удалить пользователя", "error");
    } finally {
      setDeleting(false);
      setDeleteTarget(null);
    }
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-center gap-2 text-lg font-semibold"
          aria-expanded={!collapsed}
        >
          <span className={`transition-transform ${collapsed ? "" : "rotate-90"}`}>&#9654;</span>
          Пользователи ({users.length})
        </button>
        <Button onClick={() => setShowAddUser(true)} className="text-xs">
          + Добавить
        </Button>
      </div>

      {!collapsed && (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <caption className="sr-only">Список пользователей</caption>
            <thead>
              <tr className="text-left text-zinc-400 border-b border-border">
                <th scope="col" className="pb-2 pr-4">Имя</th>
                <th scope="col" className="pb-2 pr-4">Имя пользователя Telegram</th>
                <th scope="col" className="pb-2 pr-4">Код активации</th>
                <th scope="col" className="pb-2 pr-4">Статус</th>
                <th scope="col" className="pb-2 pr-4">Подписка</th>
                <th scope="col" className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-b border-border last:border-0">
                  <td className="py-3 pr-4">{user.name}</td>
                  <td className="py-3 pr-4 text-zinc-400">{user.email ? `@${user.email.replace(/^@+/, "")}` : "—"}</td>
                  <td className="py-3 pr-4 font-mono text-xs">{user.activation_code}</td>
                  <td className="py-3 pr-4"><StatusBadge status={user.status} /></td>
                  <td className="py-3 pr-4 text-xs text-zinc-400">
                    {user.starts_at && <div>с {user.starts_at}</div>}
                    {user.expires_at && <div>до {user.expires_at}</div>}
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1.5">
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => setEditKeysUser(user)}
                      >
                        Ключи
                      </Button>
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => setEditSubUser(user)}
                      >
                        Подписка
                      </Button>
                      <Button
                        variant="ghost"
                        className="text-xs"
                        onClick={() => setHwidUser(user)}
                      >
                        HWID
                      </Button>
                      <Button
                        variant="danger"
                        className="text-xs"
                        onClick={() => handleDelete(user.id, user.name)}
                      >
                        Удалить
                      </Button>
                    </div>
                  </td>
                </tr>
              ))}
              {users.length === 0 && (
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
                    Нет пользователей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Удаление пользователя"
        message={`Вы уверены, что хотите удалить пользователя "${deleteTarget?.name}"?`}
        onConfirm={confirmDelete}
        onCancel={() => setDeleteTarget(null)}
        loading={deleting}
      />

      <AddUserModal open={showAddUser} onClose={() => setShowAddUser(false)} onRefresh={onRefresh} />
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
