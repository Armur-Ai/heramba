// Package deploy holds no Go code — only deployment manifests. This test
// validates that the shipped manifests and the Helm chart are well-formed YAML
// with the keys an operator depends on, so a typo never ships silently. Helm
// templates are checked by substituting every {{ ... }} action with a scalar
// placeholder and then parsing the result: it proves the surrounding YAML
// structure is sound without needing the `helm` binary.
package deploy

import (
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
	for _, f := range []string{"k8s/sidecar.yaml", "docker-compose.yml"} {
		docs := parseYAMLDocs(t, f, readFile(t, f))
		if len(docs) == 0 {
			t.Errorf("%s produced no documents", f)
		}
	}
}
