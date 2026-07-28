import { describe, expect, it, vi } from 'vitest'

import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('useChannelMonitorFormat', () => {
  it('shows 100% twelve-hour availability when no sample is present', () => {
    const { formatAvailability } = useChannelMonitorFormat()

    expect(formatAvailability({ primary_status: '', availability_12h: undefined })).toBe('100.00%')
  })

  it('formats the supplied twelve-hour availability', () => {
    const { formatAvailability } = useChannelMonitorFormat()

    expect(formatAvailability({ primary_status: 'operational', availability_12h: 87.5 })).toBe('87.50%')
  })
})
