package app

import (
	"testing"
	"time"

	"aerothai/itafm/model"
)

type fakeSurveillanceBatchWriter struct {
	success bool
	batches [][]*model.AODSSurveillance
}

func (f *fakeSurveillanceBatchWriter) InsertOrUpdateSurveillanceBatch(batch []*model.AODSSurveillance) bool {
	copiedBatch := make([]*model.AODSSurveillance, 0, len(batch))
	for _, surv := range batch {
		if surv == nil {
			continue
		}
		copySurv := *surv
		copiedBatch = append(copiedBatch, &copySurv)
	}
	f.batches = append(f.batches, copiedBatch)
	return f.success
}

func TestSurveillanceBatcherFlushUsesLatestByCallsign(t *testing.T) {
	writer := &fakeSurveillanceBatchWriter{success: true}
	batcher := newSurveillanceBatcher(writer, time.Hour, 100)
	defer batcher.close()

	if !batcher.add(&model.AODSSurveillance{
		CallSign:    "TG123",
		DateTime:    "2026-02-18T10:00:00Z",
		GroundSpeed: 200,
	}) {
		t.Fatal("expected add to succeed")
	}
	if !batcher.add(&model.AODSSurveillance{
		CallSign:    "TG123",
		DateTime:    "2026-02-18T10:00:01Z",
		GroundSpeed: 210,
	}) {
		t.Fatal("expected second add to succeed")
	}
	if !batcher.add(&model.AODSSurveillance{
		CallSign:    "EK500",
		DateTime:    "2026-02-18T10:00:02Z",
		GroundSpeed: 430,
	}) {
		t.Fatal("expected third add to succeed")
	}

	batcher.flush()

	if len(writer.batches) != 1 {
		t.Fatalf("expected 1 flushed batch, got %d", len(writer.batches))
	}
	if len(writer.batches[0]) != 2 {
		t.Fatalf("expected 2 records in flushed batch, got %d", len(writer.batches[0]))
	}

	latestFound := false
	for _, surv := range writer.batches[0] {
		if surv.CallSign == "TG123" && surv.DateTime == "2026-02-18T10:00:01Z" && surv.GroundSpeed == 210 {
			latestFound = true
		}
	}
	if !latestFound {
		t.Fatal("expected latest TG123 record to be flushed")
	}
}

func TestSurveillanceBatcherFlushOnMaxSize(t *testing.T) {
	writer := &fakeSurveillanceBatchWriter{success: true}
	batcher := newSurveillanceBatcher(writer, time.Hour, 2)
	defer batcher.close()

	batcher.add(&model.AODSSurveillance{CallSign: "TG111"})
	batcher.add(&model.AODSSurveillance{CallSign: "TG222"})

	if len(writer.batches) == 0 {
		t.Fatal("expected auto flush when max batch size is reached")
	}
}

func TestSurveillanceBatcherSkipsAlreadyFlushedDateTime(t *testing.T) {
	writer := &fakeSurveillanceBatchWriter{success: true}
	batcher := newSurveillanceBatcher(writer, time.Hour, 100)
	defer batcher.close()

	batcher.add(&model.AODSSurveillance{CallSign: "TG123", DateTime: "2026-02-18T10:00:00Z"})
	batcher.flush()
	if len(writer.batches) != 1 {
		t.Fatalf("expected first flush, got %d batches", len(writer.batches))
	}

	batcher.add(&model.AODSSurveillance{CallSign: "TG123", DateTime: "2026-02-18T10:00:00Z"})
	batcher.flush()
	if len(writer.batches) != 1 {
		t.Fatalf("expected duplicate datetime to be skipped, got %d batches", len(writer.batches))
	}

	batcher.add(&model.AODSSurveillance{CallSign: "TG123", DateTime: "2026-02-18T10:00:01Z"})
	batcher.flush()
	if len(writer.batches) != 2 {
		t.Fatalf("expected new datetime to flush, got %d batches", len(writer.batches))
	}
}

func TestGetSurveillanceDBBatchInterval(t *testing.T) {
	t.Setenv("SURVEILLANCE_DB_BATCH_INTERVAL", "")
	if got := getSurveillanceDBBatchInterval(); got != 10*time.Second {
		t.Fatalf("expected default interval for empty env, got %s", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_INTERVAL", "2s")
	if got := getSurveillanceDBBatchInterval(); got != 2*time.Second {
		t.Fatalf("expected parsed interval, got %s", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_INTERVAL", "invalid")
	if got := getSurveillanceDBBatchInterval(); got != 10*time.Second {
		t.Fatalf("expected default interval for invalid env, got %s", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_INTERVAL", "-1s")
	if got := getSurveillanceDBBatchInterval(); got != 10*time.Second {
		t.Fatalf("expected default interval for negative env, got %s", got)
	}
}

func TestGetSurveillanceDBBatchMaxSize(t *testing.T) {
	t.Setenv("SURVEILLANCE_DB_BATCH_MAX_SIZE", "")
	if got := getSurveillanceDBBatchMaxSize(); got != 500 {
		t.Fatalf("expected default batch size for empty env, got %d", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_MAX_SIZE", "500")
	if got := getSurveillanceDBBatchMaxSize(); got != 500 {
		t.Fatalf("expected parsed batch size, got %d", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_MAX_SIZE", "invalid")
	if got := getSurveillanceDBBatchMaxSize(); got != 500 {
		t.Fatalf("expected default batch size for invalid env, got %d", got)
	}

	t.Setenv("SURVEILLANCE_DB_BATCH_MAX_SIZE", "-10")
	if got := getSurveillanceDBBatchMaxSize(); got != 500 {
		t.Fatalf("expected default batch size for negative env, got %d", got)
	}
}
