/**
 * The query flag that asks `/model` to arm a draw tool on arrival, set by the
 * project page's "Start drawing".
 *
 * A boolean rather than a mode name, so which mode drawing starts in stays a
 * decision the map makes once. Its own module because both pages need it and
 * neither should import the other for a string.
 */
export const DRAW_PARAM = "draw";
