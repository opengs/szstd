package szstd

import (
	"bytes"
	"os"
	"testing"
	"testing/iotest"
)

func TestReaderIOTEST(t *testing.T) {
	contentFile := "testdata/silesia/dickens"
	dataBytes, err := os.ReadFile(contentFile)
	if err != nil {
		t.Fatalf("failed to read test data file: %v", err)
	}

	compressedData := bytes.NewBuffer([]byte{})
	compressWriter, err := NewWriter(compressedData, 1024*1024)
	if err != nil {
		t.Fatalf("failed to create szstd writer: %v", err)
	}

	_, err = compressWriter.Write(dataBytes)
	if err != nil {
		t.Fatalf("failed to write data to szstd writer: %v", err)
	}

	err = compressWriter.Close()
	if err != nil {
		t.Fatalf("failed to close szstd writer: %v", err)
	}

	readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
	if err != nil {
		t.Fatalf("failed to create szstd reader: %v", err)
	}
	defer readSeeker.Close()

	if err := iotest.TestReader(readSeeker, dataBytes); err != nil {
		t.Fatalf("iotest.TestReader failed: %v", err)
	}
}
