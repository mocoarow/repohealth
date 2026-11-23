package gitlog

func ParseHeader(line string) error {
	_, err := parseHeader(line)

	return err
}
