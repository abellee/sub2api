import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import GroupTagsModal from '../GroupTagsModal.vue'
import type { GroupTag } from '@/api/groups'
import type { AdminGroup } from '@/types'

const { setGroupTags, showSuccess, showError } = vi.hoisted(() => ({ setGroupTags: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { groupCategories: { setGroupTags } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

enableAutoUnmount(afterEach)
function openModal(tags: GroupTag[] = []) {
  return mount(GroupTagsModal, {
    props: { show: true, group: { id: 42, name: 'Primary' } as AdminGroup, tags },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } },
  })
}

describe('group tags modal', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    setGroupTags.mockResolvedValue(undefined)
  })

  it('adds multiple tags and saves trimmed names and descriptions', async () => {
    const wrapper = openModal()
    expect(wrapper.get('[data-test="save-tags"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="tag-name"]').setValue(' 快速 ')
    await wrapper.get('[data-test="tag-description"]').setValue(' 优先处理 ')
    await wrapper.get('[data-test="add-tag"]').trigger('click')
    await wrapper.findAll('[data-test="tag-name"]')[1].setValue('稳定')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const tags = [{ name: '快速', description: '优先处理' }, { name: '稳定', description: '' }]
    expect(setGroupTags).toHaveBeenCalledWith(42, tags)
    expect(wrapper.emitted('saved')).toEqual([[42, tags]])
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('keeps changes in a draft and restores saved values when reopened', async () => {
    const tags = [{ name: '原名称', description: '原说明' }]
    const wrapper = openModal(tags)
    await wrapper.get('[data-test="tag-name"]').setValue('未保存')
    expect(tags[0].name).toBe('原名称')
    await wrapper.findAll('button').find(button => button.text() === 'common.cancel')!.trigger('click')
    expect(setGroupTags).not.toHaveBeenCalled()
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    expect(wrapper.get<HTMLInputElement>('[data-test="tag-name"]').element.value).toBe('原名称')
  })

  it('saves an empty list after deleting all tags', async () => {
    const wrapper = openModal([{ name: '删除', description: '' }])
    await wrapper.get('[data-test="remove-tag"]').trigger('click')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(setGroupTags).toHaveBeenCalledWith(42, [])
  })

  it('rejects blank names and excessive lengths', async () => {
    const wrapper = openModal()
    for (const name of [' ', '名'.repeat(51)]) {
      await wrapper.get('[data-test="tag-name"]').setValue(name)
      await wrapper.get('form').trigger('submit')
    }
    await wrapper.get('[data-test="tag-name"]').setValue('名称')
    await wrapper.get('[data-test="tag-description"]').setValue('文'.repeat(201))
    await wrapper.get('form').trigger('submit')
    expect(setGroupTags).not.toHaveBeenCalled()
  })

  it('blocks repeated saves and retains the draft after a failed save', async () => {
    let reject!: (error: Error) => void
    setGroupTags.mockImplementationOnce(() => new Promise((_resolve, rejectPromise) => { reject = rejectPromise }))
    const wrapper = openModal([{ name: '保留', description: '说明' }])
    await wrapper.get('form').trigger('submit')
    await wrapper.get('form').trigger('submit')
    expect(setGroupTags).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="add-tag"]').attributes('disabled')).toBeDefined()
    reject(new Error('save failed'))
    await flushPromises()
    expect(showError).toHaveBeenCalledWith('save failed')
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.get<HTMLInputElement>('[data-test="tag-name"]').element.value).toBe('保留')
  })
})
