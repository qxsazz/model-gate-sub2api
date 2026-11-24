<template>
  <aside class="key-list-checkin" :aria-label="t('keys.checkinNotice.title')">
    <span class="checkin-icon" aria-hidden="true">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.65" stroke-linecap="round" stroke-linejoin="round">
        <rect x="3.5" y="5" width="17" height="15.5" rx="3" />
        <path d="M7.5 3v4m9-4v4M4 10h16M8.5 15l2 2 4.5-5" />
      </svg>
    </span>
    <strong>{{ t('keys.checkinNotice.title') }}</strong>
    <p>{{ t('keys.checkinNotice.description') }}</p>
    <RouterLink to="/achievements" target="_blank" rel="noopener noreferrer" class="checkin-link">
      {{ t('keys.checkinNotice.action') }}
      <Icon name="arrowRight" size="xs" aria-hidden="true" />
      <span class="sr-only">({{ t('keys.checkinNotice.newTab') }})</span>
    </RouterLink>
  </aside>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
</script>

<style scoped>
.key-list-checkin {
  --checkin-border: #e7ddca;
  --checkin-title: #342c21;
  --checkin-description: #665c4c;
  --checkin-gold: #775c27;
  --checkin-icon-bg: #f2ead8;
  --checkin-surface: linear-gradient(110deg, #faf6eb, #fcfaf5);

  display: grid;
  grid-template-columns: 30px auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  max-width: 100%;
  padding: 7px 12px 7px 8px;
  color: var(--checkin-title);
  background: var(--checkin-surface);
  border: 1px solid var(--checkin-border);
  border-radius: 11px;
}

.dark .key-list-checkin {
  --checkin-border: #554a36;
  --checkin-title: #f0e8d7;
  --checkin-description: #c9beac;
  --checkin-gold: #e2c989;
  --checkin-icon-bg: #3a3224;
  --checkin-surface: linear-gradient(110deg, #28251f, #25231f);
}

.checkin-icon {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  color: var(--checkin-gold);
  background: var(--checkin-icon-bg);
  border-radius: 7px;
}

.key-list-checkin strong { font-size: 14px; font-weight: 650; white-space: nowrap; }
.key-list-checkin p { color: var(--checkin-description); font-size: 12px; line-height: 1.6; }

.checkin-link {
  display: inline-flex;
  align-items: center;
  align-self: stretch;
  justify-content: center;
  gap: 4px;
  padding-left: 12px;
  border-left: 1px solid var(--checkin-border);
  color: var(--checkin-gold);
  font-size: 12px;
  font-weight: 650;
  white-space: nowrap;
}

.checkin-link:hover, .checkin-link:focus-visible { text-decoration: underline; text-underline-offset: 3px; }

@media (max-width: 760px) {
  .key-list-checkin {
    width: 100%;
    grid-template-columns: 31px minmax(0, 1fr) auto;
    gap: 1px 9px;
    padding: 9px 11px;
  }
  .checkin-icon { grid-column: 1; grid-row: 1 / 3; }
  .key-list-checkin strong { grid-column: 2; grid-row: 1; }
  .key-list-checkin p { grid-column: 2; grid-row: 2; }
  .checkin-link { grid-column: 3; grid-row: 1 / 3; padding: 0; border: 0; }
}
</style>
