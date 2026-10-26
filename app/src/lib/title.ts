const SEP = " — ";

/** Splits `Claude Code — <folder>` into its product label and project name. */
export function splitTitle(t: string): { product: string; project: string } {
  const i = t.indexOf(SEP);
  if (i < 0) return { product: "Claude Code", project: t };
  return { product: t.slice(0, i), project: t.slice(i + SEP.length) };
}
