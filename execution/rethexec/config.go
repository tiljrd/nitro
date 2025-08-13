package rethexec

import (
	flag "github.com/spf13/pflag"
)

type Config struct {
	URL string `koanf:"url"`
	Secondary []string `koanf:"secondary"`
	JWTSecretPath string `koanf:"jwt-secret-path"`
	TimeoutSeconds int `koanf:"timeout-seconds"`
}

var DefaultConfig = Config{
	URL:            "http://localhost:8547",
	Secondary:      []string{},
	JWTSecretPath:  "/jwtsecret",
	TimeoutSeconds: 30,
}

func ConfigAddOptions(prefix string, f *flag.FlagSet) {
	f.String(prefix+".url", DefaultConfig.URL, "Reth RPC URL for arb_* and eth_* methods")
	f.StringSlice(prefix+".secondary", DefaultConfig.Secondary, "Secondary RPC URLs for failover")
	f.String(prefix+".jwt-secret-path", DefaultConfig.JWTSecretPath, "Path to JWT secret for Reth RPC authentication")
	f.Int(prefix+".timeout-seconds", DefaultConfig.TimeoutSeconds, "RPC timeout in seconds for Reth RPC client")
}
