export function nested(xs: number[]): number {
  let n = 0;
  for (const x of xs) {
    if (x > 0) {
      if (x % 2 === 0) {
        n++;
      }
    }
  }
  return n;
}

export const arrow = (a: number): number => {
  if (a > 0) {
    return 1;
  }
  if (a < 0) {
    return -1;
  }
  return 2;
};

const label = "😀日本語"; export function afterWide(a: number): string { if (a > 0) { return label; } if (a < 0) { return ""; } return label; }

export function simple(): number {
  return 1;
}
