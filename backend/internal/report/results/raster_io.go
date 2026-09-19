package results

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/aconiq/backend/internal/jsonio"
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

// UnmarshalJSON decodes the sidecar's two halves from the same flat object.
//
// It has to exist. RasterMetadata carries its own UnmarshalJSON — the one that
// expands a legacy scalar unit — and an embedded field's methods are promoted,
// so without this the decoder would fill the metadata and silently drop
// data_file, encoding, created_at and the rest beside it. LoadRaster then
// refused every sidecar it had just written, for an empty encoding.
//
// The usual `type plain T` trick does not help: promotion comes from the
// embedded field's type, so the alias inherits the same method. The two halves
// are therefore decoded explicitly, over the same bytes.
func (f *rasterMetadataFile) UnmarshalJSON(data []byte) error {
	var sidecar struct {
		DataFile   string    `json:"data_file"`
		Encoding   string    `json:"encoding"`
		CreatedAt  time.Time `json:"created_at"`
		CellCount  int       `json:"cell_count"`
		DataBytes  int       `json:"data_bytes"`
		SchemaName string    `json:"schema_name"`
	}

	err := json.Unmarshal(data, &sidecar)
	if err != nil {
		return fmt.Errorf("raster sidecar: %w", err)
	}

	// Its own method, called directly: routing through json.Unmarshal would
	// reach the same code by promotion, and naming it says which decoder the
	// legacy scalar unit is expanded by.
	err = f.RasterMetadata.UnmarshalJSON(data)
	if err != nil {
		return err
	}

	f.DataFile = sidecar.DataFile
	f.Encoding = sidecar.Encoding
	f.CreatedAt = sidecar.CreatedAt
	f.CellCount = sidecar.CellCount
	f.DataBytes = sidecar.DataBytes
	f.SchemaName = sidecar.SchemaName

	return nil
}

// SaveRaster stores a raster as JSON metadata + custom binary values.
// basePath is the file prefix without extension.
func SaveRaster(basePath string, raster *Raster) (RasterPersistence, error) {
	if raster == nil {
		return RasterPersistence{}, errors.New("raster is nil")
	}

	if basePath == "" {
		return RasterPersistence{}, errors.New("base path is required")
	}

	metadataPath := basePath + ".json"
	dataPath := basePath + ".bin"

	err := os.MkdirAll(filepath.Dir(basePath), 0o750)
	if err != nil {
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

	err = os.WriteFile(dataPath, binaryPayload, 0o600)
	if err != nil {
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

	encodedMeta, err := jsonio.Marshal(meta)
	if err != nil {
		return RasterPersistence{}, fmt.Errorf("encode raster metadata: %w", err)
	}

	err = os.WriteFile(metadataPath, encodedMeta, 0o600)
	if err != nil {
		return RasterPersistence{}, fmt.Errorf("write raster metadata %s: %w", metadataPath, err)
	}

	return RasterPersistence{MetadataPath: metadataPath, DataPath: dataPath}, nil
}

// LoadRaster reconstructs a raster from metadata JSON and binary data.
func LoadRaster(metadataPath string) (*Raster, error) {
	if metadataPath == "" {
		return nil, errors.New("metadata path is required")
	}

	payload, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("read raster metadata %s: %w", metadataPath, err)
	}

	var metaFile rasterMetadataFile

	err = json.Unmarshal(payload, &metaFile)
	if err != nil {
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

	err = raster.decode(binaryPayload)
	if err != nil {
		return nil, err
	}

	return raster, nil
}

// DecodeRaster rebuilds a raster from its sidecar metadata and the bytes of its
// `.bin` payload, without either being on disk.
//
// LoadRaster is this plus reading two files. It is split out because the
// WebAssembly kernel is handed the payload by the browser, which is holding it
// in IndexedDB — so there is exactly one decoder of the byte contract, and the
// kernel does not carry a second copy of "band-major, row-major within a band,
// little-endian float64" for a browser to get subtly wrong.
func DecodeRaster(meta RasterMetadata, payload []byte) (*Raster, error) {
	raster, err := NewRaster(meta)
	if err != nil {
		return nil, err
	}

	err = raster.decode(payload)
	if err != nil {
		return nil, err
	}

	return raster, nil
}

// decode fills the raster from a headerless little-endian float64 payload,
// refusing one whose length does not match the shape it was declared with.
//
// The length is the only thing that says the payload matches the sidecar: the
// file carries no header, so a truncated or over-long one read against the
// declared shape reports cells from the wrong band, or past the end.
func (r *Raster) decode(payload []byte) error {
	expectedBytes := r.CellCount() * 8
	if len(payload) != expectedBytes {
		return fmt.Errorf("raster binary size mismatch: got %d bytes, expected %d", len(payload), expectedBytes)
	}

	for i := range r.data {
		r.data[i] = math.Float64frombits(binary.LittleEndian.Uint64(payload[i*8:]))
	}

	return nil
}
