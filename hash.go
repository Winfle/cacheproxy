package cacheproxy

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func HashBytes(b []byte) string {
	hash := sha256.New()
	hash.Write(b)

	return hex.EncodeToString(hash.Sum(nil))
}

func DecompressGzip(gzipData []byte) ([]byte, error) {

	// Create a bytes reader from the GZIP-encoded byte slice.
	reader := bytes.NewReader(gzipData)

	// Create a new gzip reader.
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	var decompressedData bytes.Buffer
	if _, err := io.Copy(&decompressedData, gzipReader); err != nil {
		return nil, err
	}

	return decompressedData.Bytes(), nil
}

func CompressGzip(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}
	writer.Close()
	return buf.Bytes(), nil
}
