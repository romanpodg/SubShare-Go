"use client";

import type { useToast } from "@/components/ui/Toast";
import { keys as keysApi } from "@/lib/api";
import type { KeyCategory, KeyEditorSchemaResponse, KeyProfileDetailResponse } from "@/lib/types";
import { useCallback } from "react";
import { applyEditorLoadedDetail } from "./apply-editor-loaded-detail";
import { buildEditorDetailProjection } from "./key-editor-detail-projection";
import type { useEditorDraft } from "./use-editor-draft";
import type { useEditorRequestSession } from "./use-editor-request-session";

interface EditorLoading {
  keyId?: number;
  toast: ReturnType<typeof useToast>["toast"];
  beginSessionRequest: ReturnType<typeof useEditorRequestSession>;
  acceptRevision: ReturnType<typeof useEditorDraft>["acceptRevision"];
  setters: ReturnType<typeof useEditorDraft>["setters"];
  setLoading: (loading: boolean) => void;
  setDetail: (detail: KeyProfileDetailResponse) => void;
  setSchemaResponse: (schema: KeyEditorSchemaResponse) => void;
  setAvailableCategories: (categories: KeyCategory[]) => void;
}

export function useKeyEditorLoading(input: EditorLoading) {
  const { keyId, toast, beginSessionRequest, acceptRevision, setLoading, setDetail, setSchemaResponse, setAvailableCategories } = input;
  const { setCategory, setDisplayName, setHy2CertSha, setHy2Insecure, setHy2ObfsType, setHy2Sni, setKind, setLabel, setLegacyDraft, setLegacySecretsRevealed, setMode, setPort, setProtocol, setServer, setSsMethod, setSsPluginName, setStatus, setTemplateText, setTuicAlpn, setTuicCc, setTuicHeartbeat, setTuicSkipCert, setTuicSni, setTuicUdpOverStream, setTuicUdpRelay, setTuicZeroRtt } = input.setters;
  const applyDetailProjection = useCallback((projection: NonNullable<ReturnType<typeof buildEditorDetailProjection>>) => {
    setServer(projection.server);
    setPort(projection.port);
    setDisplayName(projection.displayName);
    if (projection.legacy) {
      setLegacyDraft(projection.legacy);
      setLegacySecretsRevealed(false);
    }
    if (projection.shadowsocks) {
      setSsMethod(projection.shadowsocks.method);
      setSsPluginName(projection.shadowsocks.pluginName);
    }
    if (projection.hysteria2) {
      setHy2Sni(projection.hysteria2.sni);
      setHy2Insecure(projection.hysteria2.insecure);
      setHy2CertSha(projection.hysteria2.certificate);
      setHy2ObfsType(projection.hysteria2.obfuscation);
    }
    if (projection.tuic) {
      setTuicSni(projection.tuic.sni);
      setTuicAlpn(projection.tuic.alpn);
      setTuicSkipCert(projection.tuic.skipCert);
      setTuicCc(projection.tuic.cc);
      setTuicUdpRelay(projection.tuic.relay);
      setTuicUdpOverStream(projection.tuic.udpOverStream);
      setTuicZeroRtt(projection.tuic.zeroRtt);
      setTuicHeartbeat(projection.tuic.heartbeat);
    }
  }, [setServer, setPort, setDisplayName, setLegacyDraft, setLegacySecretsRevealed,
    setSsMethod, setSsPluginName, setHy2Sni, setHy2Insecure, setHy2CertSha, setHy2ObfsType,
    setTuicSni, setTuicAlpn, setTuicSkipCert, setTuicCc, setTuicUdpRelay,
    setTuicUdpOverStream, setTuicZeroRtt, setTuicHeartbeat]);

  const loadProfileDetail = useCallback(async (id: number, inSession: ReturnType<ReturnType<typeof useEditorRequestSession>>) => {
    const detailRes = await keysApi.get(id);
    inSession(() => {
      acceptRevision(detailRes.data.profile_revision);
      applyEditorLoadedDetail(detailRes.data, {
        detail: setDetail, label: setLabel, status: setStatus, kind: setKind,
        category: setCategory, templateText: setTemplateText, protocol: setProtocol,
        mode: setMode, projection: applyDetailProjection,
      });
    });
  }, [acceptRevision, setDetail, setLabel, setStatus, setKind, setCategory, setTemplateText, setProtocol, setMode, applyDetailProjection]);

  // Load detail & schemas
  const loadDetailAndSchema = useCallback(async () => {
    const inSession = beginSessionRequest();
    setLoading(true);
    try {
      const [schemaRes, catRes] = await Promise.all([
        keysApi.editorSchema(),
        keysApi.listCategories(),
      ]);
      await inSession(async () => {
        setSchemaResponse(schemaRes.data);
        setAvailableCategories(catRes.categories || []);
        if (keyId) await loadProfileDetail(keyId, inSession);
      });
    } catch (error) {
      inSession(() => toast(error instanceof Error ? error.message : "Ошибка загрузки", "error"));
    } finally {
      inSession(() => setLoading(false));
    }
  }, [keyId, toast, loadProfileDetail, beginSessionRequest,
    setLoading, setSchemaResponse, setAvailableCategories]);

  return loadDetailAndSchema;
}
