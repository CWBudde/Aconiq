import {
  ERROR_CODE_LGLN_TOO_MANY_TILES,
  ERROR_CODE_LGLN_UNAVAILABLE,
  asAPIRequestError,
} from "@/api/api-error";
import type { LonLatBBox } from "@/model/footprint";
import { m } from "@/i18n/messages";

/** The most tiles `POST /api/v1/import/lgln` loads for one request. */
export const LGLN_MAX_TILES = 9;

/**
 * Roughly how many 1 km LoD2 tiles a box touches.
 *
 * The tiles are a UTM32 grid and this is degrees, so it is an estimate and is
 * labelled one ("ca."): a kilometre is ~1/111 of a degree of latitude and
 * `cos(lat)` times that of longitude. An interval `w` km long crosses `w + 1`
 * grid lines' worth of cells on average, hence the `+ 1` per axis. The server
 * counts the real tiles and refuses above {@link LGLN_MAX_TILES}; this only
 * warns before the reader waits for that answer.
 *
 * `null` for a box that is empty or inside out.
 */
export function estimateLglnTiles(bbox: LonLatBBox): number | null {
  if (!(bbox.north > bbox.south) || !(bbox.east > bbox.west)) return null;
  const midLat = (((bbox.north + bbox.south) / 2) * Math.PI) / 180;
  const widthKm = (bbox.east - bbox.west) * 111.32 * Math.cos(midLat);
  const heightKm = (bbox.north - bbox.south) * 110.574;
  if (!Number.isFinite(widthKm) || !Number.isFinite(heightKm)) return null;
  return Math.max(1, Math.round((widthKm + 1) * (heightKm + 1)));
}

/**
 * What the reader is told when the LGLN load fails.
 *
 * The two refusals the reader can act on get a sentence of their own: a box
 * that is too large, and a download service that is down. Anything else keeps
 * the envelope's message and hint, as the OSM tab does.
 */
export function lglnFailureText(err: unknown): string {
  const apiError = asAPIRequestError(err);
  if (apiError?.code === ERROR_CODE_LGLN_TOO_MANY_TILES) {
    return m.error_lgln_too_many_tiles();
  }
  if (apiError?.code === ERROR_CODE_LGLN_UNAVAILABLE) {
    return m.error_lgln_unavailable();
  }
  if (apiError?.hint) {
    return `${apiError.message} — ${apiError.hint}`;
  }
  return err instanceof Error ? err.message : m.error_lgln_fetch_failed();
}
