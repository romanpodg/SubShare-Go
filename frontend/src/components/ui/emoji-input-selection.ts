/** Count "plain text length" of a DOM node */
function nodeLen(n: Node): number {
  if (n.nodeType === Node.TEXT_NODE) return (n.textContent ?? "").length;
  if (n instanceof HTMLImageElement) return (n.alt ?? "").length;
  let l = 0;
  for (const c of n.childNodes) l += nodeLen(c);
  return l;
}

function targetNodeOffset(node: Node, offset: number): number {
  if (node.nodeType === Node.TEXT_NODE) return offset;
  let length = 0;
  for (let i = 0; i < offset; i++) length += nodeLen(node.childNodes[i]);
  return length;
}

function boundaryOffset(root: Node, target: Node, offset: number): number {
  if (root === target) return targetNodeOffset(root, offset);
  let position = 0;
  for (const child of root.childNodes) {
    const found = boundaryOffset(child, target, offset);
    if (found >= 0) return position + found;
    position += nodeLen(child);
  }
  return -1;
}

export interface PlainTextSelection {
  start: number;
  end: number;
}

/** Save the selection as plain-text UTF-16 offsets. */
export function saveSelection(root: HTMLElement): PlainTextSelection | null {
  const selection = window.getSelection();
  if (!selection?.rangeCount) return null;
  const range = selection.getRangeAt(0);
  if (!root.contains(range.startContainer) || !root.contains(range.endContainer)) return null;
  const start = boundaryOffset(root, range.startContainer, range.startOffset);
  const end = boundaryOffset(root, range.endContainer, range.endOffset);
  if (start < 0 || end < 0) return null;
  return { start: Math.min(start, end), end: Math.max(start, end) };
}

type CursorBoundary = [Node, number];

function imageBoundary(node: HTMLImageElement, left: number): CursorBoundary | null {
  if (left > node.alt.length) return null;
  const parent = node.parentNode!;
  const index = Array.from(parent.childNodes).indexOf(node);
  return [parent, left <= 0 ? index : index + 1];
}

function cursorBoundary(node: Node, left: number): CursorBoundary | null {
  if (node.nodeType === Node.TEXT_NODE) {
    return left <= (node.textContent ?? "").length ? [node, left] : null;
  }
  if (node instanceof HTMLImageElement) return imageBoundary(node, left);
  let remaining = left;
  for (const child of node.childNodes) {
    const length = nodeLen(child);
    if (remaining <= length) return cursorBoundary(child, remaining);
    remaining -= length;
  }
  return [node, node.childNodes.length];
}

/** Restore a UTF-16 caret after replacing the editable DOM. */
export function restoreCursor(root: HTMLElement, target: number) {
  const selection = window.getSelection();
  if (!selection) return;
  if (target < 0) return;
  const boundary = cursorBoundary(root, target);
  if (!boundary) return;
  const range = document.createRange();
  range.setStart(boundary[0], boundary[1]);
  range.collapse(true);
  selection.removeAllRanges();
  selection.addRange(range);
}
