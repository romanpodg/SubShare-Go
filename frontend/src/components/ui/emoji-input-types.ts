import type { ReactNode } from "react";

export interface AppleEmojiInputProps {
  label?: string;
  error?: string;
  value?: string;
  placeholder?: string;
  onChange?: (e: { target: { value: string; }; }) => void;
  className?: string;
  id?: string;
  multiline?: boolean;
  maxLength?: number;
  showCounter?: boolean;
  rightSlot?: ReactNode;
  rightSlotWidth?: number;
}

export interface AppleEmojiInputHandle {
  insertEmoji: (emoji: string) => void;
  insertText: (text: string) => void;
  focus: () => void;
}

