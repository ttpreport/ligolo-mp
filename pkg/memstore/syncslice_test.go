package memstore

import (
	"sync"
	"testing"
)

func TestSyncslice_AppendGetExists(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[string]()

	s.Append("a")
	s.Append("b")
	s.Append("c")

	if !s.Exists(0) {
		t.Error("Exists(0) = false, want true")
	}
	if !s.Exists(2) {
		t.Error("Exists(2) = false, want true")
	}
	if s.Exists(3) {
		t.Error("Exists(3) = true for out-of-bounds index, want false")
	}

	if got := s.Get(0); got != "a" {
		t.Errorf("Get(0) = %q, want %q", got, "a")
	}
	if got := s.Get(2); got != "c" {
		t.Errorf("Get(2) = %q, want %q", got, "c")
	}
}

func TestSyncslice_Get_OutOfBounds_ReturnsZero(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[int]()
	s.Append(1)

	if got := s.Get(99); got != 0 {
		t.Errorf("Get(99) on single-element slice = %d, want zero value 0", got)
	}
}

func TestSyncslice_Delete(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[int]()
	s.Append(10)
	s.Append(20)
	s.Append(30)

	s.Delete(0) // swap-delete: [30, 20] or [20, 30] depending on impl
	if s.Exists(2) {
		t.Error("after Delete, Exists(2) should be false (len reduced to 2)")
	}
	if !s.Exists(0) || !s.Exists(1) {
		t.Error("after Delete(0), indices 0 and 1 should still be valid")
	}
}

// TestSyncslice_All_ReturnsCopy verifies that All() returns an isolated copy of
// the internal slice; mutations to the returned value must not affect the original.
func TestSyncslice_All_ReturnsCopy(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[int]()
	s.Append(1)
	s.Append(2)

	got := s.All()
	if len(got) != 2 {
		t.Fatalf("All() len = %d, want 2", len(got))
	}

	got[0] = 999

	if s.Get(0) != 1 {
		t.Error("All() returned the live internal slice; mutation of the returned value affected the original")
	}
}

// TestSyncslice_All_ConcurrentMutation verifies that iterating All()
// while another goroutine appends does not race. Run with -race.
func TestSyncslice_All_ConcurrentMutation(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[int]()
	for i := 0; i < 5; i++ {
		s.Append(i)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.Append(i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			all := s.All()
			_ = len(all)
		}
	}()

	wg.Wait()
}

func TestSyncslice_Concurrent_AppendDelete(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[int]()
	// Pre-populate so Delete has valid indices.
	for i := 0; i < 100; i++ {
		s.Append(i)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			s.Append(i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if s.Exists(0) {
				s.Delete(0)
			}
		}
	}()

	wg.Wait()
}

func TestSyncslice_Empty(t *testing.T) {
	t.Parallel()
	s := NewSyncslice[string]()

	if s.Exists(0) {
		t.Error("Exists(0) on empty slice should be false")
	}
	if got := s.Get(0); got != "" {
		t.Errorf("Get(0) on empty slice = %q, want empty string", got)
	}
	if got := s.All(); len(got) != 0 {
		t.Errorf("All() on empty slice len = %d, want 0", len(got))
	}
}
