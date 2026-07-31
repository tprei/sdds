import { View } from 'react-native';

import { componentMetrics, radius } from '@sdds/tokens';

import { AppText } from './text';
import { avatarColorsFor, avatarInitials } from './avatar-palette';
import { styles } from './avatar.styles';

type AvatarProps = {
  name: string;
  size: number;
  ring?: boolean;
  testID?: string;
};

export function Avatar({ name, size, ring = false, testID }: AvatarProps) {
  const { background, ink } = avatarColorsFor(name);
  const initials = avatarInitials(name);

  const circle = (
    <View
      style={[
        styles.circle,
        {
          width: size,
          height: size,
          borderRadius: radius.pill,
          backgroundColor: background,
        },
      ]}
    >
      <AppText
        variant="meta"
        weight="bold"
        color={ink}
        style={{ fontSize: Math.round(size * componentMetrics.avatar.initialsScale) }}
      >
        {initials}
      </AppText>
    </View>
  );

  if (!ring) {
    return (
      <View testID={testID}>{circle}</View>
    );
  }

  return (
    <View testID={testID} style={styles.ring}>
      {circle}
    </View>
  );
}
