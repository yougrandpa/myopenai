import type { AdminDataPayload } from '@/types'

export type AccountImportFormat = 'sub2api-data' | 'openai-oauth'

export interface NormalizedAdminAccountImport {
  format: AccountImportFormat
  payload: AdminDataPayload
  accountCount: number
  proxyCount: number
}

interface OpenAIImportCandidate {
  access_token?: string
  refresh_token?: string
  id_token?: string
  client_id?: string
  token_type?: string
  expires_in?: number
  expires_at?: number
  scope?: string
  email?: string
  name?: string
  chatgpt_account_id?: string
  chatgpt_user_id?: string
  organization_id?: string
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    return null
  }
  return value as Record<string, unknown>
}

function pickString(record: Record<string, unknown>, keys: string[]): string | undefined {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'string') {
      const trimmed = value.trim()
      if (trimmed) {
        return trimmed
      }
    }
  }
  return undefined
}

function pickNumber(record: Record<string, unknown>, keys: string[]): number | undefined {
  for (const key of keys) {
    const value = record[key]
    if (typeof value === 'number' && Number.isFinite(value)) {
      return value
    }
    if (typeof value === 'string' && value.trim()) {
      const parsed = Number(value)
      if (Number.isFinite(parsed)) {
        return parsed
      }
      const dateMs = Date.parse(value)
      if (Number.isFinite(dateMs)) {
        return Math.floor(dateMs / 1000)
      }
    }
  }
  return undefined
}

function isExportedAccountRecord(value: unknown): boolean {
  const record = asRecord(value)
  if (!record) {
    return false
  }
  return typeof record.platform === 'string' && typeof record.type === 'string' && !!asRecord(record.credentials)
}

function normalizeExportPayload(value: unknown): NormalizedAdminAccountImport | null {
  if (Array.isArray(value)) {
    return null
  }

  const record = asRecord(value)
  if (!record) {
    return null
  }

  const accounts = Array.isArray(record.accounts) ? record.accounts : null
  const proxies = Array.isArray(record.proxies) ? record.proxies : null
  if (!accounts) {
    return null
  }
  if (!proxies && !accounts.every(isExportedAccountRecord)) {
    return null
  }

  return {
    format: 'sub2api-data',
    payload: {
      type: typeof record.type === 'string' ? record.type : undefined,
      version: typeof record.version === 'number' ? record.version : undefined,
      exported_at:
        typeof record.exported_at === 'string' && record.exported_at.trim()
          ? record.exported_at
          : new Date().toISOString(),
      proxies: (proxies ?? []) as AdminDataPayload['proxies'],
      accounts: accounts as AdminDataPayload['accounts']
    },
    accountCount: accounts.length,
    proxyCount: proxies?.length ?? 0
  }
}

function resolveOrganizationID(record: Record<string, unknown>): string | undefined {
  const directValue = pickString(record, ['organization_id', 'organizationId'])
  if (directValue) {
    return directValue
  }

  const organizations = record.organizations
  if (!Array.isArray(organizations)) {
    return undefined
  }

  for (const item of organizations) {
    const org = asRecord(item)
    if (!org) {
      continue
    }
    if (org.is_default === true || org.isDefault === true) {
      return pickString(org, ['id'])
    }
  }

  for (const item of organizations) {
    const org = asRecord(item)
    if (!org) {
      continue
    }
    const orgID = pickString(org, ['id'])
    if (orgID) {
      return orgID
    }
  }

  return undefined
}

function extractOpenAICandidate(record: Record<string, unknown>): OpenAIImportCandidate | null {
  const accessToken = pickString(record, ['access_token', 'accessToken'])
  const refreshToken = pickString(record, ['refresh_token', 'refreshToken', 'rt'])
  const idToken = pickString(record, ['id_token', 'idToken'])

  if (!accessToken && !refreshToken && !idToken) {
    return null
  }

  return {
    access_token: accessToken,
    refresh_token: refreshToken,
    id_token: idToken,
    client_id: pickString(record, ['client_id', 'clientId']),
    token_type: pickString(record, ['token_type', 'tokenType']),
    expires_in: pickNumber(record, ['expires_in', 'expiresIn']),
    expires_at: pickNumber(record, ['expires_at', 'expiresAt', 'expired']),
    scope: pickString(record, ['scope']),
    email: pickString(record, ['email']),
    name: pickString(record, ['name']),
    chatgpt_account_id: pickString(record, ['chatgpt_account_id', 'chatgptAccountId', 'account_id', 'accountId']),
    chatgpt_user_id: pickString(record, ['chatgpt_user_id', 'chatgptUserId', 'user_id', 'userId']),
    organization_id: resolveOrganizationID(record)
  }
}

function collectOpenAICandidates(
  value: unknown,
  candidates: OpenAIImportCandidate[],
  seen: Set<string>
): void {
  if (Array.isArray(value)) {
    for (const item of value) {
      collectOpenAICandidates(item, candidates, seen)
    }
    return
  }

  const record = asRecord(value)
  if (!record) {
    return
  }

  const candidate = extractOpenAICandidate(record)
  if (candidate && candidate.access_token) {
    const candidateKey =
      candidate.refresh_token ||
      candidate.access_token ||
      candidate.id_token ||
      candidate.email ||
      JSON.stringify(candidate)
    if (!seen.has(candidateKey)) {
      seen.add(candidateKey)
      candidates.push(candidate)
    }
  }

  for (const nestedValue of Object.values(record)) {
    if (nestedValue && typeof nestedValue === 'object') {
      collectOpenAICandidates(nestedValue, candidates, seen)
    }
  }
}

function buildCredentials(candidate: OpenAIImportCandidate): Record<string, unknown> {
  const credentials: Record<string, unknown> = {}

  const stringFields: Array<keyof OpenAIImportCandidate> = [
    'access_token',
    'refresh_token',
    'id_token',
    'client_id',
    'token_type',
    'scope',
    'chatgpt_account_id',
    'chatgpt_user_id',
    'organization_id'
  ]

  for (const key of stringFields) {
    const value = candidate[key]
    if (typeof value === 'string' && value.trim()) {
      credentials[key] = value
    }
  }

  if (typeof candidate.expires_in === 'number' && Number.isFinite(candidate.expires_in)) {
    credentials.expires_in = candidate.expires_in
  }
  if (typeof candidate.expires_at === 'number' && Number.isFinite(candidate.expires_at)) {
    credentials.expires_at = candidate.expires_at
  }

  return credentials
}

function buildExtra(candidate: OpenAIImportCandidate): Record<string, unknown> | undefined {
  const extra: Record<string, unknown> = {}
  if (candidate.email) {
    extra.email = candidate.email
  }
  if (candidate.name) {
    extra.name = candidate.name
  }
  return Object.keys(extra).length > 0 ? extra : undefined
}

function resolveAccountBaseName(candidate: OpenAIImportCandidate, index: number): string {
  if (candidate.email) {
    return candidate.email
  }
  if (candidate.name) {
    return candidate.name
  }
  if (candidate.chatgpt_account_id) {
    return `openai-${candidate.chatgpt_account_id}`
  }
  if (candidate.chatgpt_user_id) {
    return `openai-${candidate.chatgpt_user_id}`
  }
  return `openai-imported-${index + 1}`
}

function resolveUniqueAccountName(baseName: string, seen: Map<string, number>): string {
  const current = seen.get(baseName) ?? 0
  seen.set(baseName, current + 1)
  if (current === 0) {
    return baseName
  }
  return `${baseName}-${current + 1}`
}

function buildOpenAIImportPayload(candidates: OpenAIImportCandidate[]): NormalizedAdminAccountImport {
  const seenNames = new Map<string, number>()
  const accounts = candidates.map((candidate, index) => ({
    name: resolveUniqueAccountName(resolveAccountBaseName(candidate, index), seenNames),
    platform: 'openai' as const,
    type: 'oauth' as const,
    credentials: buildCredentials(candidate),
    extra: buildExtra(candidate),
    concurrency: 10,
    priority: 1,
    rate_multiplier: 1
  }))

  return {
    format: 'openai-oauth',
    payload: {
      type: 'sub2api-data',
      version: 1,
      exported_at: new Date().toISOString(),
      proxies: [],
      accounts
    },
    accountCount: accounts.length,
    proxyCount: 0
  }
}

function unwrapImportCandidate(value: unknown): unknown {
  const record = asRecord(value)
  if (!record) {
    return value
  }

  const nestedData = record.data
  if (nestedData && typeof nestedData === 'object') {
    return nestedData
  }

  return value
}

export function normalizeAdminAccountImportPayload(rawText: string): NormalizedAdminAccountImport {
  const parsed = JSON.parse(rawText)
  const unwrapped = unwrapImportCandidate(parsed)

  const exportPayload = normalizeExportPayload(unwrapped)
  if (exportPayload) {
    return exportPayload
  }

  const openAICandidates: OpenAIImportCandidate[] = []
  collectOpenAICandidates(unwrapped, openAICandidates, new Set<string>())
  if (openAICandidates.length > 0) {
    return buildOpenAIImportPayload(openAICandidates)
  }

  throw new Error('UNSUPPORTED_ACCOUNT_IMPORT_FORMAT')
}
