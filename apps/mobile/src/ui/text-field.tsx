import { forwardRef, useState } from 'react';
import type { TextInputProps } from 'react-native';
import { TextInput, View } from 'react-native';

import { semanticColors } from '@sdds/tokens';

import { AppText, resolveTextVariant } from './text';
import { styles } from './text-field.styles';

type TextFieldProps = TextInputProps & {
  label: string;
  hint?: string;
  invalid?: boolean;
  counter?: { count: number; max: number };
};

export const TextField = forwardRef<TextInput, TextFieldProps>(function TextField(
  {
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
  }: TextFieldProps,
  ref,
) {
  const [focused, setFocused] = useState(false);
  const body = resolveTextVariant('body');
  const borderColor = invalid
    ? semanticColors.danger
    : focused
      ? semanticColors.accent
      : semanticColors.borderSubtle;

  const input = (
    <TextInput
      ref={ref}
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
    <View style={[styles.ringHost, focused ? styles.ring : null]}>{row}</View>
      {hint || counter ? (
        <View style={styles.footnoteRow}>
          {hint ? (
            <AppText
              variant="xs"
              color={invalid ? semanticColors.danger : semanticColors.textMeta}
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
});
