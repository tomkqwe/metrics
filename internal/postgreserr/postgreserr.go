// Package postgreserr classifies PostgreSQL connection errors.
package postgreserr

import (
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

var connectionExceptionClass = pgerrcode.ConnectionException[:2]

// IsConnectionException reports whether err wraps a PostgreSQL SQLSTATE class 08 connection exception.
func IsConnectionException(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	return strings.HasPrefix(string(pqErr.Code), connectionExceptionClass)
}
