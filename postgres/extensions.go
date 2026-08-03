package postgres

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// extensionsDatabase is the database extensions are managed against. It's
// created by initdb on every version, so no database-selection UI is needed.
const extensionsDatabase = "postgres"

// unitSeparator delimits psql's unaligned tuple output. It's vanishingly
// unlikely to appear in an extension's name or comment, unlike a comma.
const unitSeparator = "\x1f"

// Extension describes a PostgreSQL extension available to a server, and
// whether it's currently installed (i.e. CREATE EXTENSION'd) in the
// extensions database.
type Extension struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Installed bool   `json:"installed"`
	Comment   string `json:"comment"`
}

// ListExtensions returns every extension available to a version's running
// server, in the order PostgreSQL's own catalog reports them.
func ListExtensions(version string, port int) ([]Extension, error) {
	out, err := runPsql(version, port, `SELECT name, default_version, coalesce(installed_version, ''), comment FROM pg_available_extensions ORDER BY name;`)
	if err != nil {
		return nil, err
	}

	var extensions []Extension
	for _, line := range psqlLines(out) {
		fields := strings.Split(line, unitSeparator)
		if len(fields) != 4 {
			continue
		}
		extensions = append(extensions, Extension{
			Name:      fields[0],
			Version:   firstNonEmpty(fields[2], fields[1]),
			Installed: fields[2] != "",
			Comment:   fields[3],
		})
	}
	return extensions, nil
}

// InstallExtension enables an extension (CREATE EXTENSION) in the extensions
// database of a version's running server.
func InstallExtension(version string, port int, name string) error {
	_, err := runPsql(version, port, fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", quoteIdent(name)))
	return err
}

// UninstallExtension disables an extension (DROP EXTENSION) in the
// extensions database of a version's running server.
func UninstallExtension(version string, port int, name string) error {
	_, err := runPsql(version, port, fmt.Sprintf("DROP EXTENSION IF EXISTS %s;", quoteIdent(name)))
	return err
}

func runPsql(version string, port int, query string) (string, error) {
	psql, err := binPath(version, "psql")
	if err != nil {
		return "", err
	}

	cmd := exec.Command(psql,
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(port),
		"-d", extensionsDatabase,
		"-t", "-A", "-F", unitSeparator,
		"-c", query,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("psql: %w\n%s", err, out)
	}
	return string(out), nil
}

func psqlLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
