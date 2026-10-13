import type { useToast } from "@/components/ui/Toast";
import { ApiError } from "@/lib/api";
import type { KeyProfileDetailResponse, KeyRawSecretResponse, KeyStructuredSecretsResponse } from "@/lib/types";
import type { useEditorDraft } from "./use-editor-draft";

type Toast = ReturnType<typeof useToast>["toast"];

interface RequestFailureReceiver {
  toast: Toast;
  setShowConflictDialog: (open: boolean) => void;
}

export function reportEditorRequestError(error: unknown, fallback: string, receiver: RequestFailureReceiver) {
  if (error instanceof ApiError && error.status === 409) {
    receiver.setShowConflictDialog(true);
    return;
  }
  receiver.toast(error instanceof Error ? error.message : fallback, "error");
}

interface RevealReceiver extends Pick<ReturnType<typeof useEditorDraft>, "acceptRawReveal" | "acceptNativeReveal"> {
  detail: KeyProfileDetailResponse;
  toast: Toast;
}

// Call only within the request's session guard, before making secrets visible.
export function applyEditorReveal(target: "raw" | "structured-secrets", value: KeyRawSecretResponse | KeyStructuredSecretsResponse, receiver: RevealReceiver) {
  const { detail, toast } = receiver;
  if (target === "raw") {
    receiver.acceptRawReveal(detail.protocol, (value as KeyRawSecretResponse).raw_uri, detail.profile_revision);
    toast("Сырая ссылка раскрыта", "info");
    return;
  }
  receiver.acceptNativeReveal((value as KeyStructuredSecretsResponse).secrets, detail.profile_revision);
  toast("Секретные поля раскрыты", "info");
}
