package auth

type BootstrapConfig struct {
	Email    string `mapstructure:"email"`
	Password string `mapstructure:"password" mask:"true"`
	Name     string `mapstructure:"name"`
}

func (b BootstrapConfig) Enabled() bool {
	return b.Email != "" && b.Password != ""
}
