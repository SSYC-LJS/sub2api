import type { UserTokenRankingItem } from '@/api/usage'

const MASK = '****'

export function maskRankingIdentity(value: string): string {
  const text = (value || '').trim()
  if (!text) return MASK

  const chars = Array.from(text)
  if (chars.length > 6) {
    return `${chars.slice(0, 3).join('')}${MASK}${chars.slice(-3).join('')}`
  }
  if (chars.length > 4) {
    return `${chars.slice(0, 2).join('')}${MASK}${chars.slice(-2).join('')}`
  }
  return `${chars[0] ?? ''}${MASK}${chars[chars.length - 1] ?? ''}`
}

export function getRankingIdentityDisplay(item: Pick<UserTokenRankingItem, 'username' | 'email'>): string {
  const username = item.username?.trim()
  if (username) return maskRankingIdentity(username)
  return maskRankingIdentity(item.email)
}
