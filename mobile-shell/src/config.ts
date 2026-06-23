export interface MobileShellConfig {
  /** App display name used by setup/loading/error screens. */
  appName: string
  /** Storage key for the user-provided Sub2API frontend URL. */
  storageKey: string
  /** Example URL shown on the first-run setup page. */
  exampleAppUrl: string
}

export const mobileShellConfig: MobileShellConfig = {
  appName: 'Sub2API',
  storageKey: 'sub2api_mobile_app_url',
  exampleAppUrl: 'https://sub2api.example.com/',
}

export function normalizeSub2ApiUrl(input: string): string {
  const trimmed = input.trim()
  if (!trimmed) return ''

  const withProtocol = /^https?:\/\//i.test(trimmed) ? trimmed : `https://${trimmed}`
  const normalized = withProtocol.replace(/\/+$/, '')
  return `${normalized}/`
}

export function isValidSub2ApiUrl(input: string): boolean {
  const normalized = normalizeSub2ApiUrl(input)
  if (!normalized) return false

  const match = normalized.match(/^(https?):\/\/([^/?#]+)([/?#].*)?$/i)
  if (!match) return false

  const host = match[2]
  if (!host || host.includes(' ') || host.startsWith('.') || host.endsWith('.')) return false

  return host === 'localhost' || host.includes('.') || /^\d{1,3}(\.\d{1,3}){3}(:\d+)?$/.test(host)
}
