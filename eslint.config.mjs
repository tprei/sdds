import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const expoConfig = require('eslint-config-expo/flat');

const lintedFiles = [
  'apps/mobile/**/*.{ts,tsx}',
  'packages/tokens/src/**/*.ts',
  'tests/synthetics/**/*.ts',
  'playwright.config.ts',
];

// Properties whose value is a spec-locked metric. A raw number here is the
// duplication that let two hand-transcriptions of the design spec drift apart,
// so the value must come from `@sdds/tokens`.
const gatedStyleProperties = [
  'width',
  'height',
  'minWidth',
  'minHeight',
  'maxWidth',
  'maxHeight',
  'borderRadius',
  'fontSize',
  'lineHeight',
  'gap',
  'top',
  'right',
  'bottom',
  'left',
  'padding[A-Za-z]*',
  'margin[A-Za-z]*',
].join('|');

// `0` and `1` stay legal: zeroing a property and a hairline border carry no
// spec metric. A percentage (`'100%'`) is a relationship, not a metric, so the
// value must be a number to be gated. `shadowOffset`'s inner width/height
// describe a shadow, not a layout box, and are read from `shadows` tokens.
const gatedStyleKey =
  `Property[key.name=/^(${gatedStyleProperties})$/]` +
  `:not(Property[key.name='shadowOffset'] > ObjectExpression > Property)`;
// Matched on `raw` rather than `value`: esquery only applies a regex attribute
// to a string, so a numeric literal never matches one on `value`. `raw` is the
// source text, so `34` matches and `'100%'` does not.
const gatedNumber = `Literal[raw=/^\\d/][value!=0][value!=1]`;
const styleLiteralMessage =
  'Raw metric in a stylesheet. Read this value from componentMetrics, ' +
  'spacing, or radius in @sdds/tokens instead of writing the number here.';

const testFrameworkImports = [
  'vitest',
  'react-test-renderer',
  '@playwright/test',
].map((name) => ({
  name,
  message:
    `${name} MUST NOT be imported by a production module. ` +
    'Move the helper to apps/mobile/src/test-support/ or into a *.test.ts(x) file.',
}));

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
            './tsconfig.json',
          ],
        },
      },
    },
    rules: {
      'quote-props': ['error', 'consistent-as-needed'],
    },
  },
  {
    files: ['apps/mobile/src/**/*.styles.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: `${gatedStyleKey} > ${gatedNumber}`,
          message: styleLiteralMessage,
        },
        {
          selector: `${gatedStyleKey} > UnaryExpression[operator='-'] > Literal`,
          message: styleLiteralMessage,
        },
      ],
    },
  },
  {
    files: ['apps/mobile/src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', { paths: testFrameworkImports }],
    },
  },
  {
    files: [
      'apps/mobile/src/**/*.test.{ts,tsx}',
      'apps/mobile/src/test-support/**/*.{ts,tsx}',
    ],
    rules: {
      'no-restricted-imports': 'off',
    },
  },
];
