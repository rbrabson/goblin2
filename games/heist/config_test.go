package heist

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConfigLoadsCanonicalKeys(t *testing.T) {
	var cfg Config
	if err := yaml.Unmarshal([]byte("boost_percentage: 12.5\nbase_vault_recovery: 0.04\n"), &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.BoostPercentage != 12.5 {
		t.Fatalf("BoostPercentage = %v, want 12.5", cfg.BoostPercentage)
	}
	if cfg.BaseVaultRecovery != 0.04 {
		t.Fatalf("BaseVaultRecovery = %v, want 0.04", cfg.BaseVaultRecovery)
	}
}

func TestApplyLegacyConfigDefaults(t *testing.T) {
	previous := defaultConfig.BaseVaultRecovery
	defaultConfig.BaseVaultRecovery = 0.04
	t.Cleanup(func() { defaultConfig.BaseVaultRecovery = previous })

	cfg := &Config{}
	if !applyLegacyConfigDefaults(cfg) {
		t.Fatal("applyLegacyConfigDefaults() did not report a repair")
	}
	if cfg.BaseVaultRecovery != 0.04 {
		t.Fatalf("BaseVaultRecovery = %v, want 0.04", cfg.BaseVaultRecovery)
	}
	if applyLegacyConfigDefaults(cfg) {
		t.Fatal("applyLegacyConfigDefaults() reported a second repair")
	}
}
