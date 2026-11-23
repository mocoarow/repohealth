package complexity

import "io/fs"

func ParseBiome(out []byte) error {
	_, err := parseBiome(out)

	return err
}

func NameAt(sources fs.FS, file string, startLine, startColumn, endLine, endColumn int) (string, error) {
	return nameAt(sources, file, biomeLocation{
		Path:  file,
		Start: biomePos{Line: startLine, Column: startColumn},
		End:   biomePos{Line: endLine, Column: endColumn},
	})
}

func CognitOf(message string) error {
	_, err := cognitOf(message)

	return err
}

func ParseBiomeVersion(out string) error {
	_, err := parseBiomeVersion(out)

	return err
}

func FunctionsOf(sources fs.FS, dir string, files []string, diagnosticPath string) error {
	_, err := functionsOf(sources, dir, files, []biomeDiagnostic{{
		Category: biomeCategory,
		Message:  "Excessive complexity of 3 detected (max: 1).",
		Location: biomeLocation{Path: diagnosticPath, Start: biomePos{Line: 1, Column: 1}, End: biomePos{Line: 1, Column: 2}},
	}})

	return err
}
