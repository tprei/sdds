import { useState } from 'react';
 import type { TextInputProps } from 'react-native';
import { TextInput, View } from 'react-native';

import { colors, semanticColors } from '@sdds/tokens';

import { AppText, resolveTextVariant } from './text';
import { styles } from './text-field.styles';

type TextFieldProps = TextInputProps & {
  label: string;
  hint?: string;
  invalid?: boolean;
  counter?: { count: number; max: number };
};

export function TextField({
  label,
  hint,
  invalid = false,
  counter,
  onFocus,
  onBlur,
  multiline,
  testID,
  style,
  ...props
}: TextFieldProps) {
  const [focused, setFocused] = useState(false);
  const body = resolveTextVariant('body');
  const borderColor = invalid
    ? colors.danger500
    : focused
      ? semanticColors.accent
      : semanticColors.borderSubtle;

  const input = (
    <TextInput
      testID={testID}
      onFocus={(event) => {
        setFocused(true);
        onFocus?.(event);
      }}
      onBlur={(event) => {
        setFocused(false);
        onBlur?.(event);
      }}
      placeholderTextColor={semanticColors.textPlaceholder}
      multiline={multiline}
      style={[
        styles.input,
        body,
        { color: semanticColors.textStrong },
        multiline ? styles.inputMultiline : null,
        style,
      ]}
      {...props}
    />
  );

  const row = (
    <View style={[styles.fieldRow, multiline ? styles.fieldRowMultiline : styles.fieldRowFixed, { borderColor }]}>
      {input}
    </View>
  );

  return (
    <View style={styles.field}>
      <AppText
        variant="sm"
        weight="semibold"
        color={semanticColors.textStrong}
        style={styles.label}
      >
        {label}
      </AppText>
      {focused ? <View style={styles.ring}>{row}</View> : row}
      {hint || counter ? (
        <View style={styles.footnoteRow}>
          {hint ? (
            <AppText
              variant="xs"
              color={invalid ? colors.danger500 : semanticColors.textMeta}
              style={styles.hint}
            >
              {hint}
            </AppText>
          ) : null}
          {counter ? (
            <AppText
              variant="xs"
              color={
                counter.count > counter.max * 0.9
                  ? semanticColors.accent
                  : semanticColors.textMeta
              }
              style={styles.counter}
            >
              {`${counter.count}/${counter.max}`}
            </AppText>
          ) : null}
        </View>
      ) : null}
    </View>
  );
}
