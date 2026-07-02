import { describe, expect, it } from 'vitest'
import { getRankingIdentityDisplay, maskRankingEmail, maskRankingIdentity } from '../rankingIdentity'

describe('rankingIdentity', () => {
  it('masks identities by character count', () => {
    expect(maskRankingIdentity('abcdefg')).toBe('abc****efg')
    expect(maskRankingIdentity('abcde')).toBe('ab****de')
    expect(maskRankingIdentity('abcd')).toBe('a****d')
    expect(maskRankingEmail('595341366@qq.com')).toBe('595****366@qq.com')
  })

  it('counts Chinese characters as one character', () => {
    expect(maskRankingIdentity('张三李四王五赵')).toBe('张三李****王五赵')
    expect(maskRankingIdentity('张三李四王')).toBe('张三****四王')
    expect(maskRankingIdentity('张三李四')).toBe('张****四')
  })

  it('prefers username and falls back to email', () => {
    expect(getRankingIdentityDisplay({ username: '  用户名称123  ', email: 'email@example.com' })).toBe('用户名****123')
    expect(getRankingIdentityDisplay({ username: '   ', email: 'email@example.com' })).toBe('em****il@example.com')
  })
})



