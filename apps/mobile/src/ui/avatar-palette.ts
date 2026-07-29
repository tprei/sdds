export const avatarPalette = [
  { background: '#F4E2D8', ink: '#B45B3E' },
  { background: '#F1E0E6', ink: '#97516A' },
  { background: '#E0EDE7', ink: '#46796A' },
  { background: '#F3E8D2', ink: '#A8702B' },
  { background: '#E1EAEF', ink: '#4F7388' },
  { background: '#F4E3EA', ink: '#B06C86' },
] as const;

export function avatarColorsFor(name: string) {
  let hash = 0;
  for (let index = 0; index < name.length; index += 1) {
    hash = (hash * 31 + name.charCodeAt(index)) % avatarPalette.length;
  }
  return avatarPalette[hash];
}

export function avatarInitials(name: string): string {
  const words = name.trim().split(/\s+/).filter((word) => word.length > 0);
  if (words.length === 0) return '?';
  return words
    .slice(0, 2)
    .map((word) => word[0])
    .join('')
    .toUpperCase();
}
