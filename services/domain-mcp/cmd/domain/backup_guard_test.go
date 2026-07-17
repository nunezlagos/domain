package main

import (
	"errors"
	"testing"
)

// DOMAINSERV-39: el guard permite migrar solo si hubo backup o la BD es fresca.
func TestBackupGuardAllows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		backupDone bool
		version    uint
		versionErr error
		want       bool
	}{
		{"con backup previo permite aunque haya datos", true, 42, nil, true},
		{"BD fresca sin migraciones (version 0) permite", false, 0, nil, true},
		{"BD fresca sin schema_migrations (error) permite", false, 0, errors.New("no migration"), true},
		{"BD con datos y sin backup RECHAZA", false, 42, nil, false},
		{"con backup y BD fresca permite", true, 0, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := backupGuardAllows(tc.backupDone, tc.version, tc.versionErr)
			if got != tc.want {
				t.Fatalf("backupGuardAllows(%v,%d,%v)=%v, want %v",
					tc.backupDone, tc.version, tc.versionErr, got, tc.want)
			}
		})
	}
}
