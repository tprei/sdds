import { semanticColors, spacing, typography } from '@sdds/tokens';
import { Tabs } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform } from 'react-native';

import { AuthProvider } from '@/lib/auth/auth-provider';

export default function RootLayout() {
  return (
    <AuthProvider>
      <Tabs
        screenOptions={{
          headerStyle: { backgroundColor: semanticColors.appBackground },
          headerTitleStyle: {
            color: semanticColors.textStrong,
            fontSize: typography.sizeTitle,
            fontWeight: typography.weightBold,
          },
          headerTintColor: semanticColors.accent,
          sceneStyle: { backgroundColor: semanticColors.appBackground },
          tabBarActiveTintColor: semanticColors.accent,
          tabBarHideOnKeyboard: true,
          tabBarInactiveTintColor: semanticColors.textMeta,
          tabBarLabelStyle: {
            fontSize: typography.sizeExtraSmall,
            fontWeight: typography.weightSemibold,
          },
          tabBarStyle: {
            backgroundColor: semanticColors.cardSurface,
            borderTopColor: semanticColors.borderSubtle,
            height:
              spacing.bottomNavHeight +
              (Platform.OS === 'ios' ? spacing.sp5 : spacing.sp2),
            paddingBottom: Platform.OS === 'ios' ? spacing.sp5 : spacing.sp2,
            paddingTop: spacing.sp2,
          },
        }}
      >
        <Tabs.Screen
          name="index"
          options={{ tabBarLabel: 'Início', title: 'Explorar' }}
        />
        <Tabs.Screen name="search" options={{ title: 'Buscar' }} />
        <Tabs.Screen name="compose" options={{ title: 'Escrever' }} />
        <Tabs.Screen name="saved" options={{ title: 'Salvos' }} />
        <Tabs.Screen name="profile" options={{ title: 'Perfil' }} />
        <Tabs.Screen name="notes/[id]" options={{ href: null, title: 'Nota' }} />
        <Tabs.Screen name="authors/[id]" options={{ href: null, title: 'Perfil público' }} />
        <Tabs.Screen name="login" options={{ href: null, title: 'Entrar' }} />
        <Tabs.Screen
          name="signup"
          options={{ href: null, title: 'Criar conta' }}
        />
      </Tabs>
      <StatusBar style="dark" />
    </AuthProvider>
  );
}
