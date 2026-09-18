/**
 * What another page can ask `/results` for through the URL.
 *
 * A sibling of `map/map-params.ts` rather than a fourth constant inside it.
 * That module's first line scopes it to "what another page can ask **`/model`**
 * for", and the traffic now runs both ways: the receiver table links a row to
 * the map, and the map links a receiver circle back to the row. Widening that
 * comment to "every page's parameters" would leave `SELECT_PARAM` and this one
 * looking like peers when they name different routes, are read by different
 * components and can legitimately be spelled the same in one URL without
 * meaning the same thing. One module per page keeps "who is being asked" legible
 * at the import site, which is the property that made `map-params.ts` worth
 * extracting in the first place.
 *
 * Honoured once on arrival and then stripped, exactly as `/model`'s three are,
 * so a reload or a Back does not drag the reader back to a row they have
 * scrolled away from.
 */

/**
 * The receiver `/results` should scroll to and mark, set by a click on a
 * receiver drawn on the map.
 *
 * The run is not a parameter beside it: it is the `:runId` path segment the
 * results page already resolves, so a link cannot name a receiver without
 * naming the run whose table holds it. An id that run's table does not hold
 * marks nothing, which is the right outcome for a link followed after the run
 * was deleted and recomputed.
 */
export const RECEIVER_PARAM = "receiver";
