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

func TestSeekToExactEndOfFile(t *testing.T) {
	// Create test data
	testData := []byte("Hello, World! This is test data for seeking to EOF.")
	dataLen := int64(len(testData))

	// Compress the data
	compressedData := bytes.NewBuffer([]byte{})
	compressWriter, err := NewWriter(compressedData, 1024)
	if err != nil {
		t.Fatalf("failed to create szstd writer: %v", err)
	}
	_, err = compressWriter.Write(testData)
	if err != nil {
		t.Fatalf("failed to write data: %v", err)
	}
	err = compressWriter.Close()
	if err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	t.Run("SeekStart to exact EOF", func(t *testing.T) {
		readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
		if err != nil {
			t.Fatalf("failed to create reader: %v", err)
		}
		defer readSeeker.Close()

		// Seek to exact end of file using SeekStart
		pos, err := readSeeker.Seek(dataLen, 0)
		if err != nil {
			t.Fatalf("Seek to EOF failed: %v", err)
		}
		if pos != dataLen {
			t.Errorf("expected position %d, got %d", dataLen, pos)
		}

		// Reading at EOF should return EOF error
		buf := make([]byte, 10)
		n, err := readSeeker.Read(buf)
		if n != 0 {
			t.Errorf("expected 0 bytes read at EOF, got %d", n)
		}
		if err == nil {
			t.Error("expected EOF error at exact EOF position")
		}
	})

	t.Run("SeekEnd with offset 0 to exact EOF", func(t *testing.T) {
		readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
		if err != nil {
			t.Fatalf("failed to create reader: %v", err)
		}
		defer readSeeker.Close()

		// Seek to exact end of file using SeekEnd with offset 0
		pos, err := readSeeker.Seek(0, 2)
		if err != nil {
			t.Fatalf("Seek to EOF failed: %v", err)
		}
		if pos != dataLen {
			t.Errorf("expected position %d, got %d", dataLen, pos)
		}

		// Reading at EOF should return EOF error
		buf := make([]byte, 10)
		n, err := readSeeker.Read(buf)
		if n != 0 {
			t.Errorf("expected 0 bytes read at EOF, got %d", n)
		}
		if err == nil {
			t.Error("expected EOF error at exact EOF position")
		}
	})

	t.Run("SeekCurrent to exact EOF", func(t *testing.T) {
		readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
		if err != nil {
			t.Fatalf("failed to create reader: %v", err)
		}
		defer readSeeker.Close()

		// First, read half the data
		halfLen := len(testData) / 2
		buf := make([]byte, halfLen)
		n, err := readSeeker.Read(buf)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if n != halfLen {
			t.Fatalf("expected to read %d bytes, got %d", halfLen, n)
		}

		// Seek to exact end using SeekCurrent
		remainingLen := dataLen - int64(halfLen)
		pos, err := readSeeker.Seek(remainingLen, 1)
		if err != nil {
			t.Fatalf("Seek to EOF failed: %v", err)
		}
		if pos != dataLen {
			t.Errorf("expected position %d, got %d", dataLen, pos)
		}

		// Reading at EOF should return EOF error
		n, err = readSeeker.Read(buf)
		if n != 0 {
			t.Errorf("expected 0 bytes read at EOF, got %d", n)
		}
		if err == nil {
			t.Error("expected EOF error at exact EOF position")
		}
	})

	t.Run("Seek back from EOF and read again", func(t *testing.T) {
		readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
		if err != nil {
			t.Fatalf("failed to create reader: %v", err)
		}
		defer readSeeker.Close()

		// Seek to exact EOF
		pos, err := readSeeker.Seek(0, 2)
		if err != nil {
			t.Fatalf("Seek to EOF failed: %v", err)
		}
		if pos != dataLen {
			t.Errorf("expected position %d, got %d", dataLen, pos)
		}

		// Seek back from EOF by 10 bytes
		pos, err = readSeeker.Seek(-10, 1)
		if err != nil {
			t.Fatalf("Seek back from EOF failed: %v", err)
		}
		expectedPos := dataLen - 10
		if pos != expectedPos {
			t.Errorf("expected position %d, got %d", expectedPos, pos)
		}

		// Should be able to read the last 10 bytes
		buf := make([]byte, 10)
		n, err := readSeeker.Read(buf)
		if err != nil {
			t.Fatalf("failed to read after seeking back: %v", err)
		}
		if n != 10 {
			t.Errorf("expected to read 10 bytes, got %d", n)
		}
		expected := testData[len(testData)-10:]
		if !bytes.Equal(buf, expected) {
			t.Errorf("data mismatch: expected %q, got %q", expected, buf)
		}
	})

	t.Run("Multiple seeks to EOF", func(t *testing.T) {
		readSeeker, err := NewReadSeeker(bytes.NewReader(compressedData.Bytes()))
		if err != nil {
			t.Fatalf("failed to create reader: %v", err)
		}
		defer readSeeker.Close()

		// Seek to EOF multiple times with different whence values
		positions := []struct {
			offset int64
			whence int
		}{
			{dataLen, 0}, // SeekStart
			{0, 2},       // SeekEnd
			{dataLen, 0}, // SeekStart again
		}

		for i, p := range positions {
			pos, err := readSeeker.Seek(p.offset, p.whence)
			if err != nil {
				t.Fatalf("iteration %d: Seek failed: %v", i, err)
			}
			if pos != dataLen {
				t.Errorf("iteration %d: expected position %d, got %d", i, dataLen, pos)
			}
		}
	})
}
