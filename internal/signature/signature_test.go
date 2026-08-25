package signature

import "testing"

func TestCalculateAndVerify(t *testing.T) {
	body := []byte(`{"id":"Alloc","type":"gauge","value":12.5}`)
	key := "secret"

	hash := Calculate(body, key)
	if hash == "" {
		t.Fatal("Calculate() returned empty hash")
	}
	if !Verify(body, key, hash) {
		t.Fatal("Verify() = false, want true")
	}
	if Verify([]byte("other"), key, hash) {
		t.Fatal("Verify() = true for changed body, want false")
	}
	if Verify(body, "other", hash) {
		t.Fatal("Verify() = true for changed key, want false")
	}
	if Verify(body, key, "not-hex") {
		t.Fatal("Verify() = true for invalid hash, want false")
	}
}
