package postgreserr

import (
	"errors"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
)

var connectionExceptionClass = pgerrcode.ConnectionException[:2]

func IsConnectionException(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}

	return strings.HasPrefix(string(pqErr.Code), connectionExceptionClass)
}
