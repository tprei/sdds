import { useEffect, useState, type ReactNode } from 'react';
import { Animated, Easing, Modal, Pressable, View } from 'react-native';

import { motion } from '@sdds/tokens';

import { useReducedMotion } from './use-reduced-motion';
import { styles } from './sheet.styles';

type SheetProps = {
  visible: boolean;
  onClose: () => void;
  children?: ReactNode;
  testID?: string;
};

const SHEET_TRAVEL = 400;

export function Sheet({ visible, onClose, children, testID }: SheetProps) {
  const reduced = useReducedMotion();
  const [offset] = useState(() => new Animated.Value(SHEET_TRAVEL));
  useEffect(() => {
    if (reduced) {
      offset.setValue(visible ? 0 : SHEET_TRAVEL);
      return;
    }
    Animated.timing(offset, {
      toValue: visible ? 0 : SHEET_TRAVEL,
      duration: motion.durationSheet,
      easing: Easing.out(Easing.ease),
      useNativeDriver: true,
    }).start();
  }, [visible, reduced, offset]);

  if (!visible) {
    return null;
  }

  return (
    <Modal transparent animationType="none" visible={visible} onRequestClose={onClose}>
      {/*
        On web, a Pressable's onPress fires on the DOM click's first ancestor
        that has a press responder. The scrim sits behind the sheet as a
        sibling here, not a wrapper, so a click anywhere inside the sheet
        (e.g. the report textarea) never bubbles into the scrim and closes it.
      */}
      <View style={styles.root}>
        <Pressable
          accessibilityLabel="Fechar"
          accessibilityRole="button"
          onPress={onClose}
          style={styles.scrim}
        />
        <Animated.View
          style={[styles.sheet, reduced ? null : { transform: [{ translateY: offset }] }]}
        >
          <View style={styles.handle} />
          <View testID={testID}>{children}</View>
        </Animated.View>
      </View>
    </Modal>
  );
}
