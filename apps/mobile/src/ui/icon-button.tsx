import type { ReactNode } from 'react';

import { motion, radius } from '@sdds/tokens';

import { PressableScale } from './pressable-scale';
import { styles } from './icon-button.styles';

type IconButtonProps = {
  icon: ReactNode;
  accessibilityLabel: string;
  onPress?: () => void;
  size?: number;
  testID?: string;
};

export function IconButton({
  icon,
  accessibilityLabel,
  onPress,
  size = 40,
  testID,
}: IconButtonProps) {
  const inset = size < 44 ? (44 - size) / 2 : 0;
  return (
    <PressableScale
      scaleTo={motion.pressIconScale}
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel}
      hitSlop={
        inset > 0
          ? { top: inset, bottom: inset, left: inset, right: inset }
          : undefined
      }
      onPress={onPress}
      style={[
        styles.base,
        { width: size, height: size, borderRadius: radius.pill },
      ]}
    >
      {icon}
    </PressableScale>
  );
}
