import type { KeyProfileDetailResponse } from "@/lib/types";
import { buildEditorDetailProjection } from "./key-editor-detail-projection";
import { protocolEditorCapability } from "./protocol-editor-capabilities";

interface DetailStateWriter {
  detail: (value: KeyProfileDetailResponse) => void;
  label: (value: string) => void;
  status: (value: KeyProfileDetailResponse["status"]) => void;
  kind: (value: KeyProfileDetailResponse["kind"]) => void;
  category: (value: string) => void;
  templateText: (value: string) => void;
  protocol: (value: KeyProfileDetailResponse["protocol"]) => void;
  mode: (value: "raw" | "structured") => void;
  projection: (value: NonNullable<ReturnType<typeof buildEditorDetailProjection>>) => void;
}

// The session boundary invokes this only for the response it still owns.
export function applyEditorLoadedDetail(detail: KeyProfileDetailResponse, writer: DetailStateWriter) {
  writer.detail(detail);
  writer.label(detail.label);
  writer.status(detail.status);
  writer.kind(detail.kind);
  writer.category(detail.category || "");
  writer.templateText(detail.template_text || "");
  writer.protocol(detail.protocol);
  writer.mode(!detail.safe_structured || !protocolEditorCapability(detail.protocol) ? "raw" : "structured");
  const projection = buildEditorDetailProjection(detail);
  if (projection) writer.projection(projection);
}
