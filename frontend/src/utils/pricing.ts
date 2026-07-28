/**
 * formatScaled formats a per-token (or per-request) RMB price scaled by `scale`.
 *
 *   formatScaled(0.000003, 1_000_000) → "￥3"        // per 1M tokens
 *   formatScaled(0.5,        1)        → "￥0.5"      // per request
 *   formatScaled(null,       1_000_000) → "-"
 *
 * Uses toPrecision(10) then strips trailing zeros to avoid IEEE 754 display noise.
 * `minFractionDigits` pads the result back up to a minimum number of decimals.
 */
export function formatScaled(value: number | null, scale: number, minFractionDigits = 0): string {
  if (value == null) return '-'
  let formatted = (value * scale).toPrecision(10).replace(/\.?0+$/, '')
  if (minFractionDigits > 0 && !formatted.includes('e')) {
    const decimalIndex = formatted.indexOf('.')
    const fractionDigits = decimalIndex === -1 ? 0 : formatted.length - decimalIndex - 1
    if (fractionDigits < minFractionDigits) {
      formatted = (decimalIndex === -1 ? `${formatted}.` : formatted)
        + '0'.repeat(minFractionDigits - fractionDigits)
    }
  }
  return `￥${formatted}`
}
