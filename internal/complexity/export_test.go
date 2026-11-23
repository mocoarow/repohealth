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
