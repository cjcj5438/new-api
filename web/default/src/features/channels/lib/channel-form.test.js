import { describe, expect, test } from 'bun:test'
import { computePriorityFromFormula } from './channel-form'

describe('computePriorityFromFormula', () => {
  test('multiplies fractional formulas by 1000 and stores the integer as negative priority', () => {
    expect(computePriorityFromFormula('1/1*0.123456', 0)).toBe(-123)
  })

  test('keeps already-scaled formulas usable and still stores them as negative priority', () => {
    expect(computePriorityFromFormula('1/1*0.1*100', 0)).toBe(-10)
  })
})
