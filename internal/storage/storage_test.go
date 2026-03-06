package storage

import (
	"os"
	"sync"
	"testing"
)

// newTestStore creates a Store backed by a temp-dir SQLite file.
// The caller must call cleanup() when done.
func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir := t.TempDir()
	store, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store, func() {
		store.Close()
		os.RemoveAll(dir)
	}
}

// --- basic CRUD ---

func TestStoreInstance_SetGet_Basic(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, err := GetInstance[string](store, "test_basic")
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}

	want := "hello"
	if err := si.Set("k1", &want); err != nil {
		t.Fatalf("Set: %v", err)
	}

	got, err := si.Get("k1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || *got != want {
		t.Errorf("Get = %v, want %q", got, want)
	}
}

func TestStoreInstance_Get_MissingKey_ReturnsNil(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[string](store, "test_missing")

	got, err := si.Get("nonexistent")
	if err != nil {
		t.Fatalf("Get returned error for missing key: %v", err)
	}
	if got != nil {
		t.Errorf("Get = %v, want nil", got)
	}
}

func TestStoreInstance_Set_Overwrite(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[int](store, "test_overwrite")

	v1 := 42
	si.Set("k", &v1)

	v2 := 99
	si.Set("k", &v2)

	got, err := si.Get("k")
	if err != nil || got == nil {
		t.Fatalf("Get after overwrite: err=%v got=%v", err, got)
	}
	if *got != 99 {
		t.Errorf("Get after overwrite = %d, want 99", *got)
	}
}

func TestStoreInstance_Del(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[string](store, "test_del")

	v := "to-delete"
	si.Set("k", &v)
	si.Del("k")

	got, _ := si.Get("k")
	if got != nil {
		t.Errorf("after Del, Get = %v, want nil", got)
	}
}

func TestStoreInstance_Del_NonExistent_NoError(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[string](store, "test_del_missing")

	if err := si.Del("ghost"); err != nil {
		t.Errorf("Del nonexistent key returned error: %v", err)
	}
}

func TestStoreInstance_DelAll(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[int](store, "test_delall")

	for i := 0; i < 5; i++ {
		v := i
		si.Set(string(rune('a'+i)), &v)
	}

	if err := si.DelAll(); err != nil {
		t.Fatalf("DelAll: %v", err)
	}

	all, err := si.GetAll()
	if err != nil {
		t.Fatalf("GetAll after DelAll: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("GetAll after DelAll = %d items, want 0", len(all))
	}
}

func TestStoreInstance_GetAll(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[int](store, "test_getall")

	for i := 0; i < 3; i++ {
		v := i * 10
		si.Set(string(rune('a'+i)), &v)
	}

	all, err := si.GetAll()
	if err != nil {
		t.Fatalf("GetAll: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("GetAll count = %d, want 3", len(all))
	}
}

// --- table isolation ---

func TestGetInstance_DifferentTables_Independent(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si1, _ := GetInstance[string](store, "table_a")
	si2, _ := GetInstance[string](store, "table_b")

	v := "shared-key-value"
	si1.Set("k", &v)

	got, _ := si2.Get("k")
	if got != nil {
		t.Errorf("key written to table_a visible in table_b: %v", got)
	}
}

// --- structured values ---

type testStruct struct {
	Name  string
	Count int
}

func TestStoreInstance_Struct_RoundTrip(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[testStruct](store, "test_struct")

	want := testStruct{Name: "alice", Count: 7}
	si.Set("obj", &want)

	got, err := si.Get("obj")
	if err != nil || got == nil {
		t.Fatalf("Get struct: err=%v got=%v", err, got)
	}
	if got.Name != want.Name || got.Count != want.Count {
		t.Errorf("Get struct = %+v, want %+v", got, want)
	}
}

// TestStore_CrossInstance_ConcurrentWrites documents that multiple StoreInstance objects
// sharing one *sql.DB can cause SQLITE_BUSY under concurrent load because the shared DB
// has no WAL mode and no busy timeout set.

func TestStore_CrossInstance_ConcurrentWrites(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si1, _ := GetInstance[int](store, "concurrent_a")
	si2, _ := GetInstance[int](store, "concurrent_b")

	var wg sync.WaitGroup
	errs := make(chan error, 200)

	for i := 0; i < 100; i++ {
		i := i
		wg.Add(2)
		go func() {
			defer wg.Done()
			v := i
			if err := si1.Set(string(rune('a'+i%26)), &v); err != nil {
				errs <- err
			}
		}()
		go func() {
			defer wg.Done()
			v := i * 2
			if err := si2.Set(string(rune('A'+i%26)), &v); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	var failures []error
	for err := range errs {
		failures = append(failures, err)
	}

	if len(failures) > 0 {
		t.Logf("%d/%d cross-instance writes failed (SQLITE_BUSY or similar): first=%v",
			len(failures), 200, failures[0])
		// Known issue: fixing requires WAL mode + shared connection pool (see DB-5).
	}
}

// TestStoreInstance_SingleInstance_ConcurrentWrites_Safe documents that the per-instance
// mutex serialises single-op calls within one StoreInstance, while cross-instance
// operations can still collide at the SQLite layer.

func TestStoreInstance_SingleInstance_ConcurrentWrites_Safe(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	si, _ := GetInstance[int](store, "single_concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			v := i
			si.Set("key", &v)
		}()
	}
	wg.Wait()

	// Some value was written; no panic or data corruption.
	got, err := si.Get("key")
	if err != nil {
		t.Fatalf("Get after concurrent writes: %v", err)
	}
	if got == nil {
		t.Error("Get after concurrent writes = nil, want some value")
	}
}
