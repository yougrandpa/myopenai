import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import ImportDataModal from '@/components/admin/account/ImportDataModal.vue'

const { showError, showSuccess, importData } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  importData: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      importData
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('ImportDataModal', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    importData.mockReset()
  })

  it('未选择文件时提示错误', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          GroupSelector: { template: '<div />' }
        }
      }
    })

    await wrapper.find('form').trigger('submit')
    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportSelectFile')
  })

  it('单个无效 JSON 时提示解析失败', async () => {
    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          GroupSelector: { template: '<div />' }
        }
      }
    })

    const input = wrapper.get('[data-testid="file-input"]')
    const file = new File(['invalid json'], 'data.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () => Promise.resolve('invalid json')
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await Promise.resolve()
    await wrapper.find('form').trigger('submit')

    expect(showError).toHaveBeenCalledWith('admin.accounts.dataImportParseFailed')
  })

  it('识别 OpenAI 账号 JSON 并转换为导入 payload', async () => {
    importData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_updated: 0,
      account_failed: 0,
      errors: []
    })

    const wrapper = mount(ImportDataModal, {
      props: {
        show: true,
        groups: [
          {
            id: 7,
            name: 'openai-default',
            platform: 'openai',
            rate_multiplier: 1,
            subscription_type: 'standard'
          }
        ]
      },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          GroupSelector: {
            props: ['modelValue'],
            emits: ['update:modelValue'],
            template: `<button data-testid="select-group" @click="$emit('update:modelValue', [7])">group</button>`
          }
        }
      }
    })

    const input = wrapper.get('[data-testid="file-input"]')
    const file = new File(['{}'], 'codex.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () =>
        Promise.resolve(
          JSON.stringify({
            access_token: 'at-1',
            refresh_token: 'rt-1',
            id_token: 'id-1',
            email: 'import@example.com',
            account_id: 'acc-123'
          })
        )
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await Promise.resolve()
    await wrapper.get('[data-testid="select-group"]').trigger('click')
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(importData).toHaveBeenCalledTimes(1)
    expect(importData).toHaveBeenCalledWith({
      data: expect.objectContaining({
        type: 'sub2api-data',
        version: 1,
        proxies: [],
        accounts: [
          expect.objectContaining({
            name: 'import@example.com',
            platform: 'openai',
            type: 'oauth',
            concurrency: 10,
            priority: 1,
            rate_multiplier: 1,
            credentials: expect.objectContaining({
              access_token: 'at-1',
              refresh_token: 'rt-1',
              id_token: 'id-1',
              chatgpt_account_id: 'acc-123'
            }),
            extra: expect.objectContaining({
              email: 'import@example.com'
            })
          })
        ]
      }),
      skip_default_group_bind: true,
      group_ids: [7]
    })
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.dataImportSuccess')
  })

  it('未选分组时允许后端绑定默认分组', async () => {
    importData.mockResolvedValue({
      proxy_created: 0,
      proxy_reused: 0,
      proxy_failed: 0,
      account_created: 1,
      account_updated: 0,
      account_failed: 0,
      errors: []
    })

    const wrapper = mount(ImportDataModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
          GroupSelector: { template: '<div />' }
        }
      }
    })

    const input = wrapper.get('[data-testid="file-input"]')
    const file = new File(['{}'], 'codex.json', { type: 'application/json' })
    Object.defineProperty(file, 'text', {
      value: () =>
        Promise.resolve(
          JSON.stringify({
            access_token: 'at-1',
            refresh_token: 'rt-1',
            email: 'import@example.com',
            account_id: 'acc-123'
          })
        )
    })
    Object.defineProperty(input.element, 'files', {
      value: [file]
    })

    await input.trigger('change')
    await Promise.resolve()
    await wrapper.find('form').trigger('submit')
    await Promise.resolve()

    expect(importData).toHaveBeenCalledTimes(1)
    expect(importData).toHaveBeenCalledWith({
      data: expect.objectContaining({
        type: 'sub2api-data',
        version: 1
      }),
      skip_default_group_bind: false,
      group_ids: undefined
    })
  })

})
