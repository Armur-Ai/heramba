package openapi

import (
	"reflect"
	"testing"
)

const v3Spec = `{
  "openapi": "3.0.0",
  "paths": {
    "/login": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {
                "type": "object",
                "properties": {
                  "username": {"type": "string"},
                  "password": {"type": "string"},
                  "api_key": {"type": "string"}
                }
              }
            }
          }
        }
      }
    },
    "/users": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/User"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "User": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "email": {"type": "string"},
          "secret_token": {"type": "string"},
          "profile": {
            "type": "object",
            "properties": {"session_id": {"type": "string"}}
          }
        }
      }
    }
  }
}`

func TestBodyFieldsV3WithRefAndNesting(t *testing.T) {
	got, err := BodyFields([]byte(v3Spec))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"api_key", "email", "name", "password", "profile", "secret_token", "session_id", "username"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("body fields = %v, want %v", got, want)
	}
}

func TestSensitiveFieldsFilters(t *testing.T) {
	got, err := SensitiveFields([]byte(v3Spec))
	if err != nil {
		t.Fatal(err)
	}
	// username/name/profile are not sensitive; the rest are.
	want := []string{"api_key", "email", "password", "secret_token", "session_id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sensitive fields = %v, want %v", got, want)
	}
}

func TestSwaggerV2BodyAndFormData(t *testing.T) {
	spec := `{
      "swagger": "2.0",
      "paths": {
        "/auth": {
          "post": {
            "parameters": [
              {"in": "body", "name": "body", "schema": {"$ref": "#/definitions/Creds"}},
              {"in": "formData", "name": "access_token", "type": "string"},
              {"in": "query", "name": "page", "type": "integer"}
            ]
          }
        }
      },
      "definitions": {
        "Creds": {
          "type": "object",
          "properties": {"password": {"type": "string"}, "note": {"type": "string"}}
        }
      }
    }`
	got, err := SensitiveFields([]byte(spec))
	if err != nil {
		t.Fatal(err)
	}
	// password (body) + access_token (formData); page/note excluded.
	want := []string{"access_token", "password"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("v2 sensitive fields = %v, want %v", got, want)
	}
}

func TestMalformedSpecErrors(t *testing.T) {
	if _, err := SensitiveFields([]byte("{not json")); err == nil {
		t.Fatal("expected parse error on malformed spec")
	}
}

func TestEmptySpecNoFields(t *testing.T) {
	got, err := SensitiveFields([]byte(`{"openapi":"3.0.0","paths":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty spec should yield no fields, got %v", got)
	}
}

func TestIsSensitive(t *testing.T) {
	for _, s := range []string{"password", "apiKey", "X-Auth-Token", "user_email", "creditCard"} {
		if !IsSensitive(s) {
			t.Errorf("%q should be sensitive", s)
		}
	}
	for _, s := range []string{"username", "page", "note", "count"} {
		if IsSensitive(s) {
			t.Errorf("%q should not be sensitive", s)
		}
	}
}
