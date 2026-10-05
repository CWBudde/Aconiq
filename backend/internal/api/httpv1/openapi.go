package httpv1

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/aconiq/backend/internal/atomicfile"
	"github.com/aconiq/backend/internal/jsonio"
)

const OpenAPIVersion = "3.1.0"

// BuildOpenAPISpec assembles the declarative OpenAPI document.
func BuildOpenAPISpec(serverURL string) map[string]any {
	spec := buildOpenAPIPaths(serverURL)
	applyTransportContract(spec)

	return spec
}

// buildOpenAPIPaths assembles the hand-built OpenAPI document. The sections are
// separate functions purely so each stays readable; the document is still one
// literal, built the same way and in the same shape.
//
// The path items and schemas live per route area in openapi_<area>.go next to
// this file. Name any new one that way: .golangci.yml scopes the goconst
// exclusion for the OpenAPI keyword vocabulary to openapi(_[a-z0-9]+)?.go by
// path, so a file named otherwise would surface that whole vocabulary as
// findings.
func buildOpenAPIPaths(serverURL string) map[string]any {
	server := "http://127.0.0.1:8080"
	if serverURL != "" {
		server = serverURL
	}

	return map[string]any{
		"openapi":    OpenAPIVersion,
		"info":       openapiInfo(),
		"servers":    openapiServers(server),
		"security":   openapiSecurity(),
		"paths":      openapiPathItems(),
		"components": openapiComponents(),
	}
}

func openapiInfo() map[string]any {
	return map[string]any{
		"title":   "Aconiq Local API",
		"version": "v1",
		"description": "Local-first API used by the Aconiq frontend and local integrations.\n\n" +
			"Every request is checked before it is routed. Its `Host` header must name a loopback " +
			"address or the host part of `aconiq serve --listen`; anything else is refused with " +
			"`forbidden_host`, which is what closes DNS rebinding. Every state-changing method must " +
			"carry a non-empty `" + ClientHeaderName + "` header — a header a CORS simple request " +
			"cannot set, so requiring it forces a preflight — and must send its body as the media " +
			"type the endpoint parses. Bearer authentication is optional and off unless the server " +
			"was started with `--api-token`.",
	}
}

func openapiServers(server string) []map[string]any {
	return []map[string]any{
		{"url": server},
	}
}

// openapiSecurity spells "optional": an empty requirement alongside the scheme
// means the token is enforced only when the server was started with one.
func openapiSecurity() []map[string]any {
	return []map[string]any{
		{},
		{"bearerAuth": []string{}},
	}
}

// openapiPathItems merges the path groups. Grouping does not affect the
// document: it is marshalled from a map, which encoding/json emits with its
// keys sorted.
func openapiPathItems() map[string]any {
	items := map[string]any{}

	for _, group := range []map[string]any{
		openapiStatusPathItems(),
		openapiRunPathItems(),
		openapiImportPathItems(),
		openapiLGLNPathItems(),
		openapiModelPathItems(),
		openapiTransformPathItems(),
	} {
		maps.Copy(items, group)
	}

	return items
}

func openapiComponents() map[string]any {
	return map[string]any{
		"securitySchemes": openapiSecuritySchemes(),
		"schemas":         openapiSchemas(),
	}
}

func openapiSecuritySchemes() map[string]any {
	return map[string]any{
		"bearerAuth": map[string]any{
			"type":   "http",
			"scheme": "bearer",
			"description": "Opt-in. `aconiq serve --api-token` (or the ACONIQ_API_TOKEN environment " +
				"variable) makes every request present this token; started without one, the API " +
				"takes no credential and relies on the transport controls described above.",
		},
	}
}

// openapiSchemas merges the component schema groups. Grouping does not affect
// the document: it is marshalled from a map, which encoding/json emits with its
// keys sorted.
func openapiSchemas() map[string]any {
	schemas := map[string]any{}

	for _, group := range []map[string]any{
		openapiErrorSchemas(),
		openapiProjectSchemas(),
		openapiRunSchemas(),
		openapiLGLNSchemas(),
		openapiRunDeleteSchemas(),
		openapiStandardSchemas(),
		openapiModelSchemas(),
		openapiTransformSchemas(),
		openapiContourSchemas(),
	} {
		maps.Copy(schemas, group)
	}

	return schemas
}

// openapiErrorSchemas describes the error envelope schemas.
func openapiErrorSchemas() map[string]any {
	return map[string]any{
		"APIError": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"code", "message"},
			"properties": map[string]any{
				"code":    map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
				"details": map[string]any{
					"type":                 "object",
					"additionalProperties": true,
				},
				"hint": map[string]any{"type": "string"},
			},
		},
		"ErrorEnvelope": map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"error"},
			"properties": map[string]any{
				"error": map[string]any{"$ref": "#/components/schemas/APIError"},
			},
		},
	}
}

func WriteOpenAPISpec(path string, serverURL string) error {
	if path == "" {
		return errors.New("openapi output path is required")
	}

	spec := BuildOpenAPISpec(serverURL)

	encoded, err := jsonio.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode openapi spec: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("create openapi output directory: %w", err)
	}

	err = atomicfile.WriteFile(path, encoded)
	if err != nil {
		return fmt.Errorf("write openapi spec %s: %w", path, err)
	}

	return nil
}

// applyTransportContract stamps the security middleware's answers onto every
// operation.
//
// The middleware is cross-cutting — it runs before routing, so its refusals can
// appear on any path — and documenting it per operation by hand is exactly the
// kind of duplication that drifts. Adding it here the way the middleware adds it
// to a request keeps handler.go and this document in step by construction.
func applyTransportContract(spec map[string]any) {
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		return
	}

	for _, item := range paths {
		operations, ok := item.(map[string]any)
		if !ok {
			continue
		}

		for method, operation := range operations {
			typed, ok := operation.(map[string]any)
			if !ok {
				continue
			}

			applyOperationTransportContract(typed, method)
		}
	}
}

func applyOperationTransportContract(operation map[string]any, method string) {
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		responses = map[string]any{}
		operation["responses"] = responses
	}

	responses["401"] = openapiErrorResponse(
		"Missing or invalid bearer token (`unauthorized`). Only reachable when the server was started with --api-token.",
	)
	responses["403"] = openapiErrorResponse(
		"Refused before routing: the Host header is not on the allowlist (`forbidden_host`), a state-changing " +
			"request arrived without the " + ClientHeaderName + " header (`client_header_required`), or the project " +
			"manifest named a path outside the project root (`forbidden_path`).",
	)

	if method == "get" {
		return
	}

	responses["413"] = openapiErrorResponse("Request body exceeds this endpoint's limit (`request_too_large`)")

	// 415 comes from requireContentType, which only an operation that parses a
	// body ever calls. Stamping it on a bodyless DELETE would document a refusal
	// the server cannot produce.
	if _, hasBody := operation["requestBody"]; hasBody {
		responses["415"] = openapiErrorResponse(
			"Request body was not sent as the media type this endpoint parses (`unsupported_media_type`)",
		)
	}

	parameters, _ := operation["parameters"].([]map[string]any)
	operation["parameters"] = append(parameters, clientHeaderParameter())
}

// clientHeaderParameter documents the custom header required on every
// state-changing method.
func clientHeaderParameter() map[string]any {
	return map[string]any{
		"name":     ClientHeaderName,
		"in":       "header",
		"required": true,
		"description": "Any non-empty value. A CORS simple request cannot set a custom header, so requiring " +
			"one forces a preflight, which the origin allowlist then answers or does not. The value is never read.",
		"schema": map[string]any{"type": "string", "minLength": 1},
	}
}

// openapiStandardContextSchema describes the standard's assessment context, the
// member `StandardRef.Context` and `StandardDescriptor.Context` travel under.
// It is one function because the same member appears on three schemas, and a
// consumer that switches on it must read the same enum everywhere.
func openapiStandardContextSchema() map[string]any {
	return map[string]any{
		"type": "string",
		"description": "Which assessment question the standard answers: `planning` for an individual " +
			"project's approval case, `mapping` for area-wide strategic noise mapping. The two are not " +
			"interchangeable, so a result carries the context it was produced under.",
		"enum": []string{"planning", "mapping"},
	}
}

// openapiModelHashSchema describes the model receipt. The same value appears on
// three schemas, and the one thing every consumer must understand about it is
// that it is never recomputed client-side.
func openapiModelHashSchema() map[string]any {
	return map[string]any{
		"type":    "string",
		"pattern": "^[0-9a-f]{64}$",
		"description": "SHA-256 of the stored normalized model, as bare lowercase hex — the spelling " +
			"provenance.json uses for input hashes. It is a receipt issued by the server: keep the value you " +
			"were handed and compare it as a string. Do not recompute it from a model you hold, because a " +
			"re-serialised model is not the same bytes.",
	}
}

func methodNotAllowedResponse() map[string]any {
	return openapiErrorResponse("Method not allowed")
}

func openapiErrorResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": map[string]any{
					"$ref": "#/components/schemas/ErrorEnvelope",
				},
			},
		},
	}
}
