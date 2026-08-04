import { useCallback } from 'react';
import { useRouter } from 'expo-router';

/** Pops the navigation stack when there is a screen to return to, otherwise lands on Início. */
export function useBackFallback(): () => void {
  const router = useRouter();
  return useCallback(() => {
    if (router.canGoBack()) {
      router.back();
    } else {
      router.replace('/');
    }
  }, [router]);
}
