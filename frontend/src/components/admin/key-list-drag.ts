export interface KeyListDragGeometry { pointerY: number; top: number; bottom: number; edgeSize: number; maxVelocity: number }

function invalidDragGeometry(geometry: KeyListDragGeometry) {
  const invalidDimension = [geometry.bottom - geometry.top, geometry.edgeSize, geometry.maxVelocity].some((value) => value <= 0);
  return !Number.isFinite(geometry.pointerY) || invalidDimension;
}

export function keyListDragVelocity(geometry: KeyListDragGeometry) {
  const { pointerY, top, bottom, edgeSize, maxVelocity } = geometry;
  const height = bottom - top;
  if (invalidDragGeometry(geometry)) {
    return 0;
  }

  const edge = Math.min(edgeSize, height / 2);
  const upperBoundary = top + edge;
  const lowerBoundary = bottom - edge;

  if (pointerY < upperBoundary) {
    const proximity = Math.min(1, Math.max(0, (upperBoundary - pointerY) / edge));
    return proximity === 0 ? 0 : -Math.max(2, Math.round(maxVelocity * proximity * proximity));
  }
  if (pointerY > lowerBoundary) {
    const proximity = Math.min(1, Math.max(0, (pointerY - lowerBoundary) / edge));
    return proximity === 0 ? 0 : Math.max(2, Math.round(maxVelocity * proximity * proximity));
  }
  return 0;
}

interface DragPointer { clientX: number; clientY: number }

function scrollCandidates(pointer: DragPointer) {
  const candidates: HTMLElement[] = [];
  let element = document.elementFromPoint(pointer.clientX, pointer.clientY);
  while (element instanceof HTMLElement) {
    const style = window.getComputedStyle(element);
    if (/(auto|scroll)/.test(style.overflowY) && element.scrollHeight > element.clientHeight + 1) candidates.push(element);
    element = element.parentElement;
  }
  const root = document.scrollingElement;
  if (root instanceof HTMLElement && !candidates.includes(root)) candidates.push(root);
  return candidates;
}

function candidateVelocity(target: HTMLElement, pointer: DragPointer) {
  const rect = target === document.scrollingElement ? { top: 0, bottom: window.innerHeight } : target.getBoundingClientRect();
  return keyListDragVelocity({ pointerY: pointer.clientY, top: rect.top, bottom: rect.bottom, edgeSize: 120, maxVelocity: 28 });
}

function canScrollAtVelocity(target: HTMLElement, velocity: number) {
  if (velocity < 0) return target.scrollTop > 0;
  if (velocity > 0) return target.scrollTop + target.clientHeight < target.scrollHeight - 1;
  return false;
}

export function dragScrollTargetAt(pointer: DragPointer): { target: HTMLElement; velocity: number } | null {
  for (const target of scrollCandidates(pointer)) {
    const velocity = candidateVelocity(target, pointer);
    if (canScrollAtVelocity(target, velocity)) return { target, velocity };
  }
  return null;
}
