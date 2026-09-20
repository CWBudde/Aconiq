/**
 * Generated from the local API's OpenAPI document. Do not edit.
 *
 * Source of truth: backend/internal/api/httpv1/openapi.go
 * Regenerate:      bun run generate:api   (or: just fe-api-check to verify)
 *
 * `src/api/client.ts` re-exports the schemas below under the names the app
 * uses, and is where the hand-written parts of the contract live — the two
 * shapes this document cannot express are documented there.
 */

export interface paths {
    "/api/v1/artifacts/{id}/content": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Artifact file content */
        get: operations["getArtifactContent"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/events": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Server-sent event stream
         * @description SSE stream emitting `heartbeat` and `project_status` events. Reconnect interval is 3 s.
         */
        get: operations["streamEvents"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/health": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Health check */
        get: operations["getHealth"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/import/osm": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Import OSM data for a WGS84 bounding box */
        post: operations["importOSM"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/import/terrain": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /** Import a GeoTIFF terrain model */
        post: operations["importTerrain"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/model": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Read the project model
         * @description Returns the normalized model `.noise/model/model.normalized.geojson` holds, together with its hash. Without `crs` the stored bytes are returned untouched, so the payload is exactly what the hash is a receipt for; with `crs` every coordinate is reprojected into that CRS first and `crs` in the response names what the coordinates are actually in. The hash is unchanged by the reprojection: it always describes the stored file.
         */
        get: operations["getModel"];
        put?: never;
        /**
         * Replace the project model
         * @description Accepts a GeoJSON FeatureCollection in the v1 input schema — the same document `aconiq import --input` takes — normalises it into the project CRS, validates it and writes `.noise/model/model.normalized.geojson`, the model dump and the validation report, registering the three as project artifacts. The previous model is replaced. `aconiq run` and `POST /api/v1/runs` read the normalized model from there by default.
         */
        post: operations["saveModel"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/openapi.json": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** OpenAPI v1 document */
        get: operations["getOpenAPI"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/project/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Project status */
        get: operations["getProjectStatus"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/runs": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List runs (most recent first) */
        get: operations["listRuns"];
        put?: never;
        /** Create and execute a run */
        post: operations["createRun"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/runs/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post?: never;
        /**
         * Delete a run
         * @description Removes the run from the manifest, drops every artifact ref belonging to it, and deletes `.noise/runs/{id}/`. Export bundles under `.noise/exports/` are deliberately kept on disk — a bundle may already have been delivered — and the ones whose refs were dropped are listed in `retained_paths`. A run that is still `pending` or `running` is refused: its directory is being written.
         */
        delete: operations["deleteRun"];
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/runs/{id}/contours": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /**
         * Contour lines for a run's result raster
         * @description Generates ISO-band contours from the run's result raster, in the CRS asked for. Every band the raster carries is returned in one response, told apart by `band_name`; a client showing one band filters rather than asking again. The response is a JSON envelope and not a GeoJSON FeatureCollection because RFC 7946 fixes GeoJSON to WGS84 and so cannot declare the CRS these coordinates are actually in. Only a run computed over a grid has a raster: a run over individually placed receivers is refused with `run_has_no_raster`.
         */
        get: operations["getRunContours"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/runs/{id}/log": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** Run log lines */
        get: operations["getRunLog"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/standards": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        /** List available noise standards */
        get: operations["listStandards"];
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/v1/transform": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        /**
         * Project a batch of coordinates between two CRS
         * @description Projects a flat, interleaved batch of coordinates. It reads no project and writes nothing, so it answers before `aconiq init` has run. `target_crs: "auto"` makes the same decision `aconiq run` makes — a geographic model goes into the ETRS89 / UTM zone its centre falls in, a projected one is left where it is — so a client cannot place a model where `aconiq run` would refuse to compute it. The result is a projection *of* a model, never a source *for* one: a client that draws with it must not write the projected coordinates back into the stored model.
         */
        post: operations["transformCoordinates"];
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        APIError: {
            code: string;
            details?: {
                [key: string]: unknown;
            };
            hint?: string;
            message: string;
        };
        ArtifactRef: {
            /** Format: date-time */
            created_at: string;
            id: string;
            kind: string;
            path: string;
        };
        ContourLine: {
            /** @description The raster band the line came from, e.g. `Lden`. */
            band_name: string;
            /** @description The dB level this line traces. */
            level: number;
            /** @description Vertices as [x, y] pairs in the response's `crs` — nested pairs here, unlike the flat batch /api/v1/transform takes. */
            points: number[][];
        };
        ContourResult: {
            /** @description The CRS the coordinates are actually in. Always present: a consumer handed bare lines guesses WGS84, which for a metric run puts the site in the wrong hemisphere. */
            crs: string;
            /** @description The dB step the contours were generated at, echoed because the caller may have omitted it and taken the default. */
            interval: number;
            /** @description Every band's contours in one list, told apart by `band_name`. Empty rather than null. */
            lines: components["schemas"]["ContourLine"][];
        };
        CreateRunRequest: {
            /**
             * @description Required for scaffold-tier standards: they carry no normative coefficients, their base levels are invented and they have no octave bands, so a run must acknowledge that before it may emit levels. Requests without it are rejected with error code experimental_opt_in_required.
             * @default false
             */
            experimental?: boolean;
            input_paths?: string[];
            model_path?: string;
            params?: {
                [key: string]: string;
            };
            /** @enum {string} */
            receiver_mode?: "auto-grid" | "custom";
            scenario_id?: string;
            standard_id?: string;
            standard_profile?: string;
            standard_version?: string;
        };
        DeleteRunResponse: {
            /** @description Project-relative paths that were deleted. Always present, empty rather than null. */
            removed_paths: string[];
            /** @description Files whose manifest refs were dropped but whose bytes were deliberately left in place — export bundles. Always present, empty rather than null. */
            retained_paths: string[];
            run_id: string;
        };
        ErrorEnvelope: {
            error: components["schemas"]["APIError"];
        };
        HealthResponse: {
            status: string;
            /** Format: date-time */
            time: string;
            version: string;
        };
        ImportOSMRequest: {
            east: number;
            north: number;
            /**
             * Format: uri
             * @description Optional Overpass server. It arrives in a request body, so it is constrained rather than trusted: https only, and only a known Overpass host. Anything else is refused with error code `overpass_endpoint_not_allowed`, whose details.allowed_hosts lists what is accepted. Omit it to use the default server.
             * @example https://lz4.overpass-api.de/api/interpreter
             * @example https://maps.mail.ru/api/interpreter
             * @example https://overpass-api.de/api/interpreter
             * @example https://overpass.kumi.systems/api/interpreter
             * @example https://overpass.openstreetmap.fr/api/interpreter
             * @example https://overpass.openstreetmap.ru/api/interpreter
             * @example https://overpass.osm.ch/api/interpreter
             * @example https://z.overpass-api.de/api/interpreter
             */
            overpass_endpoint?: string;
            south: number;
            west: number;
        };
        LastRunStatus: {
            /**
             * @description Which assessment question the standard answers: `planning` for an individual project's approval case, `mapping` for area-wide strategic noise mapping. The two are not interchangeable, so a result carries the context it was produced under.
             * @enum {string}
             */
            context?: "planning" | "mapping";
            /** Format: date-time */
            finished_at: string;
            id: string;
            profile?: string;
            standard_id: string;
            /** Format: date-time */
            started_at: string;
            status: string;
            version: string;
        };
        ModelResponse: {
            /** @description The CRS the returned coordinates are in — the requested one, or the project CRS when none was requested. Stated rather than inferred. */
            crs: string;
            feature_count: number;
            /** @description SHA-256 of the stored normalized model, as bare lowercase hex — the spelling provenance.json uses for input hashes. It is a receipt issued by the server: keep the value you were handed and compare it as a string. Do not recompute it from a model you hold, because a re-serialised model is not the same bytes. */
            hash: string;
            /** @description The normalized GeoJSON FeatureCollection. Returned verbatim from disk unless `crs` asked for a reprojection. */
            model: Record<string, never>;
            /** @description The project's own CRS */
            project_crs: string;
        };
        ModelSaveRequest: {
            /** @description CRS of the model's coordinates, e.g. `EPSG:4326` for coordinates drawn on a web map. Like `aconiq import --input-crs`: when it differs from the project CRS every coordinate is reprojected; when omitted the coordinates are taken to be in the project CRS already. */
            crs?: string;
            /** @description GeoJSON FeatureCollection in the v1 input schema (docs/geojson-schema-v1.md): features carry `kind` = source | building | barrier | receiver | calc-area | ground-zone. */
            model: Record<string, never>;
        };
        ModelSaveResponse: {
            /** @description Project-relative path of the model dump */
            dump_path: string;
            feature_count: number;
            /** @description SHA-256 of the stored normalized model, as bare lowercase hex — the spelling provenance.json uses for input hashes. It is a receipt issued by the server: keep the value you were handed and compare it as a string. Do not recompute it from a model you hold, because a re-serialised model is not the same bytes. */
            hash: string;
            /** @description Project-relative path of the normalized GeoJSON */
            normalized_path: string;
            /** @description Project-relative path of the validation report */
            validation_report_path: string;
            /** @description Validation warnings. The model was written despite them. */
            warnings: components["schemas"]["ValidationIssue"][];
        };
        ParameterDefinition: {
            default_value?: string;
            description?: string;
            enum?: string[];
            /** @enum {string} */
            kind: "string" | "bool" | "int" | "float";
            max?: number;
            min?: number;
            name: string;
            required: boolean;
            /** @description Physical unit of the value as a short symbol (m, km/h, dB, dB/km, 1/h, 1/km, %, °C, °). Absent when the parameter is dimensionless or not numeric. */
            unit?: string;
        };
        ProfileInfo: {
            name: string;
            parameters: components["schemas"]["ParameterDefinition"][];
            supported_indicators: string[];
            supported_source_types: string[];
        };
        /** @description The saved model's receipt. Absent until a model has been saved. A client that kept the hash POST /api/v1/model returned compares the two as strings to learn whether its local draft is still the project's model, without fetching anything. */
        ProjectModelStatus: {
            /** @description SHA-256 of the stored normalized model, as bare lowercase hex — the spelling provenance.json uses for input hashes. It is a receipt issued by the server: keep the value you were handed and compare it as a string. Do not recompute it from a model you hold, because a re-serialised model is not the same bytes. */
            hash: string;
            /**
             * Format: date-time
             * @description When the model artifact was last written
             */
            updated_at: string;
        };
        ProjectStatusResponse: {
            crs: string;
            last_run?: components["schemas"]["LastRunStatus"];
            manifest_version: number;
            model?: components["schemas"]["ProjectModelStatus"];
            name: string;
            project_id: string;
            project_path: string;
            run_count: number;
            scenario_count: number;
        };
        RunLog: {
            lines: string[];
            run_id: string;
        };
        RunSummary: {
            artifacts: components["schemas"]["ArtifactRef"][];
            /**
             * @description Which assessment question the standard answers: `planning` for an individual project's approval case, `mapping` for area-wide strategic noise mapping. The two are not interchangeable, so a result carries the context it was produced under.
             * @enum {string}
             */
            context?: "planning" | "mapping";
            /** Format: date-time */
            finished_at: string;
            id: string;
            log_path: string;
            profile?: string;
            receiver_mode?: string;
            receiver_set_id?: string;
            scenario_id: string;
            standard_id: string;
            /** Format: date-time */
            started_at: string;
            /** @enum {string} */
            status: "pending" | "running" | "completed" | "failed";
            version: string;
        };
        StandardDescriptor: {
            /**
             * @description Which assessment question the standard answers: `planning` for an individual project's approval case, `mapping` for area-wide strategic noise mapping. The two are not interchangeable, so a result carries the context it was produced under.
             * @enum {string}
             */
            context: "planning" | "mapping";
            default_version: string;
            description: string;
            /**
             * @description How far this module's output may be trusted. `normative` implements the named standard's tables and equations within the boundary declared under docs/conformance/; `preview` has sound logic but consumes preview-grade input levels; `scaffold` carries no normative coefficients and its levels are invented; `test-fixture` is a non-normative demonstrator. Only `normative` output is usable for assessment.
             * @enum {string}
             */
            evidence_tier: "normative" | "preview" | "scaffold" | "test-fixture";
            id: string;
            versions: components["schemas"]["VersionInfo"][];
        };
        TerrainInfo: {
            /** @description Bounding box [min_x, min_y, max_x, max_y] */
            bounds: number[];
            /** @description Raster dimensions [width, height] */
            grid_size: number[];
            /** @description Pixel size [width, height] in CRS units */
            pixel_size: number[];
        };
        TransformRequest: {
            /** @description Flat and interleaved — x0, y0, x1, y1, … — not nested pairs. An odd number of values is refused. This carries a model, never a receiver grid: the model is projected first and the grid is then built in the compute CRS. */
            coordinates: number[];
            /** @description CRS the coordinates are in, e.g. `EPSG:4326` or `EPSG:25832`. */
            source_crs: string;
            /** @description CRS to project into. Omit it, or send `auto`, to let the server pick the ETRS89 / UTM zone the batch's centre falls in — the same choice `aconiq run` makes. An explicit value always transforms, which is what makes the inverse direction available. */
            target_crs?: string;
        };
        TransformResponse: {
            /** @description False only when nothing moved, and then `coordinates` are the input values verbatim — not a round trip that happens to land close. */
            applied: boolean;
            /** @description The projected batch, in the same flat interleaved order as the request. */
            coordinates: number[];
            /** @description The resolved source CRS */
            source_crs: string;
            /** @description The CRS the coordinates are actually in, which for an `auto` request is the resolved zone rather than the string that was asked for. */
            target_crs: string;
        };
        ValidationIssue: {
            code: string;
            /** @description The feature the finding is about; absent for model-wide findings */
            feature_id?: string;
            message: string;
        };
        VersionInfo: {
            default_profile: string;
            name: string;
            profiles: components["schemas"]["ProfileInfo"][];
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export interface operations {
    getArtifactContent: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Artifact ID */
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Artifact file content, typed by the artifact's format */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/geo+json": Record<string, never>;
                    "application/geopackage+sqlite3": string;
                    "application/json": Record<string, never>;
                    "application/octet-stream": string;
                    "application/pdf": string;
                    "image/tiff": string;
                    "text/csv": string;
                    "text/html": string;
                    "text/markdown": string;
                    "text/plain": string;
                };
            };
            /** @description Missing artifact ID */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Artifact not found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to read artifact file */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    streamEvents: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description SSE stream */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "text/event-stream": string;
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getHealth: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description API is healthy */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["HealthResponse"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    importOSM: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ImportOSMRequest"];
            };
        };
        responses: {
            /** @description Imported GeoJSON feature collection */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": Record<string, never>;
                };
            };
            /** @description Invalid OSM import request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body was not sent as the media type this endpoint parses (`unsupported_media_type`) */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Overpass API request failed. When the server answered at all, details.upstream_status carries the status it sent. */
            502: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    importTerrain: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "multipart/form-data": {
                    /** Format: binary */
                    file: string;
                };
            };
        };
        responses: {
            /** @description Imported terrain metadata */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TerrainInfo"];
                };
            };
            /** @description Invalid terrain import request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body was not sent as the media type this endpoint parses (`unsupported_media_type`) */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to persist terrain artifact */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getModel: {
        parameters: {
            query?: {
                /** @description CRS to return the coordinates in, e.g. `EPSG:4326` to draw the model on a web map. Omit it to receive the model in the project CRS, unmodified. */
                crs?: string;
            };
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The stored model */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ModelResponse"];
                };
            };
            /** @description `crs` is not a recognised CRS identifier (`bad_request`) */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description The project is not initialized (`not_found`), or it is but no model has been saved yet (`model_not_found`). The two are separate codes because a client has to be able to tell them apart. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to read the stored model */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    saveModel: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["ModelSaveRequest"];
            };
        };
        responses: {
            /** @description Model written; the paths are project-relative */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ModelSaveResponse"];
                };
            };
            /** @description Malformed body, missing `model`, or a document that is not a FeatureCollection (`bad_request`); or a model that parsed but failed schema validation (`model_invalid`, whose details.errors lists each finding as a ValidationIssue with its feature_id). Nothing is written in either case. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body was not sent as the media type this endpoint parses (`unsupported_media_type`) */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to persist the model */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getOpenAPI: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description This OpenAPI document */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": Record<string, never>;
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getProjectStatus: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Loaded project status */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ProjectStatusResponse"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Internal server error */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    listRuns: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Run summaries ordered newest first */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunSummary"][];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Internal server error */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    createRun: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["CreateRunRequest"];
            };
        };
        responses: {
            /** @description Created run summary */
            201: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunSummary"];
                };
            };
            /** @description Invalid run request */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body was not sent as the media type this endpoint parses (`unsupported_media_type`) */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Run execution failed */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    deleteRun: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path: {
                /** @description Run ID */
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description The run was deleted; the body says what was removed and what was kept */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["DeleteRunResponse"];
                };
            };
            /** @description Missing or malformed run ID */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized, or no run with this ID (`not_found`) */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description The run is still pending or running (`run_not_finished`), or it holds export bundles inside its own directory (`export_inside_run`). Nothing was removed. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to update the manifest or remove the run directory */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getRunContours: {
        parameters: {
            query?: {
                /** @description dB step between contour levels. Must be positive; omitted it defaults to the EU END convention. */
                interval?: number;
                /** @description CRS to return the contours in, as an EPSG identifier. Defaults to `EPSG:4326`. The response always names the CRS it is actually in. */
                crs?: string;
            };
            header?: never;
            path: {
                /** @description Run ID */
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Contour lines, with the CRS and the dB interval they were generated at */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ContourResult"];
                };
            };
            /** @description Missing run ID, a non-positive or unparseable `interval`, or a `crs` that is not a recognised identifier (`bad_request`); or a CRS at either end carries no EPSG code, so the contours cannot be moved into it (`crs_not_projectable`, the same code POST /api/v1/transform answers with for the same cause). A different `crs` on the same run answers. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Project not initialized, or no run with this ID (`not_found`); or the run exists but recorded no result raster (`run_has_no_raster`), which is what a receiver mode other than an auto grid produces. */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description The run's raster declares no georeference (`run_has_no_grid`): it was computed over receivers placed individually, which record no cell size, so no `interval` and no `crs` would make this call succeed. The message is the contour package's own, worded as `aconiq export` words it. */
            409: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to read the run's raster files */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    getRunLog: {
        parameters: {
            query?: never;
            header?: never;
            path: {
                /** @description Run ID */
                id: string;
            };
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description Log lines for the requested run */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["RunLog"];
                };
            };
            /** @description Missing run ID */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Run not found */
            404: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to read log */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    listStandards: {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        requestBody?: never;
        responses: {
            /** @description List of registered noise standards with their versions and profiles */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["StandardDescriptor"][];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Standards registry not configured */
            503: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
    transformCoordinates: {
        parameters: {
            query?: never;
            header: {
                /** @description Any non-empty value. A CORS simple request cannot set a custom header, so requiring one forces a preflight, which the origin allowlist then answers or does not. The value is never read. */
                "X-Aconiq-Client": string;
            };
            path?: never;
            cookie?: never;
        };
        requestBody: {
            content: {
                "application/json": components["schemas"]["TransformRequest"];
            };
        };
        responses: {
            /** @description Projected batch; `target_crs` names the CRS the coordinates are actually in */
            200: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["TransformResponse"];
                };
            };
            /** @description Malformed body, an odd number of coordinate values, or a CRS that could not be parsed (`bad_request`); or a CRS pair that cannot be projected at all — a geographic site outside ETRS89 / UTM zones 31 to 34, or a pair with no route between them (`crs_not_projectable`, whose details.reason separates the two). The message is the projector's own, worded exactly as `aconiq run` words it. */
            400: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token. */
            401: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing request arrived without the X-Aconiq-Client header (`client_header_required`), or the project manifest named a path outside the project root (`forbidden_path`). */
            403: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Method not allowed */
            405: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body exceeds this endpoint's limit (`request_too_large`) */
            413: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Request body was not sent as the media type this endpoint parses (`unsupported_media_type`) */
            415: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
            /** @description Failed to encode the projected batch */
            500: {
                headers: {
                    [name: string]: unknown;
                };
                content: {
                    "application/json": components["schemas"]["ErrorEnvelope"];
                };
            };
        };
    };
}
