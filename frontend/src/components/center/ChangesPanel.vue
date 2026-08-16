<script setup lang="ts">
import { ref, watch, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionsStore } from '@/stores/sessions'
import { useWorkspaceUiStore } from '@/stores/workspaceUi'
import { toast } from '@/utils/feedback'
import Skeleton from '@/components/common/Skeleton.vue'
import {
  gitChanges,
  gitBranches,
  gitCheckout,
  gitRemotes,
  gitAddRemote,
  gitCredentials,
  gitSaveCredential,
  gitDeleteCredential,
  gitStage,
  gitCommit,
  gitLog,
  streamGitOp,
} from '@/api/git'
import type {
  GitFileChange,
  GitChanges,
  GitBranches,
  GitCredentialInfo,
  GitRemote,
  GitCommitInfo,
} from '@/api/git'

const { t } = useI18n()
const sessions = useSessionsStore()
const workspaceUi = useWorkspaceUiStore()

const data = ref<GitChanges | null>(null)
const loading = ref(false)
const branches = ref<GitBranches | null>(null)
const selectedBranch = ref('')
const switching = ref(false)
const filterQuery = ref('')
const creds = ref<GitCredentialInfo[]>([])
const remotes = ref<GitRemote[]>([])
const view = ref<'working' | 'history'>('working')
const log = ref<GitCommitInfo[]>([])
const commitMessage = ref('')
const committing = ref(false)

const opRunning = ref<'pull' | 'push' | 'fetch' | null>(null)
const consoleOpen = ref(false)
const consoleLines = ref<{ text: string; kind: 'line' | 'meta' }[]>([])
const consoleRef = ref<HTMLElement | null>(null)
let opAbort: AbortController | null = null

const loginOpen = ref(false)
const loginForm = ref({ host: '', username: '', token: '' })
const loginSaving = ref(false)
const remoteOpen = ref(false)
const remoteForm = ref({ name: '', url: '' })
const remoteSaving = ref(false)

const projectId = computed(() => sessions.selectedProjectId)

async function loadChanges() {
  if (!projectId.value) {
    data.value = null
    return
  }
  loading.value = true
  try {
    data.value = await gitChanges(projectId.value)
    workspaceUi.changesCount = data.value?.changes?.length ?? 0
  } catch {
    data.value = { branch: '', changes: [], error: t('sessions.gitLoadFailed') }
  } finally {
    loading.value = false
  }
}

async function loadBranches() {
  if (!projectId.value) {
    branches.value = null
    selectedBranch.value = ''
    return
  }
  try {
    branches.value = await gitBranches(projectId.value)
    selectedBranch.value = branches.value?.current ?? ''
  } catch {
    branches.value = null
    selectedBranch.value = ''
  }
}

async function loadRemotes() {
  if (!projectId.value) {
    remotes.value = []
    return
  }
  try {
    const res = await gitRemotes(projectId.value)
    remotes.value = res.remotes
  } catch {
    remotes.value = []
  }
}

async function loadCredentials() {
  try {
    creds.value = await gitCredentials()
  } catch {
    creds.value = []
  }
}

async function loadLog() {
  if (!projectId.value) {
    log.value = []
    return
  }
  try {
    const res = await gitLog(projectId.value, 30)
    log.value = res.commits
  } catch {
    log.value = []
  }
}

async function refresh() {
  await Promise.all([loadChanges(), loadBranches(), loadRemotes(), loadCredentials()])
  if (view.value === 'history') await loadLog()
}

watch(() => sessions.selectedProjectId, refresh)
watch(view, (v) => {
  if (v === 'history') loadLog()
})
onMounted(refresh)
onBeforeUnmount(() => opAbort?.abort())

async function onSelectBranch(branch: unknown) {
  const target = typeof branch === 'string' ? branch : ''
  if (!projectId.value || !target || target === branches.value?.current) return
  switching.value = true
  try {
    branches.value = await gitCheckout(projectId.value, target)
    selectedBranch.value = branches.value?.current ?? target
    toast.success(t('sessions.gitCheckoutDone', { branch: selectedBranch.value }))
    await loadChanges()
  } catch (e) {
    toast.error(e instanceof Error ? e.message : t('sessions.gitCheckoutFailed'))
    selectedBranch.value = branches.value?.current ?? ''
  } finally {
    switching.value = false
  }
}

function statusLabel(s: string): string {
  switch (s) {
    case 'M': return 'M'
    case 'A': return 'A'
    case 'D': return 'D'
    case 'R': return 'R'
    case 'C': return 'C'
    case '??': return '?'
    default: return s
  }
}

function statusType(s: string): string {
  switch (s) {
    case 'M': return 'modified'
    case 'A': return 'added'
    case 'D': return 'deleted'
    case 'R': return 'renamed'
    case 'C': return 'copied'
    case '??': return 'untracked'
    default: return 'modified'
  }
}

function matchesFilter(c: GitFileChange): boolean {
  const q = filterQuery.value.trim().toLowerCase()
  if (!q) return true
  return c.file.toLowerCase().includes(q) || (c.origFile?.toLowerCase().includes(q) ?? false)
}

const stagedChanges = computed(() => data.value?.changes?.filter(c => c.staged && matchesFilter(c)) ?? [])
const unstagedChanges = computed(() => data.value?.changes?.filter(c => !c.staged && matchesFilter(c)) ?? [])
const branchList = computed(() => branches.value?.branches ?? [])
const totalCount = computed(() => data.value?.changes?.length ?? 0)
const aheadBehind = computed(() => {
  const d = data.value
  if (!d || (!d.ahead && !d.behind)) return ''
  return t('sessions.gitAheadBehind', { ahead: d.ahead ?? 0, behind: d.behind ?? 0 })
})
const segOptions = computed(() => [
  { label: t('sessions.gitWorking'), value: 'working' },
  { label: t('sessions.gitHistory'), value: 'history' },
])

function changeLabel(c: GitFileChange): string {
  if (c.origFile) return `${c.file} ← ${c.origFile}`
  return c.file
}

function askAboutFile(c: GitFileChange, e: Event) {
  e.stopPropagation()
  const stagedHint = c.staged ? t('sessions.askAboutStaged') : t('sessions.askAboutUnstaged')
  workspaceUi.prefillComposer(t('sessions.askAboutFilePrompt', { file: c.file, hint: stagedHint }))
  workspaceUi.setRightTab('changes')
}

function openDiff(c: GitFileChange) {
  workspaceUi.openStage({
    kind: 'diff',
    path: c.file,
    mode: 'view',
    staged: c.staged,
  })
}

function copyPath(file: string, e: Event) {
  e.stopPropagation()
  void navigator.clipboard?.writeText(file).catch(() => {})
  toast.success(t('sessions.gitCopyPathDone'))
}

async function toggleStage(c: GitFileChange, e: Event) {
  e.stopPropagation()
  if (!projectId.value) return
  try {
    data.value = await gitStage(projectId.value, [c.file], !c.staged)
    workspaceUi.changesCount = data.value?.changes?.length ?? 0
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('sessions.gitStageFailed'))
  }
}

async function doCommit() {
  const message = commitMessage.value.trim()
  if (!projectId.value || !message || committing.value) return
  committing.value = true
  try {
    await gitCommit(projectId.value, message)
    toast.success(t('sessions.gitCommitDone'))
    commitMessage.value = ''
    await Promise.all([loadChanges(), loadLog()])
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('sessions.gitCommitFailed'))
  } finally {
    committing.value = false
  }
}

function pushConsole(text: string, kind: 'line' | 'meta' = 'line') {
  consoleLines.value.push({ text, kind })
  if (consoleLines.value.length > 2000) consoleLines.value.splice(0, consoleLines.value.length - 2000)
  void nextTick(() => {
    if (consoleRef.value) consoleRef.value.scrollTop = consoleRef.value.scrollHeight
  })
}

async function runOp(op: 'pull' | 'push' | 'fetch') {
  if (!projectId.value || opRunning.value) return
  opRunning.value = op
  consoleOpen.value = true
  consoleLines.value = []
  opAbort = new AbortController()
  pushConsole(`$ git ${op}`, 'meta')
  try {
    const final = await streamGitOp(
      projectId.value,
      op,
      (ev) => {
        if (ev.type === 'line') pushConsole(ev.data ?? '')
        else if (ev.type === 'error') pushConsole(ev.data ?? '', 'meta')
        else if (ev.type === 'done') pushConsole(ev.data ?? '', 'meta')
      },
      opAbort.signal,
    )
    if (final.type === 'error') {
      toast.error(final.data || t('sessions.gitOpFailed', { op }))
    } else if (final.exit !== 0) {
      toast.error(t('sessions.gitOpFailed', { op }))
    }
    await Promise.all([loadChanges(), loadBranches(), loadLog()])
  } catch (err) {
    const msg = err instanceof Error ? err.message : t('sessions.gitOpFailed', { op })
    if (msg.toLowerCase().includes('in progress') || msg.includes('进行中')) {
      toast.error(t('sessions.gitBusy'))
    } else {
      toast.error(msg)
    }
  } finally {
    opRunning.value = null
    opAbort = null
  }
}

async function saveCredential() {
  const host = loginForm.value.host.trim()
  const token = loginForm.value.token.trim()
  if (!projectId.value || !host || !token || loginSaving.value) return
  loginSaving.value = true
  try {
    creds.value = await gitSaveCredential(projectId.value, host, loginForm.value.username, token)
    toast.success(t('sessions.gitCredentialSaved', { host }))
    loginForm.value = { host: '', username: '', token: '' }
    loginOpen.value = false
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('sessions.gitLoginFailed'))
  } finally {
    loginSaving.value = false
  }
}

async function removeCredential(host: string) {
  try {
    await gitDeleteCredential(host)
    creds.value = await gitCredentials()
    toast.success(t('sessions.gitLogoutDone', { host }))
  } catch (err) {
    toast.error(err instanceof Error ? err.message : '')
  }
}

async function saveRemote() {
  const name = remoteForm.value.name.trim()
  const url = remoteForm.value.url.trim()
  if (!projectId.value || !name || !url || remoteSaving.value) return
  remoteSaving.value = true
  try {
    remotes.value = (await gitAddRemote(projectId.value, name, url)).remotes
    toast.success(t('sessions.gitRemoteAdded', { name }))
    remoteForm.value = { name: '', url: '' }
    remoteOpen.value = false
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('sessions.gitStageFailed'))
  } finally {
    remoteSaving.value = false
  }
}

defineExpose({ refresh, totalCount })
</script>

<template>
  <aside class="changes-panel">
    <div v-if="!sessions.selectedProjectId" class="changes-panel__empty">
      <p>{{ $t('sessions.gitNoProject') }}</p>
    </div>

    <template v-else>
      <div class="changes-panel__bar">
        <span class="changes-panel__branch-icon">⎇</span>
        <DqSelect
          v-if="branchList.length"
          v-model="selectedBranch"
          class="changes-panel__branch-select"
          :disabled="switching"
          @update:model-value="onSelectBranch"
        >
          <DqOption v-for="b in branchList" :key="b" :value="b" :label="b" />
        </DqSelect>
        <span v-else class="changes-panel__branch-select changes-panel__branch-muted">{{
          data?.branch || '—'
        }}</span>
        <span v-if="switching" class="changes-panel__branch-switching">{{ $t('sessions.gitSwitching') }}</span>
        <span v-if="aheadBehind" class="changes-panel__ab">{{ aheadBehind }}</span>
        <button type="button" class="changes-panel__refresh" :title="$t('sessions.memoryRefresh')" @click="refresh">
          ⟳
        </button>
      </div>

      <div class="changes-panel__actions">
        <button type="button" class="changes-panel__login" @click="loginOpen = true">
          <span v-if="creds.length" class="changes-panel__login-dot" />
          <template v-if="creds.length">
            <span v-for="c in creds" :key="c.host" class="changes-panel__login-host">
              {{ c.host }}<template v-if="c.user"> ({{ c.user }})</template>
            </span>
          </template>
          <template v-else>{{ $t('sessions.gitNeedLogin') }}</template>
        </button>
        <button
          type="button"
          class="changes-panel__op"
          :disabled="!!opRunning"
          :title="$t('sessions.gitPull')"
          @click="runOp('pull')"
        >
          ↓ {{ $t('sessions.gitPull') }}
        </button>
        <button
          type="button"
          class="changes-panel__op changes-panel__op--push"
          :disabled="!!opRunning"
          :title="$t('sessions.gitPush')"
          @click="runOp('push')"
        >
          ↑ {{ $t('sessions.gitPush') }}
        </button>
      </div>

      <div class="changes-panel__remotes">
        <span v-if="!remotes.length" class="changes-panel__remotes-empty">{{ $t('sessions.gitNoRemotes') }}</span>
        <span
          v-for="r in remotes"
          :key="r.name"
          class="changes-panel__remote"
          :title="r.fetchUrl"
        >{{ r.name }} → {{ r.fetchUrl }}</span>
        <button type="button" class="changes-panel__remote-add" title="Add remote" @click="remoteOpen = true">
          +
        </button>
      </div>

      <div class="changes-panel__seg">
        <DqSegmented v-model="view" :options="segOptions" size="sm" block />
      </div>

      <div v-if="loading" class="changes-panel__empty changes-panel__loading">
        <Skeleton variant="text" width="50%" />
        <Skeleton variant="card" width="100%" />
        <Skeleton variant="card" width="100%" />
      </div>

      <template v-else-if="view === 'working'">
        <div v-if="data?.error || data?.code" class="changes-panel__empty">
          <p>{{ data.code === 'git_missing' ? $t('composer.gitMissing') : (data.error || $t('composer.gitUnavailable')) }}</p>
        </div>

        <div v-else-if="!data?.changes?.length" class="changes-panel__empty">
          <p>{{ $t('sessions.noChanges') }}</p>
          <p class="changes-panel__hint">{{ $t('sessions.noChangesHint') }}</p>
        </div>

        <div v-else class="changes-panel__list">
          <div class="changes-panel__filter">
            <DqInput v-model="filterQuery" size="sm" :placeholder="$t('sessions.filterChanges')" />
          </div>
          <template v-if="stagedChanges.length">
            <div class="changes-panel__group-label">{{ $t('sessions.changesStaged') }}</div>
            <button
              v-for="c in stagedChanges"
              :key="'s-' + c.file"
              type="button"
              class="changes-panel__item"
              :class="`is-${statusType(c.status)}`"
              @click="openDiff(c)"
            >
              <span class="changes-panel__item-status">{{ statusLabel(c.status) }}</span>
              <span class="changes-panel__item-file" :title="changeLabel(c)">{{ changeLabel(c) }}</span>
              <span
                class="changes-panel__item-ask"
                :title="$t('sessions.askAboutFile')"
                @click="askAboutFile(c, $event)"
              >{{ $t('sessions.askAboutFileShort') }}</span>
              <span
                class="changes-panel__item-stage"
                :title="$t('sessions.gitUnstage')"
                @click="toggleStage(c, $event)"
              >−</span>
              <span
                class="changes-panel__item-copy"
                :title="$t('sessions.copyPath')"
                @click="copyPath(c.file, $event)"
              >⎘</span>
            </button>
          </template>

          <template v-if="unstagedChanges.length">
            <div class="changes-panel__group-label">{{ $t('sessions.changesUnstaged') }}</div>
            <button
              v-for="c in unstagedChanges"
              :key="'u-' + c.file"
              type="button"
              class="changes-panel__item"
              :class="`is-${statusType(c.status)}`"
              @click="openDiff(c)"
            >
              <span class="changes-panel__item-status">{{ statusLabel(c.status) }}</span>
              <span class="changes-panel__item-file" :title="changeLabel(c)">{{ changeLabel(c) }}</span>
              <span
                class="changes-panel__item-ask"
                :title="$t('sessions.askAboutFile')"
                @click="askAboutFile(c, $event)"
              >{{ $t('sessions.askAboutFileShort') }}</span>
              <span
                class="changes-panel__item-stage"
                :title="$t('sessions.gitStage')"
                @click="toggleStage(c, $event)"
              >+</span>
              <span
                class="changes-panel__item-copy"
                :title="$t('sessions.copyPath')"
                @click="copyPath(c.file, $event)"
              >⎘</span>
            </button>
          </template>
        </div>

        <div v-if="stagedChanges.length" class="changes-panel__commit">
          <textarea
            v-model="commitMessage"
            class="changes-panel__commit-input"
            rows="2"
            :placeholder="$t('sessions.gitCommitPlaceholder')"
          />
          <button
            type="button"
            class="changes-panel__commit-btn"
            :disabled="!commitMessage.trim() || committing"
            @click="doCommit"
          >
            {{ $t('sessions.gitCommit') }}
          </button>
        </div>
      </template>

      <template v-else>
        <div v-if="!log.length" class="changes-panel__empty">
          <p>{{ $t('sessions.gitLogEmpty') }}</p>
        </div>
        <div v-else class="changes-panel__log">
          <div v-for="c in log" :key="c.hash" class="changes-panel__log-item">
            <span class="changes-panel__log-hash">{{ c.short }}</span>
            <span class="changes-panel__log-subject">{{ c.subject }}</span>
            <span class="changes-panel__log-meta">{{ c.author }} · {{ c.date }}</span>
          </div>
        </div>
      </template>

      <div v-if="consoleOpen" class="changes-panel__console">
        <div class="changes-panel__console-head">
          <span v-if="opRunning">git {{ opRunning }}…</span>
          <span v-else>{{ $t('sessions.gitOperation') }}</span>
          <button type="button" class="changes-panel__console-close" @click="consoleOpen = false">×</button>
        </div>
        <div ref="consoleRef" class="changes-panel__console-body">
          <div
            v-if="!consoleLines.length"
            class="changes-panel__console-hint"
          >{{ $t('sessions.gitConsoleEmpty') }}</div>
          <div
            v-for="(line, i) in consoleLines"
            :key="i"
            class="changes-panel__console-line"
            :class="{ 'is-meta': line.kind === 'meta' }"
          >{{ line.text || ' ' }}</div>
        </div>
      </div>
    </template>

    <DqDialog
      v-model:open="loginOpen"
      :title="$t('sessions.gitLoginTitle')"
      variant="glass"
      width="420px"
      :closable="true"
    >
      <div class="changes-panel__dialog">
        <p class="changes-panel__dialog-hint">{{ $t('sessions.gitNeedLoginHint') }}</p>
        <label class="changes-panel__dialog-label">{{ $t('sessions.gitHost') }}</label>
        <DqInput v-model="loginForm.host" :placeholder="$t('sessions.gitHostPlaceholder')" />
        <label class="changes-panel__dialog-label">{{ $t('sessions.gitUsername') }}</label>
        <DqInput v-model="loginForm.username" placeholder="git" />
        <label class="changes-panel__dialog-label">{{ $t('sessions.gitToken') }}</label>
        <DqInput
          v-model="loginForm.token"
          type="password"
          autocomplete="off"
          :placeholder="$t('sessions.gitTokenPlaceholder')"
        />
        <div v-if="creds.length" class="changes-panel__dialog-creds">
          <div class="changes-panel__group-label">{{ $t('sessions.gitLogin') }}</div>
          <div v-for="c in creds" :key="c.host" class="changes-panel__cred-row">
            <span class="changes-panel__cred-host">{{ c.host }}<template v-if="c.user"> ({{ c.user }})</template></span>
            <button
              type="button"
              class="changes-panel__cred-remove"
              @click="removeCredential(c.host)"
            >{{ $t('sessions.gitLogout') }}</button>
          </div>
        </div>
        <div class="changes-panel__dialog-actions">
          <DqButton @click="loginOpen = false">{{ $t('common.cancel') }}</DqButton>
          <DqButton
            type="primary"
            :disabled="!loginForm.host.trim() || !loginForm.token.trim() || loginSaving"
            @click="saveCredential"
          >
            {{ loginSaving ? $t('sessions.gitLoginSaving') : $t('common.save') }}
          </DqButton>
        </div>
      </div>
    </DqDialog>

    <DqDialog
      v-model:open="remoteOpen"
      :title="$t('sessions.gitOperation')"
      variant="glass"
      width="420px"
      :closable="true"
    >
      <div class="changes-panel__dialog">
        <label class="changes-panel__dialog-label">Name</label>
        <DqInput v-model="remoteForm.name" placeholder="origin" />
        <label class="changes-panel__dialog-label">URL</label>
        <DqInput v-model="remoteForm.url" placeholder="https://github.com/you/repo.git" />
        <div class="changes-panel__dialog-actions">
          <DqButton @click="remoteOpen = false">{{ $t('common.cancel') }}</DqButton>
          <DqButton
            type="primary"
            :disabled="!remoteForm.name.trim() || !remoteForm.url.trim() || remoteSaving"
            @click="saveRemote"
          >{{ $t('common.save') }}</DqButton>
        </div>
      </div>
    </DqDialog>
  </aside>
</template>

<style scoped>
.changes-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  background: transparent;
  font-size: var(--dq-font-size-body);
}

.changes-panel__empty {
  padding: 32px 16px;
  text-align: center;
  color: var(--dq-label-tertiary);
}

.changes-panel__loading {
  display: flex;
  flex-direction: column;
  gap: var(--dq-space-sm);
  text-align: left;
}

.changes-panel__hint {
  margin-top: 8px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
  line-height: 1.5;
}

.changes-panel__bar {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  font-size: var(--dq-font-size-footnote);
  font-weight: 600;
  color: var(--dq-label-primary);
  border-bottom: 1px solid var(--dq-separator-light);
}

.changes-panel__branch-icon {
  font-size: var(--dq-font-size-secondary);
  color: var(--dq-accent);
}

.changes-panel__branch-select {
  flex: 1;
  min-width: 0;
  font-family: var(--dq-font-mono);
}

.changes-panel__branch-muted {
  font-weight: 400;
  color: var(--dq-label-tertiary);
}

.changes-panel__branch-switching {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 400;
  color: var(--dq-label-tertiary);
}

.changes-panel__ab {
  flex-shrink: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  font-family: var(--dq-font-mono);
  color: var(--dq-accent);
}

.changes-panel__refresh {
  flex-shrink: 0;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  padding: 2px 4px;
  line-height: 1;
}

.changes-panel__refresh:hover {
  color: var(--dq-accent);
}

.changes-panel__actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--dq-separator-light);
}

.changes-panel__login {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  border: 1px dashed var(--dq-separator);
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  padding: 4px 8px;
  overflow: hidden;
}

.changes-panel__login:hover {
  border-color: var(--dq-accent);
  color: var(--dq-label-primary);
}

.changes-panel__login-dot {
  flex-shrink: 0;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--dq-success);
}

.changes-panel__login-host {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.changes-panel__op {
  flex-shrink: 0;
  border: 1px solid var(--dq-separator);
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-secondary);
  padding: 4px 8px;
}

.changes-panel__op:hover:not(:disabled) {
  border-color: var(--dq-accent);
  color: var(--dq-accent);
}

.changes-panel__op:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.changes-panel__remotes {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  padding: 4px 14px;
  border-bottom: 1px solid var(--dq-separator-light);
  font-size: var(--dq-font-size-caption);
  font-family: var(--dq-font-mono);
  color: var(--dq-label-quaternary);
}

.changes-panel__remote {
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.changes-panel__remotes-empty {
  font-style: italic;
}

.changes-panel__remote-add {
  margin-left: auto;
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
  padding: 0 2px;
}

.changes-panel__remote-add:hover {
  color: var(--dq-accent);
}

.changes-panel__seg {
  flex-shrink: 0;
  padding: 8px 14px 0;
}

.changes-panel__list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 0;
}

.changes-panel__filter {
  padding: 4px 12px 8px;
}

.changes-panel__item {
  width: 100%;
  border: none;
  background: transparent;
  text-align: left;
  cursor: pointer;
  font: inherit;
}

.changes-panel__group-label {
  padding: 8px 14px 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--dq-label-quaternary);
}

.changes-panel__item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 6px 14px;
  line-height: 1.45;
  color: var(--dq-label-primary);
}

.changes-panel__item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.changes-panel__item-status {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  font-size: var(--dq-font-size-caption);
  font-weight: 700;
  font-family: var(--dq-font-mono);
  color: #fff;
  background: color-mix(in srgb, var(--dq-label-secondary) 60%, transparent);
}

.changes-panel__item.is-modified .changes-panel__item-status {
  background: var(--dq-system-orange);
}

.changes-panel__item.is-added .changes-panel__item-status {
  background: var(--dq-success);
}

.changes-panel__item.is-deleted .changes-panel__item-status {
  background: var(--dq-danger);
}

.changes-panel__item.is-renamed .changes-panel__item-status,
.changes-panel__item.is-copied .changes-panel__item-status {
  background: var(--dq-accent);
  color: var(--dq-on-accent);
}

.changes-panel__item.is-untracked .changes-panel__item-status {
  background: color-mix(in srgb, var(--dq-label-tertiary) 80%, transparent);
}

.changes-panel__item-file {
  flex: 1;
  min-width: 0;
  word-break: break-word;
  font-size: var(--dq-font-size-footnote);
  font-family: var(--dq-font-mono);
}

.changes-panel__item-copy,
.changes-panel__item-ask,
.changes-panel__item-stage {
  flex-shrink: 0;
  opacity: 0;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-accent);
  padding: 0 4px;
  line-height: 1.45;
}

.changes-panel__item-copy {
  color: var(--dq-label-tertiary);
  font-size: var(--dq-font-size-body);
  padding: 0 2px;
}

.changes-panel__item:hover .changes-panel__item-copy,
.changes-panel__item:hover .changes-panel__item-ask,
.changes-panel__item:hover .changes-panel__item-stage {
  opacity: 1;
}

.changes-panel__item-copy:hover,
.changes-panel__item-ask:hover,
.changes-panel__item-stage:hover {
  color: var(--dq-accent);
}

.changes-panel__commit {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 14px;
  border-top: 1px solid var(--dq-separator-light);
}

.changes-panel__commit-input {
  width: 100%;
  resize: vertical;
  border: 1px solid var(--dq-separator);
  border-radius: 6px;
  background: color-mix(in srgb, var(--dq-bg-elevated) 60%, transparent);
  color: var(--dq-label-primary);
  font: inherit;
  font-size: var(--dq-font-size-footnote);
  font-family: var(--dq-font-mono);
  padding: 6px 8px;
}

.changes-panel__commit-input:focus {
  outline: none;
  border-color: var(--dq-accent);
}

.changes-panel__commit-btn {
  align-self: flex-end;
  border: 1px solid var(--dq-accent);
  border-radius: 6px;
  background: var(--dq-accent);
  color: var(--dq-on-accent);
  cursor: pointer;
  font: inherit;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  padding: 4px 12px;
}

.changes-panel__commit-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.changes-panel__log {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 0;
}

.changes-panel__log-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 14px;
  border-bottom: 1px solid var(--dq-separator-light);
}

.changes-panel__log-item:hover {
  background: color-mix(in srgb, var(--dq-label-primary) 4%, transparent);
}

.changes-panel__log-hash {
  font-family: var(--dq-font-mono);
  font-size: var(--dq-font-size-caption);
  color: var(--dq-accent);
}

.changes-panel__log-subject {
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-primary);
  word-break: break-word;
}

.changes-panel__log-meta {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
}

.changes-panel__console {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  max-height: 180px;
  border-top: 1px solid var(--dq-separator-light);
}

.changes-panel__console-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 10px;
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-tertiary);
}

.changes-panel__console-close {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: var(--dq-font-size-body);
  color: var(--dq-label-tertiary);
}

.changes-panel__console-close:hover {
  color: var(--dq-label-primary);
}

.changes-panel__console-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px 10px 8px;
  background: color-mix(in srgb, var(--dq-bg-muted, #000) 30%, transparent);
}

.changes-panel__console-hint {
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-quaternary);
  font-style: italic;
}

.changes-panel__console-line {
  font-family: var(--dq-font-mono);
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-secondary);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.5;
}

.changes-panel__console-line.is-meta {
  color: var(--dq-accent);
}

.changes-panel__dialog {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.changes-panel__dialog-hint {
  margin: 0 0 4px;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-label-tertiary);
  line-height: 1.5;
}

.changes-panel__dialog-label {
  font-size: var(--dq-font-size-caption);
  font-weight: 600;
  color: var(--dq-label-secondary);
}

.changes-panel__dialog-creds {
  margin-top: 8px;
}

.changes-panel__cred-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 4px;
}

.changes-panel__cred-host {
  font-family: var(--dq-font-mono);
  font-size: var(--dq-font-size-footnote);
  color: var(--dq-label-primary);
}

.changes-panel__cred-remove {
  border: none;
  background: transparent;
  cursor: pointer;
  font-size: var(--dq-font-size-caption);
  color: var(--dq-danger);
}

.changes-panel__dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 10px;
}
</style>
