# Geo Core v1 (Phase 5)

Status date: 2026-03-06

## Scope Delivered

- CRS model (`project CRS`, `import CRS`, transform pipeline contract)
- Geometry primitives:
  - point-line distance
  - point-in-polygon
  - bbox operations and constructors
- Spatial index:
  - deterministic grid-bucket index for bbox candidate queries (R-tree equivalent baseline)
- Receiver set types:
  - point receiver list
  - grid receiver set (bbox + resolution + height)
  - facade receiver set data model stub (generation deferred)

## Notes

- Transform pipeline currently supports identity transforms and explicit unsupported errors for other CRS pairs.
- Facade receiver generation is intentionally deferred; validation + data model are present.
- Unit and fuzz/property tests are included for geometry and receiver primitives.
