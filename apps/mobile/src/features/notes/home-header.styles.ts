import { StyleSheet } from 'react-native';

export const styles = StyleSheet.create({
  // Centers TopTabs within AppHeader's flex:1 center slot, which otherwise
  // stretches its content to fill the available width.
  tabs: {
    alignItems: 'center',
  },
});
