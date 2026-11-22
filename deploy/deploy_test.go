// Package deploy holds no Go code — only deployment manifests. This test
// validates that the shipped manifests and the Helm chart are well-formed YAML
// with the keys an operator depends on, so a typo never ships silently. Helm
// templates are checked by substituting every {{ ... }} action with a scalar
// placeholder and then parsing the result: it proves the surrounding YAML
// structure is sound without needing the `helm` binary.
package deploy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var tmplAction = regexp.MustCompile(`\{\{.*?\}\}`)

// parseYAMLDocs splits a (possibly multi-document) YAML string and unmarshals
// each document, returning them or a fatal test error.
func parseYAMLDocs(t *testing.T, name, content string) []map[string]any {
	t.Helper()
	var docs []map[string]any
	for _, raw := range strings.Split(content, "\n---") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("%s: document does not parse: %v\n---\n%s", name, err, raw)
		}
		if doc != nil {
			docs = append(docs, doc)
		}
	}
	return docs
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestChartYAMLValid(t *testing.T) {
	docs := parseYAMLDocs(t, "Chart.yaml", readFile(t, "helm/heramba/Chart.yaml"))
	if len(docs) != 1 {
		t.Fatalf("Chart.yaml should be a single document, got %d", len(docs))
	}
	c := docs[0]
	for _, key := range []string{"apiVersion", "name", "version", "appVersion"} {
		if _, ok := c[key]; !ok {
			t.Errorf("Chart.yaml missing required key %q", key)
		}
	}
	if c["name"] != "heramba" {
		t.Errorf("chart name = %v, want heramba", c["name"])
	}
	if c["apiVersion"] != "v2" {
		t.Errorf("chart apiVersion = %v, want v2", c["apiVersion"])
	}
}

func TestValuesYAMLValid(t *testing.T) {
	docs := parseYAMLDocs(t, "values.yaml", readFile(t, "helm/heramba/values.yaml"))
	v := docs[0]
	image, ok := v["image"].(map[string]any)
	if !ok || image["repository"] == nil {
		t.Fatalf("values.yaml must set image.repository, got %v", v["image"])
	}
	cfg, ok := v["config"].(map[string]any)
	if !ok || cfg["mode"] != "observe" {
		t.Errorf("values.yaml config.mode should default to observe, got %v", v["config"])
	}
	svc, ok := v["service"].(map[string]any)
	if !ok || svc["targetPort"] == nil {
		t.Errorf("values.yaml must set service.targetPort, got %v", v["service"])
	}
}

func TestHelmTemplatesWellFormed(t *testing.T) {
	dir := "helm/heramba/templates"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	seen := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue // skip _helpers.tpl
		}
		seen++
		raw := readFile(t, filepath.Join(dir, e.Name()))
		// Replace every template action with a scalar placeholder, then parse.
		rendered := tmplAction.ReplaceAllString(raw, "PLACEHOLDER")
		docs := parseYAMLDocs(t, e.Name(), rendered)
		for _, d := range docs {
			if k, ok := d["kind"].(string); ok {
				kinds[k] = true
			}
			if _, ok := d["apiVersion"]; !ok {
				t.Errorf("%s: a document is missing apiVersion", e.Name())
			}
		}
	}
	if seen == 0 {
		t.Fatal("no template .yaml files found")
	}
	for _, want := range []string{"Deployment", "Service"} {
		if !kinds[want] {
			t.Errorf("helm templates missing a %s manifest (got kinds %v)", want, kinds)
		}
	}
}

// TestExistingManifestsValid also guards the hand-written manifests already in
// deploy/ (no templating), so edits to them cannot break YAML either.
func TestExistingManifestsValid(t *testing.T) {
	for _, f := range []string{"k8s/sidecar.yaml", "docker-compose.yml", "render.yaml"} {
		docs := parseYAMLDocs(t, f, readFile(t, f))
		if len(docs) == 0 {
			t.Errorf("%s produced no documents", f)
		}
	}
}

// TestRenderBlueprintShape checks the Render recipe has the structure Render
// expects (a web service pointing at the Heramba image).
func TestRenderBlueprintShape(t *testing.T) {
	docs := parseYAMLDocs(t, "render.yaml", readFile(t, "render.yaml"))
	svcs, ok := docs[0]["services"].([]any)
	if !ok || len(svcs) == 0 {
		t.Fatalf("render.yaml must list services, got %v", docs[0]["services"])
	}
	svc := svcs[0].(map[string]any)
	if svc["type"] != "web" {
		t.Errorf("render service type = %v, want web", svc["type"])
	}
	img, _ := svc["image"].(map[string]any)
	if img == nil || !strings.Contains(img["url"].(string), "heramba") {
		t.Errorf("render service must reference the heramba image, got %v", svc["image"])
	}
}

// TestRailwayJSONValid checks the Railway recipe is valid JSON with a deploy
// start command.
func TestRailwayJSONValid(t *testing.T) {
	var cfg struct {
		Deploy struct {
			StartCommand string `json:"startCommand"`
		} `json:"deploy"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "railway.json")), &cfg); err != nil {
		t.Fatalf("railway.json must be valid JSON: %v", err)
	}
	if cfg.Deploy.StartCommand != "serve" {
		t.Errorf("railway startCommand = %q, want serve", cfg.Deploy.StartCommand)
	}
}

// TestFlyTomlPresent documents that fly.toml ships as a recipe but is NOT parsed
// here: the repo vendors no TOML library, so we validate only that the file
// exists and names the Heramba service. (Kept as a documented example.)
func TestFlyTomlPresent(t *testing.T) {
	content := readFile(t, "fly.toml")
	for _, want := range []string{"app = \"heramba-edge\"", "internal_port = 8080", "HERAMBA_MODE"} {
		if !strings.Contains(content, want) {
			t.Errorf("fly.toml missing %q", want)
		}
	}
}

// TestECSTaskDefValid checks the AWS ECS task definition is valid JSON with a
// Heramba container and env.
func TestECSTaskDefValid(t *testing.T) {
	var td struct {
		Family               string `json:"family"`
		ContainerDefinitions []struct {
			Name        string `json:"name"`
			Image       string `json:"image"`
			Environment []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"environment"`
		} `json:"containerDefinitions"`
	}
	if err := json.Unmarshal([]byte(readFile(t, "ecs-taskdef.json")), &td); err != nil {
		t.Fatalf("ecs-taskdef.json must be valid JSON: %v", err)
	}
	if td.Family == "" || len(td.ContainerDefinitions) == 0 {
		t.Fatalf("ecs taskdef missing family/containers: %+v", td)
	}
	c := td.ContainerDefinitions[0]
	if !strings.Contains(c.Image, "heramba") {
		t.Errorf("ecs container image should be heramba, got %q", c.Image)
	}
	var hasMode bool
	for _, e := range c.Environment {
		if e.Name == "HERAMBA_MODE" && e.Value == "observe" {
			hasMode = true
		}
	}
	if !hasMode {
		t.Errorf("ecs taskdef should set HERAMBA_MODE=observe, env=%+v", c.Environment)
	}
}

// TestCloudRunServiceValid checks the Cloud Run Knative service YAML shape.
func TestCloudRunServiceValid(t *testing.T) {
	docs := parseYAMLDocs(t, "cloudrun-service.yaml", readFile(t, "cloudrun-service.yaml"))
	d := docs[0]
	if d["kind"] != "Service" {
		t.Errorf("cloudrun kind = %v, want Service", d["kind"])
	}
	blob := readFile(t, "cloudrun-service.yaml")
	for _, want := range []string{"ghcr.io/armur-ai/heramba", "HERAMBA_MODE", "serving.knative.dev"} {
		if !strings.Contains(blob, want) {
			t.Errorf("cloudrun service missing %q", want)
		}
	}
}

// TestDigitalOceanAppValid checks the DO App Platform spec YAML shape.
func TestDigitalOceanAppValid(t *testing.T) {
	docs := parseYAMLDocs(t, ".do/app.yaml", readFile(t, ".do/app.yaml"))
	d := docs[0]
	svcs, ok := d["services"].([]any)
	if !ok || len(svcs) == 0 {
		t.Fatalf("do app.yaml must list services, got %v", d["services"])
	}
	svc := svcs[0].(map[string]any)
	img, _ := svc["image"].(map[string]any)
	if img == nil || img["repository"] != "heramba" {
		t.Errorf("do app service should reference the heramba image, got %v", svc["image"])
	}
}

// TestDockerStackValid checks the Docker Swarm stack file parses and references
// the Heramba image with a deploy section.
func TestDockerStackValid(t *testing.T) {
	docs := parseYAMLDocs(t, "docker-stack.yml", readFile(t, "docker-stack.yml"))
	d := docs[0]
	svcs, ok := d["services"].(map[string]any)
	if !ok {
		t.Fatalf("docker-stack must have services, got %v", d["services"])
	}
	h, ok := svcs["heramba"].(map[string]any)
	if !ok {
		t.Fatalf("docker-stack must define the heramba service")
	}
	if img, _ := h["image"].(string); !strings.Contains(img, "heramba") {
		t.Errorf("heramba service image = %v, want heramba", h["image"])
	}
	if _, ok := h["deploy"]; !ok {
		t.Errorf("swarm service should have a deploy section")
	}
}

// TestAnsiblePlaybookValid parses the Ansible playbook (a top-level YAML
// sequence of plays) and checks it has a play targeting hosts with tasks.
func TestAnsiblePlaybookValid(t *testing.T) {
	var plays []map[string]any
	if err := yaml.Unmarshal([]byte(readFile(t, "ansible-deploy.yml")), &plays); err != nil {
		t.Fatalf("ansible playbook must be valid YAML: %v", err)
	}
	if len(plays) == 0 {
		t.Fatal("playbook has no plays")
	}
	p := plays[0]
	if p["hosts"] == nil {
		t.Errorf("play must target hosts, got %+v", p)
	}
	if _, ok := p["tasks"].([]any); !ok {
		t.Errorf("play must have tasks, got %v", p["tasks"])
	}
	if !strings.Contains(readFile(t, "ansible-deploy.yml"), "HERAMBA_MODE") {
		t.Error("playbook should set HERAMBA_MODE")
	}
}

// TestSystemdUnitPresent checks the systemd unit has the required sections and
// ExecStart (it is an INI-like file; the repo vendors no INI parser, so this is
// a marker check on a documented example).
func TestSystemdUnitPresent(t *testing.T) {
	content := readFile(t, "heramba.service")
	for _, want := range []string{"[Unit]", "[Service]", "ExecStart=/usr/local/bin/heramba serve", "[Install]", "HERAMBA_MODE=observe"} {
		if !strings.Contains(content, want) {
			t.Errorf("heramba.service missing %q", want)
		}
	}
}

// TestNomadJobPresent marker-checks the Nomad job (HCL; documented example, not
// parsed since the repo vendors no HCL parser).
func TestNomadJobPresent(t *testing.T) {
	content := readFile(t, "heramba.nomad")
	for _, want := range []string{`job "heramba"`, `driver = "docker"`, "ghcr.io/armur-ai/heramba", "HERAMBA_MODE", "HERAMBA_UPSTREAM"} {
		if !strings.Contains(content, want) {
			t.Errorf("heramba.nomad missing %q", want)
		}
	}
}
