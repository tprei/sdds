import { ScrollView, View } from 'react-native';

import { colors, semanticColors } from '@sdds/tokens';
import type { ReportReason, ReportTargetType } from '@/lib/api/reports';
import { Button } from '@/ui/button';
import { PressableScale } from '@/ui/pressable-scale';
import { Sheet } from '@/ui/sheet';
import { AppText } from '@/ui/text';
import { TextField } from '@/ui/text-field';

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
    <Sheet visible onClose={onCancel} testID="report-sheet">
      <ScrollView contentContainerStyle={styles.body} keyboardShouldPersistTaps="handled">
        <AppText accessibilityRole="header" variant="h3" weight="extraBold">
          {heading}
        </AppText>
        <AppText color={semanticColors.textMuted} variant="sm">
          Conta pra gente o que aconteceu. A denúncia não fica pública.
        </AppText>
        <View style={styles.reasonGroup}>
          {REPORT_REASON_OPTIONS.map((option) => {
            const checked = state.reason === option.value;
            return (
              <PressableScale
                key={option.value}
                accessibilityRole="radio"
                accessibilityState={{ checked }}
                disabled={pending}
                onPress={() => onReasonChange(option.value)}
                style={[styles.reasonOption, checked ? styles.reasonOptionSelected : null]}
                testID={`report-reason-${option.value}`}
              >
                <AppText variant="body">{option.label}</AppText>
              </PressableScale>
            );
          })}
        </View>
        <View style={styles.field}>
          <TextField
            counter={{ count: validation.codePointCount, max: REPORT_DETAILS_MAX_CODE_POINTS }}
            label="Quer explicar melhor? (opcional)"
            multiline
            onChangeText={onDetailsChange}
            placeholder="Quer explicar melhor? (opcional)"
            testID="report-details"
            value={state.details}
          />
          {validation.error === 'too_long' ? (
            <AppText accessibilityRole="alert" color={colors.danger500} variant="sm">
              Pode ter até 1.000 caracteres.
            </AppText>
          ) : null}
        </View>
        {state.status === 'error' ? (
          <AppText accessibilityRole="alert" color={colors.danger500} variant="sm">
            Não deu pra enviar a denúncia. Tenta de novo.
          </AppText>
        ) : null}
        {state.status === 'missing' ? (
          <AppText accessibilityRole="alert" color={colors.danger500} variant="sm">
            Esse conteúdo não está mais disponível.
          </AppText>
        ) : null}
        <View style={styles.actions}>
          <Button
            disabled={pending}
            label="Cancelar"
            onPress={onCancel}
            testID="report-cancel"
            variant="ghost"
          />
          <Button
            disabled={!canSubmitReport(state)}
            label={pending ? 'Enviando...' : 'Enviar denúncia'}
            onPress={onSubmit}
            testID="report-submit"
            variant="primary"
          />
        </View>
      </ScrollView>
    </Sheet>
  );
}
