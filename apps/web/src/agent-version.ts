const semanticVersion =
  /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/;
const numeric = (s: string) => /^[0-9]+$/.test(s);
function parseVersion(value: string | undefined) {
  if (!value || value.length > 64) return null;
  const parts = semanticVersion.exec(value);
  if (
    !parts ||
    parts[0] !== value ||
    (parts[4] || "")
      .split(".")
      .some((p) => numeric(p) && p.length > 1 && p[0] === "0")
  )
    return null;
  return parts;
}
export function agentVersionAtLeast(
  actual: string | undefined,
  minimum: string | undefined,
): boolean {
  const a = parseVersion(actual),
    b = parseVersion(minimum);
  if (!a || !b) return false;
  for (let i = 1; i <= 3; i++) {
    if (BigInt(a[i]) !== BigInt(b[i])) return BigInt(a[i]) > BigInt(b[i]);
  }
  if (a[4] === b[4] || !a[4]) return true;
  if (!b[4]) return false;
  const ap = a[4].split("."),
    bp = b[4].split(".");
  for (let i = 0; i < Math.min(ap.length, bp.length); i++) {
    const x = ap[i],
      y = bp[i];
    if (x === y) continue;
    const xn = numeric(x),
      yn = numeric(y);
    if (xn && yn) return BigInt(x) > BigInt(y);
    if (xn !== yn) return !xn;
    return x > y;
  }
  return ap.length >= bp.length;
}
