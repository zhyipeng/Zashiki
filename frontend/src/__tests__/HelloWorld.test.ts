import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import HelloWorld from '../components/HelloWorld.vue'

describe('HelloWorld', () => {
  it('renders the message prop', () => {
    const wrapper = mount(HelloWorld, {
      props: { msg: 'Test Message' },
    })
    expect(wrapper.text()).toContain('Test Message')
  })

  it('has a greet button', () => {
    const wrapper = mount(HelloWorld, {
      props: { msg: 'Hello' },
    })
    const button = wrapper.find('button')
    expect(button.exists()).toBe(true)
    expect(button.text()).toBe('Greet')
  })
})
