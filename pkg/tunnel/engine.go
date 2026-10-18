package tunnel

import "fmt"

// defaultPorts maps each supported database engine to its standard port.
var defaultPorts = map[string]int{
	"mysql":    3306,
	"postgres": 5432,
}

// DefaultPort returns the standard port for a supported database engine.
func DefaultPort(engine string) (int, error) {
	port, ok := defaultPorts[engine]
	if !ok {
		return 0, fmt.Errorf("unsupported engine %q: must be one of mysql, postgres", engine)
	}
	return port, nil
}
