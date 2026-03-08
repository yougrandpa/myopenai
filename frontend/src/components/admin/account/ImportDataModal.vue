<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFile') }}</label>
        <div
          class="rounded-lg border border-dashed p-4 transition-colors"
          :class="isDragOver
            ? 'border-primary-400 bg-primary-50 dark:border-primary-500 dark:bg-primary-900/20'
            : 'border-gray-300 bg-gray-50 dark:border-dark-600 dark:bg-dark-800'"
          @dragenter.prevent="handleDragEnter"
          @dragover.prevent="handleDragOver"
          @dragleave.prevent="handleDragLeave"
          @drop.prevent="handleDrop"
        >
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div class="min-w-0">
              <div class="truncate text-sm text-gray-700 dark:text-dark-200">
                {{ fileLabel || t('admin.accounts.dataImportSelectFile') }}
              </div>
              <div class="text-xs text-gray-500 dark:text-dark-400">
                {{ t('admin.accounts.dataImportFileHint') }}
              </div>
            </div>
            <div class="flex shrink-0 gap-2">
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="importing || parsing"
                @click="openFilePicker"
              >
                {{ t('common.chooseFile') }}
              </button>
              <button
                type="button"
                class="btn btn-secondary"
                :disabled="importing || parsing"
                @click="openDirectoryPicker"
              >
                {{ t('admin.accounts.dataImportChooseFolder') }}
              </button>
            </div>
          </div>
        </div>
        <input
          ref="fileInput"
          data-testid="file-input"
          type="file"
          class="hidden"
          multiple
          accept="application/json,.json"
          @change="handleFileChange"
        />
        <input
          ref="directoryInput"
          data-testid="directory-input"
          type="file"
          class="hidden"
          multiple
          webkitdirectory
          directory
          accept="application/json,.json"
          @change="handleDirectoryChange"
        />
      </div>

      <div
        v-if="selectedImport"
        class="space-y-3 rounded-xl border border-emerald-200 bg-emerald-50 p-4 text-sm dark:border-emerald-800 dark:bg-emerald-900/20"
      >
        <div>
          <div class="font-medium text-emerald-800 dark:text-emerald-200">
            {{ t('admin.accounts.dataImportDetected') }}
          </div>
          <div class="text-emerald-700 dark:text-emerald-300">
            {{
              t('admin.accounts.dataImportDetectedSummary', {
                format: detectedFormatLabel,
                file_count: validFileCount,
                account_count: selectedImport.accountCount,
                proxy_count: selectedImport.proxyCount
              })
            }}
          </div>
        </div>

        <div
          v-if="parseWarnings.length"
          class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
        >
          <div class="font-medium">
            {{ t('admin.accounts.dataImportSkippedFilesHint', { count: parseWarnings.length }) }}
          </div>
          <div class="mt-2 max-h-32 overflow-auto space-y-1 font-mono">
            <div v-for="item in parseWarnings" :key="item">{{ item }}</div>
          </div>
        </div>

        <div class="border-t border-emerald-200 pt-3 dark:border-emerald-800">
          <p class="mb-2 text-xs text-emerald-700 dark:text-emerald-300">
            {{ t('admin.accounts.dataImportBindGroupsHint') }}
          </p>
          <GroupSelector
            v-model="selectedGroupIds"
            :groups="groups"
            :platform="groupSelectorPlatform"
          />
          <p
            v-if="hasMixedPlatforms"
            class="mt-2 text-xs text-amber-700 dark:text-amber-300"
          >
            {{ t('admin.accounts.dataImportMixedPlatformHint') }}
          </p>
        </div>
      </div>

      <div
        v-if="parseError"
        class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-600 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300"
      >
        {{ parseError }}
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', result) }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              {{ item.kind }} {{ item.name || item.proxy_key || '-' }} — {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-data-form"
          :disabled="importing || parsing || !selectedImport"
        >
          {{ importing ? t('admin.accounts.dataImporting') : t('admin.accounts.dataImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminDataImportResult, AdminGroup, GroupPlatform } from '@/types'
import {
  mergeNormalizedAdminAccountImports,
  normalizeAdminAccountImportPayload,
  type AccountImportFormat,
  type NormalizedAdminAccountImport
} from '@/utils/accountImportPayload'

interface Props {
  show: boolean
  groups?: AdminGroup[]
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = withDefaults(defineProps<Props>(), {
  groups: () => []
})
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const importing = ref(false)
const parsing = ref(false)
const isDragOver = ref(false)
const selectedFiles = ref<File[]>([])
const result = ref<AdminDataImportResult | null>(null)
const selectedImport = ref<NormalizedAdminAccountImport | null>(null)
const selectedGroupIds = ref<number[]>([])
const detectedFormats = ref<AccountImportFormat[]>([])
const validFileCount = ref(0)
const parseError = ref('')
const parseWarnings = ref<string[]>([])

const fileInput = ref<HTMLInputElement | null>(null)
const directoryInput = ref<HTMLInputElement | null>(null)

const fileLabel = computed(() => {
  if (selectedFiles.value.length === 0) {
    return ''
  }
  if (selectedFiles.value.length === 1) {
    return selectedFiles.value[0].name
  }
  return t('admin.accounts.dataImportSelectedFiles', { count: selectedFiles.value.length })
})

const errorItems = computed(() => result.value?.errors || [])
const importPlatforms = computed<GroupPlatform[]>(() => {
  if (!selectedImport.value) {
    return []
  }
  return [...new Set(selectedImport.value.payload.accounts.map((account) => account.platform as GroupPlatform))]
})
const hasMixedPlatforms = computed(() => importPlatforms.value.length > 1)
const groupSelectorPlatform = computed<GroupPlatform | undefined>(() =>
  importPlatforms.value.length === 1 ? importPlatforms.value[0] : undefined
)
const detectedFormatLabel = computed(() => {
  if (!selectedImport.value) {
    return ''
  }
  if (detectedFormats.value.length > 1) {
    return t('admin.accounts.dataImportFormatMixed')
  }
  return detectedFormats.value[0] === 'openai-oauth'
    ? t('admin.accounts.dataImportFormatOpenAI')
    : t('admin.accounts.dataImportFormatSub2api')
})

const resetState = () => {
  selectedFiles.value = []
  result.value = null
  selectedImport.value = null
  selectedGroupIds.value = []
  detectedFormats.value = []
  validFileCount.value = 0
  parseError.value = ''
  parseWarnings.value = []
  parsing.value = false
  isDragOver.value = false
  if (fileInput.value) {
    fileInput.value.value = ''
  }
  if (directoryInput.value) {
    directoryInput.value.value = ''
  }
}

watch(
  () => props.show,
  (open) => {
    if (open) {
      resetState()
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const openDirectoryPicker = () => {
  directoryInput.value?.click()
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.readAsText(sourceFile)
  })
}

const isImportableJSONFile = (sourceFile: File) => {
  const name = sourceFile.name.toLowerCase()
  return name.endsWith('.json') || sourceFile.type === 'application/json' || sourceFile.type === ''
}

const fileSortKey = (sourceFile: File) => {
  const relativePath = (sourceFile as File & { webkitRelativePath?: string }).webkitRelativePath
  return relativePath || sourceFile.name
}

const selectFiles = async (files: File[]) => {
  const importableFiles = files.filter(isImportableJSONFile).sort((left, right) => fileSortKey(left).localeCompare(fileSortKey(right)))

  selectedFiles.value = importableFiles
  result.value = null
  selectedImport.value = null
  selectedGroupIds.value = []
  detectedFormats.value = []
  validFileCount.value = 0
  parseError.value = ''
  parseWarnings.value = []

  if (importableFiles.length === 0) {
    parseError.value = t('admin.accounts.dataImportNoValidFiles')
    return
  }

  parsing.value = true
  try {
    const validImports: NormalizedAdminAccountImport[] = []
    const formats = new Set<AccountImportFormat>()
    const warnings: string[] = []
    let firstFailureReason = ''

    for (const sourceFile of importableFiles) {
      try {
        const text = await readFileAsText(sourceFile)
        const normalized = normalizeAdminAccountImportPayload(text)
        validImports.push(normalized)
        formats.add(normalized.format)
      } catch (error) {
        const reason =
          error instanceof SyntaxError
            ? t('admin.accounts.dataImportParseFailed')
            : t('admin.accounts.dataImportUnsupportedFormat')
        if (!firstFailureReason) {
          firstFailureReason = reason
        }
        warnings.push(`${fileSortKey(sourceFile)} — ${reason}`)
      }
    }

    parseWarnings.value = warnings

    if (validImports.length === 0) {
      parseError.value = importableFiles.length === 1 && firstFailureReason
        ? firstFailureReason
        : t('admin.accounts.dataImportNoValidFiles')
      return
    }

    validFileCount.value = validImports.length
    detectedFormats.value = [...formats]
    selectedImport.value = mergeNormalizedAdminAccountImports(validImports)
  } finally {
    parsing.value = false
  }
}

const handleFileChange = async (event: Event) => {
  const target = event.target as HTMLInputElement
  await selectFiles(Array.from(target.files || []))
}

const handleDirectoryChange = async (event: Event) => {
  const target = event.target as HTMLInputElement
  await selectFiles(Array.from(target.files || []))
}

const handleDragEnter = () => {
  if (!importing.value) {
    isDragOver.value = true
  }
}

const handleDragOver = () => {
  if (!importing.value) {
    isDragOver.value = true
  }
}

const handleDragLeave = () => {
  isDragOver.value = false
}

const handleDrop = async (event: DragEvent) => {
  isDragOver.value = false
  if (importing.value) {
    return
  }
  await selectFiles(Array.from(event.dataTransfer?.files || []))
}

const handleImport = async () => {
  if (selectedFiles.value.length === 0) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
    return
  }

  if (parseError.value || !selectedImport.value) {
    appStore.showError(parseError.value || t('admin.accounts.dataImportUnsupportedFormat'))
    return
  }

  importing.value = true
  try {
    const res = await adminAPI.accounts.importData({
      data: selectedImport.value.payload,
      skip_default_group_bind: true,
      group_ids: selectedGroupIds.value.length ? selectedGroupIds.value : undefined
    })

    result.value = res

    const msgParams: Record<string, unknown> = {
      account_created: res.account_created,
      account_updated: res.account_updated,
      account_failed: res.account_failed,
      proxy_created: res.proxy_created,
      proxy_reused: res.proxy_reused,
      proxy_failed: res.proxy_failed
    }
    if (res.account_failed > 0 || res.proxy_failed > 0) {
      appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', msgParams))
    } else {
      appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
      emit('imported')
    }
  } catch (error: any) {
    appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
