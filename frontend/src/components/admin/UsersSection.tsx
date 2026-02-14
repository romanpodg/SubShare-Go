"use client";

import { useState } from "react";
import type { User, VLESSKey } from "@/lib/types";
import { users as usersApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
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

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(`Удалить пользователя "${name}"?`)) return;
    try {
      await usersApi.delete(id);
      toast("User deleted", "success");
      await onRefresh();
    } catch {
      toast("Failed to delete user", "error");
    }
  };

  const statusBadge = (status: string) => {
    const colors: Record<string, string> = {
      active: "bg-green-500/10 text-green-400",
      paused: "bg-yellow-500/10 text-yellow-400",
      blocked: "bg-red-500/10 text-red-400",
    };
    return (
      <span
        className={`text-xs px-2 py-0.5 rounded-full ${colors[status] || "bg-zinc-500/10 text-zinc-400"}`}
      >
        {status}
      </span>
    );
  };

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <button
          onClick={() => setCollapsed(!collapsed)}
          className="flex items-center gap-2 text-lg font-semibold"
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
            <thead>
              <tr className="text-left text-zinc-500 border-b border-border">
                <th className="pb-2 pr-4">Имя</th>
                <th className="pb-2 pr-4">Email</th>
                <th className="pb-2 pr-4">Код активации</th>
                <th className="pb-2 pr-4">Статус</th>
                <th className="pb-2 pr-4">Подписка</th>
                <th className="pb-2">Действия</th>
              </tr>
            </thead>
            <tbody>
              {users.map((user) => (
                <tr key={user.id} className="border-b border-border last:border-0">
                  <td className="py-3 pr-4">{user.name}</td>
                  <td className="py-3 pr-4 text-zinc-400">{user.email || "—"}</td>
                  <td className="py-3 pr-4 font-mono text-xs">{user.activation_code}</td>
                  <td className="py-3 pr-4">{statusBadge(user.status)}</td>
                  <td className="py-3 pr-4 text-xs text-zinc-400">
                    {user.starts_at && <div>с {user.starts_at}</div>}
                    {user.expires_at && <div>до {user.expires_at}</div>}
                  </td>
                  <td className="py-3">
                    <div className="flex gap-1">
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
                  <td colSpan={6} className="py-8 text-center text-zinc-500">
                    Нет пользователей
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

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
