<script setup lang="ts">
import KnowledgeBaseManagement from '@/views/KnowledgeBaseManagement.vue'
import SkillsManagement from '@/views/SkillsManagement.vue'
import McpServersManagement from '@/views/McpServersManagement.vue'
import AutomationsManagement from '@/views/AutomationsManagement.vue'
import SettingsView from '@/views/SettingsView.vue'
import TeamsManagement from '@/views/TeamsManagement.vue'
import UsageView from '@/views/UsageView.vue'
import type { AppModule } from '@/types/app-module'

const props = defineProps<{ module: AppModule }>()
</script>

<template>
  <KnowledgeBaseManagement v-if="props.module === 'knowledge'" />
  <SkillsManagement v-else-if="props.module === 'skills'" />
  <McpServersManagement v-else-if="props.module === 'mcpServers'" />
  <AutomationsManagement v-else-if="props.module === 'automations'" />
  <UsageView v-else-if="props.module === 'usage'" />
  <SettingsView v-else-if="props.module === 'settings'" />
  <TeamsManagement v-else-if="props.module === 'workers'" />
</template>
