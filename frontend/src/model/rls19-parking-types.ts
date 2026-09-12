// RLS-19 §3.4 Parkplatz vocabulary. Owned by the domain model; the kernel
// transport in src/wasm/types.ts re-exports these types unchanged.
//
// Field names match Go's JSON serialization exactly.

import type { Point2D } from "./geometry";

// ParkingLotType is the RLS-19 Tabelle 6 Parkplatztyp, carried by name.
// The ordinal is a table row position and moves when the table does, so it is
// deliberately not a wire format — see parking.go's UnmarshalJSON.
export type ParkingLotType = "" | "pkw" | "motorrad" | "lkw-omnibus";

// ParkingFacilityType is the RLS-19 Tabelle 7 Parkplatztyp, which seeds the
// standard movement rates. Tabelle 7 has exactly these two rows.
export type ParkingFacilityType = "" | "park-and-ride" | "tank-rastanlage";

// ParkingSource is one RLS-19 §3.4 Parkplatzteilfläche.
//
// The movement rates are nullable on purpose: null means "not stated" and is
// refused, while an explicit 0 is a legal input meaning a period with no
// movements. They are not optional keys — a missing key must serialize as a
// visible null rather than vanish.
export interface ParkingSource {
  id: string;
  center: Point2D;
  elevation_m?: number;
  area_m2: number;
  num_spaces: number;
  parking_type: ParkingLotType;
  movements_per_space_day: number | null;
  movements_per_space_night: number | null;
}
