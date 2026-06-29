package core

// OptedOut reports whether the user has disabled ad fetching and reporting. When true,
// the engine fetches nothing and reports nothing.
func OptedOut(cfg Config) bool { return cfg.OptOut }
