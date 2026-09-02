package acctest

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	hashicorp_acctest "github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/labd/terraform-provider-contentful/internal/sdk"
)

const (
	environmentReadyTimeout  = 3 * time.Minute
	environmentReadyInterval = 3 * time.Second
)

// SpaceID returns the space used for acceptance tests.
func SpaceID() string {
	return os.Getenv("CONTENTFUL_SPACE_ID")
}

// EnvironmentID returns the environment used for acceptance tests, defaulting
// to master when CONTENTFUL_ENVIRONMENT is not set.
func EnvironmentID() string {
	if environment := os.Getenv("CONTENTFUL_ENVIRONMENT"); environment != "" {
		return environment
	}

	return "master"
}

// CreateTemporaryEnvironment creates a non-master environment, waits until it
// can be used and deletes it again when the test finishes. It returns the
// generated environment ID, which never equals the configured environment.
func CreateTemporaryEnvironment(t *testing.T) string {
	t.Helper()

	client := GetClient()
	ctx := context.Background()
	spaceID := SpaceID()

	resp, err := client.CreateEnvironmentWithResponse(ctx, spaceID, sdk.EnvironmentCreate{
		Name: "acctest-" + hashicorp_acctest.RandString(8),
	})
	if err != nil {
		t.Fatalf("error creating temporary environment: %v", err)
	}
	if resp.StatusCode() != http.StatusCreated {
		t.Fatalf("error creating temporary environment: unexpected status %d: %s", resp.StatusCode(), resp.Body)
	}

	environmentID := resp.JSON201.Sys.Id
	t.Cleanup(func() { deleteEnvironment(t, client, spaceID, environmentID) })

	waitForEnvironment(t, client, spaceID, environmentID)

	return environmentID
}

// waitForEnvironment blocks until the environment finished provisioning. A
// freshly created environment is queued for a while, during which requests for
// resources inside it fail.
func waitForEnvironment(t *testing.T, client *sdk.ClientWithResponses, spaceID, environmentID string) {
	t.Helper()

	deadline := time.Now().Add(environmentReadyTimeout)
	for {
		resp, err := client.GetAllLocalesWithResponse(context.Background(), spaceID, environmentID, nil)
		if err == nil && resp.StatusCode() == http.StatusOK {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for environment %s to become ready", environmentID)
		}

		time.Sleep(environmentReadyInterval)
	}
}

func deleteEnvironment(t *testing.T, client *sdk.ClientWithResponses, spaceID, environmentID string) {
	t.Helper()

	ctx := context.Background()
	current, err := client.GetEnvironmentWithResponse(ctx, spaceID, environmentID)
	if err != nil || current.StatusCode() != http.StatusOK {
		t.Logf("could not read temporary environment %s for cleanup", environmentID)
		return
	}

	resp, err := client.DeleteEnvironmentWithResponse(ctx, spaceID, environmentID, &sdk.DeleteEnvironmentParams{
		XContentfulVersion: current.JSON200.Sys.Version,
	})
	if err != nil || resp.StatusCode() != http.StatusNoContent {
		t.Logf("could not delete temporary environment %s", environmentID)
	}
}
