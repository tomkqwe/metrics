package postgreserr

import (
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

func TestIsConnectionException(t *testing.T) {
	err := fmt.Errorf("query failed: %w", &pq.Error{
		Code: pqerror.Code(pgerrcode.ConnectionFailure),
	})

	if !IsConnectionException(err) {
		t.Fatal("IsConnectionException() = false, want true")
	}
}

func TestIsConnectionExceptionReturnsFalseForNonConnectionError(t *testing.T) {
	err := fmt.Errorf("query failed: %w", &pq.Error{
		Code: pqerror.Code(pgerrcode.UniqueViolation),
	})

	if IsConnectionException(err) {
		t.Fatal("IsConnectionException() = true, want false")
	}
}
