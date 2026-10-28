import { describe, expect, it } from 'vitest';

import { dismissOnEscape, isTextEntry } from './preview';

/**
 * Builds a text input for the tests.
 */
function textInput(type = 'text'): HTMLInputElement {
  const input = document.createElement('input');
  input.type = type;
  document.body.appendChild(input);
  return input;
}

describe('dismissOnEscape', () => {
  it('closes the preview when an image is selected', () => {
    const action = dismissOnEscape(true, false, null);
    expect(action.clearSelection).toBe(true);
    expect(action.clearComparison).toBe(false);
  });

  it('drops the comparison before the preview', () => {
    const action = dismissOnEscape(true, true, null);
    expect(action.clearComparison).toBe(true);
    expect(action.clearSelection).toBe(false);
  });

  it('does nothing when there is nothing to dismiss', () => {
    const action = dismissOnEscape(false, false, null);
    expect(action.clearSelection).toBe(false);
    expect(action.clearComparison).toBe(false);
  });

  it('ignores the key inside a text field', () => {
    const input = textInput();
    const action = dismissOnEscape(true, true, input);
    expect(action.clearSelection).toBe(false);
    expect(action.clearComparison).toBe(false);
    document.body.removeChild(input);
  });

  it('ignores the key inside a textarea', () => {
    const area = document.createElement('textarea');
    document.body.appendChild(area);

    const action = dismissOnEscape(true, false, area);
    expect(action.clearSelection).toBe(false);
    document.body.removeChild(area);
  });

  it('ignores the key inside a content editable element', () => {
    const div = document.createElement('div');
    div.contentEditable = 'true';
    document.body.appendChild(div);

    const action = dismissOnEscape(true, false, div);
    expect(action.clearSelection).toBe(false);
    document.body.removeChild(div);
  });
});

describe('isTextEntry', () => {
  it('is true for a text input', () => {
    const input = textInput();
    expect(isTextEntry(input)).toBe(true);
    document.body.removeChild(input);
  });

  it('is true for a search input', () => {
    const input = textInput('search');
    expect(isTextEntry(input)).toBe(true);
    document.body.removeChild(input);
  });

  it('is false for a button', () => {
    const input = textInput('button');
    expect(isTextEntry(input)).toBe(false);
    document.body.removeChild(input);
  });

  it('is false for a checkbox', () => {
    const input = textInput('checkbox');
    expect(isTextEntry(input)).toBe(false);
    document.body.removeChild(input);
  });

  it('is false for a range input', () => {
    const input = textInput('range');
    expect(isTextEntry(input)).toBe(false);
    document.body.removeChild(input);
  });

  it('is false for a plain element', () => {
    const div = document.createElement('div');
    expect(isTextEntry(div)).toBe(false);
  });

  it('is false for nothing', () => {
    expect(isTextEntry(null)).toBe(false);
  });
});
