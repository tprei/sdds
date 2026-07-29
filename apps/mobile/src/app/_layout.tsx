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
      </Stack>
      <StatusBar style="dark" />
      </ProductEventProvider>
    </AuthProvider>
  );
}
