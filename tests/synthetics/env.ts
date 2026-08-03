export function requiredEnv(name: string, value: string | undefined): string {
  if (!value) {
    throw new Error(
      `${name} is required; run synthetics through \`pnpm smoke synthetics\``,
    );
  }
  return value;
}
