package postgres_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/goto/entropy/core/resource"
	"github.com/goto/entropy/internal/store/postgres"
	"github.com/goto/salt/dockertestx"
	"github.com/ory/dockertest/v3"
)

func newTestClient(t *testing.T) (*postgres.Store, *dockertest.Pool, *dockertest.Resource) {
	t.Helper()

	pgDocker, err := dockertestx.CreatePostgres(dockertestx.PostgresWithDockertestResourceExpiry(120))
	if err != nil {
		t.Fatal(err)
	}

	store, err := postgres.Open(pgDocker.GetExternalConnString(), 3*time.Second, 5*time.Second, 0, 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Migrate(context.TODO()); err != nil {
		t.Fatal(err)
	}

	return store, pgDocker.GetPool(), pgDocker.GetResource()
}

func bootstrapResources(ctx context.Context, store *postgres.Store) ([]resource.Resource, error) {
	testFixtureJSON, err := os.ReadFile("./testdata/resources.json")
	if err != nil {
		return nil, err
	}

	var data []resource.Resource
	if err = json.Unmarshal(testFixtureJSON, &data); err != nil {
		return nil, err
	}

	for _, d := range data {
		if err := store.Create(ctx, d); err != nil {
			return nil, err
		}
	}

	return data, nil
}
