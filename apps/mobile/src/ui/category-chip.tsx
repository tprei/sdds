import { View } from 'react-native';
import { categoryFilterChipAccessibility } from '@/features/notes/category-filter';
import { semanticColors } from '@sdds/tokens';
import type { CategoryHue } from '@sdds/tokens';

import { AppText } from './text';
import { PressableScale } from './pressable-scale';
import { styles } from './category-chip.styles';

type ChipSize = 'md' | 'sm';

type CategoryChipProps = {
  hue: CategoryHue;
  label: string;
  size?: ChipSize;
  selected?: boolean;
  onPress?: () => void;
  testID?: string;
};

export function CategoryChip({
  hue,
  label,
  size = 'md',
  selected = false,
  onPress,
  testID,
}: CategoryChipProps) {
  const backgroundColor = selected
    ? semanticColors.selectionBackground
    : hue.background;
  const color = selected ? semanticColors.selectionInk : hue.ink;

  const content = (
    <AppText
      variant={size === 'sm' ? 'xs' : 'sm'}
      weight="semibold"
      color={color}
    >
      {label}
    </AppText>
  );

  const chipStyle = [
    styles.base,
    size === 'sm' ? styles.sm : styles.md,
    { backgroundColor },
    selected ? styles.selected : null,
  ];

  if (!onPress) {
    return (
      <View style={chipStyle} testID={testID}>
        {content}
      </View>
    );
  }

  return (
    <PressableScale
      testID={testID}
      onPress={onPress}
      accessibilityRole="button"
      {...categoryFilterChipAccessibility(label, selected)}
      style={chipStyle}
    >
      {content}
    </PressableScale>
  );
}

type NeutralChipProps = {
  label: string;
  selected?: boolean;
  onPress?: () => void;
  testID?: string;
};

export function NeutralChip({
  label,
  selected = false,
  onPress,
  testID,
}: NeutralChipProps) {
  const backgroundColor = selected
    ? semanticColors.selectionBackground
    : semanticColors.sunkenBackground;
  const color = selected ? semanticColors.selectionInk : semanticColors.textMuted;

  const content = (
    <AppText variant="sm" weight="semibold" color={color}>
      {label}
    </AppText>
  );

  const chipStyle = [
    styles.base,
    styles.md,
    { backgroundColor },
    selected ? styles.selected : null,
  ];

  if (!onPress) {
    return (
      <View style={chipStyle} testID={testID}>
        {content}
      </View>
    );
  }

  return (
    <PressableScale
      testID={testID}
      onPress={onPress}
      accessibilityRole="button"
      {...categoryFilterChipAccessibility(label, selected)}
      style={chipStyle}
    >
      {content}
    </PressableScale>
  );
}
