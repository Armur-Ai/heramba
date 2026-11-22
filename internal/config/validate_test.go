package config

import "testing"

func TestValidateMinSeverity(t *testing.T) {
	c := Default()
	c.Alert.MinSeverity = "high"
	if err := c.Validate(); err != nil {
		t.Fatalf("valid min_severity rejected: %v", err)
	}
	c.Alert.MinSeverity = "bogus"
	if err := c.Validate(); err == nil {
		t.Fatal("invalid min_severity should be rejected")
	}
}

func TestValidateSyslogNetwork(t *testing.T) {
	c := Default()
	c.Alert.Syslog.Network = "tcp"
	if err := c.Validate(); err != nil {
		t.Fatalf("tcp syslog rejected: %v", err)
	}
	c.Alert.Syslog.Network = "sctp"
	if err := c.Validate(); err == nil {
		t.Fatal("invalid syslog network should be rejected")
	}
}

func TestValidateEmptyOptionalFieldsOK(t *testing.T) {
	if err := Default().Validate(); err != nil {
		t.Fatalf("default config must validate: %v", err)
	}
}
