import type { ReactNode } from 'react';
import { Pressable, View } from 'react-native';
import { useRouter } from 'expo-router';

import { semanticColors } from '@sdds/tokens';

import { AppText } from './text';
import { IconButton } from './icon-button';
import { IconChevronLeft } from './icons';

import { styles } from './app-header.styles';

type AppHeaderProps = {
  showWordmark?: boolean;
  onWordmarkPress?: () => void;
  back?: boolean;
  center?: ReactNode;
  right?: ReactNode;
  testID?: string;
};

/**
 * The app-wide top bar: paper0 background, a bottom hairline, and the one
 * implementation of the back affordance every screen that needs it shares.
 * `center` and `right` carry each screen's own controls so their testIDs
 * and accessibility labels stay put; only the surrounding chrome moves in.
 */
export function AppHeader({
  showWordmark = false,
  onWordmarkPress,
  back = false,
  center,
  right,
  testID,
}: AppHeaderProps) {
  const router = useRouter();

  // The single canonical back behavior: pop the stack when there's a
  // screen to return to, otherwise land on Início.
  function handleBackPress() {
    if (router.canGoBack()) {
      router.back();
    } else {
      router.replace('/');
    }
  }

  // Início passes its own onWordmarkPress to scroll the feed to the top,
  // labelled accordingly. Every other screen that shows the wordmark is
  // reached by push, so pressing it without a caller-supplied handler
  // takes the user home instead.
  function handleWordmarkPress() {
    if (onWordmarkPress) {
      onWordmarkPress();
    } else {
      router.navigate('/');
    }
  }

  return (
    <View style={styles.container} testID={testID ?? 'app-header'}>
      <View style={styles.row} testID="app-header-row">
        {back ? (
          <IconButton
            accessibilityLabel="Voltar"
            icon={<IconChevronLeft />}
            onPress={handleBackPress}
          />
        ) : null}
        {showWordmark ? (
          <Pressable
            accessibilityLabel={onWordmarkPress ? 'Voltar ao topo' : 'Ir para o início'}
            accessibilityRole="button"
            onPress={handleWordmarkPress}
            style={styles.wordmark}
          >
            <AppText color={semanticColors.textStrong} variant="h3" weight="extraBold">
              sdds
            </AppText>
            <AppText color={semanticColors.accent} variant="h3" weight="extraBold">
              .
            </AppText>
          </Pressable>
        ) : null}
        {center ? <View style={styles.center}>{center}</View> : null}
        {right}
      </View>
    </View>
  );
}
