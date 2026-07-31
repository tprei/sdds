import { TextInput, View } from 'react-native';

import { colors, semanticColors } from '@sdds/tokens';

import { AppText, resolveTextVariant } from '@/ui/text';

import { styles } from './post-it-composer.styles';

const titleVariant = resolveTextVariant('h3');
const bodyVariant = resolveTextVariant('bodyLg');

type PostItComposerProps = {
  body: string;
  bodyMax: number;
  editable?: boolean;
  onChangeBody: (value: string) => void;
  onChangeTitle: (value: string) => void;
  title: string;
};

export function PostItComposer({
  body,
  bodyMax,
  editable = true,
  onChangeBody,
  onChangeTitle,
  title,
}: PostItComposerProps) {
  const codePointCount = Array.from(body).length;
  const counterColor =
    codePointCount > bodyMax * 0.9
      ? semanticColors.accent
      : semanticColors.textMeta;

  return (
    <View style={styles.sheet}>
      <AppText
        color={colors.yellow400}
        style={styles.quote}
        variant="h1"
        weight="extraBold"
      >
        “
      </AppText>
      <TextInput
        accessibilityLabel="Título da nota"
        editable={editable}
        onChangeText={onChangeTitle}
        placeholder="Compartilhe seu achado"
        placeholderTextColor={semanticColors.textPlaceholder}
        style={[styles.title, titleVariant]}
        value={title}
      />
      <TextInput
        accessibilityLabel="Texto da nota"
        editable={editable}
        multiline
        onChangeText={onChangeBody}
        placeholder="Conta como foi, o que ajudou, o que você gostaria de ter sabido antes…"
        placeholderTextColor={semanticColors.textPlaceholder}
        style={[styles.body, bodyVariant]}
        value={body}
      />
      <AppText color={counterColor} style={styles.counter} variant="xs">
        {`${codePointCount}/${bodyMax}`}
      </AppText>
    </View>
  );
}
