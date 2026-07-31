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

// The gated number is matched as a DESCENDANT of the gated property, not as a
// direct child, so `height: cond ? 34 : 28`, a call argument, and the literal
// inside a negation (`marginTop: -4`) are all caught by the one selector.
//
// `0` and `1` stay legal: zeroing a property and a hairline border carry no
// spec metric. A percentage (`'100%'`) is a relationship, not a metric, so the
// value must be a number to be gated. `shadowOffset`'s inner width/height
// describe a shadow, not a layout box; `shadows.primaryButton` in
// `@sdds/tokens` is the only shadow a stylesheet reads.
//
// Known and accepted hole: a value laundered through a local binding
// (`const CHIP_H = 34; height: CHIP_H`) is not caught. Chasing it would mean
// tracking constants across a module for no real gain, since it takes
// deliberate circumvention rather than the habit this gate exists to break.
const gatedStyleKey =
  `Property[key.name=/^(${gatedStyleProperties})$/]` +
  `:not(Property[key.name='shadowOffset'] > ObjectExpression > Property)`;
// Matched on `raw` rather than `value`: esquery only applies a regex attribute
// to a string, so a numeric literal never matches one on `value`. `raw` is the
// source text, so `34` matches and `'100%'` does not.
const gatedNumber = `Literal[raw=/^\\d/][value!=0][value!=1]`;
const metricLiteralMessage =
  'Raw metric. Read this value from componentMetrics, spacing, radius, or ' +
  'typography in @sdds/tokens instead of writing the number here.';

// `patterns`, not `paths`: `paths` matches an exact specifier, so
// `vitest/config` and `react-test-renderer/shallow` would slip through.
const testFrameworkPatterns = [
  {
    group: [
      'vitest',
      'vitest/*',
      'react-test-renderer',
      'react-test-renderer/*',
      '@playwright/test',
    ],
    message:
      'A test framework MUST NOT be imported by a production module. Move the ' +
      'helper to apps/mobile/src/test-support/ or into a *.test.ts(x) file.',
  },
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
            './tsconfig.json',
          ],
        },
      },
    },
    rules: {
      'quote-props': ['error', 'consistent-as-needed'],
    },
  },
  // Stylesheet scope: every gated property in an `apps/mobile/src` stylesheet.
  {
    files: ['apps/mobile/src/**/*.styles.ts'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: `${gatedStyleKey} ${gatedNumber}`,
          message: metricLiteralMessage,
        },
      ],
    },
  },
  // JSX scope: a metric written straight into markup never reaches a
  // stylesheet, so the rule above cannot see it. Covers a numeric `size` prop
  // and a gated property inside an inline `style={{ … }}` object.
  {
    files: ['apps/mobile/src/**/*.tsx'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector: `JSXAttribute[name.name='size'] ${gatedNumber}`,
          message: metricLiteralMessage,
        },
        {
          selector: `JSXAttribute[name.name='style'] ${gatedStyleKey} ${gatedNumber}`,
          message: metricLiteralMessage,
        },
      ],
    },
  },
  {
    files: ['apps/mobile/src/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', { patterns: testFrameworkPatterns }],
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
