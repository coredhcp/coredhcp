package vss

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_setup4(t *testing.T) {
	tests := map[string]struct {
		args        []string
		expectedErr string
	}{
		"ok": {
			args: []string{"testdata/ok_single_vpn.yaml"},
		},
		"nok_multiple_args": {
			args:        []string{"testdata/ok_single_vpn.yaml", "some-another-arg"},
			expectedErr: "expected plugin config file",
		},
		"nok_invalid_file": {
			args:        []string{"some-another-arg"},
			expectedErr: "error loading leases from file 'some-another-arg': could not read leases file: open some-another-arg: no such file or directory",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := setup4(tt.args...)
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedErr)
			}
		})
	}
}
