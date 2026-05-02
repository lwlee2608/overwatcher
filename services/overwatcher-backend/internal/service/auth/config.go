package auth

// BootstrapConfig is the env-var bootstrap input for EnsureUserPassword.
// When Email and Password are both set the coordinator upserts that user
// at startup; otherwise bootstrap is skipped.
type BootstrapConfig struct {
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password" mask:"true"`
	Name     string `mapstructure:"name"`
}

func (b BootstrapConfig) Enabled() bool {
	return b.Email != "" && b.Password != ""
}
