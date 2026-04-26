package service

import "context"

// Test-only accessors for internal postgres lifecycle helpers. Exposed
// so integration tests can drive the managed-postgres path with real
// docker without needing to spin up a full fabricx.Committer (which
// requires org/key services, cert material, and an actual host FS).
//
// These mirror what prepareGroup's committer branch and StopGroup do.

func (s *Service) StartManagedPostgresForCommitterForTest(ctx context.Context, groupID int64, networkName string) error {
	return s.startManagedPostgresForCommitter(ctx, groupID, networkName)
}

func (s *Service) StopManagedPostgresForCommitterForTest(ctx context.Context, groupID int64, networkName string) {
	s.stopManagedPostgresForCommitter(ctx, groupID, networkName)
}
