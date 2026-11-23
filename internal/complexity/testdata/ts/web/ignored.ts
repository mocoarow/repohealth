// biome-ignore lint/complexity/noExcessiveCognitiveComplexity: fixture for a suppressed function
export function suppressed(a: number): number {
  if (a > 0) {
    return 1;
  }
  if (a < 0) {
    return -1;
  }
  return 0;
}
