/**
 * Whether the reader has asked their system for reduced motion.
 *
 * Read per use rather than memoised: the preference can change mid-session,
 * and `matchMedia` is a cheap synchronous lookup. The `typeof` guard is not
 * defensive padding — jsdom provides `matchMedia` only because
 * `vitest.setup.ts` stubs it, and a renderer that forgets to is better served
 * by the animated default than by a thrown effect.
 *
 * `globals.css` carries its own `prefers-reduced-motion` block, which collapses
 * CSS transitions. Nothing on the map canvas is CSS: MapLibre paints from the
 * style, so the camera and the flash have to ask the question themselves.
 */
export function prefersReducedMotion(): boolean {
  if (typeof window.matchMedia !== "function") return false;
  return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
}
