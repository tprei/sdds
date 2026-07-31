import { useState } from 'react';
import { TextInput, View } from 'react-native';

import { componentMetrics, motion, semanticColors } from '@sdds/tokens';

import { resolveTextVariant } from './text';
import { IconSearch, IconX } from './icons';
import { PressableScale } from './pressable-scale';
import { styles } from './search-field.styles';

type SearchSize = 'lg' | 'md';

type SearchFieldProps = {
  value: string;
  onChangeText: (text: string) => void;
  onSubmit: (value: string) => void;
  onClear?: () => void;
  placeholder?: string;
  size?: SearchSize;
  autoFocus?: boolean;
  testID?: string;
};

export function SearchField({
  value,
  onChangeText,
  onSubmit,
  onClear,
  placeholder,
  size = 'lg',
  autoFocus,
  testID,
}: SearchFieldProps) {
  const [focused, setFocused] = useState(false);
  const { height } = componentMetrics.search[size];
  const font =
    size === 'lg' ? resolveTextVariant('bodyLg') : resolveTextVariant('body');
  const borderColor = focused
    ? semanticColors.accent
    : semanticColors.borderSubtle;

  const row = (
    <View style={[styles.row, { height, borderColor }]}>
      <IconSearch size={componentMetrics.icon.sm} color={semanticColors.textMeta} />
      <TextInput
        testID={testID}
        value={value}
        onChangeText={onChangeText}
        placeholder={placeholder}
        placeholderTextColor={semanticColors.textPlaceholder}
        autoFocus={autoFocus}
        returnKeyType="search"
        onSubmitEditing={() => onSubmit(value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        style={[styles.input, font, { color: semanticColors.textStrong }]}
      />
      {value ? (
        <PressableScale
          scaleTo={motion.pressIconScale}
          onPress={onClear}
          accessibilityRole="button"
          accessibilityLabel="Limpar busca"
          style={styles.clear}
          hitSlop={componentMetrics.field.clearHitSlop}
        >
          <IconX size={componentMetrics.icon.clear} color={semanticColors.textMeta} />
        </PressableScale>
      ) : null}
    </View>
  );

  return (
    <View style={[styles.ringHost, focused ? styles.ring : null]}>{row}</View>
  );
}
