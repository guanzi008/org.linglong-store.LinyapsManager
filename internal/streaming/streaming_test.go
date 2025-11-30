package streaming

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGenerateOperationID(t *testing.T) {
	id1 := GenerateOperationID()
	id2 := GenerateOperationID()

	if id1 == "" {
		t.Error("GenerateOperationID returned empty string")
	}
	if id2 == "" {
		t.Error("GenerateOperationID returned empty string")
	}
	if id1 == id2 {
		t.Error("GenerateOperationID should return unique IDs")
	}

	if !strings.HasPrefix(id1, "op-") {
		t.Errorf("ID should start with 'op-', got %s", id1)
	}
}

func TestStreamReaderMultipleLines(t *testing.T) {
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	var lines []string
	var mu sync.Mutex

	operationID := "test-op-123"

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := pr.Read(buf)
			if err != nil {
				break
			}
			mu.Lock()
			lines = append(lines, strings.Split(string(buf[:n]), "\n")...)
			mu.Unlock()
		}
	}()

	pw.WriteString("Line 1\n")
	pw.WriteString("Line 2\n")
	pw.WriteString("Line 3\n")
	pw.Close()

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	t.Logf("Read %d line segments for operation %s", len(lines), operationID)
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Error("Context should be cancelled")
	}
}

func TestContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		// Expected - timeout
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should have timed out")
	}
}

func BenchmarkGenerateOperationID(b *testing.B) {
	for i := 0; i < b.N; i++ {
		GenerateOperationID()
	}
}
