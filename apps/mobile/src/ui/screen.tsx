import type { ReactNode } from 'react';
import { ScrollView, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { styles } from './screen.styles';

type ScreenProps = {
  children?: ReactNode;
  header?: ReactNode;
  scroll?: boolean;
  testID?: string;
};

export function Screen({ children, header, scroll = true, testID }: ScreenProps) {
  return (
    <SafeAreaView style={styles.container}>
      {header ? <View style={styles.header}>{header}</View> : null}
      {scroll ? (
        <ScrollView
          style={styles.scroll}
          keyboardShouldPersistTaps="handled"
          contentContainerStyle={styles.content}
          testID={testID}
        >
          {children}
        </ScrollView>
      ) : (
        <View style={styles.body} testID={testID}>
          {children}
        </View>
      )}
    </SafeAreaView>
  );
}
