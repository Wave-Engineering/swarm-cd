package swarmcd

import (
	"slices"
	"sync"
	"testing"

	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/stack"
)

// TestDeployArgsResolveImage verifies that every deploy carries --resolve-image with the
// configured mode, and that the arguments it had before are unchanged.
func TestDeployArgsResolveImage(t *testing.T) {
	for _, mode := range []string{"always", "changed", "never"} {
		got := deployArgs("repos/r/docker-compose.yaml", "mystack", mode)
		want := []string{
			"deploy", "--detach", "--with-registry-auth",
			"--resolve-image", mode,
			"-c", "repos/r/docker-compose.yaml",
			"mystack",
		}
		if !slices.Equal(got, want) {
			t.Errorf("mode %s: got %q, want %q", mode, got, want)
		}
	}
}

// TestDeployArgsParseAsDockerStackDeploy feeds deployArgs to docker/cli's own `stack
// deploy` command, the one deployStack runs, and reads the flags back: the arguments are
// valid for the real command, not just shaped as intended. No daemon is contacted.
func TestDeployArgsParseAsDockerStackDeploy(t *testing.T) {
	cli, err := command.NewDockerCli()
	if err != nil {
		t.Fatalf("NewDockerCli: %v", err)
	}
	root := stack.NewStackCommand(cli)
	deploy, rest, err := root.Find(deployArgs("repos/r/docker-compose.yaml", "mystack", "changed"))
	if err != nil {
		t.Fatalf("find the deploy subcommand: %v", err)
	}
	if deploy.Name() != "deploy" {
		t.Fatalf("expected the deploy subcommand, got %q", deploy.Name())
	}
	if err := deploy.ParseFlags(rest); err != nil {
		t.Fatalf("docker stack deploy rejects the arguments: %v", err)
	}
	flags := deploy.Flags()
	if v, _ := flags.GetString("resolve-image"); v != "changed" {
		t.Errorf("--resolve-image parsed as %q, want %q", v, "changed")
	}
	if v, _ := flags.GetStringSlice("compose-file"); !slices.Equal(v, []string{"repos/r/docker-compose.yaml"}) {
		t.Errorf("--compose-file parsed as %q", v)
	}
	for _, name := range []string{"detach", "with-registry-auth"} {
		if v, _ := flags.GetBool(name); !v {
			t.Errorf("--%s parsed as false", name)
		}
	}
	if args := flags.Args(); !slices.Equal(args, []string{"mystack"}) {
		t.Errorf("positional arguments parsed as %q, want [mystack]", args)
	}
}

// External objects are ignored by the rotation
func TestRotateExternalObjects(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "", "docker-compose.yaml", nil, "", false)
	objects := map[string]any{
		"my-secret": map[string]any{"external": true},
	}
	err := stack.rotateObjects(objects, "secrets")
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
}

// Secrets are discovered, external secrets are ignored
func TestSecretDiscovery(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "", "stacks/docker-compose.yaml", nil, "", false)
	stackString := []byte(`services:
  my-service:
    image: my-image
    secrets:
      - my-secret
      - my-external-secret
secrets:
  my-secret:
    file: secrets/secret.yaml
  my-external-secret:
    external: true`)
	composeMap, err := stack.parseStackString(stackString)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	sopsFiles, err := discoverSecrets(composeMap, stack.composePath)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(sopsFiles) != 1 {
		t.Errorf("unexpected number of sops files: %d", len(sopsFiles))
	}
	if sopsFiles[0] != "stacks/secrets/secret.yaml" {
		t.Errorf("unexpected sops file: %s", sopsFiles[0])
	}
}

// refAttr returns branch attribute when branch is set
func TestRefAttrBranch(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "main", "", "docker-compose.yaml", nil, "", false)
	attr := stack.refAttr()
	if attr.Key != "branch" {
		t.Errorf("expected key 'branch', got '%s'", attr.Key)
	}
	if attr.Value.String() != "main" {
		t.Errorf("expected value 'main', got '%s'", attr.Value.String())
	}
}

// refAttr returns tag attribute when tag is set
func TestRefAttrTag(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("test", repo, "", "v1.2.3", "docker-compose.yaml", nil, "", false)
	attr := stack.refAttr()
	if attr.Key != "tag" {
		t.Errorf("expected key 'tag', got '%s'", attr.Key)
	}
	if attr.Value.String() != "v1.2.3" {
		t.Errorf("expected value 'v1.2.3', got '%s'", attr.Value.String())
	}
}

// newSwarmStack correctly stores tag
func TestNewSwarmStackWithTag(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	stack := newSwarmStack("mystack", repo, "", "v2.0.0", "compose.yaml", nil, "", false)
	if stack.tag != "v2.0.0" {
		t.Errorf("expected tag 'v2.0.0', got '%s'", stack.tag)
	}
	if stack.branch != "" {
		t.Errorf("expected empty branch, got '%s'", stack.branch)
	}
}

// Verify refAttr prefers tag over branch when both could theoretically be set
func TestRefAttrPrefersTag(t *testing.T) {
	repo := &stackRepo{name: "test", path: "test", url: "", auth: nil, lock: &sync.Mutex{}, gitRepoObject: nil}
	// Even if branch has a value, tag takes precedence
	stack := newSwarmStack("test", repo, "main", "v1.0.0", "docker-compose.yaml", nil, "", false)
	attr := stack.refAttr()
	if attr.Key != "tag" {
		t.Errorf("expected key 'tag' to take precedence, got '%s'", attr.Key)
	}
}
