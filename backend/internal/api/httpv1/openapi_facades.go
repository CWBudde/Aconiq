package httpv1

// openapiFacadePathItems describes the facade elevation and window endpoints.
func openapiFacadePathItems() map[string]any {
	return map[string]any{
		"/api/v1/facades/window":  openapiFacadeWindowPathItem(),
		"/api/v1/facades/rectify": openapiFacadeRectifyPathItem(),
		"/api/v1/facades/elevation": map[string]any{
			"post": map[string]any{
				"summary":     "Unfold a building into its facades, storeys and windows",
				"operationId": "facadeElevation",
				"description": "Answers what the elevation view draws for one building of the model in the body: each " +
					"canonical edge in canonical order, with its key's endpoints in the request's `crs` and its length " +
					"in metres; the storeys' slab heights facade receiver mode places at; and the window features " +
					"keyed to each edge, with what validation finds about them. A window keyed to the building whose key " +
					"no longer names one of its edges as it is (`window.facade.moved`) is listed under `detached` " +
					"instead. A window whose `building_ref` names no building of the model " +
					"(`window.facade.unresolved`) is in no building's elevation: nothing says which building it meant, " +
					"and the validation report lists it. Like `/api/v1/transform` it reads no project and writes nothing, so it answers for the " +
					"model a client holds, saved or not. The browser's WebAssembly kernel answers the same question " +
					"from the same Go code (`aconiq.facadeElevation`).",
				"requestBody": map[string]any{
					"required": true,
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/FacadeElevationRequest"},
						},
					},
				},
				"responses": map[string]any{
					"200": map[string]any{
						"description": "The building unfolded",
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/FacadeElevationResponse"},
							},
						},
					},
					"400": openapiErrorResponse(
						"Malformed body, a missing `crs`, `model` or `building`, a model that is not a valid " +
							"FeatureCollection, a building that is not in the model or has no height, or a geographic " +
							"site with no compute CRS (`" + errorCodeBadRequest + "`).",
					),
					"405": methodNotAllowedResponse(),
					"500": openapiErrorResponse("Failed to encode the elevation"),
				},
			},
		},
	}
}

// openapiFacadeSchemas describes the facade elevation request and response.
func openapiFacadeSchemas() map[string]any {
	number := map[string]any{"type": "number"}
	pair := map[string]any{"type": "array", "items": number, "minItems": 2, "maxItems": 2}

	return map[string]any{
		"FacadeElevationRequest": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"crs", "model", facadeBuildingField},
			"properties": map[string]any{
				"crs": map[string]any{
					"type": "string",
					"description": "CRS the model's coordinates are in. Send the compute CRS a facade run would use, " +
						"so the facades are the edges its keys are numbered on.",
				},
				"model": map[string]any{
					"type": "object",
					"description": "A GeoJSON FeatureCollection in the v1 model schema, holding the building, its windows, " +
						"and every building that could stand against it, which decide its party walls.",
				},
				facadeBuildingField: map[string]any{"type": "string", "description": "The building's feature id."},
			},
		},
		"FacadeElevationResponse": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"building_ref", "height_m", "slab_heights_m", "slab_height_source", "facades", "detached"},
			"properties": map[string]any{
				"building_ref": map[string]any{"type": "string"},
				"height_m":     number,
				"levels": map[string]any{
					"type":        "integer",
					"description": "`building:levels`, absent when the building states no positive integer count.",
				},
				"slab_heights_m": map[string]any{
					"type":        "array",
					"items":       number,
					"description": "Each storey's ceiling slab above the ground, ground floor first.",
				},
				"slab_height_source": map[string]any{"type": "string", "enum": []string{"measured", "levels", "assumed"}},
				"facades": map[string]any{
					"type":  "array",
					"items": map[string]any{"$ref": "#/components/schemas/FacadeElevation"},
				},
				"detached": map[string]any{
					"type":        "array",
					"items":       map[string]any{"$ref": "#/components/schemas/ElevationWindow"},
					"description": "The windows keyed to this building whose key names no edge of it as it is now (`window.facade.moved`).",
				},
			},
		},
		"FacadeElevation":     openapiFacadeElevationSchema(number, pair),
		"ElevationWindow":     openapiElevationWindowSchema(),
		"FacadeWindowFrame":   openapiFacadeWindowFrameSchema(number),
		"PlaceWindowRequest":  openapiPlaceWindowRequestSchema(),
		"PlaceWindowResponse": openapiPlaceWindowResponseSchema(),
		"PhotoTransform":      openapiPhotoTransformSchema(pair),
		"Rectification":       openapiRectificationSchema(number),
	}
}

// openapiFacadeElevationSchema describes one facade of an elevation: its
// canonical edge, its party walls and the windows keyed to it.
func openapiFacadeElevationSchema(number, pair map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"part", "ring", "edge", "from", "to", "length_m", "facing_deg", "courtyard", "party_walls", "windows"},
		"properties": map[string]any{
			"part":     map[string]any{"type": "integer"},
			"ring":     map[string]any{"type": "integer"},
			"edge":     map[string]any{"type": "integer"},
			"from":     pair,
			"to":       pair,
			"length_m": number,
			"facing_deg": map[string]any{
				"type":        "number",
				"description": "The bearing the facade faces, its outward normal, in degrees clockwise from grid north in `crs`, in [0, 360).",
			},
			"courtyard": map[string]any{"type": "boolean", "description": "An inner ring's edge."},
			"party_walls": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"from_um", "to_um"},
					"properties":           map[string]any{"from_um": number, "to_um": number},
				},
				"description": "The stretches, in metres along the facade, another building of the model stands against: " +
					"facade mode places no point there, and a facade run over the facade refuses a sensitive window whose centre is on one.",
			},
			"windows": map[string]any{
				"type":  "array",
				"items": map[string]any{"$ref": "#/components/schemas/ElevationWindow"},
			},
		},
	}
}

// openapiElevationWindowSchema describes one window of an elevation, with its
// frame and the validation findings about it.
func openapiElevationWindowSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"id", facadeWindowField, "sensitive", "review_status", "on_party_wall", "findings"},
		"properties": map[string]any{
			"id":              map[string]any{"type": "string"},
			facadeWindowField: map[string]any{"$ref": "#/components/schemas/FacadeWindowFrame"},
			"sensitive":       map[string]any{"type": "boolean"},
			"review_status":   reviewStatusSchema(),
			"on_party_wall": map[string]any{
				"type": "boolean",
				"description": "The window's centre stands on a party wall of its facade, where facade mode places no point: " +
					"a facade run over that facade refuses it while it is sensitive and not rejected.",
			},
			"findings": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"level", "code", "message"},
					"properties": map[string]any{
						"level":      map[string]any{"type": "string", "enum": []string{"error", "warning"}},
						"code":       map[string]any{"type": "string"},
						"feature_id": map[string]any{"type": "string"},
						"message":    map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

// openapiFacadeWindowPathItem describes the window placement endpoint.
func openapiFacadeWindowPathItem() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"summary":     "Place a window drawn or moved in a facade elevation",
			"operationId": "placeFacadeWindow",
			"description": "Turns a window given in its facade's frame — the building, the canonical part, ring and " +
				"edge `/api/v1/facades/elevation` listed, and the rectangle — into the window feature the model " +
				"stores: its facade key with `from` / `to` in the request's `crs`, and a Point at its centre. It is " +
				"the placement `aconiq import --windows` makes, and refuses what that refuses. It reads no project " +
				"and writes nothing: the client puts the feature in its model and saves it. The browser's " +
				"WebAssembly kernel answers from the same Go code (`aconiq.placeWindow`).",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/PlaceWindowRequest"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "The window feature",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/PlaceWindowResponse"},
						},
					},
				},
				"400": openapiErrorResponse(
					"Malformed body, a missing `crs` or `model`, a model that is not a valid FeatureCollection, or a " +
						"window that cannot be placed: a building not in the model, an edge it does not have, a frame " +
						"that is not finite and positive, or one its edge has no room for (`" + errorCodeBadRequest + "`).",
				),
				"405": methodNotAllowedResponse(),
				"500": openapiErrorResponse("Failed to encode the window"),
			},
		},
	}
}

// openapiFacadeWindowFrameSchema is a window's rectangle in its facade's
// frame.
func openapiFacadeWindowFrameSchema(number map[string]any) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"u_m", "sill_z_m", "width_m", "height_m"},
		"description":          "The window's rectangle in its facade's frame, in metres.",
		"properties": map[string]any{
			"u_m": number, "sill_z_m": number, "width_m": number, "height_m": number,
		},
	}
}

func reviewStatusSchema() map[string]any {
	return map[string]any{"type": "string", "enum": []string{"proposed", "confirmed", "rejected"}}
}

func openapiPlaceWindowRequestSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"crs", "model", facadeWindowField},
		"properties": map[string]any{
			"crs": map[string]any{
				"type":        "string",
				"description": "CRS the model's coordinates are in: the one the elevation was asked in.",
			},
			"model": map[string]any{
				"type":        "object",
				"description": "A GeoJSON FeatureCollection in the v1 model schema, holding the window's building.",
			},
			facadeWindowField: map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"id", "building_ref", "part", "ring", "edge", facadeWindowField, "sensitive"},
				"properties": map[string]any{
					"id":              map[string]any{"type": "string", "description": "The window's feature id, without '/'."},
					"building_ref":    map[string]any{"type": "string"},
					"part":            map[string]any{"type": "integer"},
					"ring":            map[string]any{"type": "integer"},
					"edge":            map[string]any{"type": "integer"},
					facadeWindowField: map[string]any{"$ref": "#/components/schemas/FacadeWindowFrame"},
					"sensitive":       map[string]any{"type": "boolean"},
					"review_status":   reviewStatusSchema(),
					"evidence":        map[string]any{"type": "array", "items": map[string]any{"type": "object"}},
				},
			},
		},
	}
}

func openapiPlaceWindowResponseSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"feature"},
		"properties": map[string]any{
			"feature": map[string]any{
				"type": "object",
				"description": "The window feature: `kind: window`, its `facade` key and `window` frame, " +
					"`sensitive` and `review_status`, and a Point geometry at its centre, in the request's `crs`.",
			},
		},
	}
}

// openapiFacadeRectifyPathItem describes the photo rectification endpoint.
func openapiFacadeRectifyPathItem() map[string]any {
	return map[string]any{
		"post": map[string]any{
			"summary":     "Rectify a facade photo onto the facade's frame",
			"operationId": "rectifyFacade",
			"description": "Solves the homography that maps a photo of a facade onto the facade's frame, from four " +
				"point pairs: the facade's corners as clicked in the photo (pixels, y running down) and where they " +
				"are on the facade (`u` and `z` in metres). The elevation view draws the photo with it, and a window " +
				"drawn over the photo records the pairs and the photo's SHA-256 as its evidence, never the photo. It " +
				"is the solve window validation checks that evidence with, so the same pairs always give the same " +
				"matrix. It reads no project and writes nothing. The browser's WebAssembly kernel answers from the " +
				"same Go code (`aconiq.rectifyFacade`).",
			"requestBody": map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{"$ref": "#/components/schemas/PhotoTransform"},
					},
				},
			},
			"responses": map[string]any{
				"200": map[string]any{
					"description": "The homography and its inverse",
					"content": map[string]any{
						"application/json": map[string]any{
							"schema": map[string]any{"$ref": "#/components/schemas/Rectification"},
						},
					},
				},
				"400": openapiErrorResponse(
					"Malformed body, a point list that is not four [x, y] pairs, a point that is not finite, " +
						"corners that are not a convex quadrilateral in the order given, or a mirrored photo (`" +
						errorCodeBadRequest + "`).",
				),
				"405": methodNotAllowedResponse(),
				"500": openapiErrorResponse("Failed to encode the rectification"),
			},
		},
	}
}

// openapiPhotoTransformSchema is the four point pairs of a photo, as a
// window's evidence `transform` records them.
func openapiPhotoTransformSchema(pair map[string]any) map[string]any {
	four := func(description string) map[string]any {
		return map[string]any{
			"type": "array", "items": pair, "minItems": 4, "maxItems": 4, "description": description,
		}
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"image_points", "facade_points"},
		"properties": map[string]any{
			"image_points": four("The facade's corners in the photo, in pixels with y running down: " +
				"bottom-left, bottom-right, top-right, top-left, seen from outside."),
			"facade_points": four("Where those corners are in the facade's frame: `u` along the facade and `z` " +
				"above its base, in metres."),
		},
	}
}

// openapiRectificationSchema is the homography a PhotoTransform solves to.
func openapiRectificationSchema(number map[string]any) map[string]any {
	nine := func(description string) map[string]any {
		return map[string]any{
			"type": "array", "items": number, "minItems": 9, "maxItems": 9, "description": description,
		}
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"matrix", "inverse"},
		"properties": map[string]any{
			"matrix": nine("Row-major 3×3 homography from a pixel to (u, z) on the facade, scaled so that w is " +
				"positive over the photographed facade."),
			"inverse": nine("Row-major 3×3 homography from (u, z) on the facade back to a pixel."),
		},
	}
}
