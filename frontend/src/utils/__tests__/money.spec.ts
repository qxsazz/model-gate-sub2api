import { describe, it, expect } from 'vitest';
import { formatMoneyFixed } from '../format';

describe('fixed membership and achievement amounts', () => {
  it('preserves two decimals without locale grouping or a currency prefix', () => {
    expect(formatMoneyFixed(0.1)).toBe('0.10');
    expect(formatMoneyFixed(1000)).toBe('1000.00');
    expect(formatMoneyFixed(0.001)).toBe('0.00');
    expect(formatMoneyFixed(-5)).toBe('-5.00');
    expect(formatMoneyFixed(null)).toBe('0.00');
  });
  it('adds a prefix only when requested', () => {
    expect(formatMoneyFixed(1, '$')).toBe('$1.00');
  });
});
