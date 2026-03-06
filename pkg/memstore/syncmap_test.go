package memstore

import (
	"sync"
	"testing"
)

func TestSyncmap_SetGetDelete_Basic(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[string, int]()

	m.Set("a", 1)
	m.Set("b", 2)

	if got := m.Get("a"); got != 1 {
		t.Errorf("Get(a) = %d, want 1", got)
	}
	if got := m.Get("b"); got != 2 {
		t.Errorf("Get(b) = %d, want 2", got)
	}
	if got := m.Get("missing"); got != 0 {
		t.Errorf("Get(missing) = %d, want zero value 0", got)
	}

	if !m.Exists("a") {
		t.Error("Exists(a) = false, want true")
	}
	if m.Exists("missing") {
		t.Error("Exists(missing) = true, want false")
	}

	m.Delete("a")
	if m.Exists("a") {
		t.Error("Exists(a) after Delete = true, want false")
	}
}

func TestSyncmap_Set_Overwrite(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[string, string]()
	m.Set("k", "first")
	m.Set("k", "second")
	if got := m.Get("k"); got != "second" {
		t.Errorf("Get after overwrite = %q, want %q", got, "second")
	}
}

// TestSyncmap_All_ReturnsCopy verifies that All() returns an isolated copy of
// the internal map; mutations to the returned value must not affect the original.
func TestSyncmap_All_ReturnsCopy(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[string, int]()
	m.Set("a", 1)

	got := m.All()
	got["b"] = 99 // mutate the returned value

	if m.Exists("b") {
		t.Error("All() returned the live internal map; mutations to the returned value are visible in the original")
	}
}

// TestSyncmap_All_ConcurrentMutation verifies that iterating All() while
// another goroutine calls Set() does not race. Run with -race.
func TestSyncmap_All_ConcurrentMutation(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[int, int]()
	for i := 0; i < 10; i++ {
		m.Set(i, i)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			m.Set(i, i*2)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			all := m.All()
			_ = len(all)
		}
	}()

	wg.Wait()
}

// TestSyncmap_Get_TOCTOU verifies that Get() is safe under concurrent Delete.
// Get() calls Exists() (releases lock) then re-acquires — a concurrent Delete
// between the two makes the key disappear. Run with -race.
func TestSyncmap_Get_TOCTOU(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[string, *int]()
	v := 42
	m.Set("k", &v)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			m.Delete("k")
			vv := i
			m.Set("k", &vv)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			_ = m.Get("k")
		}
	}()

	wg.Wait()
}

func TestSyncmap_Exists_Concurrent(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[int, int]()
	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.Set(i%10, i)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.Delete(i % 10)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			m.Exists(i % 10)
		}
	}()

	wg.Wait()
}

func TestSyncmap_DeleteNonExistent(t *testing.T) {
	t.Parallel()
	m := NewSyncmap[string, int]()
	// Must not panic.
	m.Delete("nope")
}

func TestSyncmap_PointerValues(t *testing.T) {
	t.Parallel()
	type S struct{ V int }
	m := NewSyncmap[string, *S]()

	m.Set("x", &S{V: 7})
	got := m.Get("x")
	if got == nil || got.V != 7 {
		t.Errorf("Get pointer value = %v, want &S{V:7}", got)
	}

	// Zero value for pointer type is nil — Get on missing key returns nil.
	if m.Get("missing") != nil {
		t.Error("Get(missing) for pointer type should return nil")
	}
}
