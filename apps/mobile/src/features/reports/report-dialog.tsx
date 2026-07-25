import { Modal, Pressable, ScrollView, Text, View } from 'react-native';

import {
  FoundationButton,
  FoundationTextInput,
} from '@/components/foundation-screen';
import type { ReportReason, ReportTargetType } from '@/lib/api/reports';

import {
  canSubmitReport,
  REPORT_DETAILS_MAX_CODE_POINTS,
  REPORT_REASON_OPTIONS,
  validateReportDetails,
  type ReportFormState,
  type ReportTarget,
} from './report-form';
import { styles } from './report-dialog.styles';

export type ReportDialogProps = {
  target: ReportTarget | null;
  state: ReportFormState;
  onReasonChange: (reason: ReportReason) => void;
  onDetailsChange: (details: string) => void;
  onCancel: () => void;
  onSubmit: () => void;
};

const headingByTargetType: Record<ReportTargetType, string> = {
  note: 'Denunciar nota',
  comment: 'Denunciar comentário',
};

// The card is a pressable so it absorbs taps that would otherwise dismiss the
// dialog through the backdrop; the backdrop itself owns the cancel behavior.
function absorbPress(): void {
  /* no-op: keep the press inside the dialog */
}

export function ReportDialog({
  target,
  state,
  onReasonChange,
  onDetailsChange,
  onCancel,
  onSubmit,
}: ReportDialogProps) {
  if (target === null) {
    return null;
  }

  const pending = state.status === 'pending';
  const validation = validateReportDetails(state.details);
  const heading = headingByTargetType[target.type];

  return (
    <Modal
      animationType="fade"
      accessibilityViewIsModal
      transparent
      visible
      onRequestClose={pending ? undefined : onCancel}
    >
      <Pressable
        accessible={false}
        disabled={pending}
        onPress={onCancel}
        style={styles.backdrop}
        testID="report-backdrop"
      >
        <Pressable
          accessibilityLabel={heading}
          accessibilityRole="none"
          onPress={absorbPress}
          style={styles.dialog}
        >
          <ScrollView contentContainerStyle={styles.body} keyboardShouldPersistTaps="handled">
            <Text accessibilityRole="header" style={styles.heading}>
              {heading}
            </Text>
            <Text style={styles.intro}>
              Conta pra gente o que aconteceu. A denúncia não fica pública.
            </Text>
            <View style={styles.reasonGroup}>
              {REPORT_REASON_OPTIONS.map((option) => {
                const checked = state.reason === option.value;
                return (
                  <Pressable
                    key={option.value}
                    accessibilityRole="radio"
                    accessibilityState={{ checked }}
                    disabled={pending}
                    onPress={() => onReasonChange(option.value)}
                    style={({ pressed }) => [
                      styles.reasonOption,
                      checked ? styles.reasonOptionSelected : null,
                      pressed ? styles.reasonOptionPressed : null,
                    ]}
                    testID={`report-reason-${option.value}`}
                  >
                    <Text
                      style={[
                        styles.reasonLabel,
                        checked ? styles.reasonLabelSelected : null,
                      ]}
                    >
                      {option.label}
                    </Text>
                  </Pressable>
                );
              })}
            </View>
            <View style={styles.field}>
              <Text style={styles.detailsLabel}>
                Quer explicar melhor? (opcional)
              </Text>
              <FoundationTextInput
                accessibilityLabel="Quer explicar melhor? (opcional)"
                multiline
                onChangeText={onDetailsChange}
                placeholder="Quer explicar melhor? (opcional)"
                style={styles.detailsInput}
                testID="report-details"
                value={state.details}
              />
              <Text style={styles.counter}>
                {validation.codePointCount}/{REPORT_DETAILS_MAX_CODE_POINTS}
              </Text>
              {validation.error === 'too_long' ? (
                <Text accessibilityRole="alert" style={styles.detailsError}>
                  Pode ter até 1.000 caracteres.
                </Text>
              ) : null}
            </View>
            {state.status === 'error' ? (
              <Text accessibilityRole="alert" style={styles.inlineNotice}>
                Não deu pra enviar a denúncia. Tenta de novo.
              </Text>
            ) : null}
            {state.status === 'missing' ? (
              <Text accessibilityRole="alert" style={styles.inlineNotice}>
                Esse conteúdo não está mais disponível.
              </Text>
            ) : null}
            <View style={styles.actions}>
              <FoundationButton
                disabled={pending}
                label="Cancelar"
                onPress={onCancel}
                testID="report-cancel"
              />
              <FoundationButton
                disabled={!canSubmitReport(state)}
                label={pending ? 'Enviando...' : 'Enviar denúncia'}
                onPress={onSubmit}
                testID="report-submit"
              />
            </View>
          </ScrollView>
        </Pressable>
      </Pressable>
    </Modal>
  );
}
