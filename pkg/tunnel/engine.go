package tunnel

import "fmt"

// engine describes a supported database engine.
type engine struct {
	port       int
	clientHint string // how to connect a client; %d is the local port
}

var engines = map[string]engine{
	"mysql":    {port: 3306, clientHint: "mysql -h 127.0.0.1 -P %d -u <user> -p"},
	"postgres": {port: 5432, clientHint: "psql -h 127.0.0.1 -p %d -U <user> <database>"},
}

// DefaultPort returns the standard port for a supported database engine.
func DefaultPort(name string) (int, error) {
	e, ok := engines[name]
	if !ok {
		return 0, fmt.Errorf("unsupported engine %q: must be one of mysql, postgres", name)
	}
	return e.port, nil
}
