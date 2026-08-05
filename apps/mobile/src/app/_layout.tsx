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
import { semanticColors } from '@sdds/tokens';
import { Stack } from 'expo-router';
import { StatusBar } from 'expo-status-bar';

import { AuthProvider } from '@/lib/auth/auth-provider';
import { ProductEventProvider } from '@/lib/events/product-event-provider';

SplashScreen.preventAutoHideAsync();

// Deep links and direct URLs open a nested route with nothing beneath it.
// Anchoring the stack on the tabs group puts Início under every such screen,
// so the back affordance always has somewhere to go.
export const unstable_settings = { anchor: '(tabs)' };

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
      <Stack
        screenOptions={{
          headerShown: false,
          contentStyle: { backgroundColor: semanticColors.appBackground },
        }}
      >
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="notes/[id]" />
        <Stack.Screen name="authors/[id]" />
        <Stack.Screen name="compose" options={{ presentation: 'modal' }} />
        <Stack.Screen name="login" />
        <Stack.Screen name="signup" />
        <Stack.Screen name="email" />
        <Stack.Screen name="verify-email" />
        <Stack.Screen name="recover-password" />
        <Stack.Screen name="new-password" />
        <Stack.Screen name="delete-account" />
      </Stack>
      <StatusBar style="dark" />
      </ProductEventProvider>
    </AuthProvider>
  );
}
