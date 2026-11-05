package pool_test

import (
	"sync"
	"sync/atomic"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/pkg/pool"
)

type object struct {
	text    string
	values  []int
	entries map[string]int
	resets  *atomic.Int64
}

func (o *object) Reset() {
	o.text = ""
	o.values = o.values[:0]
	clear(o.entries)
	o.resets.Add(1)
}

func TestPool(t *testing.T) {
	var created, resets atomic.Int64
	p := pool.New(func() *object {
		created.Add(1)
		return &object{values: make([]int, 0, 8), entries: make(map[string]int), resets: &resets}
	})
	if created.Load() != 0 {
		t.Fatal("constructor should not eagerly create objects")
	}
	first := p.Get()
	second := p.Get()
	if first == nil || second == nil || first == second || created.Load() != 2 {
		t.Fatal("empty pool must create independent objects")
	}
	first.text = "used"
	first.values = append(first.values, 1, 2)
	first.entries["used"] = 3
	p.Put(first)
	if resets.Load() != 1 {
		t.Fatal("Put must reset the object before returning")
	}

	// sync.Pool may discard objects, so tests must not require pointer identity.
	reused := p.Get()
	if reused.text != "" || len(reused.values) != 0 || len(reused.entries) != 0 {
		t.Fatal("Get returned dirty state")
	}
	if cap(reused.values) != 8 || reused.entries == nil {
		t.Fatal("collection storage should remain available")
	}
	p.Put(reused)
	p.Put(second)
	if resets.Load() != 3 {
		t.Fatal("each Put must call Reset exactly once")
	}
}

func TestPoolConcurrent(t *testing.T) {
	var resets atomic.Int64
	p := pool.New(func() *object {
		return &object{entries: make(map[string]int), resets: &resets}
	})
	const workers, iterations = 16, 100
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Go(func() {
			for i := 0; i < iterations; i++ {
				obj := p.Get()
				if obj.text != "" || len(obj.values) != 0 || len(obj.entries) != 0 {
					t.Error("Get returned dirty state")
				}
				obj.text = "used"
				obj.values = append(obj.values, i)
				obj.entries["used"] = i
				p.Put(obj)
			}
		})
	}
	wg.Wait()
	if got := resets.Load(); got != workers*iterations {
		t.Fatalf("Reset calls = %d, want %d", got, workers*iterations)
	}
}

func TestPoolWithGeneratedReset(t *testing.T) {
	p := pool.New(func() *models.Metrics { return &models.Metrics{} })
	metric := p.Get()
	value := 12.5
	delta := int64(10)
	metric.ID = "Alloc"
	metric.MType = models.MetricTypeGauge
	metric.Hash = "hash"
	metric.Value = &value
	metric.Delta = &delta
	p.Put(metric)
	// Generated Reset clears values through pointers before the object is stored.
	if value != 0 || delta != 0 {
		t.Fatal("generated Reset was not called by Put")
	}
	clean := p.Get()
	if clean.ID != "" || clean.MType != "" || clean.Hash != "" {
		t.Fatal("metric strings were not reset")
	}
	if clean.Value != nil && *clean.Value != 0 || clean.Delta != nil && *clean.Delta != 0 {
		t.Fatal("metric pointer values were not reset")
	}
	p.Put(clean)
}
