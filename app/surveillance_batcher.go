package app

import (
	"fmt"
	"log"
	"sync"
	"time"

	"aerothai/itafm/model"
)

type surveillanceBatchWriter interface {
	InsertOrUpdateSurveillanceBatch([]*model.AODSSurveillance) bool
}

type surveillanceBatcher struct {
	writer       surveillanceBatchWriter
	flushTicker  *time.Ticker
	maxBatchSize int

	mu                  sync.Mutex
	pending             map[string]*model.AODSSurveillance
	lastFlushedSignature map[string]string

	stopCh chan struct{}
}

func newSurveillanceBatcher(
	writer surveillanceBatchWriter,
	flushInterval time.Duration,
	maxBatchSize int,
) *surveillanceBatcher {
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}
	if maxBatchSize <= 0 {
		maxBatchSize = 200
	}

	batcher := &surveillanceBatcher{
		writer:               writer,
		flushTicker:          time.NewTicker(flushInterval),
		maxBatchSize:         maxBatchSize,
		pending:              map[string]*model.AODSSurveillance{},
		lastFlushedSignature: map[string]string{},
		stopCh:               make(chan struct{}),
	}

	go batcher.run()
	return batcher
}

func (b *surveillanceBatcher) run() {
	for {
		select {
		case <-b.flushTicker.C:
			b.flush()
		case <-b.stopCh:
			return
		}
	}
}

func (b *surveillanceBatcher) close() {
	close(b.stopCh)
	b.flushTicker.Stop()
	b.flush()
}

func (b *surveillanceBatcher) add(surv *model.AODSSurveillance) bool {
	if surv == nil || surv.CallSign == "" {
		return false
	}

	shouldFlush := false
	b.mu.Lock()
	if b.shouldSkipLocked(surv) {
		b.mu.Unlock()
		return true
	}
	b.pending[surv.CallSign] = surv
	shouldFlush = len(b.pending) >= b.maxBatchSize
	b.mu.Unlock()

	if shouldFlush {
		b.flush()
	}

	return true
}

func (b *surveillanceBatcher) shouldSkipLocked(surv *model.AODSSurveillance) bool {
	signature := surveillanceSignature(surv)

	if pending, exists := b.pending[surv.CallSign]; exists && surveillanceSignature(pending) == signature {
		return true
	}

	if flushedSignature, exists := b.lastFlushedSignature[surv.CallSign]; exists && flushedSignature == signature {
		return true
	}

	return false
}

func (b *surveillanceBatcher) flush() {
	batch := b.drain()
	if len(batch) == 0 {
		return
	}

	if b.writer.InsertOrUpdateSurveillanceBatch(batch) {
		b.markFlushed(batch)
		return
	}

	log.Printf("failed to flush surveillance batch with %d records", len(batch))
	b.requeue(batch)
}

func (b *surveillanceBatcher) markFlushed(batch []*model.AODSSurveillance) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, surv := range batch {
		if surv == nil || surv.CallSign == "" {
			continue
		}
		b.lastFlushedSignature[surv.CallSign] = surveillanceSignature(surv)
	}
}

func (b *surveillanceBatcher) drain() []*model.AODSSurveillance {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.pending) == 0 {
		return nil
	}

	batch := make([]*model.AODSSurveillance, 0, len(b.pending))
	for _, surv := range b.pending {
		batch = append(batch, surv)
	}
	b.pending = map[string]*model.AODSSurveillance{}

	return batch
}

func surveillanceSignature(surv *model.AODSSurveillance) string {
	if surv == nil {
		return ""
	}

	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%f|%f|%f|%f|%f|%s|%d|%d|%s|%s|%d|%f|%f|%s",
		surv.CallSign,
		surv.Departure,
		surv.Destination,
		surv.AircraftType,
		surv.WakeTurbulance,
		surv.Lat,
		surv.Lon,
		surv.Altitude,
		surv.GroundSpeed,
		surv.Heading,
		surv.AircraftAddress,
		surv.SIC,
		surv.SAC,
		surv.SSRCode,
		surv.DateTime,
		surv.TrackNumber,
		surv.VX,
		surv.VY,
		surv.CDM,
	)
}

func (b *surveillanceBatcher) requeue(batch []*model.AODSSurveillance) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, surv := range batch {
		if surv == nil {
			continue
		}

		if _, exists := b.pending[surv.CallSign]; exists {
			continue
		}
		b.pending[surv.CallSign] = surv
	}
}
