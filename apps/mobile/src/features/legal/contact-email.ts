import { Linking } from 'react-native';

import { contactEmail } from './legal-content';

// Opens the user's mail client addressed to the published contact address. A
// device without a mail client must not crash the screen, and the address is
// always rendered as visible text as well, so a failed openURL never hides it.
export async function openContactEmail(): Promise<void> {
  try {
    await Linking.openURL(`mailto:${contactEmail}`);
  } catch {
    // Swallowed on purpose: the address is also shown as text.
  }
}
