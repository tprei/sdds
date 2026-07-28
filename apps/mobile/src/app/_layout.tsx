import appConfig from '../../app.json';

import { useEffect } from 'react';
import { Caveat_600SemiBold } from '@expo-google-fonts/caveat';
import {
  PlusJakartaSans_300Light,
  PlusJakartaSans_400Regular,
  PlusJakartaSans_500Medium,
  PlusJakartaSans_600SemiBold,
  PlusJakartaSans_700Bold,
  PlusJakartaSans_800ExtraBold,
} from '@expo-google-fonts/plus-jakarta-sans';
import { useFonts } from 'expo-font';
import * as SplashScreen from 'expo-splash-screen';
import { semanticColors, spacing, typography } from '@sdds/tokens';
import { Tabs } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { Platform } from 'react-native';

import { AuthProvider } from '@/lib/auth/auth-provider';
import { ProductEventProvider } from '@/lib/events/product-event-provider';

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  const [fontsLoaded, fontsError] = useFonts({
    PlusJakartaSans_300Light,
    PlusJakartaSans_400Regular,
    PlusJakartaSans_500Medium,
    PlusJakartaSans_600SemiBold,
    PlusJakartaSans_700Bold,
    PlusJakartaSans_800ExtraBold,
    Caveat_600SemiBold,
  });

  useEffect(() => {
    if (fontsLoaded || fontsError) {
      SplashScreen.hideAsync();
    }
  }, [fontsLoaded, fontsError]);

  if (!fontsLoaded && !fontsError) {
    return null;
  }

  return (
    <AuthProvider>
      <ProductEventProvider appVersion={appConfig.expo.version}>
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
      </ProductEventProvider>
    </AuthProvider>
  );
}
