import { describe, expect, it } from 'vitest'
import {
  mergeNormalizedAdminAccountImports,
  normalizeAdminAccountImportPayload
} from '@/utils/accountImportPayload'

describe('normalizeAdminAccountImportPayload', () => {
  it('passes through exported sub2api payload', () => {
    const raw = JSON.stringify({
      exported_at: '2026-03-08T00:00:00Z',
      proxies: [],
      accounts: [
        {
          name: 'exported-account',
          platform: 'openai',
          type: 'oauth',
          credentials: { access_token: 'at-1' },
          concurrency: 10,
          priority: 1
        }
      ]
    })

    const result = normalizeAdminAccountImportPayload(raw)

    expect(result.format).toBe('sub2api-data')
    expect(result.accountCount).toBe(1)
    expect(result.payload.accounts[0].name).toBe('exported-account')
  })

  it('converts a single OpenAI/Codex token bundle into import payload', () => {
    const raw = JSON.stringify({
      access_token: 'at-1',
      refresh_token: 'rt-1',
      id_token: 'id-1',
      email: 'test@example.com',
      account_id: 'acc-123',
      expired: '2026-03-18T00:41:50Z'
    })

    const result = normalizeAdminAccountImportPayload(raw)

    expect(result.format).toBe('openai-oauth')
    expect(result.accountCount).toBe(1)
    expect(result.payload.accounts[0]).toMatchObject({
      name: 'test@example.com',
      platform: 'openai',
      type: 'oauth',
      concurrency: 10,
      priority: 1,
      rate_multiplier: 1,
      credentials: {
        access_token: 'at-1',
        refresh_token: 'rt-1',
        id_token: 'id-1',
        chatgpt_account_id: 'acc-123'
      },
      extra: {
        email: 'test@example.com'
      }
    })
    expect(result.payload.accounts[0].credentials.expires_at).toBe(1773794510)
  })

  it('converts arrays of OpenAI bundles and keeps names unique', () => {
    const raw = JSON.stringify([
      {
        access_token: 'at-1',
        refresh_token: 'rt-1',
        email: 'same@example.com'
      },
      {
        access_token: 'at-2',
        refresh_token: 'rt-2',
        email: 'same@example.com'
      }
    ])

    const result = normalizeAdminAccountImportPayload(raw)

    expect(result.format).toBe('openai-oauth')
    expect(result.accountCount).toBe(2)
    expect(result.payload.accounts.map((item) => item.name)).toEqual([
      'same@example.com',
      'same@example.com-2'
    ])
  })

  it('unwraps API-style responses that nest payload in data', () => {
    const raw = JSON.stringify({
      code: 0,
      data: {
        access_token: 'at-1',
        refresh_token: 'rt-1',
        email: 'wrapped@example.com'
      }
    })

    const result = normalizeAdminAccountImportPayload(raw)

    expect(result.format).toBe('openai-oauth')
    expect(result.payload.accounts[0].name).toBe('wrapped@example.com')
  })

  it('throws for unsupported JSON shapes', () => {
    expect(() => normalizeAdminAccountImportPayload(JSON.stringify({ hello: 'world' }))).toThrow(
      'UNSUPPORTED_ACCOUNT_IMPORT_FORMAT'
    )
  })
})

describe('mergeNormalizedAdminAccountImports', () => {
  it('merges multiple imports and de-duplicates proxies by proxy key', () => {
    const exported = normalizeAdminAccountImportPayload(
      JSON.stringify({
        exported_at: '2026-03-08T00:00:00Z',
        proxies: [
          {
            proxy_key: 'http|127.0.0.1|8080||',
            name: 'proxy-1',
            protocol: 'http',
            host: '127.0.0.1',
            port: 8080,
            status: 'active'
          }
        ],
        accounts: [
          {
            name: 'exported-account',
            platform: 'openai',
            type: 'oauth',
            credentials: { access_token: 'at-export' },
            concurrency: 10,
            priority: 1,
            proxy_key: 'http|127.0.0.1|8080||'
          }
        ]
      })
    )

    const openAI = normalizeAdminAccountImportPayload(
      JSON.stringify({
        access_token: 'at-1',
        refresh_token: 'rt-1',
        email: 'merge@example.com'
      })
    )

    const merged = mergeNormalizedAdminAccountImports([exported, openAI, exported])

    expect(merged.accountCount).toBe(3)
    expect(merged.proxyCount).toBe(1)
    expect(merged.payload.proxies).toHaveLength(1)
    expect(merged.payload.accounts.map((item) => item.name)).toEqual([
      'exported-account',
      'merge@example.com',
      'exported-account'
    ])
  })
})
