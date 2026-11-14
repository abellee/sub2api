import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'
import Toggle from '@/components/common/Toggle.vue'
import Select from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'

const { listByAccount, create, update, showError } = vi.hoisted(() => ({
  listByAccount: vi.fn(), create: vi.fn(), update: vi.fn(), showError: vi.fn(),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { scheduledTests: { listByAccount, create, update } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
enableAutoUnmount(afterEach)

const plan = { id: 7, account_id: 42, model_id: 'probe-model', cron_expression: '*/5 * * * *', enabled: true, max_results: 100, auto_recover: false, recover_scheduling: true, recovery_probe_interval_seconds: 41 }

async function openPanel() {
  const wrapper = shallowMount(ScheduledTestsPanel, {
    props: { show: false, accountId: 42, modelOptions: [{ value: 'probe-model', label: 'Probe' }] },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('scheduled scheduling recovery', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    listByAccount.mockResolvedValue([])
    create.mockResolvedValue(plan)
    update.mockResolvedValue(plan)
  })

  it('requires an explicit opt-in and sends it with the selected model and five-minute schedule', async () => {
    const wrapper = await openPanel()
    await wrapper.findAll('button').find(button => button.text() === 'admin.scheduledTests.addPlan')!.trigger('click')
    const recoveryLabel = wrapper.findAll('label').find(label => label.text() === 'admin.scheduledTests.recoverScheduling')!
    expect(recoveryLabel.findComponent(Toggle).props('modelValue')).toBe(false)
    recoveryLabel.findComponent(Toggle).vm.$emit('update:modelValue', true)
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'probe-model')
    await flushPromises()
    expect(wrapper.findAllComponents(Input)[0].props('modelValue')).toBe('*/5 * * * *')
    const interval = wrapper.findAllComponents(Input).find(input => input.props('id') === 'newPlan-recovery-probe-interval')!
    expect(interval.props('modelValue')).toBe('3')
    interval.vm.$emit('update:modelValue', '17')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(create).toHaveBeenCalledWith(expect.objectContaining({ account_id: 42, model_id: 'probe-model', cron_expression: '*/5 * * * *', recover_scheduling: true, auto_recover: false, recovery_probe_interval_seconds: 17 }))
    expect(showError).not.toHaveBeenCalled()
  })

  it('loads and can turn off an existing recovery plan', async () => {
    listByAccount.mockResolvedValue([plan])
    const wrapper = await openPanel()
    expect(wrapper.text()).toContain('admin.scheduledTests.recoverScheduling')
    await wrapper.get('button[title="admin.scheduledTests.editPlan"]').trigger('click')
    const label = wrapper.findAll('label').find(label => label.text() === 'admin.scheduledTests.recoverScheduling')!
    expect(label.findComponent(Toggle).props('modelValue')).toBe(true)
    label.findComponent(Toggle).vm.$emit('update:modelValue', false)
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ recover_scheduling: false }))
    expect(showError).not.toHaveBeenCalled()
  })

  it('loads and updates the saved confirmation interval', async () => {
    listByAccount.mockResolvedValue([plan])
    const wrapper = await openPanel()
    await wrapper.get('button[title="admin.scheduledTests.editPlan"]').trigger('click')
    const interval = wrapper.findAllComponents(Input).find(input => input.props('id') === 'editForm-recovery-probe-interval')!
    expect(interval.props('modelValue')).toBe('41')
    interval.vm.$emit('update:modelValue', '90')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(update).toHaveBeenCalledWith(7, expect.objectContaining({ recovery_probe_interval_seconds: 90 }))
  })

  it.each(['0', '-1', '3601', '1.5', ''])('rejects an invalid interval: %s', async value => {
    const wrapper = await openPanel()
    await wrapper.findAll('button').find(button => button.text() === 'admin.scheduledTests.addPlan')!.trigger('click')
    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'probe-model')
    wrapper.findAll('label').find(label => label.text() === 'admin.scheduledTests.recoverScheduling')!.findComponent(Toggle).vm.$emit('update:modelValue', true)
    await flushPromises()
    wrapper.findAllComponents(Input).find(input => input.props('id') === 'newPlan-recovery-probe-interval')!.vm.$emit('update:modelValue', value)
    await flushPromises()
    const save = wrapper.findAll('button').find(button => button.text() === 'common.save')!
    expect(save.attributes('disabled')).toBeDefined()
    await save.trigger('click')
    expect(create).not.toHaveBeenCalled()
  })
})
