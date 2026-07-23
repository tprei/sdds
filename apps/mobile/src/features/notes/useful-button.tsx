import { Pressable, Text } from 'react-native';

import { styles } from './useful-button.styles';

type UsefulButtonProps = {
  count: number;
  marked: boolean;
  onPress: () => void;
  pending: boolean;
};

export function UsefulButton({
  count,
  marked,
  onPress,
  pending,
}: UsefulButtonProps) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: pending, selected: marked }}
      disabled={pending}
      onPress={onPress}
      style={({ pressed }) => [
        styles.root,
        marked ? styles.rootSelected : styles.rootIdle,
        pending ? styles.rootDisabled : null,
        pressed && !pending ? styles.pressed : null,
      ]}
    >
      <Text style={[styles.label, marked ? styles.labelSelected : styles.labelIdle]}>
        Útil {count}
      </Text>
    </Pressable>
  );
}
