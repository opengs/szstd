package szstd

import (
	"bytes"
	"os"
	"testing"
	"testing/iotest"
)

func FuzzReaderIOTest(f *testing.F) {
	// Input file index | frame size
	f.Add(0, 1024*1024)
	f.Add(1, 512*1024)
	f.Add(2, 256*1024)
	f.Add(3, 1024)

	// Load all test files into memory
	testFilesData := make([][]byte, len(testFiles))
	for i, file := range testFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			f.Fatalf("failed to read test data file %s: %v", file, err)
		}
		testFilesData[i] = data
	}

	f.Fuzz(func(t *testing.T, fileIndex int, frameSize int) {
		// Sanitize inputs
		fileIndex = int(uint32(fileIndex) % uint32(len(testFilesData)))
		frameSize = int(1024 + uint32(frameSize)%(10*1024*1024-1024)) // between 1KB and 10MB

		data := testFilesData[fileIndex]

		compressedBuf := bytes.NewBuffer(nil)

		// Create szstd writer
		writer, err := NewWriter(compressedBuf, frameSize)
		if err != nil {
			t.Fatalf("failed to create szstd writer: %v", err)
		}

		// Write data
		_, err = writer.Write(data)
		if err != nil {
			writer.Close()
			t.Fatalf("failed to write data to szstd writer: %v", err)
		}

		// Close writer
		err = writer.Close()
		if err != nil {
			t.Fatalf("failed to close szstd writer: %v", err)
		}

		// Decompress and verify data by standard zstd reader
		reader, err := NewReadSeeker(bytes.NewReader(compressedBuf.Bytes()))
		if err != nil {
			t.Fatalf("failed to create szstd reader: %v", err)
		}

		if err := iotest.TestReader(reader, data); err != nil {
			reader.Close()
			t.Fatalf("iotest.TestReader failed: %v", err)
		}

		reader.Close()
	})
}
