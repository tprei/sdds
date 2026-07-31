import { View } from 'react-native';

import { componentMetrics, semanticColors } from '@sdds/tokens';

import { AppText } from '@/ui/text';
import { IconPencil } from '@/ui/icons';
import { MetricStat } from '@/ui/metric-stat';
import { PressableScale } from '@/ui/pressable-scale';

import { styles } from './note-action-bar.styles';

type NoteActionBarProps = {
  commentCount: number;
  onFocusComposer: () => void;
  useful: {
    count: number;
    marked: boolean;
    pending: boolean;
    onToggle: () => void;
  };
};

export function NoteActionBar({
  commentCount,
  onFocusComposer,
  useful,
}: NoteActionBarProps) {
  return (
    <View style={styles.root}>
      <PressableScale
        style={styles.pill}
        accessibilityRole="button"
        accessibilityLabel="Diz alguma coisa boa…"
        onPress={onFocusComposer}
      >
        <IconPencil size={componentMetrics.icon.edit} color={semanticColors.textMeta} />
        <AppText variant="sm" color={semanticColors.textMeta}>
          Diz alguma coisa boa…
        </AppText>
      </PressableScale>
      <MetricStat
        kind="useful"
        size="md"
        count={useful.count}
        active={useful.marked}
        pending={useful.pending}
        onPress={useful.onToggle}
        accessibilityLabel={
          useful.marked ? 'Desmarcar útil' : 'Marcar como útil'
        }
      />
      <MetricStat
        kind="comment"
        size="md"
        count={commentCount}
        accessibilityLabel={`${commentCount} comentários`}
      />
    </View>
  );
}
