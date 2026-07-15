import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const expoConfig = require('eslint-config-expo/flat');

const lintedFiles = [
  'apps/mobile/**/*.{ts,tsx}',
  'packages/tokens/src/**/*.ts',
];

export default [
  {
    ignores: [
      'apps/mobile/**/*',
      '!apps/mobile/**/*/',
      '!apps/mobile/**/*.{ts,tsx}',
      'packages/tokens/**/*',
      '!packages/tokens/src/',
      '!packages/tokens/src/**/*/',
      '!packages/tokens/src/**/*.ts',
      'apps/mobile/.expo/**',
      'apps/mobile/src/lib/api/generated/**',
      '**/node_modules/**',
    ],
  },
  ...expoConfig,
  {
    files: lintedFiles,
    settings: {
      'import/resolver': {
        node: {
          extensions: ['.js', '.jsx', '.ts', '.tsx'],
        },
        typescript: {
          project: [
            './apps/mobile/tsconfig.json',
            './packages/tokens/tsconfig.json',
          ],
        },
      },
    },
    rules: {
      'quote-props': ['error', 'consistent-as-needed'],
    },
  },
];
