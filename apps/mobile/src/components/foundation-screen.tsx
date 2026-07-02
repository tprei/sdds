import type { ReactNode } from 'react';
import type {
  StyleProp,
  TextInputProps,
  ViewStyle,
} from 'react-native';
import { Pressable, ScrollView, Text, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { semanticColors } from '@sdds/tokens';

import { styles } from './foundation-screen.styles';

type FoundationScreenProps = {
  children: ReactNode;
  description: string;
  eyebrow: string;
  title: string;
};

type EmptyStateCardProps = {
  body: string;
  title: string;
};

type FoundationTextInputProps = TextInputProps;

type FoundationButtonProps = {
  disabled?: boolean;
  label: string;
  style?: StyleProp<ViewStyle>;
};

export function FoundationScreen({
  children,
  description,
  eyebrow,
  title,
}: FoundationScreenProps) {
  return (
    <SafeAreaView style={styles.safeArea}>
      <ScrollView
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled"
      >
        <View style={styles.header}>
          <Text style={styles.eyebrow}>{eyebrow}</Text>
          <Text style={styles.title}>{title}</Text>
          <Text style={styles.description}>{description}</Text>
        </View>
        <View style={styles.stack}>{children}</View>
      </ScrollView>
    </SafeAreaView>
  );
}

export function EmptyStateCard({ body, title }: EmptyStateCardProps) {
  return (
    <View style={styles.card}>
      <Text style={styles.cardTitle}>{title}</Text>
      <Text style={styles.cardBody}>{body}</Text>
    </View>
  );
}

export function FoundationTextInput({
  multiline,
  style,
  ...props
}: FoundationTextInputProps) {
  return (
    <TextInput
      multiline={multiline}
      placeholderTextColor={semanticColors.textPlaceholder}
      style={[styles.input, multiline ? styles.textarea : null, style]}
      textAlignVertical={multiline ? 'top' : undefined}
      {...props}
    />
  );
}

export function FoundationButton({
  disabled,
  label,
  style,
}: FoundationButtonProps) {
  return (
    <Pressable
      accessibilityRole="button"
      disabled={disabled}
      style={[styles.button, style]}
    >
      <Text style={styles.buttonText}>{label}</Text>
    </Pressable>
  );
}
