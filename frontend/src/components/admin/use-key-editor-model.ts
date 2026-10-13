"use client";

import { applyEditorReveal, reportEditorRequestError } from "./editor-request-results";

import { useKeyEditorLoading } from "./use-key-editor-loading";

import { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import { copyToClipboard } from "@/lib/clipboard";
import {
  parseEditableConfiguration,
  parseXrayJSONConfiguration,
  patchEditableConfiguration,
  type XrayJSONPatch,
} from "@/lib/configuration";
import type {
  KeyCategory,
  KeyEditorSchemaResponse,
  KeyProfileDetailResponse,
  KeyRawSecretResponse,
  KeyStructuredSecretsResponse
} from "@/lib/types";
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { informationalTemplatePreviewParts } from "./keyTemplateVariables";
import {
  isLegacyEditorProtocol
} from "./protocol-editor-capabilities";

import { isXrayJSONRaw } from "./key-editor-raw-policy";
import { buildEditorSaveCommand } from "./key-editor-save-command";

import { useEditorRequestSession } from "./use-editor-request-session";

import { useEditorDraft } from "./use-editor-draft";

import type { KeyEditorModalProps } from "./key-editor-types";
export function useKeyEditorModel({
  open,
  keyId,
  initialLabel = "",
  initialCategory = "",
  initialKind = "real",
  initialProtocol,
  onClose,
  onRefresh,
}: KeyEditorModalProps) {
  const { toast } = useToast();

  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revealing, setRevealing] = useState(false);
  const [cloning, setCloning] = useState(false);
  const [detail, setDetail] = useState<KeyProfileDetailResponse | null>(null);
  const [schemaResponse, setSchemaResponse] = useState<KeyEditorSchemaResponse | null>(null);

  // Form State
  const { draft, setters, resetDraft, commandState, acceptNativeReveal, acceptRawReveal, acceptRevision, rememberLegacyEdit } = useEditorDraft({ initialLabel, initialCategory, initialKind, initialProtocol, keyId });
  const { label, kind, templateText, protocol, rawUri, authoritativeRaw, revealedRaw, rawEdited, isDirty } = draft;
  const {
    setTemplateText,
    setRawUri,
    setRawEdited,
    setStructuredEdited,
    setRawValidationError,
    setRawFormattingWarning,
    setLegacyDraft,
    setLegacySecretsRevealed,
    setServer,
    setPort,
    setIsDirty
  } = setters;

  const templateTextRef = useRef<HTMLTextAreaElement | null>(null);

  // Modal dialog states
  const [availableCategories, setAvailableCategories] = useState<KeyCategory[]>([]);
  const [showCreateCategory, setShowCreateCategory] = useState(false);
  const [showConflictDialog, setShowConflictDialog] = useState(false);
  const [showDiscardConfirm, setShowDiscardConfirm] = useState(false);

  const beginSessionRequest = useEditorRequestSession(
    { open, keyId, profileRevision: detail?.profile_revision },
    { setLoading, setRevealing, setShowConflictDialog, setSaving, setCloning }
  );

  const templatePreviewParts = useMemo(
    () => informationalTemplatePreviewParts(templateText),
    [templateText]
  );

  const insertTemplateVariable = useCallback((token: string) => {
    const editor = templateTextRef.current;
    const start = editor?.selectionStart ?? templateText.length;
    const end = editor?.selectionEnd ?? start;
    const next = `${templateText.slice(0, start)}${token}${templateText.slice(end)}`;
    const caret = start + token.length;
    setTemplateText(next);
    setIsDirty(true);
    window.requestAnimationFrame(() => {
      templateTextRef.current?.focus();
      templateTextRef.current?.setSelectionRange(caret, caret);
    });
  }, [templateText, setTemplateText, setIsDirty]);

  const handleCloseAttempt = () => {
    if (isDirty) {
      setShowDiscardConfirm(true);
    } else {
      onClose();
    }
  };

  // Clear state on unmount/close
  const clearSensitiveState = useCallback(() => {
    setShowDiscardConfirm(false);
    setDetail(null);
    resetDraft({ initialLabel, initialCategory, initialKind, initialProtocol, keyId });
  }, [initialCategory, initialKind, initialLabel, initialProtocol, keyId, resetDraft]);

  const loadDetailAndSchema = useKeyEditorLoading({ keyId, toast, beginSessionRequest, acceptRevision, setters, setLoading, setDetail, setSchemaResponse, setAvailableCategories });

  useEffect(() => {
    if (open) {
      clearSensitiveState();
      void loadDetailAndSchema();
    } else {
      clearSensitiveState();
    }
  }, [open, loadDetailAndSchema, clearSensitiveState]);

  // Reveal secret call
  const handleReveal = async (target: "raw" | "structured-secrets") => {
    if (!keyId || !detail) return;
    if (revealing) return;
    const inSession = beginSessionRequest("profile");
    setRevealing(true);
    try {
      const res = await keysApi.reveal(keyId, {
        profile_revision: detail.profile_revision,
        target,
      });
      inSession(() => applyEditorReveal(target, res.data as KeyRawSecretResponse | KeyStructuredSecretsResponse, { detail, toast, acceptRawReveal, acceptNativeReveal }));
    } catch (error) {
      inSession(() => reportEditorRequestError(error, "Ошибка раскрытия секретов", { toast, setShowConflictDialog }));
    } finally {
      inSession(() => setRevealing(false));
    }
  };

  // Clone call
  const handleClone = async () => {
    if (!keyId || !detail) return;
    const inSession = beginSessionRequest();
    setCloning(true);
    try {
      await keysApi.clone(keyId, {
        expected_profile_revision: detail.profile_revision,
      });
      inSession(() => toast("Локальный клон успешно создан", "success"));
      await onRefresh();
      inSession(onClose);
    } catch (error) {
      inSession(() => reportEditorRequestError(error, "Ошибка клонирования", { toast, setShowConflictDialog }));
    } finally {
      inSession(() => setCloning(false));
    }
  };

  const isSourceOwned = detail?.ownership === "external_source";
  const sourceDefaultClientName = detail?.label || "";
  const schemaForProtocol = useMemo(() => {
    return schemaResponse?.protocols.find((p) => p.protocol === protocol);
  }, [schemaResponse, protocol]);

  const changeLegacyDraft = (patch: XrayJSONPatch) => {
    rememberLegacyEdit(patch);
    setLegacyDraft((current) => ({ ...current, ...patch }));
    if (patch.server !== undefined) setServer(patch.server);
    if (patch.port !== undefined) setPort(patch.port);
    setStructuredEdited(true);
    setIsDirty(true);
    if (revealedRaw && rawUri) {
      try {
        const nextRaw = patchEditableConfiguration(rawUri, patch);
        setRawUri(nextRaw);
        setRawValidationError("");
      } catch (error) {
        setRawValidationError(error instanceof Error ? error.message : "Не удалось обновить raw-представление");
      }
    }
  };

  const markStructuredChange = () => {
    setStructuredEdited(true);
    setIsDirty(true);
  };

  const changeRaw = (value: string) => {
    setRawUri(value);
    setRawEdited(true);
    setIsDirty(true);
    setRawValidationError("");
    setRawFormattingWarning("");
    try {
      if (isXrayJSONRaw(protocol, value)) {
        parseXrayJSONConfiguration(value);
      } else if (isLegacyEditorProtocol(protocol)) {
        const parsed = parseEditableConfiguration(value);
        setLegacyDraft(parsed);
        setLegacySecretsRevealed(true);
      }
    } catch (error) {
      setRawValidationError(error instanceof Error ? error.message : "Некорректная конфигурация");
    }
  };

  // Submit Handler
  const handleSubmit = async (event: FormEvent) => {
    event.preventDefault();
    if (!label.trim()) return;
    const inSession = beginSessionRequest();
    setSaving(true);
    try {
      const command = buildEditorSaveCommand(commandState, keyId, detail);
      if (command.type === "update") {
        await keysApi.updateProfile(command.keyId, command.input);
      } else {
        await keysApi.createProfile(command.input);
      }
      inSession(() => toast(command.message, "success"));
      await onRefresh();
      inSession(onClose);
    } catch (error) {
      inSession(() => reportEditorRequestError(error, "Ошибка сохранения", { toast, setShowConflictDialog }));
    } finally {
      inSession(() => setSaving(false));
    }
  };

  const copyRaw = async () => {
    const value = rawEdited ? rawUri : authoritativeRaw || rawUri;
    if (!value) return;
    const ok = await copyToClipboard(value);
    if (ok) toast("Сырая ссылка скопирована", "success");
    else toast("Не удалось скопировать", "error");
  };

  const titleText = keyId
    ? `${kind === "informational" ? "Изменить информационный ключ" : "Изменить конфигурацию"} — ${detail?.label || label || initialLabel || ""}`
    : kind === "informational" || initialKind === "informational"
      ? "Добавить информационный ключ"
      : "Добавить конфигурацию";

  return { ...draft, ...setters, availableCategories, changeLegacyDraft, changeRaw, cloning, copyRaw, detail, handleClone, handleCloseAttempt, handleReveal, handleSubmit, insertTemplateVariable, isSourceOwned, keyId, loadDetailAndSchema, loading, markStructuredChange, onClose, open, revealing, saving, schemaForProtocol, setAvailableCategories, setShowConflictDialog, setShowCreateCategory, setShowDiscardConfirm, showConflictDialog, showCreateCategory, showDiscardConfirm, sourceDefaultClientName, templatePreviewParts, templateTextRef, titleText };
}

export type KeyEditorModel = ReturnType<typeof useKeyEditorModel>;
