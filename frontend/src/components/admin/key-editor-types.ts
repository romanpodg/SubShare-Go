import type { ExternalProfileProtocol } from "@/lib/types";

export interface KeyEditorModalProps {
  open: boolean;
  keyId?: number; // If provided, edit mode via GET /api/v1/keys/{id}
  initialLabel?: string;
  initialCategory?: string;
  initialKind?: "real" | "informational";
  initialProtocol?: ExternalProfileProtocol;
  onClose: () => void;
  onRefresh: () => Promise<void>;
}

