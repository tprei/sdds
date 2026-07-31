import type { ViewStyle } from 'react-native';

import { spacing } from '@sdds/tokens';

/**
 * Caps a full-bleed surface at the app width and turns the leftover space into
 * even outer margin.
 *
 * The design is drawn on a phone frame, so past `maxAppWidth` a wider viewport
 * should not stretch content. Every surface that spans the screen and holds
 * feed content shares this cap, otherwise they stop lining up with each other
 * on a wide viewport.
 */
export const appWidthCap: ViewStyle = {
  alignSelf: 'center',
  maxWidth: spacing.maxAppWidth,
  width: '100%',
};
