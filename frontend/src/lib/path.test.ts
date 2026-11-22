import { describe, expect, it } from 'vitest';
import { normalizePath, parseLocation, validatePath } from './path';

describe('normalizePath', () => {
  it.each([
    ['/Faculdade/Cálculo', 'faculdade/calculo'],
    ['/lista de compras', 'lista-de-compras'],
    ['/a   b\tc', 'a-b-c'],
    ['//a///b//', 'a/b'],
    ['/ação%20coração', 'acao-coracao'],
    ['/caf%C3%A9', 'cafe'],
    ['/foo.bar!@#$', 'foobar'],
    ['/under_score-dash', 'under_score-dash'],
    ['/%E0%A4%A', 'e0a4a'],
    ['/ÀÉÎÕÜ/ñ', 'aeiou/n'],
    ['/', ''],
    ['', ''],
  ])('%s -> %s', (input, expected) => {
    expect(normalizePath(input)).toBe(expected);
  });
});

describe('validatePath', () => {
  it('accepts valid paths', () => {
    expect(validatePath('a')).toBeNull();
    expect(validatePath('a/b/c/d/e')).toBeNull();
    expect(validatePath('x'.repeat(64))).toBeNull();
  });

  it('rejects empty, deep, long and invalid paths', () => {
    expect(validatePath('')).toBe('empty');
    expect(validatePath('a/b/c/d/e/f')).toBe('too_deep');
    expect(validatePath('x'.repeat(65))).toBe('invalid_segment');
    expect(validatePath(Array(4).fill('x'.repeat(60)).join('/'))).toBe(
      'too_long',
    );
    expect(validatePath('a//b')).toBe('invalid_segment');
    expect(validatePath('A')).toBe('invalid_segment');
  });

  it('rejects reserved first segments only', () => {
    for (const reserved of ['api', 'ws', 'raw', 'notes', 'children', '404']) {
      expect(validatePath(reserved)).toBe('reserved');
      expect(validatePath(`${reserved}/x`)).toBe('reserved');
    }
    expect(validatePath('well-known')).toBe('reserved');
    expect(validatePath('x/api')).toBeNull();
    expect(validatePath('apis')).toBeNull();
  });
});

describe('parseLocation', () => {
  it('treats the root and empty normalizations as home', () => {
    expect(parseLocation('/').kind).toBe('home');
    expect(parseLocation('//').kind).toBe('home');
    expect(parseLocation('/!!!').kind).toBe('home');
  });

  it('returns the canonical pathname for notes', () => {
    expect(parseLocation('/Faculdade/Cálculo/')).toEqual({
      kind: 'note',
      path: 'faculdade/calculo',
      canonicalPathname: '/faculdade/calculo',
    });
  });

  it('flags reserved and invalid paths', () => {
    expect(parseLocation('/API/x')).toEqual({
      kind: 'invalid',
      path: 'api/x',
      error: 'reserved',
    });
    expect(parseLocation('/.well-known/x')).toMatchObject({
      kind: 'invalid',
      error: 'reserved',
    });
    expect(parseLocation('/a/b/c/d/e/f')).toMatchObject({
      kind: 'invalid',
      error: 'too_deep',
    });
  });
});
