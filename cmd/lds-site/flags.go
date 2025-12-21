package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// parseEnvFlags checks environment variables for any flags that haven't been set
// on the command line. The environment variable name is "LDS_SITE_" + the flag name
// upper-cased, with dashes replaced by underscores.
func parseEnvFlags(fs *flag.FlagSet) error {
	seen := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})

	var err error
	fs.VisitAll(func(f *flag.Flag) {
		if err != nil {
			return
		}
		if seen[f.Name] {
			return
		}

		envName := "LDS_SITE_" + strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_"))
		if val, ok := os.LookupEnv(envName); ok {
			if setErr := f.Value.Set(val); setErr != nil {
				err = fmt.Errorf("invalid value for environment variable %s: %w", envName, setErr)
			}
		}
	})
	return err
}
