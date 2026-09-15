/**
 * What another page can ask `/model` for through the URL.
 *
 * Their own module because several pages need these strings and none of them
 * should import another page for one. Each is honoured once on arrival and
 * then stripped, so a reload or a Back does not act on it again.
 */

/**
 * The query flag that asks `/model` to arm a draw tool on arrival, set by the
 * project page's "Start drawing".
 *
 * A boolean rather than a mode name, so which mode drawing starts in stays a
 * decision the map makes once.
 */
export const DRAW_PARAM = "draw";

/**
 * The id of the feature `/model` should open the editor on, set by the import
 * page's done step so a finding can be answered where the feature is.
 *
 * An id rather than an index: the import page and the map hold different
 * arrays, and an id is what the project, the validator and the store all key
 * on. An id the model does not hold selects nothing, which is the right
 * outcome for a link followed after the feature was deleted.
 */
export const SELECT_PARAM = "select";
