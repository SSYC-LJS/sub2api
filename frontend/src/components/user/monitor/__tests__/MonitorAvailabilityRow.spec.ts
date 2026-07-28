import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import MonitorAvailabilityRow from '../MonitorAvailabilityRow.vue'

describe('MonitorAvailabilityRow', () => {
  it.each([null, Number.NaN])('defaults missing availability %s to 100%%', (value) => {
    const wrapper = mount(MonitorAvailabilityRow, {
      props: {
        windowLabel: '12 hours',
        value,
      },
    })

    expect(wrapper.text()).toContain('100.00%')
  })

  it('keeps a measured zero availability value', () => {
    const wrapper = mount(MonitorAvailabilityRow, {
      props: {
        windowLabel: '12 hours',
        value: 0,
      },
    })

    expect(wrapper.text()).toContain('0.00%')
  })
})
