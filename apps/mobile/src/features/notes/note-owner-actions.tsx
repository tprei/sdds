import { View } from 'react-native';

import { semanticColors } from '@sdds/tokens';

import { Button } from '@/ui/button';
import { PressableScale } from '@/ui/pressable-scale';
import { Sheet } from '@/ui/sheet';
import { AppText } from '@/ui/text';

import { styles } from './note-owner-actions.styles';

export type NoteOwnerActionsStep = 'closed' | 'menu' | 'confirmDelete';

export type NoteOwnerActionsProps = {
  deleting: boolean;
  onCancel: () => void;
  onConfirmDelete: () => void;
  onEdit: () => void;
  step: NoteOwnerActionsStep;
};

export function NoteOwnerActions({
  deleting,
  onCancel,
  onConfirmDelete,
  onEdit,
  step,
}: NoteOwnerActionsProps) {
  if (step === 'closed') {
    return null;
  }

  return (
    <Sheet visible onClose={deleting ? () => {} : onCancel} testID="note-owner-sheet">
      <View style={styles.body}>
        {step === 'menu' ? (
          <View style={styles.menu}>
            <PressableScale
              accessibilityLabel="Editar nota"
              accessibilityRole="button"
              onPress={onEdit}
              style={styles.menuRow}
            >
              <AppText color={semanticColors.textStrong} variant="body" weight="semibold">
                Editar
              </AppText>
            </PressableScale>
            <PressableScale
              accessibilityLabel="Excluir nota"
              accessibilityRole="button"
              onPress={onConfirmDelete}
              style={styles.menuRow}
            >
              <AppText color={semanticColors.danger} variant="body" weight="semibold">
                Excluir
              </AppText>
            </PressableScale>
          </View>
        ) : (
          <View style={styles.prompt}>
            <AppText
              accessibilityRole="header"
              color={semanticColors.textStrong}
              variant="bodyLg"
              weight="bold"
            >
              Excluir nota?
            </AppText>
            <AppText color={semanticColors.textBody} variant="body">
              Isso apaga a nota e os comentários dela pra sempre.
            </AppText>
            <View style={styles.actions}>
              <Button disabled={deleting} label="Cancelar" onPress={onCancel} variant="secondary" />
              <Button
                disabled={deleting}
                label={deleting ? 'Excluindo…' : 'Excluir'}
                onPress={onConfirmDelete}
                testID="note-delete-confirm"
                variant="primary"
              />
            </View>
          </View>
        )}
      </View>
    </Sheet>
  );
}
