package service

import (
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTagCountEngine_AdjustCreatesAndClearsDuplicates(t *testing.T) {
	engine := newTagCountEngine()

	_, became, lost := engine.adjustObject("obj-1", []TagOccurrence{
		{Tag: "SHARED", ObjectID: "obj-1", SourceLabel: "Tag"},
	})
	if len(became) != 0 || len(lost) != 0 {
		t.Fatalf("single occurrence should not create duplicates, became=%v lost=%v", became, lost)
	}
	if engine.countFor("SHARED") != 1 {
		t.Fatalf("expected count 1, got %d", engine.countFor("SHARED"))
	}

	_, became, lost = engine.adjustObject("obj-2", []TagOccurrence{
		{Tag: "SHARED", ObjectID: "obj-2", SourceLabel: "Tag"},
	})
	if !reflect.DeepEqual(became, []string{"SHARED"}) {
		t.Fatalf("expected SHARED became duplicate, got %v", became)
	}
	if len(lost) != 0 {
		t.Fatalf("unexpected lost %v", lost)
	}
	if !engine.isDuplicateTag("SHARED") {
		t.Fatal("SHARED should be duplicate")
	}

	_, became, lost = engine.adjustObject("obj-2", []TagOccurrence{
		{Tag: "UNIQUE", ObjectID: "obj-2", SourceLabel: "Tag"},
	})
	if len(became) != 0 {
		t.Fatalf("unexpected became %v", became)
	}
	if !reflect.DeepEqual(lost, []string{"SHARED"}) {
		t.Fatalf("expected SHARED lost duplicate after rename, got %v", lost)
	}
	if engine.isDuplicateTag("SHARED") {
		t.Fatal("SHARED should no longer be duplicate")
	}
	if engine.countFor("SHARED") != 1 {
		t.Fatalf("expected SHARED count 1, got %d", engine.countFor("SHARED"))
	}
}

func TestBackgroundScanPlan_FullToIncremental(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	plan := newBackgroundScanPlan(ids)
	plan.BatchSize = 2

	batch1, prio, done := plan.nextBatch()
	if prio || done || len(batch1) != 2 {
		t.Fatalf("batch1 unexpected: %v prio=%v done=%v", batch1, prio, done)
	}
	batch2, prio, done := plan.nextBatch()
	if prio || done || len(batch2) != 2 {
		t.Fatalf("batch2 unexpected: %v prio=%v done=%v", batch2, prio, done)
	}
	batch3, prio, done := plan.nextBatch()
	if prio || done || len(batch3) != 1 {
		t.Fatalf("batch3 unexpected: %v prio=%v done=%v", batch3, prio, done)
	}
	if plan.Mode != "incremental" {
		t.Fatalf("expected incremental after full pass, got %s", plan.Mode)
	}
	empty, prio, done := plan.nextBatch()
	if len(empty) != 0 || prio || !done {
		t.Fatalf("incremental idle unexpected: %v prio=%v done=%v", empty, prio, done)
	}
}

func TestResumeBackgroundScanPlan_ContinuesFullCursor(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e"}
	plan := resumeBackgroundScanPlan(ids, "full", 2, 2)
	plan.BatchSize = 2
	if plan.Mode != "full" || plan.Cursor != 2 {
		t.Fatalf("expected full cursor=2, got mode=%s cursor=%d", plan.Mode, plan.Cursor)
	}
	batch, prio, done := plan.nextBatch()
	if prio || done || !reflect.DeepEqual(batch, []string{"c", "d"}) {
		t.Fatalf("expected resume c,d got %v prio=%v done=%v", batch, prio, done)
	}
}

func TestResumeBackgroundScanPlan_Incremental(t *testing.T) {
	ids := []string{"a", "b", "c"}
	plan := resumeBackgroundScanPlan(ids, "incremental", 3, 3)
	if plan.Mode != "incremental" {
		t.Fatalf("expected incremental, got %s", plan.Mode)
	}
	batch, prio, done := plan.nextBatch()
	if len(batch) != 0 || prio || !done {
		t.Fatalf("expected idle incremental, got %v prio=%v done=%v", batch, prio, done)
	}
}

func TestBackgroundScanPlan_PriorityPreemptsFullQueue(t *testing.T) {
	plan := newBackgroundScanPlan([]string{"f1", "f2", "f3", "f4"})
	plan.BatchSize = 2
	plan.DirtyQueue.enqueue("prio-1", bgDirtyPriorityHigh, "dup")
	plan.DirtyQueue.enqueue("prio-2", bgDirtyPriorityHigh, "dup")

	batch, fromPriority, done := plan.nextBatch()
	if done || !fromPriority {
		t.Fatalf("expected priority batch, got %v fromPriority=%v done=%v", batch, fromPriority, done)
	}
	if len(batch) != 2 {
		t.Fatalf("expected 2 priority items, got %v", batch)
	}
	if plan.Cursor != 0 {
		t.Fatalf("full cursor should not advance during priority, got %d", plan.Cursor)
	}

	batch, fromPriority, done = plan.nextBatch()
	if done || fromPriority {
		t.Fatalf("expected resume full queue, got %v fromPriority=%v done=%v", batch, fromPriority, done)
	}
	if !reflect.DeepEqual(batch, []string{"f1", "f2"}) {
		t.Fatalf("expected full resume f1,f2 got %v", batch)
	}
}

func TestBackgroundScanThrottle_SingleWorker(t *testing.T) {
	workers, batchSize, sleep := bgScanThrottleConfig()
	if workers != 1 {
		t.Fatalf("expected workers=1, got %d", workers)
	}
	if batchSize < 25 || batchSize > 50 {
		t.Fatalf("expected batchSize 25-50, got %d", batchSize)
	}
	if sleep < 50*time.Millisecond || sleep > 200*time.Millisecond {
		t.Fatalf("expected sleep 50-200ms, got %v", sleep)
	}
}

func TestForcedScanThrottleMatchesBackgroundShape(t *testing.T) {
	fw, fb, fs := forcedScanThrottleConfig()
	if fw != 1 {
		t.Fatalf("forced workers=%d", fw)
	}
	if fb < 20 || fb > 40 {
		t.Fatalf("forced batch=%d", fb)
	}
	if fs < 50*time.Millisecond || fs > 100*time.Millisecond {
		t.Fatalf("forced sleep=%v", fs)
	}
}

func TestEnsureBackgroundExclusive(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	if err := ensureBackgroundExclusive(id, true); err != ErrBackgroundScanActive {
		t.Fatalf("expected ErrBackgroundScanActive, got %v", err)
	}
	if err := ensureBackgroundExclusive(id, false); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestDirtyQueue_EnableDisableSemantics(t *testing.T) {
	q := &dirtyQueue{}
	q.enqueue("a", bgDirtyPriorityNormal, "change")
	q.enqueue("b", bgDirtyPriorityHigh, "dup")
	q.enqueue("a", bgDirtyPriorityHigh, "dup-upgrade")
	if q.len() != 2 {
		t.Fatalf("expected 2 items after upsert, got %d", q.len())
	}
	first, ok := q.popHighest()
	if !ok || first.ObjectID != "b" && first.ObjectID != "a" {
		t.Fatalf("expected high priority first, got %+v", first)
	}
	if first.Priority != bgDirtyPriorityHigh {
		t.Fatalf("expected high priority, got %d", first.Priority)
	}
	second, ok := q.popHighest()
	if !ok {
		t.Fatal("expected second item")
	}
	if q.len() != 0 {
		t.Fatalf("queue should be empty, got %d", q.len())
	}
	_ = second
	_, ok = q.popHighest()
	if ok {
		t.Fatal("empty queue should not pop")
	}
}
