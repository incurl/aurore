/**
 * The preview panel's dismissal rules.
 *
 * The preview is a permanent panel rather than a modal, so it has to
 * offer its own ways out: a close button in the panel header, the
 * Escape key, and a way to drop the comparison image.
 *
 * The rules live here, away from the React component, so that they can
 * be tested directly.
 */

/** What a dismissal should do. */
export interface DismissAction {
  /** The image to stop previewing, or an empty string to keep it. */
  clearSelection: boolean;
  /** The comparison to drop, or an empty string to keep it. */
  clearComparison: boolean;
}

/** Nothing is dismissed. */
const NOTHING: DismissAction = { clearSelection: false, clearComparison: false };

/**
 * Decides what Escape should do.
 *
 * The comparison is dropped first, because it is the more specific
 * thing on screen: dismissing the whole preview when the user only
 * meant to clear the comparison would take away more than they asked
 * for.
 *
 * The key is ignored while a text field has focus, so that Escape
 * still cancels an edit in a prompt or a field instead of closing the
 * preview behind it.
 *
 * @param hasSelection whether an image is being previewed.
 * @param hasComparison whether a comparison image is selected.
 * @param target the element the key was pressed on.
 */
export function dismissOnEscape(
  hasSelection: boolean,
  hasComparison: boolean,
  target: EventTarget | null,
): DismissAction {
  if (isTextEntry(target)) {
    return NOTHING;
  }
  if (hasComparison) {
    return { clearSelection: false, clearComparison: true };
  }
  if (hasSelection) {
    return { clearSelection: true, clearComparison: false };
  }
  return NOTHING;
}

/**
 * Reports whether the event target is a field the user is typing in.
 *
 * Escape has to keep its usual meaning inside a text field, where it
 * cancels an edit rather than closing whatever is behind it.
 */
export function isTextEntry(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  if (isContentEditable(target)) {
    return true;
  }
  const tag = target.tagName.toLowerCase();
  if (tag === 'textarea') {
    return true;
  }
  if (tag !== 'input') {
    return false;
  }
  // Buttons, checkboxes and the like are not text entry.
  const type = (target as HTMLInputElement).type?.toLowerCase();
  return !['button', 'submit', 'reset', 'checkbox', 'radio', 'file', 'range', 'color'].includes(
    type ?? '',
  );
}

/**
 * Reports whether the element accepts text directly.
 *
 * The isContentEditable property is the authority in a browser, but it
 * is not implemented everywhere, so the contentEditable property and
 * the attribute are checked as well.
 */
function isContentEditable(element: HTMLElement): boolean {
  if (element.isContentEditable) {
    return true;
  }
  const property = element.contentEditable?.toLowerCase();
  if (property === 'true' || property === '') {
    return true;
  }
  const attribute = element.getAttribute('contenteditable')?.toLowerCase();
  return attribute === 'true' || attribute === '';
}
