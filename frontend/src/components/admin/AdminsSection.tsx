"use client";

import React, { useState, useEffect, useCallback } from "react";
import type { Admin } from "@/lib/types";
import { admins as adminsApi } from "@/lib/api";
import { useToast } from "@/components/ui/Toast";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ConfirmDialog } from "@/components/ui/ConfirmDialog";
import { EmojiText } from "@/components/ui/EmojiText";
import { Modal } from "@/components/ui/Modal";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { PasswordInput } from "@/components/ui/PasswordInput";
import { Edit, Trash2, Plus, ShieldAlert, KeyRound, UserPlus } from "lucide-react";

interface Props {
  currentRole: string | null;
}

export function AdminsSection({ currentRole }: Props) {
  const { toast } = useToast();
  const [adminsList, setAdminsList] = useState<Admin[]>([]);
  const [loading, setLoading] = useState(true);
  const [collapsed, setCollapsed] = useState(false);

  // Modals state
  const [showAddModal, setShowAddModal] = useState(false);
  const [editingAdmin, setEditingAdmin] = useState<Admin | null>(null);
  const [deletingAdmin, setDeletingAdmin] = useState<Admin | null>(null);

  // Form states for creating
  const [newUsername, setNewUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [newRole, setNewRole] = useState("operator");
  const [submitLoading, setSubmitLoading] = useState(false);

  // Form states for editing
  const [editPassword, setEditPassword] = useState("");
  const [editRole, setEditRole] = useState("");

  const fetchAdmins = useCallback(async () => {
    try {
      const res = await adminsApi.list();
      setAdminsList(res.admins);
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось загрузить список администраторов", "error");
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    if (currentRole === "owner" || currentRole === "super_admin") {
      fetchAdmins();
    }
  }, [currentRole, fetchAdmins]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newUsername.trim() || newPassword.length < 6) {
      toast("Заполните все поля (пароль от 6 символов)", "error");
      return;
    }
    setSubmitLoading(true);
    try {
      await adminsApi.create({
        username: newUsername.trim(),
        password: newPassword,
        role: newRole,
      });
      toast("Администратор создан", "success");
      setShowAddModal(false);
      setNewUsername("");
      setNewPassword("");
      setNewRole("operator");
      await fetchAdmins();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось создать администратора", "error");
    } finally {
      setSubmitLoading(false);
    }
  };

  const handleUpdate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!editingAdmin) return;
    if (editPassword && editPassword.length < 6) {
      toast("Пароль должен быть от 6 символов", "error");
      return;
    }
    setSubmitLoading(true);
    try {
      await adminsApi.update(editingAdmin.id, {
        password: editPassword || undefined,
        role: editRole || undefined,
      });
      toast("Данные администратора обновлены", "success");
      setEditingAdmin(null);
      setEditPassword("");
      setEditRole("");
      await fetchAdmins();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось обновить данные", "error");
    } finally {
      setSubmitLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!deletingAdmin) return;
    setSubmitLoading(true);
    try {
      await adminsApi.delete(deletingAdmin.id);
      toast("Администратор удален", "success");
      setDeletingAdmin(null);
      await fetchAdmins();
    } catch (err: unknown) {
      toast(err instanceof Error ? err.message : "Не удалось удалить администратора", "error");
    } finally {
      setSubmitLoading(false);
    }
  };

  const openEditModal = (admin: Admin) => {
    setEditingAdmin(admin);
    setEditRole(admin.role);
    setEditPassword("");
  };

  const formatDate = (dateStr: string) => {
    if (!dateStr) return "—";
    try {
      const d = new Date(dateStr);
      if (isNaN(d.getTime())) return dateStr;
      return d.toLocaleDateString("ru-RU", {
        day: "2-digit",
        month: "2-digit",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return dateStr;
    }
  };

  if (currentRole !== "owner" && currentRole !== "super_admin") {
    return null;
  }

  return (
    <Card className="flex flex-col gap-4">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <button
            onClick={() => setCollapsed(!collapsed)}
            className="h-8 w-8 flex items-center justify-center rounded-lg border border-border bg-surface-2 transition-all hover:bg-surface-1"
            aria-expanded={!collapsed}
            aria-label={collapsed ? "Развернуть" : "Свернуть"}
          >
            <span className={`text-zinc-400 transition-transform ${collapsed ? "-rotate-90" : ""}`}>▼</span>
          </button>
          <h2 className="text-lg font-semibold flex items-center gap-2">
            <EmojiText text="👑 Администраторы" />
            <span className="text-sm font-normal text-zinc-500 bg-surface-2 px-2 py-0.5 rounded-full border border-border">
              {adminsList.length}
            </span>
          </h2>
        </div>
        {!collapsed && (
          <Button onClick={() => setShowAddModal(true)} className="text-xs py-1.5 flex items-center gap-1.5">
            <Plus className="w-3.5 h-3.5" />
            <span>Добавить</span>
          </Button>
        )}
      </div>

      {!collapsed && (
        <div className="overflow-x-auto">
          {loading ? (
            <div className="py-8 text-center text-sm text-zinc-400">Загрузка администраторов...</div>
          ) : adminsList.length === 0 ? (
            <div className="py-8 text-center text-sm text-zinc-400">Список администраторов пуст.</div>
          ) : (
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-zinc-400 border-b border-border">
                  <th scope="col" className="pb-3 pr-4 font-semibold">Имя пользователя</th>
                  <th scope="col" className="pb-3 pr-4 font-semibold">Роль</th>
                  <th scope="col" className="pb-3 pr-4 font-semibold">Создан</th>
                  <th scope="col" className="pb-3 text-right font-semibold">Действия</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border/30">
                {adminsList.map((admin) => (
                  <tr key={admin.id} className="hover:bg-surface-2/10 transition-colors">
                    <td className="py-3.5 pr-4 font-medium text-zinc-200">{admin.username}</td>
                    <td className="py-3.5 pr-4">
                      {admin.role === "owner" ? (
                        <span className="inline-flex items-center gap-1 text-xs px-2.5 py-0.5 rounded-full bg-red-500/10 text-red-400 font-semibold border border-red-500/20">
                          <ShieldAlert className="w-3 h-3" />
                          Super Admin
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs px-2.5 py-0.5 rounded-full bg-blue-500/10 text-blue-400 font-semibold border border-blue-500/20">
                          Support Admin
                        </span>
                      )}
                    </td>
                    <td className="py-3.5 pr-4 text-zinc-400">{formatDate(admin.created_at)}</td>
                    <td className="py-3.5 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <Button
                          variant="ghost"
                          onClick={() => openEditModal(admin)}
                          className="h-8 w-8 p-0 flex items-center justify-center text-zinc-400 hover:text-zinc-200 hover:bg-surface-2"
                          title="Редактировать"
                        >
                          <Edit className="w-4 h-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          onClick={() => setDeletingAdmin(admin)}
                          className="h-8 w-8 p-0 flex items-center justify-center text-red-400/75 hover:text-red-400 hover:bg-red-500/10"
                          title="Удалить"
                        >
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* Add Admin Modal */}
      <Modal open={showAddModal} onClose={() => setShowAddModal(false)} title="Добавить администратора">
        <form onSubmit={handleCreate} className="flex flex-col gap-4">
          <Input
            label="Имя пользователя"
            value={newUsername}
            onChange={(e) => setNewUsername(e.target.value)}
            required
            autoComplete="off"
            placeholder="Введите имя пользователя"
          />
          <PasswordInput
            label="Пароль"
            value={newPassword}
            onChange={(e) => setNewPassword(e.target.value)}
            required
            placeholder="Не менее 6 символов"
          />
          <Select
            label="Роль"
            value={newRole}
            onChange={(e) => setNewRole(e.target.value)}
            options={[
              { value: "viewer", label: "Viewer (только чтение)" },
              { value: "operator", label: "Operator (управление)" },
              { value: "owner", label: "Owner (полный доступ)" },
            ]}
          />
          <Button type="submit" loading={submitLoading} className="mt-2 flex items-center justify-center gap-2">
            <UserPlus className="w-4 h-4" />
            <span>Создать</span>
          </Button>
        </form>
      </Modal>

      {/* Edit Admin Modal */}
      <Modal open={editingAdmin !== null} onClose={() => setEditingAdmin(null)} title={`Редактировать: ${editingAdmin?.username}`}>
        <form onSubmit={handleUpdate} className="flex flex-col gap-4">
          <PasswordInput
            label="Новый пароль"
            value={editPassword}
            onChange={(e) => setEditPassword(e.target.value)}
            placeholder="Оставьте пустым, чтобы не менять"
          />
          <Select
            label="Роль"
            value={editRole}
            onChange={(e) => setEditRole(e.target.value)}
            options={[
              { value: "viewer", label: "Viewer (только чтение)" },
              { value: "operator", label: "Operator (управление)" },
              { value: "owner", label: "Owner (полный доступ)" },
            ]}
          />
          <Button type="submit" loading={submitLoading} className="mt-2 flex items-center justify-center gap-2">
            <KeyRound className="w-4 h-4" />
            <span>Сохранить</span>
          </Button>
        </form>
      </Modal>

      {/* Delete Confirmation Modal */}
      <ConfirmDialog
        open={deletingAdmin !== null}
        title="Удалить администратора"
        message={`Вы уверены, что хотите удалить администратора "${deletingAdmin?.username}"? Эта операция необратима и завершит все его активные сессии.`}
        confirmLabel="Удалить"
        onConfirm={handleDelete}
        onCancel={() => setDeletingAdmin(null)}
        loading={submitLoading}
      />
    </Card>
  );
}
