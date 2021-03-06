package api_test

import (
	"testing"

	"github.com/m-mizutani/minerva/pkg/api"
	"github.com/stretchr/testify/assert"
)

func TestArgsToSQL(t *testing.T) {
	q := api.NewRequest(
		[]string{"mizutani@cookpad.com"},
		"2019-10-24T11:14:15",
		"2019-10-24T15:14:15")

	sql, err := api.BuildSQL(q, "indices", "messages")
	assert.NoError(t, err)
	assert.Contains(t, *sql, "term = 'mizutani'")
	assert.Contains(t, *sql, "term = 'cookpad'")
	assert.Contains(t, *sql, "term = 'com'")
	assert.Contains(t, *sql, "'2019-10-24-11' <= messages.dt")
	assert.Contains(t, *sql, "messages.dt <= '2019-10-24-15'")
	assert.Contains(t, *sql, "'2019-10-24-11' <= indices.dt")
	assert.Contains(t, *sql, "indices.dt <= '2019-10-24-15'")

	// fmt.Println(*sql)
}
