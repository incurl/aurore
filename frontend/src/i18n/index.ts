/**
 * The i18next setup.
 *
 * The vendored InvokeAI components use react-i18next for their labels.
 * Aurore ships the subset of keys those components reference, with
 * English values, so the reused components behave exactly as they do
 * upstream.
 */

import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';

export const DEFAULT_NS = 'translation';

const resources = {
  en: {
    translation: {
      common: {
        generating: 'Generating',
        newSeed: 'New seed',
        decreaseValue: 'Decrease value',
        increaseValue: 'Increase value',
        seedNextBatch: 'Next seed: {{seed}}',
        seedNextBatchRange: 'Next seeds: {{first}}–{{last}}',
        seedMode: {
          label: 'Seed mode',
          random: 'Random',
          fixed: 'Fixed',
          increment: 'Increment',
          decrement: 'Decrement',
        },
        status: {
          pending: 'Pending',
          waiting: 'Waiting',
          completed: 'Completed',
          failed: 'Failed',
          canceled: 'Cancelled',
        },
      },
    },
  },
};

void i18n.use(initReactI18next).init({
  resources,
  lng: 'en',
  fallbackLng: 'en',
  defaultNS: DEFAULT_NS,
  interpolation: {
    escapeValue: false,
  },
  returnNull: false,
});

export default i18n;
