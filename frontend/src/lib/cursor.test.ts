import { describe, expect, it } from 'vitest';
import { adjustSelection, commonDiff } from './cursor';

const caret = (offset: number) => ({ start: offset, end: offset });

describe('commonDiff', () => {
  it('finds non-overlapping prefix and suffix', () => {
    expect(commonDiff('hello world', 'hello brave world')).toEqual({
      prefix: 6,
      suffix: 5,
    });
    expect(commonDiff('aaa', 'aaaa')).toEqual({ prefix: 3, suffix: 0 });
    expect(commonDiff('', 'abc')).toEqual({ prefix: 0, suffix: 0 });
  });
});

describe('adjustSelection', () => {
  it('keeps the caret when the change is after it', () => {
    expect(adjustSelection('abc def', 'abc def ghi', caret(2))).toEqual(
      caret(2),
    );
  });

  it('shifts the caret when text is inserted before it', () => {
    // "hello |world" -> "hey there hello |world"
    const before = 'hello world';
    const after = 'hey there hello world';
    expect(adjustSelection(before, after, caret(6))).toEqual(caret(16));
  });

  it('shifts the caret when text is removed before it', () => {
    expect(adjustSelection('one two three', 'one three', caret(10))).toEqual(
      caret(6),
    );
  });

  it('keeps a selection on the same characters', () => {
    const before = 'line 1\nline 2\nline 3';
    const after = 'NEW\nline 1\nline 2\nline 3';
    // select "line 2"
    expect(adjustSelection(before, after, { start: 7, end: 13 })).toEqual({
      start: 11,
      end: 17,
    });
  });

  it('clamps a caret inside a replaced region', () => {
    // "abcXYZdef" caret after Y (5) -> region XYZ replaced by Q
    expect(adjustSelection('abcXYZdef', 'abcQdef', caret(5))).toEqual(caret(4));
  });

  it('handles the remote text being cleared', () => {
    expect(adjustSelection('something', '', { start: 2, end: 6 })).toEqual(
      caret(0),
    );
  });

  it('is a no-op for identical text', () => {
    expect(adjustSelection('same', 'same', { start: 1, end: 3 })).toEqual({
      start: 1,
      end: 3,
    });
  });
});
