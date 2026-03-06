package results

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

const rasterBinaryEncoding = "float64-le-v1"

// RasterPersistence describes files written for one raster payload.
type RasterPersistence struct {
	MetadataPath string `json:"metadata_path"`
	DataPath     string `json:"data_path"`
}

type rasterMetadataFile struct {
	RasterMetadata
	DataFile   string    `json:"data_file"`
	Encoding   string    `json:"encoding"`
	CreatedAt  time.Time `json:"created_at"`
	CellCount  int       `json:"cell_count"`
	DataBytes  int       `json:"data_bytes"`
	SchemaName string    `json:"schema_name"`
}

// SaveRaster stores a raster as JSON metadata + custom binary values.
// basePath is the file prefix without extension.
func SaveRaster(basePath string, raster *Raster) (RasterPersistence, error) {
	if raster == nil {
		return RasterPersistence{}, fmt.Errorf("raster is nil")
	}
	if basePath == "" {
		return RasterPersistence{}, fmt.Errorf("base path is required")
	}

	metadataPath := basePath + ".json"
	dataPath := basePath + ".bin"

	if err := os.MkdirAll(filepath.Dir(basePath), 0o755); err != nil {
		return RasterPersistence{}, fmt.Errorf("create raster output directory: %w", err)
	}

	values := raster.Values()
	binaryPayload := make([]byte, len(values)*8)
	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return RasterPersistence{}, fmt.Errorf("raster value at index %d is non-finite", i)
		}
		binary.LittleEndian.PutUint64(binaryPayload[i*8:], math.Float64bits(value))
	}

	if err := os.WriteFile(dataPath, binaryPayload, 0o644); err != nil {
		return RasterPersistence{}, fmt.Errorf("write raster data %s: %w", dataPath, err)
	}

	meta := rasterMetadataFile{
		RasterMetadata: raster.Metadata(),
		DataFile:       filepath.Base(dataPath),
		Encoding:       rasterBinaryEncoding,
		CreatedAt:      time.Now().UTC(),
		CellCount:      len(values),
		DataBytes:      len(binaryPayload),
		SchemaName:     "aconiq.raster.v1",
	}
	encodedMeta, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return RasterPersistence{}, fmt.Errorf("encode raster metadata: %w", err)
	}
	encodedMeta = append(encodedMeta, '\n')

	if err := os.WriteFile(metadataPath, encodedMeta, 0o644); err != nil {
		return RasterPersistence{}, fmt.Errorf("write raster metadata %s: %w", metadataPath, err)
	}

	return RasterPersistence{MetadataPath: metadataPath, DataPath: dataPath}, nil
}

// LoadRaster reconstructs a raster from metadata JSON and binary data.
func LoadRaster(metadataPath string) (*Raster, error) {
	if metadataPath == "" {
		return nil, fmt.Errorf("metadata path is required")
	}

	payload, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read raster metadata %s: %w", metadataPath, err)
	}

	var metaFile rasterMetadataFile
	if err := json.Unmarshal(payload, &metaFile); err != nil {
		return nil, fmt.Errorf("decode raster metadata %s: %w", metadataPath, err)
	}

	if metaFile.Encoding != rasterBinaryEncoding {
		return nil, fmt.Errorf("unsupported raster encoding %q", metaFile.Encoding)
	}

	raster, err := NewRaster(metaFile.RasterMetadata)
	if err != nil {
		return nil, fmt.Errorf("reconstruct raster metadata: %w", err)
	}

	dataPath := metaFile.DataFile
	if !filepath.IsAbs(dataPath) {
		dataPath = filepath.Join(filepath.Dir(metadataPath), dataPath)
	}

	binaryPayload, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, fmt.Errorf("read raster binary %s: %w", dataPath, err)
	}

	expectedBytes := raster.CellCount() * 8
	if len(binaryPayload) != expectedBytes {
		return nil, fmt.Errorf("raster binary size mismatch: got %d bytes, expected %d", len(binaryPayload), expectedBytes)
	}

	for i := 0; i < raster.CellCount(); i++ {
		value := math.Float64frombits(binary.LittleEndian.Uint64(binaryPayload[i*8:]))
		raster.data[i] = value
	}

	return raster, nil
}
