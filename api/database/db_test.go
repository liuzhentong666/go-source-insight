package database

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestConnectionDSNPreservesInjectedCredentials(t *testing.T) {
	for _, host := range []string{"localhost", "::1"} {
		cfg := &DBConfig{Host: host, Port: "5432", User: "fixture user", Password: "A fictional 'credential' with @:/?#&= symbols", DBName: "fixture database"}
		parsed, err := pgx.ParseConfig(connectionDSN(cfg))
		if err != nil {
			t.Fatal(err)
		}
		if parsed.Host != cfg.Host || parsed.Port != 5432 || parsed.User != cfg.User || parsed.Password != cfg.Password || parsed.Database != cfg.DBName || parsed.ConnectTimeout.Seconds() != 10 || parsed.TLSConfig != nil {
			t.Fatal("injected database settings were changed during DSN parsing")
		}
	}
}
