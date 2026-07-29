const MINUTE = 60_000;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

export function relativeTimeLabel(iso: string, now: Date): string {
  const then = new Date(iso).getTime();
  const elapsed = now.getTime() - then;
  if (elapsed < MINUTE) {
    return 'agora';
  }
  const minutes = Math.floor(elapsed / MINUTE);
  if (minutes < 60) {
    return `há ${minutes} min`;
  }
  const hours = Math.floor(elapsed / HOUR);
  if (hours < 24) {
    return `há ${hours} h`;
  }
  const days = Math.floor(elapsed / DAY);
  if (days < 7) {
    return `há ${days} ${days === 1 ? 'dia' : 'dias'}`;
  }
  return formatMonthDay(then);
}

function formatMonthDay(timestamp: number): string {
  const formatted = new Intl.DateTimeFormat('pt-BR', {
    day: 'numeric',
    month: 'short',
  }).format(new Date(timestamp));
  return formatted.replace(' de ', ' ').replace(/\.$/, '');
}
