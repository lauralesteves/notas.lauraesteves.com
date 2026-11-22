export interface Selection {
  start: number;
  end: number;
}

export interface TextDiff {
  /** Length of the common prefix. */
  prefix: number;
  /** Length of the common suffix, never overlapping the prefix. */
  suffix: number;
}

export function commonDiff(before: string, after: string): TextDiff {
  const max = Math.min(before.length, after.length);
  let prefix = 0;
  while (prefix < max && before[prefix] === after[prefix]) prefix++;
  let suffix = 0;
  while (
    suffix < max - prefix &&
    before[before.length - 1 - suffix] === after[after.length - 1 - suffix]
  ) {
    suffix++;
  }
  return { prefix, suffix };
}

/**
 * Maps a caret offset from `before` to `after` given their diff.
 * Offsets before the changed region stay put, offsets after it shift by the
 * length delta, and offsets inside it are clamped to the new region.
 */
function mapOffset(
  offset: number,
  before: string,
  after: string,
  { prefix, suffix }: TextDiff,
): number {
  if (offset <= prefix) return offset;
  const oldChangeEnd = before.length - suffix;
  const newChangeEnd = after.length - suffix;
  if (offset >= oldChangeEnd) return offset + (after.length - before.length);
  return Math.min(offset, newChangeEnd);
}

/** Keeps the caret/selection on the same text when `before` is replaced by `after`. */
export function adjustSelection(
  before: string,
  after: string,
  selection: Selection,
): Selection {
  if (before === after) return selection;
  const diff = commonDiff(before, after);
  const start = mapOffset(selection.start, before, after, diff);
  const end = mapOffset(selection.end, before, after, diff);
  return { start, end: Math.max(start, end) };
}
